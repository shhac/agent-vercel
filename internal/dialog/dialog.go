// Package dialog provides a native OS prompt for secret entry, so a human can
// supply a token without it transiting the agent's conversation. It is the one
// sanctioned interactive surface in this otherwise LLM-only tool (family
// precedent: agent-slack, agent-posthog).
//
// It now delegates to lib-agent-cli/dialog; this thin wrapper keeps the
// Secret(title, prompt) signature the auth command uses. (Migration shim.)
package dialog

import (
	"context"

	"github.com/shhac/agent-vercel/internal/errors"
	clidialog "github.com/shhac/lib-agent-cli/dialog"
)

// Secret opens a masked (password-style) entry dialog and returns the value the
// human typed. A cancelled dialog, or a headless host with no GUI, returns a
// non-nil error — neutral, so callers classify it via Classify.
func Secret(title, prompt string) (string, error) {
	return clidialog.PromptSecret(context.Background(), title, prompt)
}

// Classify maps a dialog error onto agent-vercel's fixable_by taxonomy and a
// hint. As of lib-agent-cli v0.4.0 dialog errors are neutral; this is the one
// place that translates the library's Category into our error contract: a
// user-cancelled dialog is a retry, a headless host is a human problem, and
// anything else is the agent's to correct.
func Classify(err error) (errors.FixableBy, string) {
	cat, hint := clidialog.ClassifyError(err)
	switch cat {
	case clidialog.CategoryRetry:
		return errors.FixableByRetry, hint
	case clidialog.CategoryHuman:
		return errors.FixableByHuman, hint
	default:
		return errors.FixableByAgent, hint
	}
}
