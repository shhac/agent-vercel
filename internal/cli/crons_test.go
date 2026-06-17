package cli

import (
	"net/http/httptest"
	"testing"

	"github.com/shhac/agent-vercel/internal/mockvercel"
)

func TestProjectCrons(t *testing.T) {
	srv := httptest.NewServer(mockvercel.New())
	defer srv.Close()

	out, _, err := execCLI(t, srv.URL, "project", "crons", "web")
	if err != nil {
		t.Fatalf("err: %v", err)
	}
	m := decodeJSON(t, out)
	if m["project"] != "web" || m["enabled"] != true {
		t.Fatalf("crons header = %v", m)
	}
	if m["deployment_id"] != "dpl_ready" {
		t.Fatalf("expected deployment_id: %v", m)
	}
	jobs, ok := m["jobs"].([]any)
	if !ok || len(jobs) != 2 {
		t.Fatalf("expected 2 jobs: %v", m["jobs"])
	}
	first := jobs[0].(map[string]any)
	if first["path"] != "/api/cron/sync" || first["schedule"] != "0 5 * * *" {
		t.Fatalf("job projection = %v", first)
	}
}

func TestProjectCronsDisabled(t *testing.T) {
	srv := httptest.NewServer(mockvercel.New(mockvercel.WithProjectCrons(map[string]any{
		"crons": map[string]any{
			"enabledAt":   int64(1716200000000),
			"disabledAt":  int64(1716300000000),
			"definitions": []any{},
		},
	})))
	defer srv.Close()

	out, _, err := execCLI(t, srv.URL, "project", "crons", "web")
	if err != nil {
		t.Fatalf("err: %v", err)
	}
	m := decodeJSON(t, out)
	// A later disabledAt means crons are off, even though enabledAt is set.
	if m["enabled"] != false {
		t.Fatalf("expected enabled=false when disabledAt is set: %v", m)
	}
	jobs := m["jobs"].([]any)
	if len(jobs) != 0 {
		t.Fatalf("expected no jobs: %v", jobs)
	}
}
