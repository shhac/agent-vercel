package cli

import (
	"encoding/json"
	"net/url"

	"github.com/spf13/cobra"
)

func registerDomain(root *cobra.Command, g *GlobalFlags) {
	cmd := &cobra.Command{
		Use:   "domain",
		Short: "Inspect domains, DNS records, configuration, and certs",
		RunE:  func(c *cobra.Command, args []string) error { return handleUnknownSubcommand(c, args) },
	}

	list := &cobra.Command{
		Use:   "list",
		Short: "List domains in the scope",
		Args:  cobra.NoArgs,
		RunE: func(cmd *cobra.Command, _ []string) error {
			return run(func() error {
				r, err := resolveClient(g)
				if err != nil {
					return err
				}
				items, page, err := r.client.ListDomains(cmd.Context(), url.Values{})
				if err != nil {
					return err
				}
				rows, err := compactRows(items, g.Full, compactDomain)
				if err != nil {
					return err
				}
				return emitList(g, rows, paginationMeta(page.Next))
			})
		},
	}

	get := &cobra.Command{
		Use:   "get <domain>",
		Short: "Get one domain (verification, nameservers, verified state)",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			return run(func() error {
				r, err := resolveClient(g)
				if err != nil {
					return err
				}
				raw, err := r.client.GetDomain(cmd.Context(), args[0])
				if err != nil {
					return err
				}
				if g.Full {
					return printRaw(g, raw)
				}
				m, err := compactDomain(raw)
				if err != nil {
					return err
				}
				return printSingle(g, m)
			})
		},
	}

	inspect := &cobra.Command{
		Use:   "inspect <domain>",
		Short: "Configuration check: intended vs actual nameservers, misconfiguration",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			return run(func() error {
				r, err := resolveClient(g)
				if err != nil {
					return err
				}
				cfg, err := r.client.DomainConfig(cmd.Context(), args[0])
				if err != nil {
					return err
				}
				if g.Full {
					return printRaw(g, cfg)
				}
				out := map[string]any{"domain": args[0]}
				var c struct {
					Misconfigured bool `json:"misconfigured"`
				}
				_ = json.Unmarshal(cfg, &c)
				out["misconfigured"] = c.Misconfigured
				// The actionable bit — intended vs actual nameservers, verified —
				// lives on the domain record; fold it in best-effort.
				if raw, err := r.client.GetDomain(cmd.Context(), args[0]); err == nil {
					var d rawDomain
					if json.Unmarshal(raw, &d) == nil {
						out["verified"] = d.Verified
						if len(d.Nameservers) > 0 {
							out["nameservers"] = d.Nameservers
						}
						if len(d.IntendedNameservers) > 0 {
							out["intended_nameservers"] = d.IntendedNameservers
						}
					}
				}
				return printSingle(g, out)
			})
		},
	}

	records := &cobra.Command{
		Use:   "records <domain>",
		Short: "List DNS records for a domain",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			return run(func() error {
				r, err := resolveClient(g)
				if err != nil {
					return err
				}
				items, page, err := r.client.DomainRecords(cmd.Context(), args[0], url.Values{})
				if err != nil {
					return err
				}
				rows, err := compactRows(items, g.Full, compactRecord)
				if err != nil {
					return err
				}
				return emitList(g, rows, paginationMeta(page.Next))
			})
		},
	}

	cert := &cobra.Command{
		Use:   "cert <id>",
		Short: "Get a certificate (expiry, autoRenew, covered names)",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			return run(func() error {
				r, err := resolveClient(g)
				if err != nil {
					return err
				}
				raw, err := r.client.GetCert(cmd.Context(), args[0])
				if err != nil {
					return err
				}
				if g.Full {
					return printRaw(g, raw)
				}
				m, err := compactCert(raw)
				if err != nil {
					return err
				}
				return printSingle(g, m)
			})
		},
	}

	cmd.AddCommand(list, get, inspect, records, cert)
	root.AddCommand(cmd)
}

type rawDomain struct {
	Name                string   `json:"name"`
	Verified            bool     `json:"verified"`
	ServiceType         string   `json:"serviceType"`
	Nameservers         []string `json:"nameservers"`
	IntendedNameservers []string `json:"intendedNameservers"`
	ExpiresAt           int64    `json:"expiresAt"`
	Renew               bool     `json:"renew"`
}

func compactDomain(raw json.RawMessage) (map[string]any, error) {
	var d rawDomain
	if err := json.Unmarshal(raw, &d); err != nil {
		return nil, wrapAgent(err)
	}
	m := map[string]any{"name": d.Name, "verified": d.Verified, "renew": d.Renew}
	putIf(m, "service_type", d.ServiceType)
	if len(d.IntendedNameservers) > 0 {
		m["intended_nameservers"] = d.IntendedNameservers
	}
	if len(d.Nameservers) > 0 {
		m["nameservers"] = d.Nameservers
	}
	putIf(m, "expires", msToRFC3339(d.ExpiresAt))
	return m, nil
}

func compactRecord(raw json.RawMessage) (map[string]any, error) {
	var rec struct {
		ID    string `json:"id"`
		Type  string `json:"type"`
		Name  string `json:"name"`
		Value string `json:"value"`
		TTL   int    `json:"ttl"`
	}
	if err := json.Unmarshal(raw, &rec); err != nil {
		return nil, wrapAgent(err)
	}
	m := map[string]any{"id": rec.ID, "type": rec.Type, "name": rec.Name, "value": rec.Value}
	if rec.TTL != 0 {
		m["ttl"] = rec.TTL
	}
	return m, nil
}

func compactCert(raw json.RawMessage) (map[string]any, error) {
	var c struct {
		ID        string   `json:"id"`
		CreatedAt int64    `json:"createdAt"`
		ExpiresAt int64    `json:"expiresAt"`
		AutoRenew bool     `json:"autoRenew"`
		CNS       []string `json:"cns"`
	}
	if err := json.Unmarshal(raw, &c); err != nil {
		return nil, wrapAgent(err)
	}
	m := map[string]any{"id": c.ID, "auto_renew": c.AutoRenew}
	putIf(m, "created", msToRFC3339(c.CreatedAt))
	putIf(m, "expires", msToRFC3339(c.ExpiresAt))
	if len(c.CNS) > 0 {
		m["covers"] = c.CNS
	}
	return m, nil
}
