package cli

import (
	"encoding/json"
	"net/url"

	agenterrors "github.com/shhac/agent-vercel/internal/errors"
	"github.com/spf13/cobra"
)

// envSharedCmd groups reads of the team-level shared environment variables —
// values defined once at the team and linked into multiple projects, distinct
// from the per-project vars the rest of the `env` commands operate on.
func envSharedCmd(g *GlobalFlags) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "shared",
		Short: "Inspect team-level shared environment variables (reused across projects)",
		RunE:  func(c *cobra.Command, args []string) error { return handleUnknownSubcommand(c, args) },
	}
	cmd.AddCommand(envSharedListCmd(g), envSharedGetCmd(g))
	return cmd
}

func envSharedListCmd(g *GlobalFlags) *cobra.Command {
	var decrypt bool
	cmd := &cobra.Command{
		Use:   "list",
		Short: "List the team's shared environment variables",
		Args:  cobra.NoArgs,
		RunE: func(cmd *cobra.Command, _ []string) error {
			vars, err := fetchSharedEnv(g, cmd, decrypt)
			if err != nil {
				return err
			}
			rows := make([]any, 0, len(vars))
			for _, e := range vars {
				rows = append(rows, compactSharedEnv(e, decrypt))
			}
			return emitList(g, rows, nil)
		},
	}
	cmd.Flags().BoolVar(&decrypt, "decrypt", false, "include decrypted values")
	return cmd
}

func envSharedGetCmd(g *GlobalFlags) *cobra.Command {
	var decrypt bool
	cmd := &cobra.Command{
		Use:   "get <key|id>",
		Short: "Get one shared environment variable, matched by key or id",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			vars, err := fetchSharedEnv(g, cmd, decrypt)
			if err != nil {
				return err
			}
			for _, e := range vars {
				if e.Key == args[0] || e.ID == args[0] {
					return printSingle(g, compactSharedEnv(e, decrypt))
				}
			}
			return agenterrors.Newf(agenterrors.FixableByAgent, "no shared env var %q in this team", args[0]).
				WithHint("run 'agent-vercel env shared list' to see keys")
		},
	}
	cmd.Flags().BoolVar(&decrypt, "decrypt", false, "include the decrypted value")
	return cmd
}

type rawSharedEnv struct {
	ID         string   `json:"id"`
	Key        string   `json:"key"`
	Type       string   `json:"type"`
	Target     []string `json:"target"`
	ProjectIDs []string `json:"projectId"`
	Value      string   `json:"value"`
	CreatedAt  int64    `json:"createdAt"`
	UpdatedAt  int64    `json:"updatedAt"`
}

// compactSharedEnv projects a shared var: its key/type/targets, the projects it
// is linked into, and (only with --decrypt) the value.
func compactSharedEnv(e rawSharedEnv, withValue bool) map[string]any {
	m := map[string]any{"id": e.ID, "key": e.Key}
	putIf(m, "type", e.Type)
	if len(e.Target) > 0 {
		m["target"] = e.Target
	}
	if len(e.ProjectIDs) > 0 {
		m["projects"] = e.ProjectIDs
	}
	putIf(m, "created", msToRFC3339(e.CreatedAt))
	putIf(m, "updated", msToRFC3339(e.UpdatedAt))
	if withValue && e.Value != "" {
		m["value"] = e.Value
	}
	return m
}

func fetchSharedEnv(g *GlobalFlags, cmd *cobra.Command, decrypt bool) ([]rawSharedEnv, error) {
	q := url.Values{}
	if decrypt {
		q.Set("decrypt", "true")
	}
	r, err := resolveClient(g)
	if err != nil {
		return nil, err
	}
	items, err := r.client.SharedEnv(cmd.Context(), q)
	if err != nil {
		return nil, err
	}
	out := make([]rawSharedEnv, 0, len(items))
	for _, it := range items {
		var e rawSharedEnv
		if err := json.Unmarshal(it, &e); err != nil {
			return nil, wrapAgent(err)
		}
		out = append(out, e)
	}
	return out, nil
}
