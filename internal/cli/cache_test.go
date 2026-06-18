package cli

import (
	"net/http/httptest"
	"testing"

	"github.com/shhac/agent-vercel/internal/mockvercel"
)

func TestCachePurgeWithYes(t *testing.T) {
	srv := httptest.NewServer(mockvercel.New())
	defer srv.Close()

	out, _, err := execCLI(t, srv.URL, "cache", "purge", "prj_web", "--tag", "products", "--tag", "home", "--yes")
	if err != nil {
		t.Fatalf("err: %v", err)
	}
	m := decodeJSON(t, out)
	tags, ok := m["purged"].([]any)
	if !ok || len(tags) != 2 || m["project"] != "prj_web" {
		t.Fatalf("cache purge = %v", m)
	}
}

func TestCachePurgeRequiresTag(t *testing.T) {
	srv := httptest.NewServer(mockvercel.New())
	defer srv.Close()

	// --yes given but no --tag → agent error (not the gate).
	_, errOut, err := execCLI(t, srv.URL, "cache", "purge", "prj_web", "--yes")
	if err == nil {
		t.Fatal("expected missing-tag error")
	}
	if m := decodeJSON(t, errOut); m["fixable_by"] != "agent" {
		t.Fatalf("missing --tag should be agent error: %v", m)
	}
}
