package cli

import (
	"encoding/json"
	"net/url"
	"strconv"

	agenterrors "github.com/shhac/agent-vercel/internal/errors"
	"github.com/shhac/agent-vercel/internal/vercel"
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
	var cursor *string
	var all *bool
	list := &cobra.Command{
		Use:   "list",
		Short: "List projects in the scope",
		Args:  cobra.NoArgs,
		RunE: func(cmd *cobra.Command, _ []string) error {
			q := url.Values{}
			setIf(q, "search", search)
			if limit > 0 {
				q.Set("limit", strconv.Itoa(limit))
			}
			r, err := resolveClient(g)
			if err != nil {
				return err
			}
			return emitPaged(g, q, *cursor, *all, func(q url.Values) ([]json.RawMessage, vercel.Page, error) {
				return r.client.ListProjects(cmd.Context(), q)
			}, compactProject)
		},
	}
	cursor, all = addPageFlags(list)
	list.Flags().StringVar(&search, "search", "", "filter projects by name substring")
	list.Flags().IntVar(&limit, "limit", 0, "max projects to return")

	get := &cobra.Command{
		Use:   "get <id|name>",
		Short: "Get one project",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			return emitOne(g, func(c *vercel.Client) (json.RawMessage, error) {
				return c.GetProject(cmd.Context(), args[0])
			}, compactProject)
		},
	}

	crons := &cobra.Command{
		Use:   "crons <id|name>",
		Short: "Show the cron jobs a project runs and whether crons are enabled",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			return emitOne(g, func(c *vercel.Client) (json.RawMessage, error) {
				return c.ProjectCrons(cmd.Context(), args[0])
			}, func(raw json.RawMessage) (map[string]any, error) {
				m, err := compactCrons(raw)
				if err != nil {
					return nil, err
				}
				m["project"] = args[0]
				return m, nil
			})
		},
	}

	customEnvs := &cobra.Command{
		Use:     "custom-environments <id|name>",
		Aliases: []string{"custom-envs"},
		Short:   "List a project's custom deployment environments (slug, branch binding, domains)",
		Args:    cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			r, err := resolveClient(g)
			if err != nil {
				return err
			}
			items, err := r.client.ProjectCustomEnvironments(cmd.Context(), args[0])
			if err != nil {
				return err
			}
			return emitRows(g, items, compactCustomEnv)
		},
	}

	cmd.AddCommand(list, get, crons, customEnvs)
	root.AddCommand(cmd)
}

type rawCustomEnv struct {
	ID            string `json:"id"`
	Slug          string `json:"slug"`
	Type          string `json:"type"`
	Description   string `json:"description"`
	BranchMatcher struct {
		Type    string `json:"type"`
		Pattern string `json:"pattern"`
	} `json:"branchMatcher"`
	Domains []struct {
		Name string `json:"name"`
	} `json:"domains"`
	CreatedAt int64 `json:"createdAt"`
	UpdatedAt int64 `json:"updatedAt"`
}

// compactCustomEnv projects a custom environment: its slug and type, the git
// branch binding (as "matcherType:pattern", e.g. "startsWith:release/"), and
// any attached domains.
func compactCustomEnv(raw json.RawMessage) (map[string]any, error) {
	var e rawCustomEnv
	if err := json.Unmarshal(raw, &e); err != nil {
		return nil, wrapAgent(err)
	}
	m := map[string]any{"id": e.ID, "slug": e.Slug, "type": e.Type}
	putIf(m, "description", e.Description)
	if e.BranchMatcher.Type != "" {
		m["branch_matcher"] = e.BranchMatcher.Type + ":" + e.BranchMatcher.Pattern
	}
	if len(e.Domains) > 0 {
		names := make([]string, len(e.Domains))
		for i, d := range e.Domains {
			names[i] = d.Name
		}
		m["domains"] = names
	}
	putIf(m, "created", msToRFC3339(e.CreatedAt))
	putIf(m, "updated", msToRFC3339(e.UpdatedAt))
	return m, nil
}

type rawCrons struct {
	Crons struct {
		EnabledAt    int64  `json:"enabledAt"`
		DisabledAt   *int64 `json:"disabledAt"`
		UpdatedAt    int64  `json:"updatedAt"`
		DeploymentID string `json:"deploymentId"`
		Definitions  []struct {
			Host     string `json:"host"`
			Path     string `json:"path"`
			Schedule string `json:"schedule"`
		} `json:"definitions"`
	} `json:"crons"`
}

// compactCrons projects the crons config: whether crons are enabled (an
// enabledAt with no later disabledAt) and the path/schedule of each job.
func compactCrons(raw json.RawMessage) (map[string]any, error) {
	var c rawCrons
	if err := json.Unmarshal(raw, &c); err != nil {
		return nil, wrapAgent(err)
	}
	cr := c.Crons
	m := map[string]any{"enabled": cr.EnabledAt != 0 && cr.DisabledAt == nil}
	putIf(m, "deployment_id", cr.DeploymentID)
	putIf(m, "updated", msToRFC3339(cr.UpdatedAt))
	jobs := make([]any, 0, len(cr.Definitions))
	for _, d := range cr.Definitions {
		j := map[string]any{"path": d.Path, "schedule": d.Schedule}
		putIf(j, "host", d.Host)
		jobs = append(jobs, j)
	}
	m["jobs"] = jobs
	return m, nil
}

type rawProject struct {
	ID              string  `json:"id"`
	Name            string  `json:"name"`
	Framework       string  `json:"framework"`
	NodeVersion     string  `json:"nodeVersion"`
	RootDirectory   *string `json:"rootDirectory"`
	OutputDirectory *string `json:"outputDirectory"`
	BuildCommand    *string `json:"buildCommand"`
	InstallCommand  *string `json:"installCommand"`
	IgnoreCommand   *string `json:"commandForIgnoringBuildStep"`
	Paused          bool    `json:"paused"`
	UpdatedAt       int64   `json:"updatedAt"`
	Link            struct {
		Org              string `json:"org"`
		Repo             string `json:"repo"`
		Type             string `json:"type"`
		ProductionBranch string `json:"productionBranch"`
	} `json:"link"`
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
	if p.Paused {
		m["paused"] = true
	}
	putIf(m, "framework", p.Framework)
	putIf(m, "node_version", p.NodeVersion)
	putPtr(m, "root_directory", p.RootDirectory)
	putPtr(m, "output_directory", p.OutputDirectory)
	putPtr(m, "build_command", p.BuildCommand)
	putPtr(m, "install_command", p.InstallCommand)
	putPtr(m, "ignore_command", p.IgnoreCommand)
	if p.Link.Org != "" && p.Link.Repo != "" {
		m["repo"] = p.Link.Org + "/" + p.Link.Repo
	}
	putIf(m, "production_branch", p.Link.ProductionBranch)
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
