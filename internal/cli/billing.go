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
		Short: "List billing/usage charges over a date range, or aggregate cost by service/project/region",
		Args:  cobra.NoArgs,
		RunE: func(cmd *cobra.Command, _ []string) error {
			items, err := fetchCharges(g, cmd, from, to)
			if err != nil {
				return err
			}
			if by != "" {
				charges, err := parseCharges(items)
				if err != nil {
					return err
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
	f.StringVar(&by, "by", "", "aggregate billed cost by: service | project | region")

	var ufrom, uto string
	usage := &cobra.Command{
		Use:   "consumption",
		Short: "Aggregate consumed quantity by service (volume + unit, not just $) — what resource is the spike",
		Args:  cobra.NoArgs,
		RunE: func(cmd *cobra.Command, _ []string) error {
			items, err := fetchCharges(g, cmd, ufrom, uto)
			if err != nil {
				return err
			}
			charges, err := parseCharges(items)
			if err != nil {
				return err
			}
			return emitList(g, aggregateUsage(charges), nil)
		},
	}
	uf := usage.Flags()
	uf.StringVar(&ufrom, "from", "", "start of range: date (2006-01-02), RFC3339, or duration like 30d (default 30d ago)")
	uf.StringVar(&uto, "to", "", "end of range (default now)")

	cmd.AddCommand(charges, usage)
	root.AddCommand(cmd)
}

// fetchCharges resolves the date range (defaults: 30d ago → now) and the client,
// then pulls the FOCUS billing charges — shared by `charges` and `usage`.
func fetchCharges(g *GlobalFlags, cmd *cobra.Command, from, to string) ([]json.RawMessage, error) {
	now := time.Now()
	fromISO, err := toISO(from, now.AddDate(0, 0, -30))
	if err != nil {
		return nil, err
	}
	toISOStr, err := toISO(to, now)
	if err != nil {
		return nil, err
	}
	r, err := resolveClient(g)
	if err != nil {
		return nil, err
	}
	return r.client.BillingCharges(cmd.Context(), fromISO, toISOStr)
}

func parseCharges(items []json.RawMessage) ([]charge, error) {
	charges := make([]charge, 0, len(items))
	for _, raw := range items {
		c, err := parseCharge(raw)
		if err != nil {
			return nil, err
		}
		charges = append(charges, c)
	}
	return charges, nil
}

// charge is the compact view of a FOCUS billing charge.
type charge struct {
	Service     string
	Category    string
	Project     string
	Region      string
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
		RegionID          string         `json:"RegionId"`
		RegionName        string         `json:"RegionName"`
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
		Region:   firstNonEmpty(c.RegionName, c.RegionID),
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
// chargeAgg accumulates one group's totals. It is the union of what both the
// cost (`charges --by`) and consumption (`consumption`) views need; each view's
// projection reads only the fields it cares about.
type chargeAgg struct {
	consumed float64
	cost     float64
	count    int
	unit     string
	currency string
}

// groupCharges buckets charges by keyFn, summing consumed quantity and billed
// cost, and returns the keys in cost-descending order alongside their totals —
// the shared skeleton behind both aggregate views. unit/currency are captured
// from each group's first member.
func groupCharges(charges []charge, keyFn func(charge) string) ([]string, map[string]*chargeAgg) {
	groups := map[string]*chargeAgg{}
	var order []string
	for _, c := range charges {
		k := keyFn(c)
		a, ok := groups[k]
		if !ok {
			a = &chargeAgg{unit: c.Unit, currency: c.Currency}
			groups[k] = a
			order = append(order, k)
		}
		a.consumed += c.Consumed
		a.cost += c.BilledCost
		a.count++
	}
	sort.Slice(order, func(i, j int) bool { return groups[order[i]].cost > groups[order[j]].cost })
	return order, groups
}

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
	case "region":
		key = func(c charge) string {
			if c.Region == "" {
				return "(global)"
			}
			return c.Region
		}
	default:
		return nil, agenterrors.Newf(agenterrors.FixableByAgent, "unknown --by %q; use service, project, or region", by)
	}
	order, groups := groupCharges(charges, key)
	rows := make([]any, 0, len(order))
	for _, k := range order {
		a := groups[k]
		row := map[string]any{by: k, "cost": a.cost, "charges": a.count}
		putIf(row, "currency", a.currency)
		rows = append(rows, row)
	}
	return rows, nil
}

// aggregateUsage groups charges by service, summing consumed quantity (the unit
// is consistent within a service) alongside billed cost — the "what resource,
// in what volume, is driving the spike" view. Sorted by cost descending.
func aggregateUsage(charges []charge) []any {
	order, groups := groupCharges(charges, func(c charge) string { return c.Service })
	rows := make([]any, 0, len(order))
	for _, k := range order {
		a := groups[k]
		row := map[string]any{"service": k, "consumed": a.consumed, "cost": a.cost, "charges": a.count}
		putIf(row, "unit", a.unit)
		putIf(row, "currency", a.currency)
		rows = append(rows, row)
	}
	return rows
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
