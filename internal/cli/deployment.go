package cli

import (
	"encoding/json"
	"net/url"
	"strconv"
	"strings"
	"time"

	agenterrors "github.com/shhac/agent-vercel/internal/errors"
	"github.com/spf13/cobra"
)

func registerDeployment(root *cobra.Command, g *GlobalFlags) {
	cmd := &cobra.Command{
		Use:     "deployment",
		Aliases: []string{"deploy"},
		Short:   "Inspect deployments (cross-project, filterable) and what is live",
		RunE:    func(c *cobra.Command, args []string) error { return handleUnknownSubcommand(c, args) },
	}

	cmd.AddCommand(
		deploymentListCmd(g),
		deploymentGetCmd(g),
		deploymentChecksCmd(g),
		deploymentCurrentCmd(g),
		deploymentLogsCmd(g),
		deploymentRuntimeLogsCmd(g),
		deploymentPromoteCmd(g),
		deploymentRollbackCmd(g),
		deploymentCancelCmd(g),
		deploymentRedeployCmd(g),
	)
	root.AddCommand(cmd)
}

func deploymentListCmd(g *GlobalFlags) *cobra.Command {
	var project, state, target, branch, sha, user, since, until, customEnv string
	var limit int
	var cursor *string
	var all *bool
	cmd := &cobra.Command{
		Use:   "list",
		Short: "List deployments across the scope, filterable",
		Args:  cobra.NoArgs,
		RunE: func(cmd *cobra.Command, _ []string) error {
			q := url.Values{}
			setIf(q, "projectId", project)
			setIf(q, "state", strings.ToUpper(state))
			setIf(q, "target", target)
			setIf(q, "branch", branch)
			setIf(q, "sha", sha)
			setIf(q, "users", user)
			if limit > 0 {
				q.Set("limit", strconv.Itoa(limit))
			}
			if err := setTimeFilter(q, "since", since); err != nil {
				return err
			}
			if err := setTimeFilter(q, "until", until); err != nil {
				return err
			}

			r, err := resolveClient(g)
			if err != nil {
				return err
			}
			items, next, err := fetchPaged(q, *cursor, *all, func(q url.Values) ([]json.RawMessage, *int64, error) {
				it, p, e := r.client.ListDeployments(cmd.Context(), q)
				return it, p.Next, e
			})
			if err != nil {
				return err
			}
			items = filterByCustomEnv(items, customEnv)
			rows, err := compactRows(items, g.Full, compactDeployment)
			if err != nil {
				return err
			}
			return emitList(g, rows, paginationMeta(next))
		},
	}
	f := cmd.Flags()
	cursor, all = addPageFlags(cmd)
	f.StringVar(&project, "project", "", "filter by project id or name")
	f.StringVar(&customEnv, "custom-env", "", "filter to a custom environment (slug or id); pair with --all to scan")
	f.StringVar(&state, "state", "", "filter by state: BUILDING,ERROR,INITIALIZING,QUEUED,READY,CANCELED")
	f.StringVar(&target, "target", "", "filter by target environment (e.g. production)")
	f.StringVar(&branch, "branch", "", "filter by git branch")
	f.StringVar(&sha, "sha", "", "filter by git commit sha")
	f.StringVar(&user, "user", "", "filter by creator user id(s), comma-separated")
	f.StringVar(&since, "since", "", "only deployments after this time (duration like 24h/7d or date)")
	f.StringVar(&until, "until", "", "only deployments before this time (duration or date)")
	f.IntVar(&limit, "limit", 0, "max deployments to return")
	return cmd
}

func deploymentGetCmd(g *GlobalFlags) *cobra.Command {
	return &cobra.Command{
		Use:   "get <id|url>",
		Short: "Get one deployment",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			r, err := resolveClient(g)
			if err != nil {
				return err
			}
			raw, err := r.client.GetDeployment(cmd.Context(), cleanRef(args[0]))
			if err != nil {
				return err
			}
			return getOne(g, raw, compactDeployment)
		},
	}
}

func deploymentCurrentCmd(g *GlobalFlags) *cobra.Command {
	var customEnv string
	cmd := &cobra.Command{
		Use:   "current <project>",
		Short: "Show the deployment live in production (or a custom env), plus any rolling release",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			r, err := resolveClient(g)
			if err != nil {
				return err
			}
			out := map[string]any{"project": args[0]}
			if customEnv != "" {
				// No target param for custom envs: pull recent READY deploys and
				// pick the newest matching the custom environment.
				items, err := listReadyDeployments(cmd, r, args[0], "", 30)
				if err != nil {
					return err
				}
				out["custom_environment"] = customEnv
				if matches := filterByCustomEnv(items, customEnv); len(matches) > 0 {
					if m, err := compactDeployment(matches[0]); err == nil {
						out["live"] = m
					}
				}
				return printSingle(g, out)
			}

			items, err := listReadyDeployments(cmd, r, args[0], "production", 1)
			if err != nil {
				return err
			}
			if len(items) > 0 {
				if m, err := compactDeployment(items[0]); err == nil {
					out["live"] = m
				}
			}
			// Rolling-release state is best-effort: a project without one
			// (or the feature disabled) shouldn't fail the command.
			if rr, err := r.client.RollingRelease(cmd.Context(), args[0]); err == nil {
				var env struct {
					RollingRelease json.RawMessage `json:"rollingRelease"`
				}
				if json.Unmarshal(rr, &env) == nil && len(env.RollingRelease) > 0 && string(env.RollingRelease) != "null" {
					out["rolling_release"] = json.RawMessage(env.RollingRelease)
				}
			}
			return printSingle(g, out)
		},
	}
	cmd.Flags().StringVar(&customEnv, "custom-env", "", "show the newest READY deploy in this custom environment (slug or id) instead of production")
	return cmd
}

type rawDeployment struct {
	UID           string  `json:"uid"`
	ID            string  `json:"id"`
	Name          string  `json:"name"`
	ProjectID     string  `json:"projectId"`
	URL           string  `json:"url"`
	State         string  `json:"state"`
	ReadyState    string  `json:"readyState"`
	Target        *string `json:"target"`
	ReadySubstate string  `json:"readySubstate"`
	InspectorURL  string  `json:"inspectorUrl"`
	ErrorCode     string  `json:"errorCode"`
	ErrorMessage  string  `json:"errorMessage"`
	Created       int64   `json:"created"`
	CreatedAt     int64   `json:"createdAt"`
	Creator       struct {
		Username string `json:"username"`
		Email    string `json:"email"`
	} `json:"creator"`
	Meta              map[string]any `json:"meta"`
	OomReport         string         `json:"oomReport"`
	ChecksConclusion  string         `json:"checksConclusion"`
	ChecksState       string         `json:"checksState"`
	CustomEnvironment struct {
		ID   string `json:"id"`
		Slug string `json:"slug"`
	} `json:"customEnvironment"`
}

// listReadyDeployments fetches recent READY deployments for a project, sharing
// the query construction between the production and custom-environment paths of
// 'deployment current'. A non-empty target scopes server-side (e.g.
// "production"); an empty target with a larger limit backs the client-side
// custom-environment scan.
func listReadyDeployments(cmd *cobra.Command, r *resolved, projectID, target string, limit int) ([]json.RawMessage, error) {
	q := url.Values{}
	q.Set("projectId", projectID)
	q.Set("state", "READY")
	setIf(q, "target", target)
	q.Set("limit", strconv.Itoa(limit))
	items, _, err := r.client.ListDeployments(cmd.Context(), q)
	return items, err
}

// filterByCustomEnv keeps deployments whose customEnvironment id or slug matches
// env. Vercel's /v6/deployments has no custom-environment query param, so this
// filters client-side (pair with --all to scan beyond one page).
func filterByCustomEnv(items []json.RawMessage, env string) []json.RawMessage {
	if env == "" {
		return items
	}
	out := make([]json.RawMessage, 0, len(items))
	for _, raw := range items {
		var d struct {
			CustomEnvironment struct {
				ID   string `json:"id"`
				Slug string `json:"slug"`
			} `json:"customEnvironment"`
		}
		if json.Unmarshal(raw, &d) == nil && (d.CustomEnvironment.Slug == env || d.CustomEnvironment.ID == env) {
			out = append(out, raw)
		}
	}
	return out
}

func compactDeployment(raw json.RawMessage) (map[string]any, error) {
	var d rawDeployment
	if err := json.Unmarshal(raw, &d); err != nil {
		return nil, agenterrors.Wrap(err, agenterrors.FixableByAgent)
	}
	id := firstNonEmpty(d.UID, d.ID)
	state := firstNonEmpty(d.State, d.ReadyState)
	created := d.Created
	if created == 0 {
		created = d.CreatedAt
	}
	m := map[string]any{"id": id, "name": d.Name, "project_id": d.ProjectID, "state": state, "url": d.URL}
	if d.Target != nil {
		m["target"] = *d.Target
	}
	putIf(m, "custom_environment", d.CustomEnvironment.Slug)
	putIf(m, "ready_substate", d.ReadySubstate)
	putIf(m, "inspector_url", d.InspectorURL)
	putIf(m, "error_code", d.ErrorCode)
	putIf(m, "error_message", d.ErrorMessage)
	if d.OomReport != "" {
		m["oom"] = true
	}
	putIf(m, "checks", firstNonEmpty(d.ChecksConclusion, d.ChecksState))
	putIf(m, "created", msToRFC3339(created))
	putIf(m, "creator", d.Creator.Username)
	putIf(m, "branch", metaStr(d.Meta, "githubCommitRef", "gitlabCommitRef", "bitbucketCommitRef"))
	putIf(m, "sha", metaStr(d.Meta, "githubCommitSha", "gitlabCommitSha", "bitbucketCommitSha"))
	putIf(m, "commit_message", metaStr(d.Meta, "githubCommitMessage", "gitlabCommitMessage", "bitbucketCommitMessage"))
	return m, nil
}

// setTimeFilter sets a Vercel ms-timestamp query param from a duration (24h,
// 7d, 2w) interpreted as "ago", or an absolute RFC3339/date.
func setTimeFilter(q url.Values, key, s string) error {
	s = strings.TrimSpace(s)
	if s == "" {
		return nil
	}
	if ms, ok := relativeMS(s); ok {
		q.Set(key, strconv.FormatInt(ms, 10))
		return nil
	}
	for _, layout := range []string{time.RFC3339, "2006-01-02"} {
		if t, err := time.Parse(layout, s); err == nil {
			q.Set(key, strconv.FormatInt(t.UTC().UnixMilli(), 10))
			return nil
		}
	}
	return agenterrors.Newf(agenterrors.FixableByAgent, "invalid time %q; use a duration (24h, 7d) or date (2006-01-02)", s)
}

func relativeMS(s string) (int64, bool) {
	if n := len(s); n >= 2 {
		unit := map[byte]time.Duration{'d': 24 * time.Hour, 'w': 7 * 24 * time.Hour}
		if d, ok := unit[s[n-1]]; ok {
			if num, err := strconv.Atoi(s[:n-1]); err == nil {
				return time.Now().Add(-time.Duration(num) * d).UnixMilli(), true
			}
		}
	}
	if dur, err := time.ParseDuration(s); err == nil {
		return time.Now().Add(-dur).UnixMilli(), true
	}
	return 0, false
}
