package cli

import (
	"fmt"
	"os"
	"strconv"
	"strings"

	"github.com/shhac/agent-vercel/internal/credential"
	agenterrors "github.com/shhac/agent-vercel/internal/errors"
	"github.com/shhac/agent-vercel/internal/output"
	libcli "github.com/shhac/lib-agent-cli/cli"
	agentmcp "github.com/shhac/lib-agent-mcp"
	"github.com/spf13/cobra"
)

// GlobalFlags carries the persistent flags shared by every command. A
// credential (--auth, the secret) and a scope (--scope, which team) are
// separate axes: one credential reaches many teams. The shared
// --format/--timeout/--debug live in the embedded libcli.Globals.
type GlobalFlags struct {
	libcli.Globals // Format, TimeoutMS, Debug

	Scope        string // team slug or id; "" resolves to the default/personal account
	Auth         string // credential label selecting which stored credential to use
	Full         bool
	BaseURL      string // hidden; overrides https://api.vercel.com for tests
	MaxBodyChars int
}

// applyConfigDefaults fills unset presentation/transport flags from config.json
// (precedence: explicit flag > config > built-in default). Best-effort: a
// missing or unreadable config never blocks a command. Credential/scope defaults
// are NOT here — those live in credentials.json via auth/scope set-default.
func applyConfigDefaults(g *GlobalFlags) {
	m := loadConfig()
	if g.Format == "" {
		g.Format = m["format"]
	}
	if g.MaxBodyChars == 0 {
		if n, err := strconv.Atoi(m["max-body-chars"]); err == nil {
			g.MaxBodyChars = n
		}
	}
	if g.TimeoutMS == 0 {
		if n, err := strconv.Atoi(m["timeout"]); err == nil {
			g.TimeoutMS = n
		}
	}
}

// NewRootCmd builds the root command. Errors — from a RunE body, a
// PersistentPreRunE check, flag parsing, or the unknown-command handler — are
// rendered as the family's structured JSON on stderr exactly once by
// libcli.Run, which also sets the exit code. (cobra's own printing is silenced
// by NewRoot.)
func NewRootCmd(version string) *cobra.Command {
	g := &GlobalFlags{}

	root := libcli.NewRoot(libcli.Options{
		Use:            "agent-vercel",
		Short:          "Vercel CLI for AI agents — token-efficient, structured output",
		Version:        version,
		Globals:        &g.Globals,
		DefaultFormat:  output.FormatNDJSON,
		ConfigDefaults: func() { applyConfigDefaults(g) },
		UnknownHint:    "run 'agent-vercel usage' to see the available domains",
	})

	pf := root.PersistentFlags()
	pf.StringVarP(&g.Scope, "scope", "s", "", "team slug or id to act on (default: personal account / stored default)")
	pf.StringVar(&g.Auth, "auth", "", "credential label selecting which stored credential to use")
	pf.BoolVar(&g.Full, "full", false, "return raw Vercel API payloads instead of compact projections")
	pf.IntVar(&g.MaxBodyChars, "max-body-chars", 0, "truncate long body/log fields (0 = per-command default, -1 = unlimited)")
	pf.StringVar(&g.BaseURL, "base-url", "", "override the API base URL (testing)")
	_ = pf.MarkHidden("base-url")

	// RunE wrappers return structured errors; libcli.Run renders them once and
	// sets the exit code.
	cobra.EnableCommandSorting = false

	registerUsage(root)
	registerAuth(root, g)
	registerScope(root, g)
	registerDeployment(root, g)
	registerProject(root, g)
	registerEnv(root, g)
	registerDomain(root, g)
	registerAlias(root, g)
	registerFirewall(root, g)
	registerCache(root, g)
	registerBilling(root, g)
	registerWebhook(root, g)
	registerDrains(root, g)
	registerEdgeConfig(root, g)
	registerAPI(root, g)
	registerConfig(root, g)

	// Attach a `usage` subcommand to every domain group (generated from the
	// command tree), so `agent-vercel <domain> usage` works uniformly.
	attachDomainUsage(root)

	// Surface flag-parse errors as fixable_by: agent; libcli.Run renders them.
	root.SetFlagErrorFunc(func(_ *cobra.Command, err error) error {
		return agenterrors.Wrap(err, agenterrors.FixableByAgent)
	})

	// An unknown *top-level* command never reaches NewRoot's RunE handler:
	// cobra's default Args validator (legacyArgs) rejects it first with a bare,
	// hintless "unknown command" error. Replace that validator so the root emits
	// the same structured, hinted error the domain groups do; libcli.Run then
	// renders it once.
	root.Args = func(cmd *cobra.Command, args []string) error {
		if len(args) == 0 {
			return nil
		}
		return agenterrors.Newf(agenterrors.FixableByAgent,
			"unknown command %q for %q", args[0], cmd.CommandPath()).
			WithHint("run 'agent-vercel usage' to see the available domains")
	}

	// Expose the whole command tree as an MCP server (added last, so it reflects
	// the complete tree). --color/--expose are output-shaping, irrelevant to a
	// tool call, so hide them from the generated schemas.
	// Opt the agent-facing groups into the MCP tool surface: each becomes one
	// coarse tool that dispatches its subcommands (with a "help" verb), so the
	// surface is ~one-tool-per-group instead of one-per-leaf. Credential/config/
	// usage commands are deliberately left out — they aren't agent tasks.
	exposeGroups(root,
		"deployment", "project", "env", "domain", "alias", "firewall", "cache", "billing", "webhook", "drains", "edge-config", "api")

	root.AddCommand(agentmcp.Command(root, agentmcp.WithHiddenFlags("color", "expose"), agentmcp.WithOAuthKeyringService(credential.MCPKeychainService())))

	return root
}

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

// exposeGroups opts the named top-level commands into the MCP tool surface.
// A name with no matching command is skipped silently — the list is a curation
// of agent-facing groups, not a registration check.
func exposeGroups(root *cobra.Command, names ...string) {
	want := make(map[string]bool, len(names))
	for _, n := range names {
		want[n] = true
	}
	for _, c := range root.Commands() {
		if want[c.Name()] {
			agentmcp.Expose(c)
		}
	}
}
