package cli

import (
	"net/http/httptest"
	"testing"

	"github.com/shhac/agent-vercel/internal/mockvercel"
)

func TestProjectCustomEnvironments(t *testing.T) {
	srv := httptest.NewServer(mockvercel.New())
	defer srv.Close()

	out, _, err := execCLI(t, srv.URL, "project", "custom-environments", "web")
	if err != nil {
		t.Fatalf("err: %v", err)
	}
	rows := ndjsonLines(t, out)
	if len(rows) != 2 {
		t.Fatalf("got %d rows, want 2: %s", len(rows), out)
	}
	if rows[0]["slug"] != "staging" || rows[0]["type"] != "preview" {
		t.Fatalf("row0 = %v", rows[0])
	}
	// branch binding is flattened to "matcherType:pattern"
	if rows[0]["branch_matcher"] != "startsWith:release/" {
		t.Fatalf("branch_matcher = %v", rows[0]["branch_matcher"])
	}
	domains, ok := rows[0]["domains"].([]any)
	if !ok || len(domains) != 1 || domains[0] != "staging.example.com" {
		t.Fatalf("domains = %v", rows[0]["domains"])
	}
	// the second env has no domains/description — those keys are pruned
	if _, has := rows[1]["domains"]; has {
		t.Fatalf("env without domains should omit the key: %v", rows[1])
	}
}

func TestProjectCustomEnvironmentsAlias(t *testing.T) {
	srv := httptest.NewServer(mockvercel.New())
	defer srv.Close()

	out, _, err := execCLI(t, srv.URL, "project", "custom-envs", "web")
	if err != nil {
		t.Fatalf("err: %v", err)
	}
	if len(ndjsonLines(t, out)) != 2 {
		t.Fatalf("alias should resolve to the same command: %s", out)
	}
}

func TestProjectCustomEnvironmentsFull(t *testing.T) {
	srv := httptest.NewServer(mockvercel.New())
	defer srv.Close()

	// --full returns the raw API objects (with the nested branchMatcher object),
	// not the compact projection's flattened "branch_matcher" string.
	out, _, err := execCLI(t, srv.URL, "project", "custom-environments", "web", "--full")
	if err != nil {
		t.Fatalf("err: %v", err)
	}
	rows := ndjsonLines(t, out)
	if len(rows) != 2 {
		t.Fatalf("got %d rows, want 2: %s", len(rows), out)
	}
	if _, projected := rows[0]["branch_matcher"]; projected {
		t.Fatalf("--full should not carry the compact branch_matcher key: %v", rows[0])
	}
	if _, raw := rows[0]["branchMatcher"]; !raw {
		t.Fatalf("--full should expose the raw branchMatcher object: %v", rows[0])
	}
}

func TestProjectCustomEnvironmentsEmpty(t *testing.T) {
	srv := httptest.NewServer(mockvercel.New(mockvercel.WithCustomEnvironments([]map[string]any{})))
	defer srv.Close()

	out, _, err := execCLI(t, srv.URL, "project", "custom-environments", "web")
	if err != nil {
		t.Fatalf("err: %v", err)
	}
	if rows := ndjsonLines(t, out); len(rows) != 0 {
		t.Fatalf("expected no rows for a project with no custom envs: %s", out)
	}
}
