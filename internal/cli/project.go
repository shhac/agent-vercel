package cli

import (
	"encoding/json"
	"net/url"
	"strconv"

	agenterrors "github.com/shhac/agent-vercel/internal/errors"
	"github.com/spf13/cobra"
)

func registerProject(root *cobra.Command, g *GlobalFlags) {
	cmd := &cobra.Command{
		Use:   "project",
		Short: "List and inspect projects",
		RunE:  func(c *cobra.Command, args []string) error { return handleUnknownSubcommand(c, args) },
	}

	var search string
	var limit int
	list := &cobra.Command{
		Use:   "list",
		Short: "List projects in the scope",
		Args:  cobra.NoArgs,
		RunE: func(cmd *cobra.Command, _ []string) error {
			return run(func() error {
				q := url.Values{}
				setIf(q, "search", search)
				if limit > 0 {
					q.Set("limit", strconv.Itoa(limit))
				}
				r, err := resolveClient(g)
				if err != nil {
					return err
				}
				items, page, err := r.client.ListProjects(cmd.Context(), q)
				if err != nil {
					return err
				}
				rows, err := compactRows(items, g.Full, compactProject)
				if err != nil {
					return err
				}
				return emitList(g, rows, paginationMeta(page.Next))
			})
		},
	}
	list.Flags().StringVar(&search, "search", "", "filter projects by name substring")
	list.Flags().IntVar(&limit, "limit", 0, "max projects to return")

	get := &cobra.Command{
		Use:   "get <id|name>",
		Short: "Get one project",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			return run(func() error {
				r, err := resolveClient(g)
				if err != nil {
					return err
				}
				raw, err := r.client.GetProject(cmd.Context(), args[0])
				if err != nil {
					return err
				}
				if g.Full {
					return printRaw(g, raw)
				}
				m, err := compactProject(raw)
				if err != nil {
					return err
				}
				return printSingle(g, m)
			})
		},
	}

	cmd.AddCommand(list, get)
	root.AddCommand(cmd)
}

type rawProject struct {
	ID                string `json:"id"`
	Name              string `json:"name"`
	Framework         string `json:"framework"`
	UpdatedAt         int64  `json:"updatedAt"`
	LatestDeployments []struct {
		UID        string  `json:"uid"`
		URL        string  `json:"url"`
		ReadyState string  `json:"readyState"`
		Target     *string `json:"target"`
	} `json:"latestDeployments"`
}

func compactProject(raw json.RawMessage) (map[string]any, error) {
	var p rawProject
	if err := json.Unmarshal(raw, &p); err != nil {
		return nil, agenterrors.Wrap(err, agenterrors.FixableByAgent)
	}
	m := map[string]any{"id": p.ID, "name": p.Name}
	putIf(m, "framework", p.Framework)
	putIf(m, "updated", msToRFC3339(p.UpdatedAt))
	for _, d := range p.LatestDeployments {
		if d.Target != nil && *d.Target == "production" {
			ld := map[string]any{"id": d.UID, "url": d.URL, "state": d.ReadyState}
			m["latest_prod_deployment"] = ld
			break
		}
	}
	return m, nil
}
