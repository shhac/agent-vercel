package cli

import (
	"fmt"
	"os"

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
  alias        list | set* | rm*
  scope        list | current | set-default
  auth         add | list | test | set-default | remove | import-cli
  api          call <METHOD> <path>       (raw REST escape hatch)
  config       get | set | list | unset
  cache        info | warm | purge

  * destructive / state-changing — requires --yes.

Run 'agent-vercel <domain> usage' for per-domain detail.`

func registerUsage(root *cobra.Command) {
	cmd := &cobra.Command{
		Use:   "usage",
		Short: "LLM-oriented overview of agent-vercel",
		Args:  cobra.NoArgs,
		RunE: func(_ *cobra.Command, _ []string) error {
			_, _ = fmt.Fprintln(os.Stdout, usageOverview)
			return nil
		},
	}
	root.AddCommand(cmd)
}
