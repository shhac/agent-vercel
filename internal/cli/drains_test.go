package cli

import (
	"net/http/httptest"
	"testing"

	"github.com/shhac/agent-vercel/internal/mockvercel"
)

func TestDrainsList(t *testing.T) {
	srv := httptest.NewServer(mockvercel.New())
	defer srv.Close()

	out, _, err := execCLI(t, srv.URL, "drains", "list")
	if err != nil {
		t.Fatalf("err: %v", err)
	}
	rows := ndjsonLines(t, out)
	if len(rows) != 2 {
		t.Fatalf("want 2 drains, got %d: %s", len(rows), out)
	}
	if rows[0]["id"] != "drain_logs" || rows[0]["status"] != "enabled" {
		t.Fatalf("drain row = %v", rows[0])
	}
	types, ok := rows[0]["types"].([]any)
	if !ok || len(types) != 1 || types[0] != "log" {
		t.Fatalf("drain types = %v; want [log]", rows[0]["types"])
	}
	// the errored/disabled drain is surfaced as disabled:true
	if rows[1]["disabled"] != true || rows[1]["status"] != "errored" {
		t.Fatalf("errored drain = %v", rows[1])
	}
	// the delivery URL (with its token) is NOT in the compact projection
	for _, r := range rows {
		if _, leaked := r["delivery"]; leaked {
			t.Fatalf("compact drain should omit delivery: %v", r)
		}
	}
}

func TestDrainsListByProject(t *testing.T) {
	srv := httptest.NewServer(mockvercel.New())
	defer srv.Close()

	out, _, err := execCLI(t, srv.URL, "drains", "list", "--project", "prj_web")
	if err != nil {
		t.Fatalf("err: %v", err)
	}
	if rows := ndjsonLines(t, out); len(rows) != 1 || rows[0]["id"] != "drain_logs" {
		t.Fatalf("filtered drains = %s", out)
	}
}

func TestDrainsListEmpty(t *testing.T) {
	srv := httptest.NewServer(mockvercel.New())
	defer srv.Close()

	// no data: a project with no drains targeting it → empty list, not an error.
	out, _, err := execCLI(t, srv.URL, "drains", "list", "--project", "prj_api")
	if err != nil {
		t.Fatalf("err: %v", err)
	}
	if rows := ndjsonLines(t, out); len(rows) != 0 {
		t.Fatalf("expected no drains, got %d: %s", len(rows), out)
	}
}

func TestDrainsGet(t *testing.T) {
	srv := httptest.NewServer(mockvercel.New())
	defer srv.Close()

	out, _, err := execCLI(t, srv.URL, "drains", "get", "drain_traces")
	if err != nil {
		t.Fatalf("err: %v", err)
	}
	if m := decodeJSON(t, out); m["id"] != "drain_traces" || m["disabled"] != true {
		t.Fatalf("drain get = %v", m)
	}
}
