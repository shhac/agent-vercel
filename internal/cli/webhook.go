package cli

import (
	"encoding/json"
	"net/url"

	"github.com/spf13/cobra"
)

func registerWebhook(root *cobra.Command, g *GlobalFlags) {
	cmd := &cobra.Command{
		Use:   "webhook",
		Short: "Inspect the team's webhooks (which events fire where)",
		RunE:  func(c *cobra.Command, args []string) error { return handleUnknownSubcommand(c, args) },
	}

	var project string
	list := &cobra.Command{
		Use:   "list",
		Short: "List webhooks in the scope, optionally filtered to a project",
		Args:  cobra.NoArgs,
		RunE: func(cmd *cobra.Command, _ []string) error {
			q := url.Values{}
			setIf(q, "projectId", project)
			r, err := resolveClient(g)
			if err != nil {
				return err
			}
			items, err := r.client.ListWebhooks(cmd.Context(), q)
			if err != nil {
				return err
			}
			rows, err := compactRows(items, g.Full, compactWebhook)
			if err != nil {
				return err
			}
			return emitList(g, rows, nil)
		},
	}
	list.Flags().StringVar(&project, "project", "", "filter to webhooks targeting this project id")

	cmd.AddCommand(list)
	root.AddCommand(cmd)
}

type rawWebhook struct {
	ID         string   `json:"id"`
	URL        string   `json:"url"`
	Events     []string `json:"events"`
	ProjectIDs []string `json:"projectIds"`
	CreatedAt  int64    `json:"createdAt"`
	UpdatedAt  int64    `json:"updatedAt"`
}

// compactWebhook projects a webhook: its endpoint URL, the events it subscribes
// to, and any projects it targets (account-wide when projectIds is empty).
func compactWebhook(raw json.RawMessage) (map[string]any, error) {
	var w rawWebhook
	if err := json.Unmarshal(raw, &w); err != nil {
		return nil, wrapAgent(err)
	}
	m := map[string]any{"id": w.ID, "url": w.URL}
	if len(w.Events) > 0 {
		m["events"] = toAnySlice(w.Events)
	}
	if len(w.ProjectIDs) > 0 {
		m["project_ids"] = toAnySlice(w.ProjectIDs)
	}
	putIf(m, "created", msToRFC3339(w.CreatedAt))
	putIf(m, "updated", msToRFC3339(w.UpdatedAt))
	return m, nil
}
