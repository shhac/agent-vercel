package cli

import (
	"net/http/httptest"
	"testing"

	"github.com/shhac/agent-vercel/internal/mockvercel"
)

func TestDeploymentList(t *testing.T) {
	srv := httptest.NewServer(mockvercel.New())
	defer srv.Close()

	out, _, err := execCLI(t, srv.URL, "deployment", "list")
	if err != nil {
		t.Fatalf("err: %v", err)
	}
	rows := ndjsonLines(t, out)
	if len(rows) != 2 {
		t.Fatalf("got %d rows, want 2: %s", len(rows), out)
	}
	if rows[0]["id"] != "dpl_ready" || rows[0]["state"] != "READY" {
		t.Fatalf("row0 = %v", rows[0])
	}
	// compact projection surfaces git metadata
	if rows[0]["branch"] != "main" || rows[0]["sha"] != "abc123" {
		t.Fatalf("missing git meta: %v", rows[0])
	}
}

func TestDeploymentListStateFilter(t *testing.T) {
	srv := httptest.NewServer(mockvercel.New())
	defer srv.Close()

	out, _, err := execCLI(t, srv.URL, "deployment", "list", "--state", "ERROR")
	if err != nil {
		t.Fatalf("err: %v", err)
	}
	rows := ndjsonLines(t, out)
	if len(rows) != 1 || rows[0]["id"] != "dpl_err" {
		t.Fatalf("state filter wrong: %s", out)
	}
	if rows[0]["error_code"] != "BUILD_FAILED" {
		t.Fatalf("expected error_code on failed deploy: %v", rows[0])
	}
}

func TestDeploymentGet(t *testing.T) {
	srv := httptest.NewServer(mockvercel.New())
	defer srv.Close()

	out, _, err := execCLI(t, srv.URL, "deployment", "get", "dpl_ready")
	if err != nil {
		t.Fatalf("err: %v", err)
	}
	m := decodeJSON(t, out)
	if m["id"] != "dpl_ready" || m["target"] != "production" {
		t.Fatalf("get = %v", m)
	}
}

func TestDeploymentGetNotFoundIsAgentError(t *testing.T) {
	srv := httptest.NewServer(mockvercel.New())
	defer srv.Close()

	_, errOut, err := execCLI(t, srv.URL, "deployment", "get", "dpl_missing")
	if err == nil {
		t.Fatal("expected error")
	}
	m := decodeJSON(t, errOut)
	if m["fixable_by"] != "agent" {
		t.Fatalf("404 should map to agent: %v", m)
	}
}

func TestDeploymentCurrent(t *testing.T) {
	srv := httptest.NewServer(mockvercel.New())
	defer srv.Close()

	out, _, err := execCLI(t, srv.URL, "deployment", "current", "web")
	if err != nil {
		t.Fatalf("err: %v", err)
	}
	m := decodeJSON(t, out)
	live, ok := m["live"].(map[string]any)
	if !ok || live["id"] != "dpl_ready" {
		t.Fatalf("live deployment missing: %v", m)
	}
	if _, ok := m["rolling_release"]; !ok {
		t.Fatalf("rolling_release missing: %v", m)
	}
}

func TestProjectListAndGet(t *testing.T) {
	srv := httptest.NewServer(mockvercel.New())
	defer srv.Close()

	out, _, err := execCLI(t, srv.URL, "project", "list")
	if err != nil {
		t.Fatalf("list err: %v", err)
	}
	if rows := ndjsonLines(t, out); len(rows) != 2 || rows[0]["framework"] != "nextjs" {
		t.Fatalf("project list = %s", out)
	}

	out, _, err = execCLI(t, srv.URL, "project", "get", "web")
	if err != nil {
		t.Fatalf("get err: %v", err)
	}
	if m := decodeJSON(t, out); m["id"] != "prj_web" {
		t.Fatalf("project get = %v", m)
	}
}
