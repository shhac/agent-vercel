package cli

import (
	"strings"

	agenterrors "github.com/shhac/agent-vercel/internal/errors"
	"github.com/spf13/cobra"
)

// Gated (--yes) environment-variable mutations, split from env.go to mirror the
// deployment.go / deployment_write.go read-vs-write convention. The shared
// readers (fetchEnv, compactEnv, targets) stay in env.go.

func envSetCmd(g *GlobalFlags) *cobra.Command {
	var environments, gitBranch, varType string
	var yes *bool
	cmd := &cobra.Command{
		Use:   "set <project> <key> <value>",
		Short: "Create or update an environment variable",
		Args:  cobra.ExactArgs(3),
		RunE: func(cmd *cobra.Command, args []string) error {
			project, key, value := args[0], args[1], args[2]
			targetList := splitCSV(environments)
			if len(targetList) == 0 {
				return agenterrors.New("no environment specified", agenterrors.FixableByAgent).
					WithHint("pass --environment production[,preview,development]")
			}
			r, err := confirmAndClient(g, *yes,
				"set env "+key+" on "+project+" ("+strings.Join(targetList, ",")+")",
				"agent-vercel env set "+project+" "+key+" <value> --environment "+strings.Join(targetList, ",")+" --yes")
			if err != nil {
				return err
			}
			body := map[string]any{"key": key, "value": value, "type": varType, "target": targetList}
			if gitBranch != "" {
				body["gitBranch"] = gitBranch
			}
			raw, err := r.client.CreateEnv(cmd.Context(), project, body)
			if err != nil {
				return err
			}
			if g.Full {
				return printRaw(g, raw)
			}
			return printSingle(g, map[string]any{"set": key, "target": targetList})
		},
	}
	f := cmd.Flags()
	f.StringVar(&environments, "environment", "production", "comma-separated environments (production,preview,development)")
	f.StringVar(&gitBranch, "git-branch", "", "limit a preview var to a git branch")
	f.StringVar(&varType, "type", "encrypted", "variable type: encrypted|plain|sensitive")
	yes = addYesFlag(cmd)
	return cmd
}

func envRmCmd(g *GlobalFlags) *cobra.Command {
	var environment string
	var yes *bool
	cmd := &cobra.Command{
		Use:   "rm <project> <key>",
		Short: "Remove an environment variable",
		Args:  cobra.ExactArgs(2),
		RunE: func(cmd *cobra.Command, args []string) error {
			project, key := args[0], args[1]
			if err := requireYes(*yes, "remove env "+key+" from "+project,
				"agent-vercel env rm "+project+" "+key+" --yes"); err != nil {
				return err
			}
			r, envs, err := fetchEnv(g, cmd, project, "", "", false)
			if err != nil {
				return err
			}
			var ids []string
			for _, e := range envs {
				if e.Key != key {
					continue
				}
				if environment != "" && !targets(e)[environment] {
					continue
				}
				ids = append(ids, e.ID)
			}
			switch len(ids) {
			case 0:
				return agenterrors.Newf(agenterrors.FixableByAgent, "no env var %q in project %q", key, project).
					WithHint("run 'agent-vercel env list " + project + "' to see keys")
			case 1:
			default:
				return agenterrors.Newf(agenterrors.FixableByAgent, "%q matches %d env entries; narrow with --environment", key, len(ids))
			}
			if _, err := r.client.DeleteEnv(cmd.Context(), project, ids[0]); err != nil {
				return err
			}
			return printSingle(g, map[string]any{"removed": key, "id": ids[0]})
		},
	}
	cmd.Flags().StringVar(&environment, "environment", "", "limit to a single environment")
	yes = addYesFlag(cmd)
	return cmd
}

// splitCSV splits a comma-separated flag value, trimming blanks.
func splitCSV(s string) []string {
	parts := strings.Split(s, ",")
	out := make([]string, 0, len(parts))
	for _, p := range parts {
		if p = strings.TrimSpace(p); p != "" {
			out = append(out, p)
		}
	}
	return out
}
