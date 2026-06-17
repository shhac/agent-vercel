package cli

import (
	"encoding/json"
	"net/url"
	"strconv"
	"strings"

	agenterrors "github.com/shhac/agent-vercel/internal/errors"
	"github.com/spf13/cobra"
)

func deploymentLogsCmd(g *GlobalFlags) *cobra.Command {
	var status, since, until, direction string
	var limit int
	cmd := &cobra.Command{
		Use:   "logs <id|url>",
		Short: "Build logs for a deployment (GET /v3/deployments/{id}/events)",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			q := url.Values{}
			setIf(q, "statusCode", status)
			setIf(q, "direction", direction)
			if limit != 0 {
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
			events, err := r.client.DeploymentEvents(cmd.Context(), args[0], q)
			if err != nil {
				return err
			}
			max := bodyLimit(g, 4000)
			rows := make([]any, 0, len(events))
			for _, ev := range events {
				if g.Full {
					rows = append(rows, ev)
					continue
				}
				rows = append(rows, compactBuildEvent(ev, max))
			}
			return emitList(g, rows, nil)
		},
	}
	f := cmd.Flags()
	f.StringVar(&status, "status", "", "filter by HTTP status range (e.g. 5xx)")
	f.StringVar(&since, "since", "", "only events after this time (duration or date)")
	f.StringVar(&until, "until", "", "only events before this time (duration or date)")
	f.StringVar(&direction, "direction", "", "order: forward|backward")
	f.IntVar(&limit, "limit", 0, "max events (-1 for all)")
	return cmd
}

func deploymentRuntimeLogsCmd(g *GlobalFlags) *cobra.Command {
	var level, status, path string
	cmd := &cobra.Command{
		Use:   "runtime-logs <id|url>",
		Short: "Runtime (function) logs for a deployment",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			r, err := resolveClient(g)
			if err != nil {
				return err
			}
			// Runtime logs are keyed by projectId + deploymentId; resolve
			// both from the deployment first.
			raw, err := r.client.GetDeployment(cmd.Context(), args[0])
			if err != nil {
				return err
			}
			dep, err := compactDeployment(raw)
			if err != nil {
				return err
			}
			projectID, _ := dep["project_id"].(string)
			depID, _ := dep["id"].(string)
			if projectID == "" || depID == "" {
				return agenterrors.New("could not resolve project for deployment", agenterrors.FixableByAgent)
			}
			logs, err := r.client.RuntimeLogs(cmd.Context(), projectID, depID, url.Values{})
			if err != nil {
				return err
			}
			max := bodyLimit(g, 4000)
			rows := make([]any, 0, len(logs))
			for _, lg := range logs {
				rl, raw := compactRuntimeLog(lg, max)
				if !matchRuntimeFilter(rl, level, status, path) {
					continue
				}
				if g.Full {
					rows = append(rows, raw)
				} else {
					rows = append(rows, rl)
				}
			}
			return emitList(g, rows, nil)
		},
	}
	f := cmd.Flags()
	f.StringVar(&level, "level", "", "filter by level (trace|debug|info|warning|error|fatal)")
	f.StringVar(&status, "status", "", "filter by response status code or class (e.g. 500 or 5xx)")
	f.StringVar(&path, "path", "", "filter by request path prefix")
	return cmd
}

func compactBuildEvent(raw json.RawMessage, max int) map[string]any {
	var e struct {
		Type    string `json:"type"`
		Created int64  `json:"created"`
		Payload struct {
			Text       string `json:"text"`
			Date       int64  `json:"date"`
			StatusCode int    `json:"statusCode"`
		} `json:"payload"`
	}
	_ = json.Unmarshal(raw, &e)
	created := e.Created
	if created == 0 {
		created = e.Payload.Date
	}
	m := map[string]any{"type": e.Type}
	putIf(m, "created", msToRFC3339(created))
	putIf(m, "text", truncate(e.Payload.Text, max))
	if e.Payload.StatusCode != 0 {
		m["status"] = e.Payload.StatusCode
	}
	return m
}

func compactRuntimeLog(raw json.RawMessage, max int) (map[string]any, json.RawMessage) {
	var l struct {
		Level        string `json:"level"`
		Source       string `json:"source"`
		Message      string `json:"message"`
		TimestampMs  int64  `json:"timestampInMs"`
		Method       string `json:"requestMethod"`
		Path         string `json:"requestPath"`
		ResponseCode int    `json:"responseStatusCode"`
	}
	_ = json.Unmarshal(raw, &l)
	m := map[string]any{"level": l.Level}
	putIf(m, "source", l.Source)
	putIf(m, "timestamp", msToRFC3339(l.TimestampMs))
	putIf(m, "method", l.Method)
	putIf(m, "path", l.Path)
	if l.ResponseCode != 0 {
		m["status"] = l.ResponseCode
	}
	putIf(m, "message", truncate(l.Message, max))
	return m, raw
}

// matchRuntimeFilter applies the client-side --level/--status/--path filters.
func matchRuntimeFilter(m map[string]any, level, status, path string) bool {
	if level != "" {
		if lv, _ := m["level"].(string); !strings.EqualFold(lv, level) {
			return false
		}
	}
	if status != "" {
		code, _ := m["status"].(int)
		if !statusMatches(code, status) {
			return false
		}
	}
	if path != "" {
		p, _ := m["path"].(string)
		if !strings.HasPrefix(p, path) {
			return false
		}
	}
	return true
}

// statusMatches supports an exact code ("500") or a class ("5xx").
func statusMatches(code int, want string) bool {
	want = strings.ToLower(strings.TrimSpace(want))
	if strings.HasSuffix(want, "xx") && len(want) == 3 {
		return code/100 == int(want[0]-'0')
	}
	if n, err := strconv.Atoi(want); err == nil {
		return code == n
	}
	return false
}
