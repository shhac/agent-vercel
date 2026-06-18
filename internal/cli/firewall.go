package cli

import (
	"encoding/json"

	"github.com/shhac/agent-vercel/internal/vercel"
	"github.com/spf13/cobra"
)

// registerFirewall wires the `firewall` group: read-only inspection of a
// project's Vercel Firewall (WAF) — config, active-attack status, and system
// bypass rules. These are spec-documented reads; the payload shapes are not
// live-validated, so compact projections stay defensive and `--full` returns the
// raw API object.
func registerFirewall(root *cobra.Command, g *GlobalFlags) {
	cmd := &cobra.Command{
		Use:   "firewall",
		Short: "Inspect a project's Vercel Firewall (WAF rules, attack status, bypass)",
		RunE:  func(c *cobra.Command, args []string) error { return handleUnknownSubcommand(c, args) },
	}

	config := &cobra.Command{
		Use:   "config <project>",
		Short: "Show the active firewall config: enabled state, custom rules, IP rules, managed rulesets",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			return emitOne(g, func(c *vercel.Client) (json.RawMessage, error) {
				return c.FirewallConfig(cmd.Context(), args[0])
			}, compactFirewallConfig)
		},
	}

	var since int
	attack := &cobra.Command{
		Use:   "attack-status <project>",
		Short: "Show active-attack / DDoS anomalies detected for a project",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			return emitOne(g, func(c *vercel.Client) (json.RawMessage, error) {
				return c.FirewallAttackStatus(cmd.Context(), args[0], since)
			}, compactAttackStatus)
		},
	}
	attack.Flags().IntVar(&since, "since", 0, "look back this many days (0 = API default, 1 day)")

	bypass := &cobra.Command{
		Use:   "bypass <project>",
		Short: "List the firewall system-bypass rules (sources allowed to skip the WAF)",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			r, err := resolveClient(g)
			if err != nil {
				return err
			}
			raw, err := r.client.FirewallBypass(cmd.Context(), args[0])
			if err != nil {
				return err
			}
			// Shape is uncertain (object or array of rules); print it raw.
			return printRaw(g, raw)
		},
	}

	cmd.AddCommand(config, attack, bypass)
	root.AddCommand(cmd)
}

// compactFirewallConfig projects the active firewall config defensively: the
// enabled flag, the names of active custom rules, an IP-rule count, and the
// active managed rulesets. Unknown/absent fields are simply omitted.
func compactFirewallConfig(raw json.RawMessage) (map[string]any, error) {
	var cfg struct {
		FirewallEnabled bool `json:"firewallEnabled"`
		Rules           []struct {
			Name   string `json:"name"`
			Active bool   `json:"active"`
		} `json:"rules"`
		IPs          []json.RawMessage `json:"ips"`
		ManagedRules map[string]struct {
			Active bool `json:"active"`
		} `json:"managedRules"`
	}
	if err := json.Unmarshal(raw, &cfg); err != nil {
		return nil, wrapAgent(err)
	}
	m := map[string]any{"enabled": cfg.FirewallEnabled}
	var active []string
	for _, r := range cfg.Rules {
		if r.Active && r.Name != "" {
			active = append(active, r.Name)
		}
	}
	if len(active) > 0 {
		m["custom_rules"] = active
	}
	if len(cfg.IPs) > 0 {
		m["ip_rules"] = len(cfg.IPs)
	}
	var managed []string
	for name, mr := range cfg.ManagedRules {
		if mr.Active {
			managed = append(managed, name)
		}
	}
	if len(managed) > 0 {
		m["managed_rulesets"] = managed
	}
	return m, nil
}

// compactAttackStatus projects the attack-status payload to a triage glance:
// whether anomalies were detected and how many.
func compactAttackStatus(raw json.RawMessage) (map[string]any, error) {
	var s struct {
		Anomalies []json.RawMessage `json:"anomalies"`
	}
	if err := json.Unmarshal(raw, &s); err != nil {
		return nil, wrapAgent(err)
	}
	return map[string]any{
		"under_attack": len(s.Anomalies) > 0,
		"anomalies":    len(s.Anomalies),
	}, nil
}
