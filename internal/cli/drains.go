package cli

import (
	"encoding/json"
	"net/url"
	"sort"

	"github.com/shhac/agent-vercel/internal/vercel"
	"github.com/spf13/cobra"
)

// registerDrains wires the `drains` group: read-only inspection of observability
// data exports (log/trace/analytics/speed-insights). Drains are the only REST
// handle on where this otherwise-unqueryable data is shipped, so this answers
// "is observability leaving the platform, and is a drain failing delivery?".
func registerDrains(root *cobra.Command, g *GlobalFlags) {
	cmd := &cobra.Command{
		Use:   "drains",
		Short: "Inspect observability drains (where log/trace/analytics data is shipped)",
		RunE:  func(c *cobra.Command, args []string) error { return handleUnknownSubcommand(c, args) },
	}

	var project string
	list := &cobra.Command{
		Use:   "list",
		Short: "List drains in the scope (type, status, target) and flag failing delivery",
		Args:  cobra.NoArgs,
		RunE: func(cmd *cobra.Command, _ []string) error {
			r, err := resolveClient(g)
			if err != nil {
				return err
			}
			q := url.Values{}
			setIf(q, "projectId", project)
			items, err := r.client.Drains(cmd.Context(), q)
			if err != nil {
				return err
			}
			return emitRows(g, items, compactDrain)
		},
	}
	list.Flags().StringVar(&project, "project", "", "filter to drains targeting this project id")

	get := &cobra.Command{
		Use:   "get <id>",
		Short: "Get one drain",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			return emitOne(g, func(c *vercel.Client) (json.RawMessage, error) {
				return c.GetDrain(cmd.Context(), args[0])
			}, compactDrain)
		},
	}

	cmd.AddCommand(list, get)
	root.AddCommand(cmd)
}

// compactDrain projects a drain defensively: id, name, status/disabled, the
// configured schema types, and target-project count. The delivery endpoint is
// left to --full (it can carry a destination token in its URL).
func compactDrain(raw json.RawMessage) (map[string]any, error) {
	var d struct {
		ID         string                     `json:"id"`
		Name       string                     `json:"name"`
		Status     string                     `json:"status"`
		Disabled   bool                       `json:"disabled"`
		Schemas    map[string]json.RawMessage `json:"schemas"`
		ProjectIDs []string                   `json:"projectIds"`
		CreatedAt  int64                      `json:"createdAt"`
	}
	if err := json.Unmarshal(raw, &d); err != nil {
		return nil, wrapAgent(err)
	}
	m := map[string]any{"id": d.ID}
	putIf(m, "name", d.Name)
	putIf(m, "status", d.Status)
	if d.Disabled {
		m["disabled"] = true
	}
	if len(d.Schemas) > 0 {
		types := make([]string, 0, len(d.Schemas))
		for t := range d.Schemas {
			types = append(types, t)
		}
		sort.Strings(types)
		m["types"] = types
	}
	if len(d.ProjectIDs) > 0 {
		m["projects"] = len(d.ProjectIDs)
	}
	putIf(m, "created", msToRFC3339(d.CreatedAt))
	return m, nil
}
