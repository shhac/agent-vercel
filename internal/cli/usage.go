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
a {"@pagination":…} line when more pages exist); single resources are pretty
JSON. Errors are JSON on stderr with fixable_by (agent|human|retry) and a hint.

CREDENTIAL vs SCOPE (two separate axes)
  One Vercel credential reaches many teams. The credential is the secret; the
  team is a per-request scope.
    auth    manage stored credential(s) — secret, kept in the macOS Keychain
    scope   list/select which team (account) to act on — not a secret
  Select per-invocation with --auth <label> and --scope <team-slug|id>.
  The secret is NEVER printed; there is no command to read it back out.

SETUP (once)
  export VERCEL_TOKEN=...                  # create one at vercel.com/account/tokens
  agent-vercel auth add --label personal   # stores it in the Keychain
  agent-vercel auth test                   # verify (calls /v2/user)
  agent-vercel scope list                  # teams this credential can reach
  agent-vercel scope set-default acme      # default scope for later calls

CORE DOMAINS (proposed; see design-docs/cli-design.md)
  deployment   list | get | logs | runtime-logs | current | promote* | rollback* | cancel* | redeploy*
  project      list | get
  env          list | diff | get | set* | rm*
  domain       list | get | inspect | records | verify* | add* | rm*
  alias        list | set* | rm* | bypass*
  billing      charges [--by service|project]   (what is driving spend)
  scope        list | current | set-default
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
