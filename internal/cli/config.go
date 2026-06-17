package cli

import (
	agenterrors "github.com/shhac/agent-vercel/internal/errors"
	"github.com/shhac/agent-vercel/internal/settings"
	"github.com/spf13/cobra"
)

func registerConfig(root *cobra.Command, g *GlobalFlags) {
	cmd := &cobra.Command{
		Use:   "config",
		Short: "Persisted settings (e.g. cache TTLs) in config.json",
		RunE:  func(c *cobra.Command, args []string) error { return handleUnknownSubcommand(c, args) },
	}

	get := &cobra.Command{
		Use:   "get <key>",
		Short: "Read one setting",
		Args:  cobra.ExactArgs(1),
		RunE: func(_ *cobra.Command, args []string) error {
			return run(func() error {
				s, err := settings.New()
				if err != nil {
					return agenterrors.Wrap(err, agenterrors.FixableByHuman)
				}
				v, ok, err := s.Get(args[0])
				if err != nil {
					return agenterrors.Wrap(err, agenterrors.FixableByHuman)
				}
				if !ok {
					return agenterrors.Newf(agenterrors.FixableByAgent, "no setting %q", args[0]).
						WithHint("run 'agent-vercel config list' to see settings")
				}
				return printSingle(g, map[string]any{"key": args[0], "value": v})
			})
		},
	}

	set := &cobra.Command{
		Use:   "set <key> <value>",
		Short: "Set one setting",
		Args:  cobra.ExactArgs(2),
		RunE: func(_ *cobra.Command, args []string) error {
			return run(func() error {
				s, err := settings.New()
				if err != nil {
					return agenterrors.Wrap(err, agenterrors.FixableByHuman)
				}
				if err := s.Set(args[0], args[1]); err != nil {
					return agenterrors.Wrap(err, agenterrors.FixableByHuman)
				}
				return printSingle(g, map[string]any{"key": args[0], "value": args[1]})
			})
		},
	}

	list := &cobra.Command{
		Use:   "list",
		Short: "List all settings",
		Args:  cobra.NoArgs,
		RunE: func(_ *cobra.Command, _ []string) error {
			return run(func() error {
				s, err := settings.New()
				if err != nil {
					return agenterrors.Wrap(err, agenterrors.FixableByHuman)
				}
				m, err := s.Load()
				if err != nil {
					return agenterrors.Wrap(err, agenterrors.FixableByHuman)
				}
				out := make(map[string]any, len(m))
				for k, v := range m {
					out[k] = v
				}
				return printSingle(g, out)
			})
		},
	}

	unset := &cobra.Command{
		Use:   "unset <key>",
		Short: "Remove one setting",
		Args:  cobra.ExactArgs(1),
		RunE: func(_ *cobra.Command, args []string) error {
			return run(func() error {
				s, err := settings.New()
				if err != nil {
					return agenterrors.Wrap(err, agenterrors.FixableByHuman)
				}
				if err := s.Unset(args[0]); err != nil {
					return agenterrors.Wrap(err, agenterrors.FixableByHuman)
				}
				return printSingle(g, map[string]any{"unset": args[0]})
			})
		},
	}

	cmd.AddCommand(get, set, list, unset)
	root.AddCommand(cmd)
}
