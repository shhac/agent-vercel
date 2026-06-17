package cli

import (
	"encoding/json"
	"io"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// execCLI runs the root command in-process against the given args, capturing
// stdout and stderr. It points credentials at a temp file and provides a token
// via env so commands authenticate without touching the real Keychain.
func execCLI(t *testing.T, baseURL string, args ...string) (stdout, stderr string, err error) {
	t.Helper()
	// Isolate creds/config from the real ~/.config, but only establish a temp
	// path if the test hasn't already set one — so a test's multiple execCLI
	// calls share state (the first call's temp dir is reused by later calls).
	if os.Getenv("AGENT_VERCEL_CREDENTIALS") == "" {
		t.Setenv("AGENT_VERCEL_CREDENTIALS", filepath.Join(t.TempDir(), "credentials.json"))
	}
	if os.Getenv("AGENT_VERCEL_CONFIG") == "" {
		t.Setenv("AGENT_VERCEL_CONFIG", filepath.Join(t.TempDir(), "config.json"))
	}
	t.Setenv("VERCEL_TOKEN", "test-token")

	full := append([]string{"--base-url", baseURL}, args...)

	oldOut, oldErr, oldArgs := os.Stdout, os.Stderr, os.Args
	rOut, wOut, _ := os.Pipe()
	rErr, wErr, _ := os.Pipe()
	os.Stdout, os.Stderr = wOut, wErr
	os.Args = append([]string{"agent-vercel"}, full...)

	runErr := Execute("test")

	_ = wOut.Close()
	_ = wErr.Close()
	os.Stdout, os.Stderr, os.Args = oldOut, oldErr, oldArgs
	ob, _ := io.ReadAll(rOut)
	eb, _ := io.ReadAll(rErr)
	return string(ob), string(eb), runErr
}

// ndjsonLines splits NDJSON stdout into decoded objects (meta lines included).
func ndjsonLines(t *testing.T, s string) []map[string]any {
	t.Helper()
	var out []map[string]any
	for _, line := range strings.Split(strings.TrimSpace(s), "\n") {
		if line == "" {
			continue
		}
		var m map[string]any
		if err := json.Unmarshal([]byte(line), &m); err != nil {
			t.Fatalf("bad NDJSON line %q: %v", line, err)
		}
		out = append(out, m)
	}
	return out
}

func decodeJSON(t *testing.T, s string) map[string]any {
	t.Helper()
	var m map[string]any
	if err := json.Unmarshal([]byte(s), &m); err != nil {
		t.Fatalf("bad JSON %q: %v", s, err)
	}
	return m
}
