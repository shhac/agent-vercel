// Package output re-exports the shared output contract from lib-agent-output,
// keeping the internal/output import path while the implementation lives in one
// place. The only thing that stays local is the YAML encoder (registered with
// lib-agent-output so the core stays dependency-free) and its number
// normalization. (Migration shim.)
package output

import (
	"bytes"
	"io"
	"math"

	out "github.com/shhac/lib-agent-output"
	"gopkg.in/yaml.v3"
)

type (
	Format       = out.Format
	Pagination   = out.Pagination
	NDJSONWriter = out.NDJSONWriter
)

const (
	FormatJSON   = out.FormatJSON
	FormatYAML   = out.FormatYAML
	FormatNDJSON = out.FormatNDJSON
)

var (
	ParseFormat     = out.ParseFormat
	ResolveFormat   = out.ResolveFormat
	NewNDJSONWriter = out.NewNDJSONWriter
	WriteError      = out.WriteError
)

// init registers agent-vercel's YAML encoder with lib-agent-output, so YAML
// support (and its yaml.v3 dependency) stays in this CLI while the core library
// remains dependency-free.
func init() {
	out.RegisterEncoder(out.FormatYAML, func(v any) ([]byte, error) {
		var buf bytes.Buffer
		enc := yaml.NewEncoder(&buf)
		enc.SetIndent(2)
		if err := enc.Encode(normalizeYAMLNumbers(v)); err != nil {
			return nil, err
		}
		_ = enc.Close()
		return buf.Bytes(), nil
	})
}

// Print writes data to w in the given format, optionally pruning nulls. It keeps
// this package's void signature; prune maps to PruneNils, the nil-only policy
// agent-vercel used.
func Print(w io.Writer, data any, format Format, prune bool) {
	var p out.Pruner
	if prune {
		p = out.PruneNils
	}
	_ = out.Print(w, data, format, p)
}

func normalizeYAMLNumbers(v any) any {
	return walkTree(v, func(leaf any) any {
		f, ok := leaf.(float64)
		if !ok || math.IsInf(f, 0) || math.IsNaN(f) || math.Trunc(f) != f {
			return leaf
		}
		return int64(f)
	})
}

// walkTree transforms each scalar (non-container) value of a decoded JSON tree
// via leaf. (Trimmed from the original now that pruning lives in lib-agent-output.)
func walkTree(v any, leaf func(any) any) any {
	switch val := v.(type) {
	case map[string]any:
		out := make(map[string]any, len(val))
		for k, child := range val {
			out[k] = walkTree(child, leaf)
		}
		return out
	case []any:
		out := make([]any, len(val))
		for i, child := range val {
			out[i] = walkTree(child, leaf)
		}
		return out
	default:
		return leaf(val)
	}
}
