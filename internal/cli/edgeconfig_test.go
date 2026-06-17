package cli

import (
	"net/http/httptest"
	"testing"

	"github.com/shhac/agent-vercel/internal/mockvercel"
)

func TestEdgeConfigList(t *testing.T) {
	srv := httptest.NewServer(mockvercel.New())
	defer srv.Close()

	out, _, err := execCLI(t, srv.URL, "edge-config", "list")
	if err != nil {
		t.Fatalf("err: %v", err)
	}
	rows := ndjsonLines(t, out)
	if len(rows) != 2 {
		t.Fatalf("got %d rows, want 2: %s", len(rows), out)
	}
	if rows[0]["id"] != "ecfg_flags" || rows[0]["slug"] != "flags" {
		t.Fatalf("row0 = %v", rows[0])
	}
	// item_count is always present (even 0); size_bytes is omitted when absent
	if rows[0]["item_count"].(float64) != 2 || rows[0]["size_bytes"].(float64) != 128 {
		t.Fatalf("counts = %v", rows[0])
	}
	if rows[1]["item_count"].(float64) != 0 {
		t.Fatalf("empty config should still report item_count 0: %v", rows[1])
	}
	if _, has := rows[1]["size_bytes"]; has {
		t.Fatalf("absent size should omit size_bytes: %v", rows[1])
	}
}

func TestEdgeConfigItems(t *testing.T) {
	srv := httptest.NewServer(mockvercel.New())
	defer srv.Close()

	out, _, err := execCLI(t, srv.URL, "edge-config", "items", "ecfg_flags")
	if err != nil {
		t.Fatalf("err: %v", err)
	}
	rows := ndjsonLines(t, out)
	if len(rows) != 2 {
		t.Fatalf("got %d rows, want 2: %s", len(rows), out)
	}
	if rows[0]["key"] != "maintenance_mode" || rows[0]["value"] != false {
		t.Fatalf("scalar item = %v", rows[0])
	}
	// a structured value is preserved as-is
	obj, ok := rows[1]["value"].(map[string]any)
	if !ok || obj["enabled"] != true {
		t.Fatalf("object item = %v", rows[1]["value"])
	}
}

func TestEdgeConfigItemsEmpty(t *testing.T) {
	srv := httptest.NewServer(mockvercel.New())
	defer srv.Close()

	// A config with no items returns empty output, not an error.
	out, _, err := execCLI(t, srv.URL, "edge-config", "items", "ecfg_redirects")
	if err != nil {
		t.Fatalf("err: %v", err)
	}
	if rows := ndjsonLines(t, out); len(rows) != 0 {
		t.Fatalf("expected no items: %s", out)
	}
}

func TestEdgeConfigListFull(t *testing.T) {
	srv := httptest.NewServer(mockvercel.New())
	defer srv.Close()

	// --full returns raw Edge Config objects (camelCase sizeInBytes) instead of
	// the compact snake_case projection.
	out, _, err := execCLI(t, srv.URL, "edge-config", "list", "--full")
	if err != nil {
		t.Fatalf("err: %v", err)
	}
	rows := ndjsonLines(t, out)
	if len(rows) != 2 {
		t.Fatalf("got %d rows, want 2: %s", len(rows), out)
	}
	if _, projected := rows[0]["item_count"]; projected {
		t.Fatalf("--full should not carry compact item_count: %v", rows[0])
	}
	if _, raw := rows[0]["itemCount"]; !raw {
		t.Fatalf("--full should expose raw itemCount: %v", rows[0])
	}
}

func TestEdgeConfigAlias(t *testing.T) {
	srv := httptest.NewServer(mockvercel.New())
	defer srv.Close()

	out, _, err := execCLI(t, srv.URL, "edge", "list")
	if err != nil {
		t.Fatalf("err: %v", err)
	}
	if len(ndjsonLines(t, out)) != 2 {
		t.Fatalf("edge alias should resolve to edge-config: %s", out)
	}
}
