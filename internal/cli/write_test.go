package cli

import (
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/shhac/agent-vercel/internal/mockvercel"
)

// gated commands must refuse without --yes and emit a human-fixable error that
// names the --yes rerun.
func TestWritesAreGatedWithoutYes(t *testing.T) {
	srv := httptest.NewServer(mockvercel.New())
	defer srv.Close()

	cases := [][]string{
		{"deployment", "promote", "dpl_ready"},
		{"deployment", "rollback", "dpl_ready"},
		{"deployment", "cancel", "dpl_ready"},
		{"deployment", "redeploy", "dpl_ready"},
		{"env", "set", "web", "KEY", "val"},
		{"env", "rm", "web", "API_URL"},
		{"domain", "add", "web", "new.example.com"},
		{"domain", "rm", "web", "example.com"},
		{"domain", "verify", "example.com", "--project", "web"},
		{"alias", "set", "dpl_ready", "app.example.com"},
		{"alias", "rm", "alias_1"},
	}
	for _, args := range cases {
		_, errOut, err := execCLI(t, srv.URL, args...)
		if err == nil {
			t.Fatalf("%v: expected gate error", args)
		}
		m := decodeJSON(t, errOut)
		if m["fixable_by"] != "human" {
			t.Fatalf("%v: gate should be human-fixable, got %v", args, m)
		}
		if h, _ := m["hint"].(string); !strings.Contains(h, "--yes") {
			t.Fatalf("%v: hint should name --yes, got %q", args, h)
		}
	}
}

func TestPromoteWithYes(t *testing.T) {
	srv := httptest.NewServer(mockvercel.New())
	defer srv.Close()

	out, _, err := execCLI(t, srv.URL, "deployment", "promote", "dpl_ready", "--yes")
	if err != nil {
		t.Fatalf("err: %v", err)
	}
	if m := decodeJSON(t, out); m["promoted"] != "dpl_ready" || m["project"] != "prj_web" {
		t.Fatalf("promote = %v", m)
	}
}

func TestCancelWithYes(t *testing.T) {
	srv := httptest.NewServer(mockvercel.New())
	defer srv.Close()

	out, _, err := execCLI(t, srv.URL, "deployment", "cancel", "dpl_ready", "--yes")
	if err != nil {
		t.Fatalf("err: %v", err)
	}
	if m := decodeJSON(t, out); m["canceled"] != "dpl_ready" {
		t.Fatalf("cancel = %v", m)
	}
}

func TestEnvRmResolvesIDWithYes(t *testing.T) {
	srv := httptest.NewServer(mockvercel.New())
	defer srv.Close()

	// API_URL has two entries (prod + preview); --environment narrows to one.
	out, _, err := execCLI(t, srv.URL, "env", "rm", "web", "API_URL", "--environment", "production", "--yes")
	if err != nil {
		t.Fatalf("err: %v", err)
	}
	if m := decodeJSON(t, out); m["removed"] != "API_URL" || m["id"] != "env_apiprod" {
		t.Fatalf("env rm = %v", m)
	}
}

func TestEnvRmAmbiguousIsAgentError(t *testing.T) {
	srv := httptest.NewServer(mockvercel.New())
	defer srv.Close()

	// without --environment, API_URL matches two entries → agent error
	_, errOut, err := execCLI(t, srv.URL, "env", "rm", "web", "API_URL", "--yes")
	if err == nil {
		t.Fatal("expected ambiguity error")
	}
	if m := decodeJSON(t, errOut); m["fixable_by"] != "agent" {
		t.Fatalf("ambiguous env rm should be agent error: %v", m)
	}
}

func TestDomainVerifyRequiresProject(t *testing.T) {
	srv := httptest.NewServer(mockvercel.New())
	defer srv.Close()

	// --yes given but no --project → agent error (not the gate)
	_, errOut, err := execCLI(t, srv.URL, "domain", "verify", "example.com", "--yes")
	if err == nil {
		t.Fatal("expected missing-project error")
	}
	if m := decodeJSON(t, errOut); m["fixable_by"] != "agent" {
		t.Fatalf("missing --project should be agent error: %v", m)
	}
}

func TestAliasSetWithYes(t *testing.T) {
	srv := httptest.NewServer(mockvercel.New())
	defer srv.Close()

	out, _, err := execCLI(t, srv.URL, "alias", "set", "dpl_ready", "app.example.com", "--yes")
	if err != nil {
		t.Fatalf("err: %v", err)
	}
	if m := decodeJSON(t, out); m["uid"] != "alias_new" {
		t.Fatalf("alias set = %v", m)
	}
}
