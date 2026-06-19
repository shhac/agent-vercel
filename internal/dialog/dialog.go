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

	clidialog "github.com/shhac/lib-agent-cli/dialog"
)

// Secret opens a masked (password-style) entry dialog and returns the value the
// human typed. A cancelled dialog, or a headless host with no GUI, returns a
// non-nil error.
func Secret(title, prompt string) (string, error) {
	return clidialog.PromptSecret(context.Background(), title, prompt)
}
