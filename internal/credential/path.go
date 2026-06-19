package credential

import (
	"os"
	"path/filepath"

	"github.com/shhac/lib-agent-cli/xdg"
)

// configDirName follows the agent-* family convention: the plain tool name
// under XDG config. (agent-slack deviated to a reverse-DNS dir only to avoid
// colliding with the TS tool's file — there is no such collision here.)
const configDirName = "agent-vercel"

// defaultPath follows the agent-* family convention (per lin):
// $XDG_CONFIG_HOME, else ~/.config — on every platform, deliberately not
// os.UserConfigDir (which would scatter macOS state into
// ~/Library/Application Support).
func defaultPath() (string, error) {
	if env := os.Getenv("AGENT_VERCEL_CREDENTIALS"); env != "" {
		return env, nil
	}
	return filepath.Join(xdg.ConfigDir(configDirName), "credentials.json"), nil
}

// Path returns the credentials file path (for reporting, not secrets).
func (s *Store) Path() string { return s.path }

func isPlaceholder(v string) bool { return v == "" || v == keychainPlaceholder }

// IsPlaceholder reports whether v is empty or the keychain placeholder — i.e.
// not real secret material. Exported so callers (e.g. the CLI) can check a
// hydrated secret without re-encoding the placeholder sentinel themselves.
func IsPlaceholder(v string) bool { return isPlaceholder(v) }

// secretAccount is the Keychain account key for one credential's secret, keyed
// by "<type>:<label>" so several credentials (and, later, secret kinds) coexist
// under one service.
func secretAccount(a Auth) string { return string(a.normalizeType()) + ":" + a.Label }
