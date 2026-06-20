// Package output re-exports the shared output contract from lib-agent-output,
// keeping the internal/output import path while the implementation lives in one
// place. YAML support comes from the shared lib-agent-cli/yaml encoder
// (blank-imported below for its registration side effect), so the yaml.v3
// dependency stays out of the dependency-free core. (Migration shim.)
package output

import (
	"io"

	out "github.com/shhac/lib-agent-output"

	_ "github.com/shhac/lib-agent-cli/yaml" // registers the FormatYAML encoder
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
