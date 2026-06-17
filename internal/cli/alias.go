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
		},
	}

	cmd.AddCommand(list, aliasSetCmd(g), aliasRmCmd(g))
	root.AddCommand(cmd)
}

func aliasSetCmd(g *GlobalFlags) *cobra.Command {
	var yes *bool
	cmd := &cobra.Command{
		Use:   "set <deployment> <alias>",
		Short: "Assign an alias to a deployment (repoints it from any prior deployment)",
		Args:  cobra.ExactArgs(2),
		RunE: func(cmd *cobra.Command, args []string) error {
			deployment, alias := args[0], args[1]
			if err := requireYes(*yes, "point "+alias+" at "+deployment,
				"agent-vercel alias set "+deployment+" "+alias+" --yes"); err != nil {
				return err
			}
			r, err := resolveClient(g)
			if err != nil {
				return err
			}
			raw, err := r.client.AssignAlias(cmd.Context(), deployment, alias)
			if err != nil {
				return err
			}
			return printRaw(g, raw)
		},
	}
	yes = addYesFlag(cmd)
	return cmd
}

func aliasRmCmd(g *GlobalFlags) *cobra.Command {
	var yes *bool
	cmd := &cobra.Command{
		Use:   "rm <alias|id>",
		Short: "Delete an alias",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			if err := requireYes(*yes, "delete alias "+args[0],
				"agent-vercel alias rm "+args[0]+" --yes"); err != nil {
				return err
			}
			r, err := resolveClient(g)
			if err != nil {
				return err
			}
			if _, err := r.client.DeleteAlias(cmd.Context(), args[0]); err != nil {
				return err
			}
			return printSingle(g, map[string]any{"removed": args[0]})
		},
	}
	yes = addYesFlag(cmd)
	return cmd
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
