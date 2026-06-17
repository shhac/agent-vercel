package cli

import (
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"

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

func TestCacheWarmAndInfo(t *testing.T) {
	srv := httptest.NewServer(mockvercel.New())
	defer srv.Close()
	t.Setenv("AGENT_VERCEL_CACHE", filepath.Join(t.TempDir(), "cache.json"))

	out, _, err := execCLI(t, srv.URL, "cache", "warm")
	if err != nil {
		t.Fatalf("warm: %v", err)
	}
	if m := decodeJSON(t, out); m["scopes"] != float64(2) || m["projects"] != float64(2) {
		t.Fatalf("warm = %v", m)
	}
	out, _, err = execCLI(t, srv.URL, "cache", "info")
	if err != nil {
		t.Fatalf("info: %v", err)
	}
	if m := decodeJSON(t, out); m["projects"] != float64(2) {
		t.Fatalf("info = %v", m)
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

	out, _, err := execCLI(t, srv.URL, "auth", "import-cli", "--label", "fromcli")
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
