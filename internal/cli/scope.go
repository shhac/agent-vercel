package cli

import (
	agenterrors "github.com/shhac/agent-vercel/internal/errors"
	"github.com/spf13/cobra"
)

// registerScope wires the `scope` group: selection of which team (account) to
// act on — the scope half of the credential/scope split. A scope is a team
// slug/id, or empty for the personal account. Scopes are not secret.
func registerScope(root *cobra.Command, g *GlobalFlags) {
	cmd := &cobra.Command{
		Use:   "scope",
		Short: "List and select the team (account) scope to act on",
		RunE:  func(c *cobra.Command, args []string) error { return handleUnknownSubcommand(c, args) },
	}

	list := &cobra.Command{
		Use:     "list",
		Aliases: []string{"ls"},
		Short:   "List scopes (teams) reachable by the active credential (GET /v2/teams)",
		Args:    cobra.NoArgs,
		RunE: func(cmd *cobra.Command, _ []string) error {
			r, err := resolveClient(g)
			if err != nil {
				return err
			}
			teams, err := r.client.ListTeams(cmd.Context())
			if err != nil {
				return err
			}
			rows := make([]any, 0, len(teams))
			for _, t := range teams {
				rows = append(rows, map[string]any{
					"id":      t.ID,
					"slug":    t.Slug,
					"name":    t.Name,
					"default": t.Slug == r.creds.DefaultScope,
				})
			}
			return emitList(g, rows, nil)
		},
	}

	current := &cobra.Command{
		Use:   "current",
		Short: "Show the active scope and credential",
		Args:  cobra.NoArgs,
		RunE: func(_ *cobra.Command, _ []string) error {
			store, err := newCredStore()
			if err != nil {
				return agenterrors.Wrap(err, agenterrors.FixableByHuman)
			}
			creds, err := store.Load()
			if err != nil {
				return agenterrors.Wrap(err, agenterrors.FixableByHuman)
			}
			scope := g.Scope
			if scope == "" {
				scope = creds.DefaultScope
			}
			if scope == "" {
				scope = "(personal account)"
			}
			return printSingle(g, map[string]any{
				"scope":         scope,
				"default_scope": creds.DefaultScope,
				"default_auth":  creds.DefaultAuth,
			})
		},
	}

	setDefault := &cobra.Command{
		Use:   "set-default <team-slug>",
		Short: "Set the default scope (empty arg resets to personal account)",
		Args:  cobra.MaximumNArgs(1),
		RunE: func(_ *cobra.Command, args []string) error {
			store, err := newCredStore()
			if err != nil {
				return agenterrors.Wrap(err, agenterrors.FixableByHuman)
			}
			scope := ""
			if len(args) == 1 {
				scope = args[0]
			}
			if err := store.SetDefaultScope(scope); err != nil {
				return agenterrors.Wrap(err, agenterrors.FixableByHuman)
			}
			return printSingle(g, map[string]any{"default_scope": scope})
		},
	}

	cmd.AddCommand(list, current, setDefault, scopeMemberCmd(g))
	root.AddCommand(cmd)
}
