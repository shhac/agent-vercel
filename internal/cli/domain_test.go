package cli

import (
	"net/http/httptest"
	"testing"

	"github.com/shhac/agent-vercel/internal/mockvercel"
)

func TestDomainListAndGet(t *testing.T) {
	srv := httptest.NewServer(mockvercel.New())
	defer srv.Close()

	out, _, err := execCLI(t, srv.URL, "domain", "list")
	if err != nil {
		t.Fatalf("list err: %v", err)
	}
	if rows := ndjsonLines(t, out); len(rows) != 1 || rows[0]["name"] != "example.com" {
		t.Fatalf("domain list = %s", out)
	}

	out, _, err = execCLI(t, srv.URL, "domain", "get", "example.com")
	if err != nil {
		t.Fatalf("get err: %v", err)
	}
	m := decodeJSON(t, out)
	if m["name"] != "example.com" || m["verified"] != true {
		t.Fatalf("domain get = %v", m)
	}
}

func TestDomainInspect(t *testing.T) {
	srv := httptest.NewServer(mockvercel.New())
	defer srv.Close()

	out, _, err := execCLI(t, srv.URL, "domain", "inspect", "example.com")
	if err != nil {
		t.Fatalf("err: %v", err)
	}
	m := decodeJSON(t, out)
	if m["misconfigured"] != true {
		t.Fatalf("expected misconfigured: %v", m)
	}
	if _, ok := m["intended_nameservers"]; !ok {
		t.Fatalf("inspect should fold in intended_nameservers: %v", m)
	}
}

func TestDomainRecords(t *testing.T) {
	srv := httptest.NewServer(mockvercel.New())
	defer srv.Close()

	out, _, err := execCLI(t, srv.URL, "domain", "records", "example.com")
	if err != nil {
		t.Fatalf("err: %v", err)
	}
	rows := ndjsonLines(t, out)
	if len(rows) != 2 || rows[0]["type"] != "A" {
		t.Fatalf("records = %s", out)
	}
}

func TestDomainCert(t *testing.T) {
	srv := httptest.NewServer(mockvercel.New())
	defer srv.Close()

	out, _, err := execCLI(t, srv.URL, "domain", "cert", "cert_1")
	if err != nil {
		t.Fatalf("err: %v", err)
	}
	m := decodeJSON(t, out)
	if m["id"] != "cert_1" || m["auto_renew"] != true {
		t.Fatalf("cert = %v", m)
	}
}

func TestAliasListSurfacesProtection(t *testing.T) {
	srv := httptest.NewServer(mockvercel.New())
	defer srv.Close()

	out, _, err := execCLI(t, srv.URL, "alias", "list", "dpl_ready")
	if err != nil {
		t.Fatalf("err: %v", err)
	}
	rows := ndjsonLines(t, out)
	if len(rows) != 2 {
		t.Fatalf("want 2 aliases, got %d: %s", len(rows), out)
	}
	// the second alias carries protection bypass state
	if _, ok := rows[1]["protection_bypass"]; !ok {
		t.Fatalf("protection_bypass should be surfaced: %v", rows[1])
	}
}
