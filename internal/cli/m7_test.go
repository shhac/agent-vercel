package cli

import (
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/shhac/agent-vercel/internal/credential"
	"github.com/shhac/agent-vercel/internal/mockvercel"
	clidialog "github.com/shhac/lib-agent-cli/dialog"
)

func TestAPICallGetUngated(t *testing.T) {
	srv := httptest.NewServer(mockvercel.New())
	defer srv.Close()

	out, _, err := execCLI(t, srv.URL, "api", "call", "GET", "/v2/user")
	if err != nil {
		t.Fatalf("err: %v", err)
	}
	m := decodeJSON(t, out)
	user, ok := m["user"].(map[string]any)
	if !ok || user["username"] != "acme-bot" {
		t.Fatalf("api call = %v", m)
	}
}

func TestAPICallNonGetGated(t *testing.T) {
	srv := httptest.NewServer(mockvercel.New())
	defer srv.Close()

	_, errOut, err := execCLI(t, srv.URL, "api", "call", "DELETE", "/v2/aliases/alias_1")
	if err == nil {
		t.Fatal("expected gate")
	}
	if m := decodeJSON(t, errOut); m["fixable_by"] != "human" {
		t.Fatalf("non-GET api call should be gated: %v", m)
	}
}

func TestConfigRoundTrip(t *testing.T) {
	srv := httptest.NewServer(mockvercel.New())
	defer srv.Close()
	t.Setenv("AGENT_VERCEL_CONFIG", filepath.Join(t.TempDir(), "config.json"))

	if _, _, err := execCLI(t, srv.URL, "config", "set", "max-body-chars", "5000"); err != nil {
		t.Fatalf("set: %v", err)
	}
	out, _, err := execCLI(t, srv.URL, "config", "get", "max-body-chars")
	if err != nil {
		t.Fatalf("get: %v", err)
	}
	if rows := ndjsonLines(t, out); len(rows) != 1 || rows[0]["value"] != "5000" {
		t.Fatalf("config get = %v", out)
	}
	if _, _, err := execCLI(t, srv.URL, "config", "unset", "max-body-chars"); err != nil {
		t.Fatalf("unset: %v", err)
	}
	// After unset, config get emits @unresolved on stdout, exit 0.
	unsetOut, _, err2 := execCLI(t, srv.URL, "config", "get", "max-body-chars")
	if err2 != nil {
		t.Fatalf("config get after unset should exit 0 (got @unresolved): %v", err2)
	}
	if rows := ndjsonLines(t, unsetOut); len(rows) != 1 || rows[0]["@unresolved"] == nil {
		t.Fatalf("expected @unresolved for missing config key: %v", unsetOut)
	}
}

func TestConfigDefaultsAppliedAndRejected(t *testing.T) {
	srv := httptest.NewServer(mockvercel.New())
	defer srv.Close()
	t.Setenv("AGENT_VERCEL_CONFIG", filepath.Join(t.TempDir(), "config.json"))

	// unknown key and auth/scope keys are rejected with agent errors
	for _, k := range []string{"frmat", "scope", "auth"} {
		if _, errOut, err := execCLI(t, srv.URL, "config", "set", k, "x"); err == nil {
			t.Fatalf("config set %q should fail", k)
		} else if m := decodeJSON(t, errOut); m["fixable_by"] != "agent" {
			t.Fatalf("config set %q: %v", k, m)
		}
	}

	// a stored format default applies, and an explicit --format overrides it
	if _, _, err := execCLI(t, srv.URL, "config", "set", "format", "yaml"); err != nil {
		t.Fatalf("set format: %v", err)
	}
	out, _, err := execCLI(t, srv.URL, "scope", "current")
	if err != nil {
		t.Fatalf("scope current: %v", err)
	}
	if !strings.Contains(out, "default_scope:") || strings.HasPrefix(strings.TrimSpace(out), "{") {
		t.Fatalf("config format=yaml should make output YAML, got: %s", out)
	}
	out, _, err = execCLI(t, srv.URL, "--format", "json", "scope", "current")
	if err != nil {
		t.Fatalf("override: %v", err)
	}
	if !strings.HasPrefix(strings.TrimSpace(out), "{") {
		t.Fatalf("--format json should override config yaml, got: %s", out)
	}
}

func TestAuthAddForm(t *testing.T) {
	srv := httptest.NewServer(mockvercel.New())
	defer srv.Close()

	store := credential.NewWithStore(filepath.Join(t.TempDir(), "creds.json"), credential.NewMemoryKeychain())
	oldStore := newCredStore
	newCredStore = func() (*credential.Store, error) { return store, nil }
	t.Cleanup(func() { newCredStore = oldStore })
	oldPrompt := promptSecret
	promptSecret = func(_, _ string) (string, error) { return "dialog-token", nil }
	t.Cleanup(func() { promptSecret = oldPrompt })

	out, _, err := execCLI(t, srv.URL, "auth", "add", "viaform", "--form")
	if err != nil {
		t.Fatalf("add --form: %v", err)
	}
	if m := decodeJSON(t, out); m["stored"] != true || m["label"] != "viaform" {
		t.Fatalf("add --form = %v", m)
	}
	creds, _ := store.Load()
	if len(creds.Auths) != 1 || creds.Auths[0].Secret != "dialog-token" {
		t.Fatalf("token not stored via dialog: %+v", creds.Auths)
	}
	raw, _ := os.ReadFile(store.Path())
	if strings.Contains(string(raw), "dialog-token") {
		t.Fatalf("dialog token leaked to file:\n%s", raw)
	}
}

func TestAuthAddCapturesIdentity(t *testing.T) {
	srv := httptest.NewServer(mockvercel.New())
	defer srv.Close()

	store := credential.NewWithStore(filepath.Join(t.TempDir(), "creds.json"), credential.NewMemoryKeychain())
	oldStore := newCredStore
	newCredStore = func() (*credential.Store, error) { return store, nil }
	t.Cleanup(func() { newCredStore = oldStore })

	// env-token path (execCLI sets VERCEL_TOKEN); add verifies against the mock
	out, _, err := execCLI(t, srv.URL, "auth", "add", "cap")
	if err != nil {
		t.Fatalf("add: %v", err)
	}
	if m := decodeJSON(t, out); m["verified"] != true || m["username"] != "acme-bot" {
		t.Fatalf("add should verify and capture username: %v", m)
	}
	// username is persisted, so `auth list` shows it without a separate auth test
	creds, _ := store.Load()
	if len(creds.Auths) != 1 || creds.Auths[0].Username != "acme-bot" {
		t.Fatalf("username not captured at add time: %+v", creds.Auths)
	}
}

func TestAuthAddFormCancelled(t *testing.T) {
	srv := httptest.NewServer(mockvercel.New())
	defer srv.Close()

	store := credential.NewWithStore(filepath.Join(t.TempDir(), "creds.json"), credential.NewMemoryKeychain())
	oldStore := newCredStore
	newCredStore = func() (*credential.Store, error) { return store, nil }
	t.Cleanup(func() { newCredStore = oldStore })
	oldPrompt := promptSecret
	promptSecret = func(_, _ string) (string, error) { return "", clidialog.ErrCancelled }
	t.Cleanup(func() { promptSecret = oldPrompt })

	_, errOut, err := execCLI(t, srv.URL, "auth", "add", "--form")
	if err == nil {
		t.Fatal("expected cancellation error")
	}
	m := decodeJSON(t, errOut)
	// As of lib-agent-cli v0.4.0 a user-cancelled dialog is a retry (re-running
	// pops the prompt again), classified via dialog.Classify.
	if m["fixable_by"] != "retry" {
		t.Fatalf("cancel should be retry-fixable: %v", m)
	}
	if h, _ := m["hint"].(string); h == "" {
		t.Fatalf("cancel should carry a hint: %v", m)
	}
}

func TestAuthImportCLI(t *testing.T) {
	srv := httptest.NewServer(mockvercel.New())
	defer srv.Close()
	dir := t.TempDir()
	authFile := filepath.Join(dir, "auth.json")
	if err := os.WriteFile(authFile, []byte(`{"token":"imported-tok"}`), 0o600); err != nil {
		t.Fatal(err)
	}
	t.Setenv("AGENT_VERCEL_CLI_AUTH", authFile)
	t.Setenv("AGENT_VERCEL_CREDENTIALS", filepath.Join(dir, "credentials.json"))

	out, _, err := execCLI(t, srv.URL, "auth", "import-cli", "fromcli")
	if err != nil {
		t.Fatalf("import-cli: %v", err)
	}
	m := decodeJSON(t, out)
	if m["label"] != "fromcli" || m["stored"] != true {
		t.Fatalf("import-cli = %v", m)
	}
}

func TestDomainUsageSubcommand(t *testing.T) {
	srv := httptest.NewServer(mockvercel.New())
	defer srv.Close()

	out, _, err := execCLI(t, srv.URL, "deployment", "usage")
	if err != nil {
		t.Fatalf("usage: %v", err)
	}
	if !strings.Contains(out, "deployment list") || !strings.Contains(out, "deployment promote") {
		t.Fatalf("domain usage missing entries:\n%s", out)
	}
}
