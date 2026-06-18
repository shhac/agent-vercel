package cli

import (
	"net/http/httptest"
	"testing"

	"github.com/shhac/agent-vercel/internal/mockvercel"
)

func TestDomainCerts(t *testing.T) {
	srv := httptest.NewServer(mockvercel.New())
	defer srv.Close()

	out, _, err := execCLI(t, srv.URL, "domain", "cert", "list")
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

func TestDomainCertsFull(t *testing.T) {
	srv := httptest.NewServer(mockvercel.New())
	defer srv.Close()

	// --full returns raw cert objects (cns / expiresAt) instead of the compact
	// projection (covers / expires).
	out, _, err := execCLI(t, srv.URL, "domain", "cert", "list", "--full")
	if err != nil {
		t.Fatalf("err: %v", err)
	}
	rows := ndjsonLines(t, out)
	if _, projected := rows[0]["covers"]; projected {
		t.Fatalf("--full should not carry compact 'covers': %v", rows[0])
	}
	if _, raw := rows[0]["cns"]; !raw {
		t.Fatalf("--full should expose raw cns: %v", rows[0])
	}
}

func TestDomainCertsExpiring(t *testing.T) {
	srv := httptest.NewServer(mockvercel.New())
	defer srv.Close()

	// --expiring 30 keeps only the already-expired cert_1; cert_2 (far future)
	// is dropped.
	out, _, err := execCLI(t, srv.URL, "domain", "cert", "list", "--expiring", "30")
	if err != nil {
		t.Fatalf("err: %v", err)
	}
	rows := ndjsonLines(t, out)
	if len(rows) != 1 || rows[0]["id"] != "cert_1" {
		t.Fatalf("--expiring should keep only cert_1: %s", out)
	}
}
