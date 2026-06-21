package cli

import (
	"encoding/json"
	"fmt"
	"os"
	"strings"

	"github.com/spf13/cobra"
)

const usageOverview = `agent-vercel — Vercel CLI for AI agents

JSON in, JSON out, no interactivity. Lists are NDJSON (one object per line, then
a {"@pagination":…} line when more pages exist); single resources are also NDJSON
by default (one line; was pretty JSON before — pass --format json for the
object). Errors are JSON on stderr with fixable_by (agent|human|retry) and a
hint.

GET (SINGLE + MULTI)
  get <id>... takes one or more ids and returns one result per id, in input
  order. Default output is NDJSON: one line per id — the record, or
  {"@unresolved":{"id","reason","fixable_by","hint"?}} for an id that couldn't
  be resolved (e.g. not found / bad id). --format json|yaml collapses to one
  {"data":[…], "@unresolved":[…]} envelope. A single get <id> is just the
  one-element case. Item-level misses stay on stdout and exit 0; only a
  command-level failure (auth, network) goes to stderr with exit 1 and empty
  stdout.

  env get <project> <key>...  — scope fixed (project), then 1..N keys
  domain cert get <id>...     — 1..N cert ids
  config get <key>...         — 1..N local config keys

CREDENTIAL vs SCOPE (two separate axes)
  One Vercel credential reaches many teams. The credential is the secret; the
  team is a per-request scope.
    auth    manage stored credential(s) — secret, kept in the macOS Keychain
    scope   list/select which team (account) to act on — not a secret
  Select per-invocation with --auth <label> and --scope <team-slug|id>.
  The secret is NEVER printed; there is no command to read it back out.

SETUP (once)
  agent-vercel auth add --form             # human pastes the token into an OS dialog (preferred)
  export VERCEL_TOKEN=...                  # …or create one at vercel.com/account/tokens
  agent-vercel auth add personal           # stores it in the Keychain (label optional)
  agent-vercel auth test                   # verify (calls /v2/user)
  agent-vercel scope list                  # teams this credential can reach
  agent-vercel scope set-default acme      # default scope for later calls

CORE DOMAINS (see design-docs/cli-design.md)
  deployment   list | get | checks | routes | logs | runtime-logs | current | promote* | rollback* | cancel* | redeploy*
  project      list | get | crons | custom-environments | protection | routes
  env          list | diff | get <project> <key>... | pull | shared list/get | set* | rm*
  domain       list | get | inspect | records | cert list/get | projects | transfer | verify* | add* | rm*
  alias        list | set* | rm* | bypass*
  firewall     config <project> | attack-status <project> | bypass <project>   (WAF triage)
  cache        purge <project> --tag <t>*                  (invalidate CDN cache by tag)
  billing      charges [--by service|project|region] | consumption   (what is driving spend / consumed volume)
  webhook      list [--project]                  (which events fire where)
  drains       list [--project] | get <id>      (where log/trace/analytics data is shipped)
  edge-config  list | items <id>                  (live non-secret key/value config)
  scope        list | current | set-default | member list/get
  auth         add | list | test | set-default | remove | import-cli
  api          call <METHOD> <path>       (raw REST escape hatch)
  config       get | set | list | unset

  * destructive / state-changing — requires --yes.

Run 'agent-vercel <domain> usage' for per-domain detail.`

func registerUsage(root *cobra.Command) {
	var asJSON bool
	cmd := &cobra.Command{
		Use:   "usage",
		Short: "LLM-oriented overview of agent-vercel (--json for a machine-readable catalog)",
		Args:  cobra.NoArgs,
		RunE: func(cmd *cobra.Command, _ []string) error {
			if asJSON {
				return printCommandCatalog(cmd.Root())
			}
			_, _ = fmt.Fprintln(os.Stdout, usageOverview)
			return nil
		},
	}
	cmd.Flags().BoolVar(&asJSON, "json", false, "emit the command catalog as JSON for programmatic discovery")
	root.AddCommand(cmd)
}

// printCommandCatalog emits the command tree as JSON so an agent can discover the
// surface (domains, subcommands, one-line descriptions) without parsing prose.
func printCommandCatalog(root *cobra.Command) error {
	type sub struct {
		Name  string `json:"name"`
		Use   string `json:"use"`
		Short string `json:"short"`
	}
	type domain struct {
		Name        string `json:"name"`
		Short       string `json:"short"`
		Subcommands []sub  `json:"subcommands,omitempty"`
	}
	var domains []domain
	for _, c := range root.Commands() {
		if c.Hidden || c.Name() == "help" || c.Name() == "completion" {
			continue
		}
		d := domain{Name: c.Name(), Short: c.Short}
		for _, s := range c.Commands() {
			if s.Hidden || s.Name() == "usage" {
				continue
			}
			d.Subcommands = append(d.Subcommands, sub{Name: s.Name(), Use: s.Use, Short: s.Short})
		}
		domains = append(domains, d)
	}
	enc := json.NewEncoder(os.Stdout)
	enc.SetIndent("", "  ")
	enc.SetEscapeHTML(false)
	return enc.Encode(map[string]any{"commands": domains})
}

// attachDomainUsage gives every domain group a `usage` subcommand generated from
// the command tree, so `agent-vercel <domain> usage` documents that domain.
func attachDomainUsage(root *cobra.Command) {
	for _, c := range root.Commands() {
		if c.Name() == "usage" || !c.HasSubCommands() {
			continue
		}
		c.AddCommand(&cobra.Command{
			Use:   "usage",
			Short: "Usage for the " + c.Name() + " domain",
			Args:  cobra.NoArgs,
			RunE: func(cmd *cobra.Command, _ []string) error {
				printDomainUsage(cmd.Parent())
				return nil
			},
		})
	}
}

func printDomainUsage(c *cobra.Command) {
	var b strings.Builder
	fmt.Fprintf(&b, "%s — %s\n\n", c.Name(), c.Short)
	for _, sub := range c.Commands() {
		if sub.Name() == "usage" || sub.Hidden {
			continue
		}
		fmt.Fprintf(&b, "  %s %s\n      %s\n", c.Name(), sub.Use, sub.Short)
	}
	_, _ = fmt.Fprint(os.Stdout, b.String())
}
