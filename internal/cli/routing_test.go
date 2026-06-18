package cli

import (
	"encoding/json"
	"net/http/httptest"
	"testing"

	"github.com/shhac/agent-vercel/internal/mockvercel"
)

func TestProjectProtection(t *testing.T) {
	srv := httptest.NewServer(mockvercel.New())
	defer srv.Close()

	out, _, err := execCLI(t, srv.URL, "project", "protection", "web")
	if err != nil {
		t.Fatalf("err: %v", err)
	}
	m := decodeJSON(t, out)
	if m["protected"] != true {
		t.Fatalf("expected protected: %v", m)
	}
	if m["vercel_authentication"] != "preview" {
		t.Fatalf("sso scope = %v; want preview", m["vercel_authentication"])
	}
	if m["password_protection"] != "all" {
		t.Fatalf("password gate = %v; want all", m["password_protection"])
	}
	tip, ok := m["trusted_ips"].(map[string]any)
	if !ok || tip["scope"] != "all" || tip["addresses"].(float64) != 1 {
		t.Fatalf("trusted_ips = %v", m["trusted_ips"])
	}
	// presence of an automation bypass is reported, never the secret itself
	if m["automation_bypass"] != true {
		t.Fatalf("automation_bypass should be true: %v", m)
	}
	if _, leaked := m["protectionBypass"]; leaked {
		t.Fatalf("must not surface the raw protectionBypass map: %v", m)
	}
}

func TestProjectProtectionNone(t *testing.T) {
	srv := httptest.NewServer(mockvercel.New())
	defer srv.Close()

	// prj_api has no protection configured.
	out, _, err := execCLI(t, srv.URL, "project", "protection", "api")
	if err != nil {
		t.Fatalf("err: %v", err)
	}
	if m := decodeJSON(t, out); m["protected"] != false {
		t.Fatalf("api should report protected:false: %v", m)
	}
}

func TestProjectRoutes(t *testing.T) {
	srv := httptest.NewServer(mockvercel.New())
	defer srv.Close()

	out, _, err := execCLI(t, srv.URL, "project", "routes", "web")
	if err != nil {
		t.Fatalf("err: %v", err)
	}
	m := decodeJSON(t, out)
	version, ok := m["version"].(map[string]any)
	if !ok || version["isLive"] != true {
		t.Fatalf("project routes version = %v", m["version"])
	}
	if _, ok := m["routes"].([]any); !ok {
		t.Fatalf("project routes should carry routes[]: %v", m)
	}
}

func TestDeploymentRoutes(t *testing.T) {
	srv := httptest.NewServer(mockvercel.New())
	defer srv.Close()

	out, _, err := execCLI(t, srv.URL, "deployment", "routes", "dpl_ready")
	if err != nil {
		t.Fatalf("err: %v", err)
	}
	// the compiled routes array is printed directly (a JSON array)
	var routes []any
	if err := json.Unmarshal([]byte(out), &routes); err != nil {
		t.Fatalf("routes output not a JSON array: %v (%s)", err, out)
	}
	if len(routes) != 2 {
		t.Fatalf("want 2 compiled routes, got %d: %s", len(routes), out)
	}
}
