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
