package cli

import (
	"encoding/json"
	"net/url"
	"os"

	agenterrors "github.com/shhac/agent-vercel/internal/errors"
	"github.com/shhac/agent-vercel/internal/output"
)

// wrapAgent wraps a (usually decode) error as fixable_by: agent.
func wrapAgent(err error) error { return agenterrors.Wrap(err, agenterrors.FixableByAgent) }

// requireYes returns a fixable_by:human error describing a gated mutation when
// --yes was not passed. action describes what would happen; rerun is the exact
// command to rerun with --yes.
func requireYes(yes bool, action, rerun string) error {
	if yes {
		return nil
	}
	return agenterrors.Newf(agenterrors.FixableByHuman, "refusing to %s without confirmation", action).
		WithHint("rerun with --yes: " + rerun)
}

// setIf sets a query param only when val is non-empty.
func setIf(q url.Values, key, val string) {
	if val != "" {
		q.Set(key, val)
	}
}

// putIf sets a map entry only when val is non-empty.
func putIf(m map[string]any, key, val string) {
	if val != "" {
		m[key] = val
	}
}

// metaStr returns the first non-empty string value among the given meta keys.
func metaStr(meta map[string]any, keys ...string) string {
	for _, k := range keys {
		if v, ok := meta[k].(string); ok && v != "" {
			return v
		}
	}
	return ""
}

// compactRows maps raw API objects to output rows: the raw object when full is
// set, otherwise the compact projection produced by fn.
func compactRows(items []json.RawMessage, full bool, fn func(json.RawMessage) (map[string]any, error)) ([]any, error) {
	rows := make([]any, 0, len(items))
	for _, it := range items {
		if full {
			rows = append(rows, it)
			continue
		}
		m, err := fn(it)
		if err != nil {
			return nil, err
		}
		rows = append(rows, m)
	}
	return rows, nil
}

// bodyLimit resolves the effective max body length: the global --max-body-chars
// when set, else the per-command default. A negative value means unlimited.
func bodyLimit(g *GlobalFlags, def int) int {
	if g.MaxBodyChars != 0 {
		return g.MaxBodyChars
	}
	return def
}

// truncate shortens s to n runes with a "\n…" marker. n < 0 means unlimited.
func truncate(s string, n int) string {
	if n < 0 {
		return s
	}
	r := []rune(s)
	if len(r) <= n {
		return s
	}
	return string(r[:n]) + "\n…"
}

// printRaw prints a raw API payload in the resolved single-resource format
// (decoded so --format and null-pruning apply).
func printRaw(g *GlobalFlags, raw json.RawMessage) error {
	var v any
	if err := json.Unmarshal(raw, &v); err != nil {
		return agenterrors.Wrap(err, agenterrors.FixableByAgent)
	}
	format, err := output.ResolveFormat(g.Format, output.FormatJSON)
	if err != nil {
		return err
	}
	output.Print(os.Stdout, v, format, true)
	return nil
}
