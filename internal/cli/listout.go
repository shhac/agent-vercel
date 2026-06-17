package cli

import (
	"os"
	"sort"
	"strconv"
	"time"

	"github.com/shhac/agent-vercel/internal/output"
)

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
