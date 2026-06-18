package cli

import (
	"net/http/httptest"
	"testing"
	"time"

	"github.com/shhac/agent-vercel/internal/mockvercel"
)

func TestDeploymentList(t *testing.T) {
	srv := httptest.NewServer(mockvercel.New())
	defer srv.Close()

	out, _, err := execCLI(t, srv.URL, "deployment", "list")
	if err != nil {
		t.Fatalf("err: %v", err)
	}
	rows := ndjsonLines(t, out)
	if len(rows) != 2 {
		t.Fatalf("got %d rows, want 2: %s", len(rows), out)
	}
	if rows[0]["id"] != "dpl_ready" || rows[0]["state"] != "READY" {
		t.Fatalf("row0 = %v", rows[0])
	}
	// compact projection surfaces git metadata
	if rows[0]["branch"] != "main" || rows[0]["sha"] != "abc123" {
		t.Fatalf("missing git meta: %v", rows[0])
	}
}

func TestDeploymentListStateFilter(t *testing.T) {
	srv := httptest.NewServer(mockvercel.New())
	defer srv.Close()

	out, _, err := execCLI(t, srv.URL, "deployment", "list", "--state", "ERROR")
	if err != nil {
		t.Fatalf("err: %v", err)
	}
	rows := ndjsonLines(t, out)
	if len(rows) != 1 || rows[0]["id"] != "dpl_err" {
		t.Fatalf("state filter wrong: %s", out)
	}
	if rows[0]["error_code"] != "BUILD_FAILED" {
		t.Fatalf("expected error_code on failed deploy: %v", rows[0])
	}
	if rows[0]["checks"] != "failed" || rows[0]["oom"] != true {
		t.Fatalf("expected checks/oom failure signals: %v", rows[0])
	}
}

func TestDeploymentGet(t *testing.T) {
	srv := httptest.NewServer(mockvercel.New())
	defer srv.Close()

	out, _, err := execCLI(t, srv.URL, "deployment", "get", "dpl_ready")
	if err != nil {
		t.Fatalf("err: %v", err)
	}
	m := decodeJSON(t, out)
	if m["id"] != "dpl_ready" || m["target"] != "production" {
		t.Fatalf("get = %v", m)
	}
}

func TestDeploymentGetNotFoundIsAgentError(t *testing.T) {
	srv := httptest.NewServer(mockvercel.New())
	defer srv.Close()

	_, errOut, err := execCLI(t, srv.URL, "deployment", "get", "dpl_missing")
	if err == nil {
		t.Fatal("expected error")
	}
	m := decodeJSON(t, errOut)
	if m["fixable_by"] != "agent" {
		t.Fatalf("404 should map to agent: %v", m)
	}
}

func TestDeploymentCurrent(t *testing.T) {
	srv := httptest.NewServer(mockvercel.New())
	defer srv.Close()

	out, _, err := execCLI(t, srv.URL, "deployment", "current", "web")
	if err != nil {
		t.Fatalf("err: %v", err)
	}
	m := decodeJSON(t, out)
	live, ok := m["live"].(map[string]any)
	if !ok || live["id"] != "dpl_ready" {
		t.Fatalf("live deployment missing: %v", m)
	}
	if _, ok := m["rolling_release"]; !ok {
		t.Fatalf("rolling_release missing: %v", m)
	}
}

func TestProjectListAndGet(t *testing.T) {
	srv := httptest.NewServer(mockvercel.New())
	defer srv.Close()

	out, _, err := execCLI(t, srv.URL, "project", "list")
	if err != nil {
		t.Fatalf("list err: %v", err)
	}
	if rows := ndjsonLines(t, out); len(rows) != 2 || rows[0]["framework"] != "nextjs" {
		t.Fatalf("project list = %s", out)
	}

	out, _, err = execCLI(t, srv.URL, "project", "get", "web")
	if err != nil {
		t.Fatalf("get err: %v", err)
	}
	m := decodeJSON(t, out)
	if m["id"] != "prj_web" {
		t.Fatalf("project get = %v", m)
	}
	if m["repo"] != "acme/web" || m["production_branch"] != "main" || m["node_version"] != "20.x" {
		t.Fatalf("project get enrichment missing: %v", m)
	}
	// build-config surfaced; installCommand is null in the fixture → omitted.
	if m["root_directory"] != "apps/web" || m["build_command"] != "turbo run build" || m["ignore_command"] != "npx turbo-ignore" {
		t.Fatalf("project get build-config missing: %v", m)
	}
	if _, hasInstall := m["install_command"]; hasInstall {
		t.Fatalf("null installCommand should be omitted: %v", m)
	}

	// prj_api is paused → surfaced as paused:true.
	apiOut, _, err := execCLI(t, srv.URL, "project", "get", "api")
	if err != nil {
		t.Fatalf("get api err: %v", err)
	}
	if decodeJSON(t, apiOut)["paused"] != true {
		t.Fatalf("paused project should report paused:true: %s", apiOut)
	}
}

func TestCompactDeploymentSurfacesBuildTriageFields(t *testing.T) {
	raw := []byte(`{
		"uid":"dpl_x","name":"web","projectId":"prj_web","readyState":"QUEUED",
		"buildSkipped":true,"isFirstBranchDeployment":true,
		"isInConcurrentBuildsQueue":true,
		"errorStep":"build","errorLink":"https://vercel.com/docs/errors/x",
		"readyStateReason":"build exceeded 45 minutes","source":"git",
		"created":1000,"buildingAt":1500,"ready":4500
	}`)
	m, err := compactDeployment(raw)
	if err != nil {
		t.Fatalf("compactDeployment: %v", err)
	}
	if m["build_skipped"] != true || m["first_branch_deployment"] != true {
		t.Fatalf("skip flags missing: %v", m)
	}
	if m["queued"] != "concurrent_builds" {
		t.Fatalf("queue reason = %v", m["queued"])
	}
	if m["error_step"] != "build" || m["state_reason"] != "build exceeded 45 minutes" || m["source"] != "git" {
		t.Fatalf("error/source fields missing: %v", m)
	}
	if m["queue_wait_ms"] != int64(500) || m["build_duration_ms"] != int64(3000) {
		t.Fatalf("derived timing wrong: queue_wait=%v build=%v", m["queue_wait_ms"], m["build_duration_ms"])
	}
}

func metaCursor(t *testing.T, ndjson string) (string, bool) {
	t.Helper()
	for _, r := range ndjsonLines(t, ndjson) {
		if p, ok := r["@pagination"].(map[string]any); ok {
			c, _ := p["next_cursor"].(string)
			return c, true
		}
	}
	return "", false
}

func TestDeploymentListCursorRoundTrip(t *testing.T) {
	srv := httptest.NewServer(mockvercel.New())
	defer srv.Close()

	// Page 1: one item + a next cursor.
	out, _, err := execCLI(t, srv.URL, "deployment", "list", "--limit", "1")
	if err != nil {
		t.Fatalf("page1: %v", err)
	}
	cursor, ok := metaCursor(t, out)
	if !ok || cursor == "" {
		t.Fatalf("expected @pagination.next_cursor on page 1: %s", out)
	}
	// the cursor must round-trip through --cursor (the bug this fixes)
	out2, _, err := execCLI(t, srv.URL, "deployment", "list", "--limit", "1", "--cursor", cursor)
	if err != nil {
		t.Fatalf("page2: %v", err)
	}
	rows := ndjsonLines(t, out2)
	if rows[0]["id"] != "dpl_err" {
		t.Fatalf("page2 first row = %v; want dpl_err", rows[0])
	}
}

func TestDeploymentListAllFollowsPages(t *testing.T) {
	srv := httptest.NewServer(mockvercel.New())
	defer srv.Close()

	// page size 1 + --all should traverse and return both deployments, no trailer.
	out, _, err := execCLI(t, srv.URL, "deployment", "list", "--limit", "1", "--all")
	if err != nil {
		t.Fatalf("all: %v", err)
	}
	var ids []string
	for _, r := range ndjsonLines(t, out) {
		if id, ok := r["id"].(string); ok {
			ids = append(ids, id)
		}
	}
	if len(ids) != 2 {
		t.Fatalf("--all should return both pages, got %v: %s", ids, out)
	}
	if _, ok := metaCursor(t, out); ok {
		t.Fatalf("--all that exhausted pages should emit no cursor: %s", out)
	}
}

func TestRuntimeLogsBoundedByOpenStream(t *testing.T) {
	// Mock holds the runtime-logs connection open (like real Vercel); the
	// client's --timeout window must bound the read and return what arrived,
	// not hang.
	srv := httptest.NewServer(mockvercel.New(mockvercel.WithRuntimeLogsHang()))
	defer srv.Close()

	start := time.Now()
	out, _, err := execCLI(t, srv.URL, "deployment", "runtime-logs", "dpl_ready", "--timeout", "400")
	elapsed := time.Since(start)
	if err != nil {
		t.Fatalf("err: %v", err)
	}
	if rows := ndjsonLines(t, out); len(rows) != 2 {
		t.Fatalf("want 2 buffered logs from the stream, got %d: %s", len(rows), out)
	}
	if elapsed > 5*time.Second {
		t.Fatalf("runtime-logs did not bound the open stream (took %s)", elapsed)
	}
}

func TestDeploymentCustomEnvFilter(t *testing.T) {
	deps := []map[string]any{
		{"uid": "dpl_stg", "name": "web", "projectId": "prj_web", "url": "web-stg.example.com",
			"state": "READY", "readyState": "READY", "created": int64(2),
			"customEnvironment": map[string]any{"id": "env_stg", "slug": "staging"}},
		{"uid": "dpl_prod", "name": "web", "projectId": "prj_web", "url": "web-prod.example.com",
			"state": "READY", "readyState": "READY", "target": "production", "created": int64(1)},
	}
	srv := httptest.NewServer(mockvercel.New(mockvercel.WithDeployments(deps)))
	defer srv.Close()

	out, _, err := execCLI(t, srv.URL, "deployment", "list", "--custom-env", "staging")
	if err != nil {
		t.Fatalf("err: %v", err)
	}
	rows := ndjsonLines(t, out)
	if len(rows) != 1 || rows[0]["id"] != "dpl_stg" || rows[0]["custom_environment"] != "staging" {
		t.Fatalf("custom-env filter = %s", out)
	}

	// deployment current --custom-env picks the newest READY in that env
	out, _, err = execCLI(t, srv.URL, "deployment", "current", "web", "--custom-env", "staging")
	if err != nil {
		t.Fatalf("current err: %v", err)
	}
	m := decodeJSON(t, out)
	live, ok := m["live"].(map[string]any)
	if !ok || live["id"] != "dpl_stg" {
		t.Fatalf("current --custom-env = %v", m)
	}
}
