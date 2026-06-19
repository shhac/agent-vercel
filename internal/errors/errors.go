// Package errors re-exports the shared error contract from lib-agent-output, so
// the rest of agent-vercel keeps using the internal/errors import path while the
// implementation lives in one place. (Migration shim — call sites can later be
// pointed at lib-agent-output directly and this package deleted.)
package errors

import (
	stderrors "errors"

	out "github.com/shhac/lib-agent-output"
)

type (
	FixableBy = out.FixableBy
	// APIError is the family name for the shared output.Error type.
	APIError = out.Error
)

const (
	FixableByAgent = out.FixableByAgent
	FixableByHuman = out.FixableByHuman
	FixableByRetry = out.FixableByRetry
)

var (
	New  = out.New
	Newf = out.Newf
	// Wrap delegates to lib-agent-output, which nil-guards err as of v0.4.3
	// (a nil err wraps to a nil *Error) — the nil-safety this package used to
	// keep locally.
	Wrap = out.Wrap
)

// As keeps the loose target signature the rest of the package expects.
func As(err error, target any) bool {
	return stderrors.As(err, target)
}
