// Package dialog provides a native OS prompt for secret entry, so a human can
// supply a token without it transiting the agent's conversation. It is the one
// sanctioned interactive surface in this otherwise LLM-only tool (family
// precedent: agent-slack, agent-posthog).
package dialog

import "github.com/ncruces/zenity"

// Secret opens a masked (password-style) entry dialog and returns the value the
// human typed. A cancelled/closed dialog returns zenity.ErrCanceled.
func Secret(title, prompt string) (string, error) {
	return zenity.Entry(prompt, zenity.Title(title), zenity.HideText())
}
