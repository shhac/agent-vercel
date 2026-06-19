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
)

// Wrap preserves agent-vercel's nil-safety: a nil error wraps to a nil
// *APIError. (lib-agent-output's Wrap dereferences err unconditionally, so this
// guard is kept here — see migration notes.)
func Wrap(err error, fixableBy FixableBy) *APIError {
	if err == nil {
		return nil
	}
	return out.Wrap(err, fixableBy)
}

// As keeps the loose target signature the rest of the package expects.
func As(err error, target any) bool {
	return stderrors.As(err, target)
}
