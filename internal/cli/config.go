package cli

import (
	"os"
	"path/filepath"
	"strconv"

	agenterrors "github.com/shhac/agent-vercel/internal/errors"
	"github.com/shhac/agent-vercel/internal/output"
	"github.com/shhac/lib-agent-cli/creds"
	"github.com/spf13/cobra"
)

// configKeys are the persisted defaults config understands, each backing a
// global flag (precedence: explicit flag > config > built-in default).
var configKeys = map[string]string{
	"format":         "default output format: json|yaml|jsonl",
	"max-body-chars": "default body/log truncation (0 = per-command default, -1 = unlimited)",
	"timeout":        "default request timeout in milliseconds",
}

// configStore persists the flat string→string config map (non-secret defaults).
// $AGENT_VERCEL_CONFIG overrides the file path; otherwise it lives beside the
// credentials under the XDG config dir. Storage (0600, XDG dir) is provided by
// lib-agent-cli/creds; the key set and validation stay here.
func configStore() creds.Store {
	path := os.Getenv("AGENT_VERCEL_CONFIG")
	if path == "" {
		path = filepath.Join(creds.ConfigDir("agent-vercel"), "config.json")
	}
	return creds.Store{Path: path}
}

func loadConfig() map[string]string {
	m := map[string]string{}
	_ = configStore().Load(&m)
	return m
}

func saveConfig(m map[string]string) error { return configStore().Save(m) }

// validateConfig checks a key is a known config setting with a valid value,
// returning a fixable_by:agent error otherwise.
func validateConfig(key, value string) error {
	if key == "auth" || key == "scope" {
		return agenterrors.Newf(agenterrors.FixableByAgent, "%q is not a config setting", key).
			WithHint("set the default credential/scope with 'agent-vercel " +
				map[string]string{"auth": "auth set-default", "scope": "scope set-default"}[key] + "'")
	}
	if _, ok := configKeys[key]; !ok {
		return agenterrors.Newf(agenterrors.FixableByAgent, "unknown config key %q; valid: format, max-body-chars, timeout", key)
	}
	switch key {
	case "format":
		if _, err := output.ParseFormat(value); err != nil {
			return err
		}
	case "max-body-chars", "timeout":
		if _, err := strconv.Atoi(value); err != nil {
			return agenterrors.Newf(agenterrors.FixableByAgent, "%s must be an integer, got %q", key, value)
		}
	}
	return nil
}

func registerConfig(root *cobra.Command, g *GlobalFlags) {
	cmd := &cobra.Command{
		Use:   "config",
		Short: "Persisted defaults for output/transport flags (config.json)",
		RunE:  func(c *cobra.Command, args []string) error { return handleUnknownSubcommand(c, args) },
	}

	get := &cobra.Command{
		Use:   "get <key>",
		Short: "Read one setting",
		Args:  cobra.ExactArgs(1),
		RunE: func(_ *cobra.Command, args []string) error {
			v, ok := loadConfig()[args[0]]
			if !ok {
				return agenterrors.Newf(agenterrors.FixableByAgent, "no setting %q", args[0]).
					WithHint("run 'agent-vercel config list' to see settings")
			}
			return printSingle(g, map[string]any{"key": args[0], "value": v})
		},
	}

	set := &cobra.Command{
		Use:   "set <key> <value>",
		Short: "Set one setting",
		Args:  cobra.ExactArgs(2),
		RunE: func(_ *cobra.Command, args []string) error {
			if err := validateConfig(args[0], args[1]); err != nil {
				return err
			}
			m := loadConfig()
			m[args[0]] = args[1]
			if err := saveConfig(m); err != nil {
				return agenterrors.Wrap(err, agenterrors.FixableByHuman)
			}
			return printSingle(g, map[string]any{"key": args[0], "value": args[1]})
		},
	}

	list := &cobra.Command{
		Use:   "list",
		Short: "List all settings",
		Args:  cobra.NoArgs,
		RunE: func(_ *cobra.Command, _ []string) error {
			m := loadConfig()
			out := make(map[string]any, len(m))
			for k, v := range m {
				out[k] = v
			}
			return printSingle(g, out)
		},
	}

	unset := &cobra.Command{
		Use:   "unset <key>",
		Short: "Remove one setting",
		Args:  cobra.ExactArgs(1),
		RunE: func(_ *cobra.Command, args []string) error {
			m := loadConfig()
			delete(m, args[0])
			if err := saveConfig(m); err != nil {
				return agenterrors.Wrap(err, agenterrors.FixableByHuman)
			}
			return printSingle(g, map[string]any{"unset": args[0]})
		},
	}

	cmd.AddCommand(get, set, list, unset)
	root.AddCommand(cmd)
}
