package cli

import (
	"encoding/json"
	"net/url"
	"os"
	"sort"
	"strconv"
	"time"

	"github.com/shhac/agent-vercel/internal/output"
	"github.com/spf13/cobra"
)

// allPagesCap bounds --all traversal so a runaway listing can't loop forever or
// flood; when hit, the remaining cursor is surfaced via @pagination so the agent
// knows the result was truncated.
const allPagesCap = 1000

// pageFunc fetches one page for a list endpoint and returns the page's items and
// the next cursor (nil when there are no more pages).
type pageFunc func(q url.Values) ([]json.RawMessage, *int64, error)

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
		items, next, err := fetch(q)
		if err != nil {
			return nil, nil, err
		}
		acc = append(acc, items...)
		if !all {
			return acc, next, nil
		}
		if next == nil {
			return acc, nil, nil
		}
		if len(acc) >= allPagesCap {
			return acc, next, nil
		}
		q.Set("until", strconv.FormatInt(*next, 10))
	}
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
