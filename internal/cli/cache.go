package cli

import (
	"strings"

	agenterrors "github.com/shhac/agent-vercel/internal/errors"
	"github.com/spf13/cobra"
)

// registerCache wires the `cache` group: edge/CDN cache operations. Today only
// tag-based invalidation; room to grow (stats, config).
func registerCache(root *cobra.Command, g *GlobalFlags) {
	cmd := &cobra.Command{
		Use:   "cache",
		Short: "Edge/CDN cache operations (invalidate by tag)",
		RunE:  func(c *cobra.Command, args []string) error { return handleUnknownSubcommand(c, args) },
	}

	var tags []string
	var yes *bool
	purge := &cobra.Command{
		Use:   "purge <project>",
		Short: "Invalidate CDN/runtime/data cache entries by tag (background revalidate)",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			if len(tags) == 0 {
				return agenterrors.New("no cache tags given", agenterrors.FixableByAgent).
					WithHint("pass --tag <tag> (repeatable, max 16)")
			}
			r, err := confirmAndClient(g, *yes,
				"purge cache tags ["+strings.Join(tags, ",")+"] on "+args[0],
				"agent-vercel cache purge "+args[0]+" --tag "+tags[0]+" --yes")
			if err != nil {
				return err
			}
			raw, err := r.client.PurgeCacheByTags(cmd.Context(), args[0], tags)
			if err != nil {
				return err
			}
			if len(raw) == 0 {
				return printSingle(g, map[string]any{"purged": tags, "project": args[0]})
			}
			return printRaw(g, raw)
		},
	}
	purge.Flags().StringArrayVar(&tags, "tag", nil, "cache tag to invalidate (repeatable, max 16)")
	yes = addYesFlag(purge)

	cmd.AddCommand(purge)
	root.AddCommand(cmd)
}
