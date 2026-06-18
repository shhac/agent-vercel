package cli

import (
	"net/http/httptest"
	"testing"

	"github.com/shhac/agent-vercel/internal/mockvercel"
)

func TestDomainProjects(t *testing.T) {
	srv := httptest.NewServer(mockvercel.New())
	defer srv.Close()

	out, _, err := execCLI(t, srv.URL, "domain", "projects", "example.com")
	if err != nil {
		t.Fatalf("err: %v", err)
	}
	rows := ndjsonLines(t, out)
	if len(rows) != 2 {
		t.Fatalf("want 2 project-domains, got %d: %s", len(rows), out)
	}
	if rows[0]["name"] != "example.com" || rows[0]["project_id"] != "prj_web" || rows[0]["verified"] != true {
		t.Fatalf("apex row = %v", rows[0])
	}
	// the www entry carries its redirect binding
	if rows[1]["redirect"] != "example.com" || rows[1]["redirect_status"].(float64) != 308 {
		t.Fatalf("www redirect = %v", rows[1])
	}
}

func TestDomainTransfer(t *testing.T) {
	srv := httptest.NewServer(mockvercel.New())
	defer srv.Close()

	out, _, err := execCLI(t, srv.URL, "domain", "transfer", "example.com")
	if err != nil {
		t.Fatalf("err: %v", err)
	}
	if m := decodeJSON(t, out); m["status"] != "pending_transfer" || m["transferable"] != false {
		t.Fatalf("transfer status = %v", m)
	}
}
