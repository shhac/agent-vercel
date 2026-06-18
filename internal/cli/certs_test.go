package cli

import (
	"net/http/httptest"
	"testing"

	"github.com/shhac/agent-vercel/internal/mockvercel"
)

func TestDomainCerts(t *testing.T) {
	srv := httptest.NewServer(mockvercel.New())
	defer srv.Close()

	out, _, err := execCLI(t, srv.URL, "domain", "certs")
	if err != nil {
		t.Fatalf("err: %v", err)
	}
	rows := ndjsonLines(t, out)
	if len(rows) != 2 {
		t.Fatalf("got %d rows, want 2: %s", len(rows), out)
	}
	if rows[0]["id"] != "cert_1" || rows[0]["auto_renew"] != true {
		t.Fatalf("cert row = %v", rows[0])
	}
	covers, ok := rows[0]["covers"].([]any)
	if !ok || len(covers) != 2 {
		t.Fatalf("covers = %v", rows[0]["covers"])
	}
}

func TestDomainCertsExpiring(t *testing.T) {
	srv := httptest.NewServer(mockvercel.New())
	defer srv.Close()

	// --expiring 30 keeps only the already-expired cert_1; cert_2 (far future)
	// is dropped.
	out, _, err := execCLI(t, srv.URL, "domain", "certs", "--expiring", "30")
	if err != nil {
		t.Fatalf("err: %v", err)
	}
	rows := ndjsonLines(t, out)
	if len(rows) != 1 || rows[0]["id"] != "cert_1" {
		t.Fatalf("--expiring should keep only cert_1: %s", out)
	}
}
