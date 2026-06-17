package cli

import (
	"encoding/json"

	"github.com/spf13/cobra"
)

func registerEdgeConfig(root *cobra.Command, g *GlobalFlags) {
	cmd := &cobra.Command{
		Use:     "edge-config",
		Aliases: []string{"edge"},
		Short:   "Inspect Edge Configs (live non-secret key/value config, e.g. feature flags)",
		RunE:    func(c *cobra.Command, args []string) error { return handleUnknownSubcommand(c, args) },
	}

	list := &cobra.Command{
		Use:   "list",
		Short: "List the Edge Configs in the scope",
		Args:  cobra.NoArgs,
		RunE: func(cmd *cobra.Command, _ []string) error {
			r, err := resolveClient(g)
			if err != nil {
				return err
			}
			items, err := r.client.ListEdgeConfigs(cmd.Context(), nil)
			if err != nil {
				return err
			}
			rows, err := compactRows(items, g.Full, compactEdgeConfig)
			if err != nil {
				return err
			}
			return emitList(g, rows, nil)
		},
	}

	items := &cobra.Command{
		Use:   "items <edge-config-id>",
		Short: "List the key/value items in one Edge Config",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			r, err := resolveClient(g)
			if err != nil {
				return err
			}
			rawItems, err := r.client.EdgeConfigItems(cmd.Context(), args[0])
			if err != nil {
				return err
			}
			rows, err := compactRows(rawItems, g.Full, compactEdgeConfigItem)
			if err != nil {
				return err
			}
			return emitList(g, rows, nil)
		},
	}

	cmd.AddCommand(list, items)
	root.AddCommand(cmd)
}

type rawEdgeConfig struct {
	ID        string `json:"id"`
	Slug      string `json:"slug"`
	ItemCount int    `json:"itemCount"`
	SizeBytes int    `json:"sizeInBytes"`
	CreatedAt int64  `json:"createdAt"`
	UpdatedAt int64  `json:"updatedAt"`
}

func compactEdgeConfig(raw json.RawMessage) (map[string]any, error) {
	var e rawEdgeConfig
	if err := json.Unmarshal(raw, &e); err != nil {
		return nil, wrapAgent(err)
	}
	m := map[string]any{"id": e.ID}
	putIf(m, "slug", e.Slug)
	m["item_count"] = e.ItemCount
	if e.SizeBytes > 0 {
		m["size_bytes"] = e.SizeBytes
	}
	putIf(m, "created", msToRFC3339(e.CreatedAt))
	putIf(m, "updated", msToRFC3339(e.UpdatedAt))
	return m, nil
}

func compactEdgeConfigItem(raw json.RawMessage) (map[string]any, error) {
	var it struct {
		Key   string          `json:"key"`
		Value json.RawMessage `json:"value"`
	}
	if err := json.Unmarshal(raw, &it); err != nil {
		return nil, wrapAgent(err)
	}
	m := map[string]any{"key": it.Key}
	if len(it.Value) > 0 {
		m["value"] = it.Value
	}
	return m, nil
}
