package cli

import (
	"net/http/httptest"
	"strings"
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

func TestDeploymentChecksBlockingAndFailed(t *testing.T) {
	srv := httptest.NewServer(mockvercel.New())
	defer srv.Close()

	// Both flags together intersect: only a check that is blocking AND not
	// passing survives — the top triage question "what's both blocking me and
	// actually broken". The E2E check is the only one matching both.
	out, _, err := execCLI(t, srv.URL, "deployment", "checks", "dpl_err", "--blocking", "--failed")
	if err != nil {
		t.Fatalf("err: %v", err)
	}
	rows := ndjsonLines(t, out)
	if len(rows) != 1 || rows[0]["name"] != "E2E" {
		t.Fatalf("blocking+failed should yield only E2E: %s", out)
	}
}

func TestDeploymentChecksEmpty(t *testing.T) {
	srv := httptest.NewServer(mockvercel.New(mockvercel.WithDeploymentChecks([]map[string]any{})))
	defer srv.Close()

	// A deployment with no checks attached must succeed with empty output, not
	// error — common for simple deploys with no CI integration.
	out, _, err := execCLI(t, srv.URL, "deployment", "checks", "dpl_ready")
	if err != nil {
		t.Fatalf("err: %v", err)
	}
	if strings.TrimSpace(out) != "" {
		t.Fatalf("expected empty output, got: %q", out)
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
