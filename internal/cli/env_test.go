package cli

import (
	"net/http/httptest"
	"testing"

	"github.com/shhac/agent-vercel/internal/mockvercel"
)

func TestEnvListNoValueWithoutDecrypt(t *testing.T) {
	srv := httptest.NewServer(mockvercel.New())
	defer srv.Close()

	out, _, err := execCLI(t, srv.URL, "env", "list", "web")
	if err != nil {
		t.Fatalf("err: %v", err)
	}
	rows := ndjsonLines(t, out)
	if len(rows) != 5 {
		t.Fatalf("want 5 env rows, got %d: %s", len(rows), out)
	}
	for _, r := range rows {
		if _, ok := r["value"]; ok {
			t.Fatalf("value leaked without --decrypt: %v", r)
		}
	}
}

func TestEnvListEnvironmentFilter(t *testing.T) {
	srv := httptest.NewServer(mockvercel.New())
	defer srv.Close()

	out, _, err := execCLI(t, srv.URL, "env", "list", "web", "--environment", "production")
	if err != nil {
		t.Fatalf("err: %v", err)
	}
	// production targets: KEY_SHARED, API_URL(prod), ONLY_PROD = 3
	if rows := ndjsonLines(t, out); len(rows) != 3 {
		t.Fatalf("want 3 production vars, got %d: %s", len(rows), out)
	}
}

func TestEnvListDecryptShowsValue(t *testing.T) {
	srv := httptest.NewServer(mockvercel.New())
	defer srv.Close()

	out, _, err := execCLI(t, srv.URL, "env", "list", "web", "--decrypt", "--environment", "production")
	if err != nil {
		t.Fatalf("err: %v", err)
	}
	rows := ndjsonLines(t, out)
	found := false
	for _, r := range rows {
		if r["key"] == "API_URL" && r["value"] == "https://prod.example.com" {
			found = true
		}
	}
	if !found {
		t.Fatalf("decrypted value not shown: %s", out)
	}
}

func TestEnvDiff(t *testing.T) {
	srv := httptest.NewServer(mockvercel.New())
	defer srv.Close()

	out, _, err := execCLI(t, srv.URL, "env", "diff", "web")
	if err != nil {
		t.Fatalf("err: %v", err)
	}
	rows := ndjsonLines(t, out)
	status := map[string]string{}
	for _, r := range rows {
		status[r["key"].(string)] = r["status"].(string)
	}
	// KEY_SHARED is identical in both → omitted from the diff.
	if _, ok := status["KEY_SHARED"]; ok {
		t.Fatalf("identical key should be omitted: %v", status)
	}
	if status["API_URL"] != "different" {
		t.Fatalf("API_URL should differ: %v", status)
	}
	if status["ONLY_PROD"] != "only_production" {
		t.Fatalf("ONLY_PROD status = %q", status["ONLY_PROD"])
	}
	if status["ONLY_PREVIEW"] != "only_preview" {
		t.Fatalf("ONLY_PREVIEW status = %q", status["ONLY_PREVIEW"])
	}
}

func TestDeploymentBuildLogs(t *testing.T) {
	srv := httptest.NewServer(mockvercel.New())
	defer srv.Close()

	out, _, err := execCLI(t, srv.URL, "deployment", "logs", "dpl_err")
	if err != nil {
		t.Fatalf("err: %v", err)
	}
	rows := ndjsonLines(t, out)
	if len(rows) != 3 || rows[1]["type"] != "stderr" {
		t.Fatalf("build logs = %s", out)
	}
}

func TestDeploymentRuntimeLogsStatusFilter(t *testing.T) {
	srv := httptest.NewServer(mockvercel.New())
	defer srv.Close()

	out, _, err := execCLI(t, srv.URL, "deployment", "runtime-logs", "dpl_ready", "--status", "5xx")
	if err != nil {
		t.Fatalf("err: %v", err)
	}
	rows := ndjsonLines(t, out)
	if len(rows) != 1 || rows[0]["level"] != "error" {
		t.Fatalf("runtime 5xx filter = %s", out)
	}
}

func TestEnvSetPostsTargetAndType(t *testing.T) {
	srv := httptest.NewServer(mockvercel.New())
	defer srv.Close()

	// --full prints the echoed mock response, so we can assert the wire body.
	out, _, err := execCLI(t, srv.URL, "env", "set", "web", "API_KEY", "secret",
		"--environment", "production,preview", "--type", "encrypted", "--yes", "--full")
	if err != nil {
		t.Fatalf("env set: %v", err)
	}
	created, ok := decodeJSON(t, out)["created"].(map[string]any)
	if !ok {
		t.Fatalf("no created object: %s", out)
	}
	if created["type"] != "encrypted" || created["key"] != "API_KEY" {
		t.Fatalf("wire body wrong: %v", created)
	}
	target, ok := created["target"].([]any)
	if !ok || len(target) != 2 || target[0] != "production" || target[1] != "preview" {
		t.Fatalf("target not posted as expected: %v", created["target"])
	}
}

func TestEnvSetEmptyEnvironmentIsAgentError(t *testing.T) {
	srv := httptest.NewServer(mockvercel.New())
	defer srv.Close()

	_, errOut, err := execCLI(t, srv.URL, "env", "set", "web", "K", "v", "--environment", "", "--yes")
	if err == nil {
		t.Fatal("expected error for empty --environment")
	}
	if m := decodeJSON(t, errOut); m["fixable_by"] != "agent" {
		t.Fatalf("empty --environment should be agent error: %v", m)
	}
}

func TestEnvDiffEmptyValuesClassifiedSame(t *testing.T) {
	env := []map[string]any{
		{"id": "e1", "key": "EMPTY_BOTH", "target": []any{"production", "preview"}, "type": "encrypted", "value": ""},
		{"id": "e2", "key": "ONLY_P", "target": []any{"production"}, "type": "plain", "value": "x"},
	}
	srv := httptest.NewServer(mockvercel.New(mockvercel.WithEnv(env)))
	defer srv.Close()

	out, _, err := execCLI(t, srv.URL, "env", "diff", "web")
	if err != nil {
		t.Fatalf("diff: %v", err)
	}
	status := map[string]string{}
	for _, r := range ndjsonLines(t, out) {
		status[r["key"].(string)] = r["status"].(string)
	}
	// Identical empty values in both environments → same → omitted from the diff.
	if _, ok := status["EMPTY_BOTH"]; ok {
		t.Fatalf("EMPTY_BOTH (empty in both) should be omitted, got %q", status["EMPTY_BOTH"])
	}
	if status["ONLY_P"] != "only_production" {
		t.Fatalf("ONLY_P = %q; want only_production", status["ONLY_P"])
	}
}
