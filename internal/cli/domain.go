package cli

import (
	"encoding/json"
	"net/url"

	agenterrors "github.com/shhac/agent-vercel/internal/errors"
	"github.com/spf13/cobra"
)

func registerDomain(root *cobra.Command, g *GlobalFlags) {
	cmd := &cobra.Command{
		Use:   "domain",
		Short: "Inspect domains, DNS records, configuration, and certs",
		RunE:  func(c *cobra.Command, args []string) error { return handleUnknownSubcommand(c, args) },
	}

	var listCursor *string
	var listAll *bool
	list := &cobra.Command{
		Use:   "list",
		Short: "List domains in the scope",
		Args:  cobra.NoArgs,
		RunE: func(cmd *cobra.Command, _ []string) error {
			r, err := resolveClient(g)
			if err != nil {
				return err
			}
			items, next, err := fetchPaged(url.Values{}, *listCursor, *listAll, func(q url.Values) ([]json.RawMessage, *int64, error) {
				it, p, e := r.client.ListDomains(cmd.Context(), q)
				return it, p.Next, e
			})
			if err != nil {
				return err
			}
			rows, err := compactRows(items, g.Full, compactDomain)
			if err != nil {
				return err
			}
			return emitList(g, rows, paginationMeta(next))
		},
	}
	listCursor, listAll = addPageFlags(list)

	get := &cobra.Command{
		Use:   "get <domain>",
		Short: "Get one domain (verification, nameservers, verified state)",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			r, err := resolveClient(g)
			if err != nil {
				return err
			}
			raw, err := r.client.GetDomain(cmd.Context(), args[0])
			if err != nil {
				return err
			}
			return getOne(g, raw, compactDomain)
		},
	}

	inspect := &cobra.Command{
		Use:   "inspect <domain>",
		Short: "Configuration check: intended vs actual nameservers, misconfiguration",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
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
		},
	}

	var recCursor *string
	var recAll *bool
	records := &cobra.Command{
		Use:   "records <domain>",
		Short: "List DNS records for a domain",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			r, err := resolveClient(g)
			if err != nil {
				return err
			}
			items, next, err := fetchPaged(url.Values{}, *recCursor, *recAll, func(q url.Values) ([]json.RawMessage, *int64, error) {
				it, p, e := r.client.DomainRecords(cmd.Context(), args[0], q)
				return it, p.Next, e
			})
			if err != nil {
				return err
			}
			rows, err := compactRows(items, g.Full, compactRecord)
			if err != nil {
				return err
			}
			return emitList(g, rows, paginationMeta(next))
		},
	}
	recCursor, recAll = addPageFlags(records)

	cert := &cobra.Command{
		Use:   "cert <id>",
		Short: "Get a certificate (expiry, autoRenew, covered names)",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			r, err := resolveClient(g)
			if err != nil {
				return err
			}
			raw, err := r.client.GetCert(cmd.Context(), args[0])
			if err != nil {
				return err
			}
			return getOne(g, raw, compactCert)
		},
	}

	cmd.AddCommand(list, get, inspect, records, cert, domainAddCmd(g), domainRmCmd(g), domainVerifyCmd(g))
	root.AddCommand(cmd)
}

func domainAddCmd(g *GlobalFlags) *cobra.Command {
	var redirect, gitBranch string
	var yes *bool
	cmd := &cobra.Command{
		Use:   "add <project> <domain>",
		Short: "Add a domain to a project",
		Args:  cobra.ExactArgs(2),
		RunE: func(cmd *cobra.Command, args []string) error {
			project, domain := args[0], args[1]
			if err := requireYes(*yes, "add domain "+domain+" to "+project,
				"agent-vercel domain add "+project+" "+domain+" --yes"); err != nil {
				return err
			}
			r, err := resolveClient(g)
			if err != nil {
				return err
			}
			body := map[string]any{"name": domain}
			putIf(body, "redirect", redirect)
			putIf(body, "gitBranch", gitBranch)
			raw, err := r.client.AddProjectDomain(cmd.Context(), project, body)
			if err != nil {
				return err
			}
			return printRaw(g, raw)
		},
	}
	cmd.Flags().StringVar(&redirect, "redirect", "", "redirect target domain")
	cmd.Flags().StringVar(&gitBranch, "git-branch", "", "git branch to link the domain to")
	yes = addYesFlag(cmd)
	return cmd
}

func domainRmCmd(g *GlobalFlags) *cobra.Command {
	var yes *bool
	cmd := &cobra.Command{
		Use:   "rm <project> <domain>",
		Short: "Remove a domain from a project",
		Args:  cobra.ExactArgs(2),
		RunE: func(cmd *cobra.Command, args []string) error {
			project, domain := args[0], args[1]
			if err := requireYes(*yes, "remove domain "+domain+" from "+project,
				"agent-vercel domain rm "+project+" "+domain+" --yes"); err != nil {
				return err
			}
			r, err := resolveClient(g)
			if err != nil {
				return err
			}
			if _, err := r.client.RemoveProjectDomain(cmd.Context(), project, domain); err != nil {
				return err
			}
			return printSingle(g, map[string]any{"removed": domain, "project": project})
		},
	}
	yes = addYesFlag(cmd)
	return cmd
}

func domainVerifyCmd(g *GlobalFlags) *cobra.Command {
	var project string
	var yes *bool
	cmd := &cobra.Command{
		Use:   "verify <domain> --project <p>",
		Short: "Trigger verification of a project domain",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			domain := args[0]
			if project == "" {
				return agenterrors.New("--project is required", agenterrors.FixableByAgent).
					WithHint("pass --project <id|name>")
			}
			if err := requireYes(*yes, "verify domain "+domain+" on "+project,
				"agent-vercel domain verify "+domain+" --project "+project+" --yes"); err != nil {
				return err
			}
			r, err := resolveClient(g)
			if err != nil {
				return err
			}
			raw, err := r.client.VerifyProjectDomain(cmd.Context(), project, domain)
			if err != nil {
				return err
			}
			return printRaw(g, raw)
		},
	}
	cmd.Flags().StringVar(&project, "project", "", "project id or name (required)")
	yes = addYesFlag(cmd)
	return cmd
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
