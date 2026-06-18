package cli

import (
	"encoding/json"
	"net/url"

	"github.com/shhac/agent-vercel/internal/vercel"
	"github.com/spf13/cobra"
)

func registerAlias(root *cobra.Command, g *GlobalFlags) {
	cmd := &cobra.Command{
		Use:   "alias",
		Short: "Inspect deployment aliases (and their protection state)",
		RunE:  func(c *cobra.Command, args []string) error { return handleUnknownSubcommand(c, args) },
	}

	var listCursor *string
	var listAll *bool
	list := &cobra.Command{
		Use:   "list <deployment>",
		Short: "List the aliases assigned to a deployment",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			r, err := resolveClient(g)
			if err != nil {
				return err
			}
			return emitPaged(g, url.Values{}, *listCursor, *listAll, func(q url.Values) ([]json.RawMessage, vercel.Page, error) {
				return r.client.DeploymentAliases(cmd.Context(), cleanRef(args[0]), q)
			}, compactAlias)
		},
	}
	listCursor, listAll = addPageFlags(list)

	cmd.AddCommand(list, aliasSetCmd(g), aliasRmCmd(g), aliasBypassCmd(g))
	root.AddCommand(cmd)
}

func aliasBypassCmd(g *GlobalFlags) *cobra.Command {
	var ttl int
	var revoke string
	var regenerate bool
	var yes *bool
	cmd := &cobra.Command{
		Use:   "bypass <alias|deployment-id>",
		Short: "Create or revoke a shareable protection-bypass link for a gated (401-ing) preview",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			action := "create a protection-bypass link for " + args[0]
			if revoke != "" {
				action = "revoke a protection-bypass link for " + args[0]
			}
			r, err := confirmAndClient(g, *yes, action, "agent-vercel alias bypass "+args[0]+" --yes")
			if err != nil {
				return err
			}
			body := map[string]any{}
			if ttl > 0 {
				body["ttl"] = ttl
			}
			if revoke != "" {
				body["revoke"] = map[string]any{"secret": revoke, "regenerate": regenerate}
			}
			raw, err := r.client.SetAliasProtectionBypass(cmd.Context(), args[0], body)
			if err != nil {
				return err
			}
			if len(raw) == 0 {
				return printSingle(g, map[string]any{"ok": true})
			}
			return printRaw(g, raw)
		},
	}
	cmd.Flags().IntVar(&ttl, "ttl", 0, "seconds the shareable link stays valid (0 = no expiry)")
	cmd.Flags().StringVar(&revoke, "revoke", "", "revoke this bypass secret instead of creating one")
	cmd.Flags().BoolVar(&regenerate, "regenerate", false, "with --revoke, mint a fresh link after revoking")
	yes = addYesFlag(cmd)
	return cmd
}

func aliasSetCmd(g *GlobalFlags) *cobra.Command {
	var yes *bool
	cmd := &cobra.Command{
		Use:   "set <deployment> <alias>",
		Short: "Assign an alias to a deployment (repoints it from any prior deployment)",
		Args:  cobra.ExactArgs(2),
		RunE: func(cmd *cobra.Command, args []string) error {
			deployment, alias := args[0], args[1]
			r, err := confirmAndClient(g, *yes, "point "+alias+" at "+deployment,
				"agent-vercel alias set "+deployment+" "+alias+" --yes")
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
			r, err := confirmAndClient(g, *yes, "delete alias "+args[0],
				"agent-vercel alias rm "+args[0]+" --yes")
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
