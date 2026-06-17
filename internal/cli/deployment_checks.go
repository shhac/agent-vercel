package cli

import (
	"encoding/json"

	agenterrors "github.com/shhac/agent-vercel/internal/errors"
	"github.com/spf13/cobra"
)

func deploymentChecksCmd(g *GlobalFlags) *cobra.Command {
	var blockingOnly, failedOnly bool
	cmd := &cobra.Command{
		Use:   "checks <id|url>",
		Short: "List the CI / integration checks on a deployment (what is blocking or failing it)",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			r, err := resolveClient(g)
			if err != nil {
				return err
			}
			items, err := r.client.DeploymentChecks(cmd.Context(), cleanRef(args[0]))
			if err != nil {
				return err
			}
			items = filterChecks(items, blockingOnly, failedOnly)
			rows, err := compactRows(items, g.Full, compactCheck)
			if err != nil {
				return err
			}
			return emitList(g, rows, nil)
		},
	}
	f := cmd.Flags()
	f.BoolVar(&blockingOnly, "blocking", false, "only checks that block promotion")
	f.BoolVar(&failedOnly, "failed", false, "only checks whose conclusion is not succeeded/skipped/neutral")
	return cmd
}

type rawCheck struct {
	ID            string `json:"id"`
	Name          string `json:"name"`
	Status        string `json:"status"`
	Conclusion    string `json:"conclusion"`
	Blocking      bool   `json:"blocking"`
	IntegrationID string `json:"integrationId"`
	DetailsURL    string `json:"detailsUrl"`
	Path          string `json:"path"`
	Rerequestable bool   `json:"rerequestable"`
	StartedAt     int64  `json:"startedAt"`
	CompletedAt   int64  `json:"completedAt"`
}

func compactCheck(raw json.RawMessage) (map[string]any, error) {
	var c rawCheck
	if err := json.Unmarshal(raw, &c); err != nil {
		return nil, agenterrors.Wrap(err, agenterrors.FixableByAgent)
	}
	m := map[string]any{"id": c.ID, "name": c.Name, "status": c.Status, "blocking": c.Blocking}
	putIf(m, "conclusion", c.Conclusion)
	putIf(m, "integration_id", c.IntegrationID)
	putIf(m, "details_url", c.DetailsURL)
	putIf(m, "path", c.Path)
	if c.Rerequestable {
		m["rerequestable"] = true
	}
	putIf(m, "started", msToRFC3339(c.StartedAt))
	putIf(m, "completed", msToRFC3339(c.CompletedAt))
	return m, nil
}

// checkPassed reports whether a check's conclusion counts as non-failing. An
// empty conclusion (check still registered/running) is not a pass.
func checkPassed(conclusion string) bool {
	switch conclusion {
	case "succeeded", "skipped", "neutral":
		return true
	default:
		return false
	}
}

// filterChecks narrows checks to those that are blocking and/or not-passing,
// client-side (the checks endpoint has no filter params).
func filterChecks(items []json.RawMessage, blockingOnly, failedOnly bool) []json.RawMessage {
	if !blockingOnly && !failedOnly {
		return items
	}
	out := make([]json.RawMessage, 0, len(items))
	for _, raw := range items {
		var c rawCheck
		if json.Unmarshal(raw, &c) != nil {
			continue
		}
		if blockingOnly && !c.Blocking {
			continue
		}
		if failedOnly && checkPassed(c.Conclusion) {
			continue
		}
		out = append(out, raw)
	}
	return out
}
