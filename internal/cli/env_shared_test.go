package cli

import (
	"net/http/httptest"
	"testing"

	"github.com/shhac/agent-vercel/internal/mockvercel"
)

func TestEnvSharedList(t *testing.T) {
	srv := httptest.NewServer(mockvercel.New())
	defer srv.Close()

	out, _, err := execCLI(t, srv.URL, "env", "shared", "list")
	if err != nil {
		t.Fatalf("err: %v", err)
	}
	rows := ndjsonLines(t, out)
	if len(rows) != 2 {
		t.Fatalf("got %d rows, want 2: %s", len(rows), out)
	}
	if rows[0]["key"] != "DATABASE_URL" || rows[0]["type"] != "encrypted" {
		t.Fatalf("row0 = %v", rows[0])
	}
	// the linked projects are surfaced so an agent sees the blast radius
	projects, ok := rows[0]["projects"].([]any)
	if !ok || len(projects) != 2 {
		t.Fatalf("projects = %v", rows[0]["projects"])
	}
	// value is withheld unless --decrypt
	if _, has := rows[0]["value"]; has {
		t.Fatalf("value must be hidden without --decrypt: %v", rows[0])
	}
}

func TestEnvSharedListDecrypt(t *testing.T) {
	srv := httptest.NewServer(mockvercel.New())
	defer srv.Close()

	out, _, err := execCLI(t, srv.URL, "env", "shared", "list", "--decrypt")
	if err != nil {
		t.Fatalf("err: %v", err)
	}
	rows := ndjsonLines(t, out)
	if rows[0]["value"] != "postgres://shared" {
		t.Fatalf("--decrypt should surface the value: %v", rows[0])
	}
}

func TestEnvSharedGetByKey(t *testing.T) {
	srv := httptest.NewServer(mockvercel.New())
	defer srv.Close()

	out, _, err := execCLI(t, srv.URL, "env", "shared", "get", "FEATURE_X")
	if err != nil {
		t.Fatalf("err: %v", err)
	}
	m := decodeJSON(t, out)
	if m["id"] != "env_shared_flag" || m["key"] != "FEATURE_X" {
		t.Fatalf("shared get = %v", m)
	}
}

func TestEnvSharedGetNotFound(t *testing.T) {
	srv := httptest.NewServer(mockvercel.New())
	defer srv.Close()

	_, errOut, err := execCLI(t, srv.URL, "env", "shared", "get", "NOPE")
	if err == nil {
		t.Fatalf("expected not-found error")
	}
	m := decodeJSON(t, errOut)
	if m["fixable_by"] != "agent" {
		t.Fatalf("expected fixable_by agent, got %v", m)
	}
}
