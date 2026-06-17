package cli

import (
	"errors"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/shhac/agent-vercel/internal/credential"
	"github.com/shhac/agent-vercel/internal/mockvercel"
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

	if _, _, err := execCLI(t, srv.URL, "config", "set", "cache.ttl", "30m"); err != nil {
		t.Fatalf("set: %v", err)
	}
	out, _, err := execCLI(t, srv.URL, "config", "get", "cache.ttl")
	if err != nil {
		t.Fatalf("get: %v", err)
	}
	if m := decodeJSON(t, out); m["value"] != "30m" {
		t.Fatalf("config get = %v", m)
	}
	if _, _, err := execCLI(t, srv.URL, "config", "unset", "cache.ttl"); err != nil {
		t.Fatalf("unset: %v", err)
	}
	if _, _, err := execCLI(t, srv.URL, "config", "get", "cache.ttl"); err == nil {
		t.Fatal("expected missing after unset")
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

func TestAuthAddFormCancelled(t *testing.T) {
	srv := httptest.NewServer(mockvercel.New())
	defer srv.Close()

	store := credential.NewWithStore(filepath.Join(t.TempDir(), "creds.json"), credential.NewMemoryKeychain())
	oldStore := newCredStore
	newCredStore = func() (*credential.Store, error) { return store, nil }
	t.Cleanup(func() { newCredStore = oldStore })
	oldPrompt := promptSecret
	promptSecret = func(_, _ string) (string, error) { return "", errors.New("cancelled") }
	t.Cleanup(func() { promptSecret = oldPrompt })

	_, errOut, err := execCLI(t, srv.URL, "auth", "add", "--form")
	if err == nil {
		t.Fatal("expected cancellation error")
	}
	m := decodeJSON(t, errOut)
	if m["fixable_by"] != "human" {
		t.Fatalf("cancel should be human-fixable: %v", m)
	}
	if h, _ := m["hint"].(string); !strings.Contains(h, "--form") {
		t.Fatalf("hint should name --form: %q", h)
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
