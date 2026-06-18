package cli

import (
	"encoding/json"
	"net/url"
	"os"
	"sort"
	"strconv"
	"time"

	"github.com/shhac/agent-vercel/internal/output"
	"github.com/shhac/agent-vercel/internal/vercel"
	"github.com/spf13/cobra"
)

// allPagesCap bounds --all traversal so a runaway listing can't loop forever or
// flood; when hit, the remaining cursor is surfaced via @pagination so the agent
// knows the result was truncated.
const allPagesCap = 1000

// pageFunc fetches one page for a list endpoint, returning the page's items and
// the Vercel pagination block. A bound client list method (e.g.
// r.client.ListProjects) satisfies this directly — fetchPaged reads .Next.
type pageFunc func(q url.Values) ([]json.RawMessage, vercel.Page, error)

// fetchPaged drives a paginated list. With all=false it fetches one page
// (optionally starting at cursor) and returns that page's next cursor so the
// caller can emit @pagination. With all=true it follows next cursors until the
// listing is exhausted or allPagesCap is reached, returning a non-nil cursor
// only when the cap stopped it early. The Vercel pagination param is `until`.
func fetchPaged(q url.Values, cursor string, all bool, fetch pageFunc) ([]json.RawMessage, *int64, error) {
	if cursor != "" {
		q.Set("until", cursor) // --cursor overrides any --until time filter
	}
	var acc []json.RawMessage
	for {
		items, page, err := fetch(q)
		if err != nil {
			return nil, nil, err
		}
		acc = append(acc, items...)
		if !all {
			return acc, page.Next, nil
		}
		if page.Next == nil {
			return acc, nil, nil
		}
		if len(acc) >= allPagesCap {
			return acc, page.Next, nil
		}
		q.Set("until", strconv.FormatInt(*page.Next, 10))
	}
}

// emitPaged drives a paginated list command end to end: it pages through fetch,
// projects each item via compact (or passes the raw object under --full), and
// writes the rows with the "@pagination" trailer. It collapses the
// fetchPaged → compactRows → emitList(paginationMeta(...)) sequence that every
// plain paginated list command repeats; callers still pass the fetch closure
// (the client method legitimately varies) and the compact projection.
func emitPaged(g *GlobalFlags, q url.Values, cursor string, all bool, fetch pageFunc, compact func(json.RawMessage) (map[string]any, error)) error {
	items, next, err := fetchPaged(q, cursor, all, fetch)
	if err != nil {
		return err
	}
	rows, err := compactRows(items, g.Full, compact)
	if err != nil {
		return err
	}
	return emitList(g, rows, paginationMeta(next))
}

// emitRows projects a fetched (unpaginated) item slice and writes it with no
// pagination trailer — the compactRows → emitList(..., nil) tail shared by the
// list commands whose endpoint returns all items at once. Callers fetch (and
// optionally filter) items first, then hand them here.
func emitRows(g *GlobalFlags, items []json.RawMessage, compact func(json.RawMessage) (map[string]any, error)) error {
	rows, err := compactRows(items, g.Full, compact)
	if err != nil {
		return err
	}
	return emitList(g, rows, nil)
}

// emitOne resolves the client, fetches a single resource via fetch, and prints
// it through getOne (the raw payload under --full, else the compact projection).
// The single-resource counterpart to emitPaged/emitRows; the fetch closure keeps
// the varying client method at the call site.
func emitOne(g *GlobalFlags, fetch func(*vercel.Client) (json.RawMessage, error), compact func(json.RawMessage) (map[string]any, error)) error {
	r, err := resolveClient(g)
	if err != nil {
		return err
	}
	raw, err := fetch(r.client)
	if err != nil {
		return err
	}
	return getOne(g, raw, compact)
}

// emitRaw resolves the client, fetches a single resource via fetch, and prints
// it raw. The counterpart to emitOne for endpoints whose payload shape is
// variable/uncertain and has no compact projection (firewall bypass, project
// routes, domain transfer); --full is moot since the output is already raw.
func emitRaw(g *GlobalFlags, fetch func(*vercel.Client) (json.RawMessage, error)) error {
	r, err := resolveClient(g)
	if err != nil {
		return err
	}
	raw, err := fetch(r.client)
	if err != nil {
		return err
	}
	return printRaw(g, raw)
}

// addPageFlags registers the shared pagination flags on a list command,
// returning pointers to the cursor and all values.
func addPageFlags(cmd *cobra.Command) (cursor *string, all *bool) {
	cursor = cmd.Flags().String("cursor", "", "page cursor (the @pagination.next_cursor from a prior call)")
	all = cmd.Flags().Bool("all", false, "follow pagination and return all pages (capped)")
	return cursor, all
}

// emitList writes items as NDJSON (the default) with sorted trailing meta lines
// (e.g. "@pagination"). Under --format json|yaml it emits a single envelope
// {"data":[…], "@pagination":…} instead, so the output stays one document.
func emitList(g *GlobalFlags, items []any, meta map[string]any) error {
	format, err := output.ResolveFormat(g.Format, output.FormatNDJSON)
	if err != nil {
		return err
	}
	if format == output.FormatNDJSON {
		w := output.NewNDJSONWriter(os.Stdout)
		for _, it := range items {
			_ = w.WriteItem(it)
		}
		for _, k := range sortedKeys(meta) {
			_ = w.WriteMetaLine(k, meta[k])
		}
		return nil
	}
	envelope := map[string]any{"data": items}
	for k, v := range meta {
		envelope[k] = v
	}
	output.Print(os.Stdout, envelope, format, true)
	return nil
}

func sortedKeys(m map[string]any) []string {
	keys := make([]string, 0, len(m))
	for k := range m {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	return keys
}

// paginationMeta builds the family "@pagination" trailer from a Vercel
// timestamp cursor. next is the ms cursor for the following page (nil = no more).
func paginationMeta(next *int64) map[string]any {
	if next == nil {
		return nil
	}
	return map[string]any{
		"@pagination": map[string]any{
			"has_more":    true,
			"next_cursor": strconv.FormatInt(*next, 10),
		},
	}
}

// msToRFC3339 converts a Vercel epoch-millisecond timestamp to an RFC3339 UTC
// string. Zero/absent returns "".
func msToRFC3339(ms int64) string {
	if ms == 0 {
		return ""
	}
	return time.UnixMilli(ms).UTC().Format(time.RFC3339)
}
