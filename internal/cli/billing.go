package cli

import (
	"encoding/json"
	"sort"
	"time"

	agenterrors "github.com/shhac/agent-vercel/internal/errors"
	"github.com/spf13/cobra"
)

func registerBilling(root *cobra.Command, g *GlobalFlags) {
	cmd := &cobra.Command{
		Use:   "billing",
		Short: "Inspect billing/usage charges (what is driving spend)",
		RunE:  func(c *cobra.Command, args []string) error { return handleUnknownSubcommand(c, args) },
	}

	var from, to, by string
	charges := &cobra.Command{
		Use:   "charges",
		Short: "List billing/usage charges over a date range, or aggregate by service/project",
		Args:  cobra.NoArgs,
		RunE: func(cmd *cobra.Command, _ []string) error {
			now := time.Now()
			fromISO, err := toISO(from, now.AddDate(0, 0, -30))
			if err != nil {
				return err
			}
			toISOStr, err := toISO(to, now)
			if err != nil {
				return err
			}
			r, err := resolveClient(g)
			if err != nil {
				return err
			}
			items, err := r.client.BillingCharges(cmd.Context(), fromISO, toISOStr)
			if err != nil {
				return err
			}
			if by != "" {
				charges := make([]charge, 0, len(items))
				for _, raw := range items {
					c, err := parseCharge(raw)
					if err != nil {
						return err
					}
					charges = append(charges, c)
				}
				rows, err := aggregateCharges(charges, by)
				if err != nil {
					return err
				}
				return emitList(g, rows, nil)
			}
			return emitRows(g, items, compactCharge)
		},
	}
	f := charges.Flags()
	f.StringVar(&from, "from", "", "start of range: date (2006-01-02), RFC3339, or duration like 30d (default 30d ago)")
	f.StringVar(&to, "to", "", "end of range (default now)")
	f.StringVar(&by, "by", "", "aggregate billed cost by: service | project")

	cmd.AddCommand(charges)
	root.AddCommand(cmd)
}

// charge is the compact view of a FOCUS billing charge.
type charge struct {
	Service     string
	Category    string
	Project     string
	Consumed    float64
	Unit        string
	BilledCost  float64
	Currency    string
	PeriodStart string
	PeriodEnd   string
}

func parseCharge(raw json.RawMessage) (charge, error) {
	var c struct {
		BilledCost        float64        `json:"BilledCost"`
		BillingCurrency   string         `json:"BillingCurrency"`
		ChargeCategory    string         `json:"ChargeCategory"`
		ConsumedQuantity  float64        `json:"ConsumedQuantity"`
		ConsumedUnit      string         `json:"ConsumedUnit"`
		ServiceName       string         `json:"ServiceName"`
		ChargePeriodStart string         `json:"ChargePeriodStart"`
		ChargePeriodEnd   string         `json:"ChargePeriodEnd"`
		Tags              map[string]any `json:"Tags"`
	}
	if err := json.Unmarshal(raw, &c); err != nil {
		return charge{}, wrapAgent(err)
	}
	project, _ := c.Tags["ProjectName"].(string)
	if project == "" {
		project, _ = c.Tags["ProjectId"].(string)
	}
	return charge{
		Service: c.ServiceName, Category: c.ChargeCategory, Project: project,
		Consumed: c.ConsumedQuantity, Unit: c.ConsumedUnit,
		BilledCost: c.BilledCost, Currency: c.BillingCurrency,
		PeriodStart: c.ChargePeriodStart, PeriodEnd: c.ChargePeriodEnd,
	}, nil
}

// compactCharge is the free-function projection used on the list path, matching
// the package's canonical compactX(json.RawMessage) shape so the charges list
// flows through compactRows/emitRows like every other resource.
func compactCharge(raw json.RawMessage) (map[string]any, error) {
	c, err := parseCharge(raw)
	if err != nil {
		return nil, err
	}
	return c.compact(), nil
}

func (c charge) compact() map[string]any {
	m := map[string]any{"service": c.Service, "cost": c.BilledCost}
	putIf(m, "category", c.Category)
	putIf(m, "project", c.Project)
	putIf(m, "currency", c.Currency)
	if c.Unit != "" {
		m["consumed"] = c.Consumed
		m["unit"] = c.Unit
	}
	putIf(m, "period_start", c.PeriodStart)
	putIf(m, "period_end", c.PeriodEnd)
	return m
}

// aggregateCharges sums billed cost grouped by service or project, sorted by
// cost descending — the "what is driving spend" answer.
func aggregateCharges(charges []charge, by string) ([]any, error) {
	key := func(c charge) string { return c.Service }
	switch by {
	case "service":
	case "project":
		key = func(c charge) string {
			if c.Project == "" {
				return "(unattributed)"
			}
			return c.Project
		}
	default:
		return nil, agenterrors.Newf(agenterrors.FixableByAgent, "unknown --by %q; use service or project", by)
	}
	type agg struct {
		cost     float64
		count    int
		currency string
	}
	groups := map[string]*agg{}
	var order []string
	for _, c := range charges {
		k := key(c)
		a, ok := groups[k]
		if !ok {
			a = &agg{currency: c.Currency}
			groups[k] = a
			order = append(order, k)
		}
		a.cost += c.BilledCost
		a.count++
	}
	sort.Slice(order, func(i, j int) bool { return groups[order[i]].cost > groups[order[j]].cost })
	rows := make([]any, 0, len(order))
	for _, k := range order {
		a := groups[k]
		row := map[string]any{by: k, "cost": a.cost, "charges": a.count}
		putIf(row, "currency", a.currency)
		rows = append(rows, row)
	}
	return rows, nil
}

// toISO converts a date (2006-01-02), RFC3339, or relative duration (30d) into an
// RFC3339 UTC string; an empty input uses def.
func toISO(s string, def time.Time) (string, error) {
	if s == "" {
		return def.UTC().Format(time.RFC3339), nil
	}
	t, ok := parseUserTime(s)
	if !ok {
		return "", agenterrors.Newf(agenterrors.FixableByAgent, "invalid date %q; use a date (2006-01-02), RFC3339, or a duration like 30d", s)
	}
	return t.UTC().Format(time.RFC3339), nil
}
