package cli

import (
	"encoding/json"
	"net/url"

	"github.com/spf13/cobra"
)

func registerAlias(root *cobra.Command, g *GlobalFlags) {
	cmd := &cobra.Command{
		Use:   "alias",
		Short: "Inspect deployment aliases (and their protection state)",
		RunE:  func(c *cobra.Command, args []string) error { return handleUnknownSubcommand(c, args) },
	}

	list := &cobra.Command{
		Use:   "list <deployment>",
		Short: "List the aliases assigned to a deployment",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			return run(func() error {
				r, err := resolveClient(g)
				if err != nil {
					return err
				}
				items, page, err := r.client.DeploymentAliases(cmd.Context(), args[0], url.Values{})
				if err != nil {
					return err
				}
				rows, err := compactRows(items, g.Full, compactAlias)
				if err != nil {
					return err
				}
				return emitList(g, rows, paginationMeta(page.Next))
			})
		},
	}

	cmd.AddCommand(list)
	root.AddCommand(cmd)
}

func compactAlias(raw json.RawMessage) (map[string]any, error) {
	var a struct {
		UID              string          `json:"uid"`
		Alias            string          `json:"alias"`
		Created          string          `json:"created"`
		Redirect         string          `json:"redirect"`
		ProtectionBypass json.RawMessage `json:"protectionBypass"`
	}
	if err := json.Unmarshal(raw, &a); err != nil {
		return nil, wrapAgent(err)
	}
	m := map[string]any{"id": a.UID, "alias": a.Alias}
	putIf(m, "created", a.Created)
	putIf(m, "redirect", a.Redirect)
	if len(a.ProtectionBypass) > 0 && string(a.ProtectionBypass) != "null" {
		m["protection_bypass"] = json.RawMessage(a.ProtectionBypass)
	}
	return m, nil
}
