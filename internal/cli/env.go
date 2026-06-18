package cli

import (
	"encoding/json"
	"net/url"
	"os"
	"sort"
	"strings"

	agenterrors "github.com/shhac/agent-vercel/internal/errors"
	"github.com/spf13/cobra"
)

func registerEnv(root *cobra.Command, g *GlobalFlags) {
	cmd := &cobra.Command{
		Use:   "env",
		Short: "Inspect a project's environment variables (and diff across environments)",
		RunE:  func(c *cobra.Command, args []string) error { return handleUnknownSubcommand(c, args) },
	}
	cmd.AddCommand(envListCmd(g), envGetCmd(g), envDiffCmd(g), envSharedCmd(g), envSetCmd(g), envRmCmd(g), envPullCmd(g))
	root.AddCommand(cmd)
}

func envPullCmd(g *GlobalFlags) *cobra.Command {
	var environment, out, gitBranch string
	cmd := &cobra.Command{
		Use:   "pull <project>",
		Short: "Write a project's (decrypted) env vars for one environment to a dotenv file",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			_, envs, err := fetchEnv(g, cmd, args[0], gitBranch, "", true)
			if err != nil {
				return err
			}
			body, count := buildDotenv(envs, environment)
			if err := os.WriteFile(out, []byte(body), 0o600); err != nil {
				return agenterrors.Wrap(err, agenterrors.FixableByHuman)
			}
			return printSingle(g, map[string]any{"written": out, "count": count, "environment": environment})
		},
	}
	cmd.Flags().StringVar(&environment, "environment", "development", "which environment to pull (production|preview|development)")
	cmd.Flags().StringVar(&out, "out", ".env", "output dotenv file path")
	cmd.Flags().StringVar(&gitBranch, "git-branch", "", "preview branch to pull")
	return cmd
}

// buildDotenv renders the dotenv body for one environment from a project's env
// vars: keys targeting environment, sorted, last value winning on duplicates.
// Returns the file body and the number of keys written. Pure, so the dedup /
// quoting / last-write-wins logic is unit-testable without I/O.
func buildDotenv(envs []rawEnv, environment string) (string, int) {
	keys := make([]string, 0, len(envs))
	vals := map[string]string{}
	for _, e := range envs {
		if !targets(e)[environment] {
			continue
		}
		if _, seen := vals[e.Key]; !seen {
			keys = append(keys, e.Key)
		}
		vals[e.Key] = e.Value
	}
	sort.Strings(keys)
	var b strings.Builder
	for _, k := range keys {
		b.WriteString(k + "=" + dotenvQuote(vals[k]) + "\n")
	}
	return b.String(), len(keys)
}

// dotenvQuote double-quotes a value and escapes backslashes, quotes, and
// newlines so the dotenv line round-trips.
func dotenvQuote(v string) string {
	r := strings.NewReplacer(`\`, `\\`, `"`, `\"`, "\n", `\n`, "\r", `\r`)
	return `"` + r.Replace(v) + `"`
}

type rawEnv struct {
	ID        string   `json:"id"`
	Key       string   `json:"key"`
	Target    []string `json:"target"`
	Type      string   `json:"type"`
	GitBranch string   `json:"gitBranch"`
	Comment   string   `json:"comment"`
	Value     string   `json:"value"`
}

func compactEnv(e rawEnv, withValue bool) map[string]any {
	m := map[string]any{"id": e.ID, "key": e.Key, "type": e.Type}
	if len(e.Target) > 0 {
		m["target"] = e.Target
	}
	putIf(m, "git_branch", e.GitBranch)
	putIf(m, "comment", e.Comment)
	if withValue && e.Value != "" {
		m["value"] = e.Value
	}
	return m
}

func targets(e rawEnv) map[string]bool {
	set := map[string]bool{}
	for _, t := range e.Target {
		set[t] = true
	}
	return set
}

// fetchEnv resolves a client and returns a project's env vars along with the
// *resolved it built, so a caller that also mutates (env rm) can reuse the same
// client instead of opening the credential store and reading the Keychain twice.
func fetchEnv(g *GlobalFlags, cmd *cobra.Command, project, gitBranch, customEnv string, decrypt bool) (*resolved, []rawEnv, error) {
	q := url.Values{}
	if decrypt {
		q.Set("decrypt", "true")
	}
	setIf(q, "gitBranch", gitBranch)
	setIf(q, "customEnvironmentId", customEnv)
	r, err := resolveClient(g)
	if err != nil {
		return nil, nil, err
	}
	items, err := r.client.ProjectEnv(cmd.Context(), project, q)
	if err != nil {
		return nil, nil, err
	}
	out := make([]rawEnv, 0, len(items))
	for _, it := range items {
		var e rawEnv
		if err := json.Unmarshal(it, &e); err != nil {
			return nil, nil, agenterrors.Wrap(err, agenterrors.FixableByAgent)
		}
		out = append(out, e)
	}
	return r, out, nil
}

func envListCmd(g *GlobalFlags) *cobra.Command {
	var environment, gitBranch, customEnv string
	var decrypt bool
	cmd := &cobra.Command{
		Use:   "list <project>",
		Short: "List a project's environment variables",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			_, envs, err := fetchEnv(g, cmd, args[0], gitBranch, customEnv, decrypt)
			if err != nil {
				return err
			}
			rows := make([]any, 0, len(envs))
			for _, e := range envs {
				if environment != "" && !targets(e)[environment] {
					continue
				}
				rows = append(rows, compactEnv(e, decrypt))
			}
			return emitList(g, rows, nil)
		},
	}
	f := cmd.Flags()
	f.StringVar(&environment, "environment", "", "filter to vars targeting this environment (production|preview|development)")
	f.StringVar(&gitBranch, "git-branch", "", "filter preview vars to a git branch")
	f.StringVar(&customEnv, "custom-env", "", "custom environment id")
	f.BoolVar(&decrypt, "decrypt", false, "include decrypted values")
	return cmd
}

func envGetCmd(g *GlobalFlags) *cobra.Command {
	var environment string
	var decrypt bool
	cmd := &cobra.Command{
		Use:   "get <project> <key>",
		Short: "Get one environment variable (across or within an environment)",
		Args:  cobra.ExactArgs(2),
		RunE: func(cmd *cobra.Command, args []string) error {
			_, envs, err := fetchEnv(g, cmd, args[0], "", "", decrypt)
			if err != nil {
				return err
			}
			var matches []any
			for _, e := range envs {
				if e.Key != args[1] {
					continue
				}
				if environment != "" && !targets(e)[environment] {
					continue
				}
				matches = append(matches, compactEnv(e, decrypt))
			}
			switch len(matches) {
			case 0:
				return agenterrors.Newf(agenterrors.FixableByAgent, "no env var %q in project %q", args[1], args[0]).
					WithHint("run 'agent-vercel env list " + args[0] + "' to see keys")
			case 1:
				return printSingle(g, matches[0])
			default:
				return emitList(g, matches, nil)
			}
		},
	}
	cmd.Flags().StringVar(&environment, "environment", "", "limit to this environment")
	cmd.Flags().BoolVar(&decrypt, "decrypt", false, "include the decrypted value")
	return cmd
}

func envDiffCmd(g *GlobalFlags) *cobra.Command {
	var environments string
	cmd := &cobra.Command{
		Use:   "diff <project>",
		Short: "Diff env vars between two environments (which keys differ or are missing)",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			parts := strings.Split(environments, ",")
			if len(parts) != 2 {
				return agenterrors.Newf(agenterrors.FixableByAgent, "--environments needs exactly two, comma-separated (got %q)", environments)
			}
			a, b := strings.TrimSpace(parts[0]), strings.TrimSpace(parts[1])

			// Decrypt so value-level differences (not just presence) surface.
			_, envs, err := fetchEnv(g, cmd, args[0], "", "", true)
			if err != nil {
				return err
			}
			byKey := map[string]map[string]string{}
			for _, e := range envs {
				tg := targets(e)
				for _, env := range []string{a, b} {
					if tg[env] {
						if byKey[e.Key] == nil {
							byKey[e.Key] = map[string]string{}
						}
						byKey[e.Key][env] = e.Value
					}
				}
			}
			return emitList(g, envDiffRows(byKey, a, b), nil)
		},
	}
	cmd.Flags().StringVar(&environments, "environments", "production,preview", "two environments to compare, comma-separated")
	return cmd
}

// envDiffStatus classifies a key across two environments a and b given each
// side's value and presence: "only_<env>" when present on just one side,
// "same" when both values match, "different" otherwise.
func envDiffStatus(va string, oka bool, vb string, okb bool, a, b string) string {
	switch {
	case oka && !okb:
		return "only_" + a
	case okb && !oka:
		return "only_" + b
	case va == vb:
		return "same"
	default:
		return "different"
	}
}

// envDiffRows builds the sorted diff output for two environments from the
// per-key presence/values, emitting only the keys that differ (present on one
// side only, or differing value) — "same" keys are dropped.
func envDiffRows(byKey map[string]map[string]string, a, b string) []any {
	keys := make([]string, 0, len(byKey))
	for k := range byKey {
		keys = append(keys, k)
	}
	sort.Strings(keys)

	rows := make([]any, 0)
	for _, k := range keys {
		va, oka := byKey[k][a]
		vb, okb := byKey[k][b]
		status := envDiffStatus(va, oka, vb, okb, a, b)
		if status == "same" {
			continue
		}
		row := map[string]any{"key": k, "status": status}
		if oka {
			row[a] = va
		}
		if okb {
			row[b] = vb
		}
		rows = append(rows, row)
	}
	return rows
}
