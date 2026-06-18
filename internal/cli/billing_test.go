package cli

import (
	"net/http/httptest"
	"testing"

	"github.com/shhac/agent-vercel/internal/mockvercel"
)

func TestBillingChargesList(t *testing.T) {
	srv := httptest.NewServer(mockvercel.New())
	defer srv.Close()

	out, _, err := execCLI(t, srv.URL, "billing", "charges")
	if err != nil {
		t.Fatalf("charges: %v", err)
	}
	rows := ndjsonLines(t, out)
	if len(rows) != 3 {
		t.Fatalf("want 3 charges, got %d: %s", len(rows), out)
	}
	if rows[0]["service"] != "Functions" || rows[0]["cost"] != 12.5 || rows[0]["project"] != "web" {
		t.Fatalf("charge row = %v", rows[0])
	}
}

func TestBillingChargesByService(t *testing.T) {
	srv := httptest.NewServer(mockvercel.New())
	defer srv.Close()

	out, _, err := execCLI(t, srv.URL, "billing", "charges", "--by", "service")
	if err != nil {
		t.Fatalf("by service: %v", err)
	}
	rows := ndjsonLines(t, out)
	// Bandwidth 40.00 should outrank Functions 12.50+3.00=15.50, sorted desc.
	if rows[0]["service"] != "Bandwidth" || rows[0]["cost"] != 40.0 {
		t.Fatalf("top service = %v; want Bandwidth 40", rows[0])
	}
	if rows[1]["service"] != "Functions" || rows[1]["cost"] != 15.5 || rows[1]["charges"] != float64(2) {
		t.Fatalf("functions agg = %v; want 15.5 over 2 charges", rows[1])
	}
}

func TestBillingChargesByProject(t *testing.T) {
	srv := httptest.NewServer(mockvercel.New())
	defer srv.Close()

	out, _, err := execCLI(t, srv.URL, "billing", "charges", "--by", "project")
	if err != nil {
		t.Fatalf("by project: %v", err)
	}
	rows := ndjsonLines(t, out)
	if rows[0]["project"] != "web" || rows[0]["cost"] != 52.5 {
		t.Fatalf("top project = %v; want web 52.5", rows[0])
	}
}

func TestBillingChargesByRegion(t *testing.T) {
	srv := httptest.NewServer(mockvercel.New())
	defer srv.Close()

	out, _, err := execCLI(t, srv.URL, "billing", "charges", "--by", "region")
	if err != nil {
		t.Fatalf("by region: %v", err)
	}
	rows := ndjsonLines(t, out)
	// iad1: 12.50+40.00=52.50 outranks sfo1: 3.00.
	if rows[0]["region"] != "iad1" || rows[0]["cost"] != 52.5 {
		t.Fatalf("top region = %v; want iad1 52.5", rows[0])
	}
}

func TestBillingUsageByService(t *testing.T) {
	srv := httptest.NewServer(mockvercel.New())
	defer srv.Close()

	out, _, err := execCLI(t, srv.URL, "billing", "usage")
	if err != nil {
		t.Fatalf("usage: %v", err)
	}
	rows := ndjsonLines(t, out)
	// Sorted by cost desc: Bandwidth (40) then Functions (15.5).
	if rows[0]["service"] != "Bandwidth" || rows[0]["consumed"] != 200.0 || rows[0]["unit"] != "GB" {
		t.Fatalf("top usage = %v; want Bandwidth 200 GB", rows[0])
	}
	// Functions consumed quantity sums across both charges: 1,000,000 + 50,000.
	if rows[1]["service"] != "Functions" || rows[1]["consumed"] != 1050000.0 || rows[1]["unit"] != "invocations" {
		t.Fatalf("functions usage = %v; want 1,050,000 invocations", rows[1])
	}
}

func TestBillingChargesBadBy(t *testing.T) {
	srv := httptest.NewServer(mockvercel.New())
	defer srv.Close()

	_, errOut, err := execCLI(t, srv.URL, "billing", "charges", "--by", "bogus")
	if err == nil {
		t.Fatal("expected error")
	}
	if m := decodeJSON(t, errOut); m["fixable_by"] != "agent" {
		t.Fatalf("bad --by should be agent error: %v", m)
	}
}
