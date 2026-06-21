package cli

import (
	"encoding/json"
	"net/url"
	"os"
	"strconv"
	"strings"

	agenterrors "github.com/shhac/agent-vercel/internal/errors"
	"github.com/shhac/agent-vercel/internal/vercel"
	libcli "github.com/shhac/lib-agent-cli/cli"
	"github.com/spf13/cobra"
)

// resolveTeamID resolves the active scope to a concrete team id for endpoints
// that take the team in the path. The scope may already be an id (team_…), a
// slug (resolved via the team list), or empty — the personal account has no
// team members.
func resolveTeamID(cmd *cobra.Command, r *resolved) (string, error) {
	if r.scope == "" {
		return "", agenterrors.New("no team scope selected; members belong to a team, not the personal account", agenterrors.FixableByHuman).
			WithHint("pass --scope <team> or run 'agent-vercel scope set-default <team>'")
	}
	if strings.HasPrefix(r.scope, "team_") {
		return r.scope, nil
	}
	teams, err := r.client.ListTeams(cmd.Context())
	if err != nil {
		return "", err
	}
	for _, t := range teams {
		if t.Slug == r.scope || t.ID == r.scope {
			return t.ID, nil
		}
	}
	return "", agenterrors.Newf(agenterrors.FixableByAgent, "no team matches scope %q", r.scope).
		WithHint("run 'agent-vercel scope list' to see reachable teams")
}

// scopeMemberCmd is the `scope member` group: list the team roster and get one
// member by id/email/username.
// scopeTeamID resolves the active scope to a concrete teamId for endpoints that
// require it explicitly (the Vercel Firewall API rejects the slug). It returns
// "" for the personal account (no scope), and otherwise the resolved id.
func scopeTeamID(cmd *cobra.Command, r *resolved) (string, error) {
	if r.scope == "" {
		return "", nil
	}
	return resolveTeamID(cmd, r)
}

func scopeMemberCmd(g *GlobalFlags) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "member",
		Short: "Inspect the members of the active team scope (role, email, confirmed state)",
		RunE:  func(c *cobra.Command, args []string) error { return handleUnknownSubcommand(c, args) },
	}
	cmd.AddCommand(scopeMemberListCmd(g), scopeMemberGetCmd(g))
	return cmd
}

func scopeMemberListCmd(g *GlobalFlags) *cobra.Command {
	var limit int
	var cursor *string
	var all *bool
	cmd := &cobra.Command{
		Use:   "list",
		Short: "List the members of the active team scope (role, email, confirmed state)",
		Args:  cobra.NoArgs,
		RunE: func(cmd *cobra.Command, _ []string) error {
			r, err := resolveClient(g)
			if err != nil {
				return err
			}
			teamID, err := resolveTeamID(cmd, r)
			if err != nil {
				return err
			}
			q := url.Values{}
			if limit > 0 {
				q.Set("limit", strconv.Itoa(limit))
			}
			return emitPaged(g, q, *cursor, *all, func(q url.Values) ([]json.RawMessage, vercel.Page, error) {
				return r.client.TeamMembers(cmd.Context(), teamID, q)
			}, compactMember)
		},
	}
	cursor, all = addPageFlags(cmd)
	cmd.Flags().IntVar(&limit, "limit", 0, "max members to return")
	return cmd
}

func scopeMemberGetCmd(g *GlobalFlags) *cobra.Command {
	return &cobra.Command{
		Use:   "get <id|email|username>",
		Short: "Show one member of the active team scope (matched by id, email, or username)",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			r, err := resolveClient(g)
			if err != nil {
				return err
			}
			teamID, err := resolveTeamID(cmd, r)
			if err != nil {
				return err
			}
			// The team-members endpoint has no per-member GET, so fetch the
			// roster (following pages) and match client-side.
			items, _, err := fetchPaged(url.Values{}, "", true, func(q url.Values) ([]json.RawMessage, vercel.Page, error) {
				return r.client.TeamMembers(cmd.Context(), teamID, q)
			})
			if err != nil {
				return err
			}
			needle := args[0]
			for _, raw := range items {
				var m rawMember
				if json.Unmarshal(raw, &m) != nil {
					continue
				}
				if m.UID == needle || m.Email == needle || m.Username == needle {
					if g.Full {
						return printRaw(g, raw)
					}
					compact, cerr := compactMember(raw)
					if cerr != nil {
						return cerr
					}
					return libcli.EmitItem(os.Stdout, g.Format, compact)
				}
			}
			return agenterrors.Newf(agenterrors.FixableByAgent, "no member matches %q in this team", needle).
				WithHint("run 'agent-vercel scope member list' to list members")
		},
	}
}

type rawMember struct {
	UID       string `json:"uid"`
	Username  string `json:"username"`
	Email     string `json:"email"`
	Name      string `json:"name"`
	Role      string `json:"role"`
	Confirmed bool   `json:"confirmed"`
	CreatedAt int64  `json:"createdAt"`
}

func compactMember(raw json.RawMessage) (map[string]any, error) {
	var m rawMember
	if err := json.Unmarshal(raw, &m); err != nil {
		return nil, wrapAgent(err)
	}
	out := map[string]any{"uid": m.UID, "confirmed": m.Confirmed}
	putIf(out, "username", m.Username)
	putIf(out, "email", m.Email)
	putIf(out, "name", m.Name)
	putIf(out, "role", m.Role)
	putIf(out, "joined", msToRFC3339(m.CreatedAt))
	return out, nil
}
