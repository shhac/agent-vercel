package cli

import (
	"fmt"
	"os"
	"strings"

	agenterrors "github.com/shhac/agent-vercel/internal/errors"
	"github.com/shhac/agent-vercel/internal/output"
	"github.com/spf13/cobra"
)

// GlobalFlags carries the persistent flags shared by every command. A
// credential (--auth, the secret) and a scope (--scope, which team) are
// separate axes: one credential reaches many teams.
type GlobalFlags struct {
	Scope        string // team slug or id; "" resolves to the default/personal account
	Auth         string // credential label selecting which stored credential to use
	Format       string // json | yaml | jsonl ("" = per-command default)
	TimeoutMS    int
	Debug        bool
	Full         bool
	BaseURL      string // hidden; overrides https://api.vercel.com for tests
	MaxBodyChars int
}

// Execute builds the root command and runs it. Any error — from a RunE body, a
// PersistentPreRunE check, flag parsing, or an unknown-subcommand handler — is
// rendered here as the family's structured JSON on stderr, exactly once, then
// signalled as a non-zero exit. (cobra's own printing is silenced.)
func Execute(version string) error {
	root := newRootCmd(version)
	if err := root.Execute(); err != nil {
		writeErr(err)
		return err
	}
	return nil
}

func newRootCmd(version string) *cobra.Command {
	g := &GlobalFlags{}

	root := &cobra.Command{
		Use:           "agent-vercel",
		Short:         "Vercel CLI for AI agents — token-efficient, structured output",
		Version:       version,
		SilenceUsage:  true,
		SilenceErrors: true,
		PersistentPreRunE: func(_ *cobra.Command, _ []string) error {
			// Validate --format up front so a bad value fails before any work.
			if g.Format != "" {
				if _, err := output.ParseFormat(g.Format); err != nil {
					return err
				}
			}
			return nil
		},
		RunE: func(cmd *cobra.Command, _ []string) error {
			return cmd.Help()
		},
	}

	pf := root.PersistentFlags()
	pf.StringVarP(&g.Scope, "scope", "s", "", "team slug or id to act on (default: personal account / stored default)")
	pf.StringVar(&g.Auth, "auth", "", "credential label selecting which stored credential to use")
	pf.StringVarP(&g.Format, "format", "f", "", "output format: json|yaml|jsonl")
	pf.IntVarP(&g.TimeoutMS, "timeout", "t", 0, "request timeout in milliseconds (0 = client default)")
	pf.BoolVarP(&g.Debug, "debug", "d", false, "log redacted HTTP records to stderr")
	pf.BoolVar(&g.Full, "full", false, "return raw Vercel API payloads instead of compact projections")
	pf.IntVar(&g.MaxBodyChars, "max-body-chars", 0, "truncate long body/log fields (0 = per-command default)")
	pf.StringVar(&g.BaseURL, "base-url", "", "override the API base URL (testing)")
	_ = pf.MarkHidden("base-url")

	// RunE wrappers render structured errors; Execute just sets the exit code.
	cobra.EnableCommandSorting = false

	registerUsage(root)
	registerAuth(root, g)
	registerScope(root, g)
	registerDeployment(root, g)
	registerProject(root, g)
	registerEnv(root, g)
	registerDomain(root, g)
	registerAlias(root, g)
	registerAPI(root, g)
	registerConfig(root, g)

	// Attach a `usage` subcommand to every domain group (generated from the
	// command tree), so `agent-vercel <domain> usage` works uniformly.
	attachDomainUsage(root)

	// Surface flag-parse errors as fixable_by: agent; Execute renders them.
	root.SetFlagErrorFunc(func(_ *cobra.Command, err error) error {
		return agenterrors.Wrap(err, agenterrors.FixableByAgent)
	})

	return root
}

func writeErr(err error) { output.WriteError(os.Stderr, err) }

// handleUnknownSubcommand returns a structured error listing valid subcommands
// rather than cobra's bare help text, matching the family convention.
func handleUnknownSubcommand(cmd *cobra.Command, args []string) error {
	var names []string
	for _, c := range cmd.Commands() {
		if !c.Hidden {
			names = append(names, c.Name())
		}
	}
	got := ""
	if len(args) > 0 {
		got = args[0]
	}
	return agenterrors.Newf(agenterrors.FixableByAgent,
		"unknown subcommand %q for %q; valid: %s", got, cmd.CommandPath(), strings.Join(names, ", ")).
		WithHint(fmt.Sprintf("run '%s usage' for documentation", cmd.Root().Name()))
}

// printSingle renders one resource in the resolved format (default JSON).
func printSingle(g *GlobalFlags, data any) error {
	format, err := output.ResolveFormat(g.Format, output.FormatJSON)
	if err != nil {
		return err
	}
	output.Print(os.Stdout, data, format, true)
	return nil
}
