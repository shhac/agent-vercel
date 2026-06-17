package cli

import (
	"net/http/httptest"
	"testing"

	"github.com/shhac/agent-vercel/internal/mockvercel"
)

func TestWebhookList(t *testing.T) {
	srv := httptest.NewServer(mockvercel.New())
	defer srv.Close()

	out, _, err := execCLI(t, srv.URL, "webhook", "list")
	if err != nil {
		t.Fatalf("err: %v", err)
	}
	rows := ndjsonLines(t, out)
	if len(rows) != 2 {
		t.Fatalf("got %d rows, want 2: %s", len(rows), out)
	}
	events, ok := rows[0]["events"].([]any)
	if !ok || len(events) != 3 || events[0] != "deployment.created" {
		t.Fatalf("events projection = %v", rows[0]["events"])
	}
	pids, ok := rows[0]["project_ids"].([]any)
	if !ok || len(pids) != 1 || pids[0] != "prj_web" {
		t.Fatalf("project_ids = %v", rows[0]["project_ids"])
	}
	// an account-wide webhook (no projectIds) omits the key
	if _, has := rows[1]["project_ids"]; has {
		t.Fatalf("account-wide webhook should omit project_ids: %v", rows[1])
	}
}

func TestWebhookListProjectFilter(t *testing.T) {
	srv := httptest.NewServer(mockvercel.New())
	defer srv.Close()

	out, _, err := execCLI(t, srv.URL, "webhook", "list", "--project", "prj_web")
	if err != nil {
		t.Fatalf("err: %v", err)
	}
	rows := ndjsonLines(t, out)
	if len(rows) != 1 || rows[0]["id"] != "hook_deploys" {
		t.Fatalf("project filter should yield only the prj_web hook: %s", out)
	}
}

func TestWebhookListFull(t *testing.T) {
	srv := httptest.NewServer(mockvercel.New())
	defer srv.Close()

	// --full returns raw webhook objects with camelCase keys, not the compact
	// snake_case projection.
	out, _, err := execCLI(t, srv.URL, "webhook", "list", "--full")
	if err != nil {
		t.Fatalf("err: %v", err)
	}
	rows := ndjsonLines(t, out)
	if len(rows) != 2 {
		t.Fatalf("got %d rows, want 2: %s", len(rows), out)
	}
	if _, projected := rows[0]["project_ids"]; projected {
		t.Fatalf("--full should not carry the compact project_ids key: %v", rows[0])
	}
	if _, raw := rows[0]["projectIds"]; !raw {
		t.Fatalf("--full should expose the raw projectIds key: %v", rows[0])
	}
}

func TestWebhookListBareArray(t *testing.T) {
	// The webhooks endpoint returns a bare array; decodeKeyedArray must handle
	// it (no {webhooks:[...]} envelope).
	srv := httptest.NewServer(mockvercel.New(mockvercel.WithWebhooks([]map[string]any{
		{"id": "hook_one", "url": "https://h.example.com", "events": []any{"deployment.error"}},
	})))
	defer srv.Close()

	out, _, err := execCLI(t, srv.URL, "webhook", "list")
	if err != nil {
		t.Fatalf("err: %v", err)
	}
	if rows := ndjsonLines(t, out); len(rows) != 1 || rows[0]["id"] != "hook_one" {
		t.Fatalf("bare array not decoded: %s", out)
	}
}
