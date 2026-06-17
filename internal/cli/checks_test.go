package cli

import (
	"net/http/httptest"
	"testing"

	"github.com/shhac/agent-vercel/internal/mockvercel"
)

func TestDeploymentChecks(t *testing.T) {
	srv := httptest.NewServer(mockvercel.New())
	defer srv.Close()

	out, _, err := execCLI(t, srv.URL, "deployment", "checks", "dpl_err")
	if err != nil {
		t.Fatalf("err: %v", err)
	}
	rows := ndjsonLines(t, out)
	if len(rows) != 3 {
		t.Fatalf("got %d rows, want 3: %s", len(rows), out)
	}
	// compact projection surfaces conclusion + blocking; a still-running check
	// carries no conclusion key (empty values are pruned).
	if rows[1]["name"] != "E2E" || rows[1]["conclusion"] != "failed" || rows[1]["blocking"] != true {
		t.Fatalf("failed check row = %v", rows[1])
	}
	if _, hasConclusion := rows[2]["conclusion"]; hasConclusion {
		t.Fatalf("running check should omit conclusion: %v", rows[2])
	}
	if rows[2]["status"] != "running" {
		t.Fatalf("expected running status: %v", rows[2])
	}
}

func TestDeploymentChecksFailedOnly(t *testing.T) {
	srv := httptest.NewServer(mockvercel.New())
	defer srv.Close()

	out, _, err := execCLI(t, srv.URL, "deployment", "checks", "dpl_err", "--failed")
	if err != nil {
		t.Fatalf("err: %v", err)
	}
	rows := ndjsonLines(t, out)
	// Lint succeeded (dropped); E2E failed and Lighthouse is still running
	// (no conclusion → not a pass), so both remain.
	if len(rows) != 2 {
		t.Fatalf("got %d rows, want 2: %s", len(rows), out)
	}
	for _, r := range rows {
		if r["conclusion"] == "succeeded" {
			t.Fatalf("--failed should drop succeeded checks: %v", r)
		}
	}
}

func TestDeploymentChecksBlockingOnly(t *testing.T) {
	srv := httptest.NewServer(mockvercel.New())
	defer srv.Close()

	out, _, err := execCLI(t, srv.URL, "deployment", "checks", "dpl_err", "--blocking")
	if err != nil {
		t.Fatalf("err: %v", err)
	}
	rows := ndjsonLines(t, out)
	if len(rows) != 2 {
		t.Fatalf("got %d rows, want 2 blocking: %s", len(rows), out)
	}
	for _, r := range rows {
		if r["blocking"] != true {
			t.Fatalf("--blocking should keep only blocking checks: %v", r)
		}
	}
}

func TestDeploymentChecksAcceptsURL(t *testing.T) {
	srv := httptest.NewServer(mockvercel.New())
	defer srv.Close()

	// A pasted deployment URL is normalized to the host the API expects.
	out, _, err := execCLI(t, srv.URL, "deployment", "checks", "https://web-err.vercel.app/some/path")
	if err != nil {
		t.Fatalf("err: %v", err)
	}
	if len(ndjsonLines(t, out)) != 3 {
		t.Fatalf("url form should resolve to checks: %s", out)
	}
}
