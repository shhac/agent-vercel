package cli

import (
	"context"
	"encoding/json"
	"net/url"
	"time"

	"github.com/shhac/agent-vercel/internal/vercel"
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
			return emitPaged(g, url.Values{}, *listCursor, *listAll, func(q url.Values) ([]json.RawMessage, vercel.Page, error) {
				return r.client.ListDomains(cmd.Context(), q)
			}, compactDomain)
		},
	}
	listCursor, listAll = addPageFlags(list)

	get := &cobra.Command{
		Use:   "get <domain>...",
		Short: "Get one or more domains (verification, nameservers, verified state)",
		Args:  cobra.MinimumNArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			return GetEntities(g, cmd.Context(), args, func(ctx context.Context, c *vercel.Client, id string) (any, error) {
				raw, err := c.GetDomain(ctx, id)
				if err != nil {
					return nil, err
				}
				return resolveRawAsAny(g, raw, compactDomain)
			})
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
			out := compactDomainConfig(args[0], cfg)
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

	projects := &cobra.Command{
		Use:   "projects <domain>",
		Short: "List the projects a domain (apex) is attached to (wrong-project / conflict triage)",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			r, err := resolveClient(g)
			if err != nil {
				return err
			}
			items, err := r.client.ProjectDomainsByApex(cmd.Context(), args[0])
			if err != nil {
				return err
			}
			return emitRows(g, items, compactProjectDomain)
		},
	}

	transfer := &cobra.Command{
		Use:   "transfer <domain>",
		Short: "Show a domain's registration / transfer status (registrar)",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			// small registrar status object; shape varies — print raw
			return emitRaw(g, func(c *vercel.Client) (json.RawMessage, error) {
				return c.DomainTransfer(cmd.Context(), args[0])
			})
		},
	}

	cmd.AddCommand(list, get, inspect, domainRecordsCmd(g), domainCertCmd(g), projects, transfer, domainAddCmd(g), domainRmCmd(g), domainVerifyCmd(g))
	root.AddCommand(cmd)
}

// domainCertCmd is the `domain cert` group: list the scope's TLS certificates
// (bulk expiry triage) and get one by id.
func domainCertCmd(g *GlobalFlags) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "cert",
		Short: "Inspect TLS certificates (list for bulk expiry triage, or get one)",
		RunE:  func(c *cobra.Command, args []string) error { return handleUnknownSubcommand(c, args) },
	}
	cmd.AddCommand(domainCertListCmd(g), domainCertGetCmd(g))
	return cmd
}

func domainCertListCmd(g *GlobalFlags) *cobra.Command {
	var expiring int
	cmd := &cobra.Command{
		Use:   "list",
		Short: "List the scope's TLS certificates and their expiry (bulk renewal triage)",
		Args:  cobra.NoArgs,
		RunE: func(cmd *cobra.Command, _ []string) error {
			r, err := resolveClient(g)
			if err != nil {
				return err
			}
			items, err := r.client.ListCerts(cmd.Context(), nil)
			if err != nil {
				return err
			}
			if cmd.Flags().Changed("expiring") {
				items = filterExpiringCerts(items, expiring)
			}
			return emitRows(g, items, compactCert)
		},
	}
	cmd.Flags().IntVar(&expiring, "expiring", 0, "only certs expiring within this many days (0 = already expired/expiring today)")
	return cmd
}

func domainCertGetCmd(g *GlobalFlags) *cobra.Command {
	return &cobra.Command{
		Use:   "get <id>...",
		Short: "Get one or more certificates (expiry, autoRenew, covered names)",
		Args:  cobra.MinimumNArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			return GetEntities(g, cmd.Context(), args, func(ctx context.Context, c *vercel.Client, id string) (any, error) {
				raw, err := c.GetCert(ctx, id)
				if err != nil {
					return nil, err
				}
				return resolveRawAsAny(g, raw, compactCert)
			})
		},
	}
}

// filterExpiringCerts keeps certs whose expiry is at or before now + days
// (days may be 0 to surface only already-expired/expiring-today certs),
// client-side — the certs endpoint has no expiry filter.
func filterExpiringCerts(items []json.RawMessage, days int) []json.RawMessage {
	cutoff := time.Now().AddDate(0, 0, days).UnixMilli()
	out := make([]json.RawMessage, 0, len(items))
	for _, raw := range items {
		var c struct {
			ExpiresAt int64 `json:"expiresAt"`
		}
		if json.Unmarshal(raw, &c) != nil || c.ExpiresAt == 0 {
			continue
		}
		if c.ExpiresAt <= cutoff {
			out = append(out, raw)
		}
	}
	return out
}

// domainRecordsCmd is the `domain records` group: list/add/rm DNS records.
// It's a group (not a leaf) to keep DNS-record add/rm distinct from the
// project-domain `domain add`/`rm`.
func domainRecordsCmd(g *GlobalFlags) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "records",
		Short: "List, add, or remove a domain's DNS records",
		RunE:  func(c *cobra.Command, args []string) error { return handleUnknownSubcommand(c, args) },
	}

	var cursor *string
	var all *bool
	list := &cobra.Command{
		Use:   "list <domain>",
		Short: "List DNS records for a domain",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			r, err := resolveClient(g)
			if err != nil {
				return err
			}
			return emitPaged(g, url.Values{}, *cursor, *all, func(q url.Values) ([]json.RawMessage, vercel.Page, error) {
				return r.client.DomainRecords(cmd.Context(), args[0], q)
			}, compactRecord)
		},
	}
	cursor, all = addPageFlags(list)

	var ttl int
	var yes *bool
	add := &cobra.Command{
		Use:   "add <domain> <type> <name> <value>",
		Short: "Add a DNS record (type e.g. A, AAAA, CNAME, TXT, MX)",
		Args:  cobra.ExactArgs(4),
		RunE: func(cmd *cobra.Command, args []string) error {
			domain, recType, name, value := args[0], args[1], args[2], args[3]
			r, err := confirmAndClient(g, *yes, "add "+recType+" record on "+domain,
				"agent-vercel domain records add "+domain+" "+recType+" "+name+" <value> --yes")
			if err != nil {
				return err
			}
			body := map[string]any{"type": recType, "name": name, "value": value}
			if ttl > 0 {
				body["ttl"] = ttl
			}
			raw, err := r.client.CreateDNSRecord(cmd.Context(), domain, body)
			if err != nil {
				return err
			}
			return printRaw(g, raw)
		},
	}
	add.Flags().IntVar(&ttl, "ttl", 0, "record TTL in seconds (omit for Vercel default)")
	yes = addYesFlag(add)

	var rmYes *bool
	rm := &cobra.Command{
		Use:   "rm <domain> <record-id>",
		Short: "Remove a DNS record by id",
		Args:  cobra.ExactArgs(2),
		RunE: func(cmd *cobra.Command, args []string) error {
			domain, recID := args[0], args[1]
			r, err := confirmAndClient(g, *rmYes, "remove DNS record "+recID+" from "+domain,
				"agent-vercel domain records rm "+domain+" "+recID+" --yes")
			if err != nil {
				return err
			}
			if _, err := r.client.DeleteDNSRecord(cmd.Context(), domain, recID); err != nil {
				return err
			}
			return printSingle(g, map[string]any{"removed": recID, "domain": domain})
		},
	}
	rmYes = addYesFlag(rm)

	cmd.AddCommand(list, add, rm)
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

// compactDomainConfig projects a domain's /config payload for `domain inspect`:
// the misconfiguration flag plus SSL/ACME readiness (configuredBy,
// acceptedChallenges — empty ⇒ a cert cannot be issued — and the recommended
// IPv4/CNAME remediation values). Best-effort: a malformed payload yields the
// zero-value projection rather than failing the command (the caller folds in
// nameserver state from the domain record separately).
func compactDomainConfig(domain string, cfg json.RawMessage) map[string]any {
	var c struct {
		Misconfigured      bool            `json:"misconfigured"`
		ConfiguredBy       string          `json:"configuredBy"`
		AcceptedChallenges []string        `json:"acceptedChallenges"`
		RecommendedIPv4    json.RawMessage `json:"recommendedIPv4"`
		RecommendedCNAME   json.RawMessage `json:"recommendedCNAME"`
	}
	_ = json.Unmarshal(cfg, &c)
	out := map[string]any{"domain": domain, "misconfigured": c.Misconfigured}
	putIf(out, "configured_by", c.ConfiguredBy)
	if len(c.AcceptedChallenges) > 0 {
		out["accepted_challenges"] = c.AcceptedChallenges
	}
	if len(c.RecommendedIPv4) > 0 && string(c.RecommendedIPv4) != "null" {
		out["recommended_ipv4"] = c.RecommendedIPv4
	}
	if len(c.RecommendedCNAME) > 0 && string(c.RecommendedCNAME) != "null" {
		out["recommended_cname"] = c.RecommendedCNAME
	}
	return out
}

// compactProjectDomain projects one entry of the apex→project reverse map: the
// domain name, the project it is on, verified state, and any redirect binding.
func compactProjectDomain(raw json.RawMessage) (map[string]any, error) {
	var d struct {
		Name               string `json:"name"`
		ProjectID          string `json:"projectId"`
		Verified           bool   `json:"verified"`
		Redirect           string `json:"redirect"`
		RedirectStatusCode int    `json:"redirectStatusCode"`
		GitBranch          string `json:"gitBranch"`
	}
	if err := json.Unmarshal(raw, &d); err != nil {
		return nil, wrapAgent(err)
	}
	m := map[string]any{"name": d.Name, "project_id": d.ProjectID, "verified": d.Verified}
	putIf(m, "redirect", d.Redirect)
	if d.RedirectStatusCode != 0 {
		m["redirect_status"] = d.RedirectStatusCode
	}
	putIf(m, "git_branch", d.GitBranch)
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
