//go:build integration

// Live integration tests: opt-in, hit the real Vercel API, and assert the
// SHAPE of responses (field presence) that our compact mappers depend on —
// never the values. This is the guard against mock-vs-reality drift.
//
// Run with a real token (read-only calls only):
//
//	AGENT_VERCEL_IT_TOKEN=<token> [AGENT_VERCEL_IT_SCOPE=<team-slug|id>] \
//	  go test -tags integration ./internal/vercel -run Live -v
//
// Without AGENT_VERCEL_IT_TOKEN the tests skip, so the default build and CI stay
// green. No ids, names, or values are logged — only which field was missing.
package vercel

import (
	"context"
	"encoding/json"
	"net/url"
	"os"
	"testing"
	"time"

	"github.com/shhac/agent-vercel/internal/credential"
)

// liveToken resolves a token from $AGENT_VERCEL_IT_TOKEN, else from a stored
// credential ($AGENT_VERCEL_IT_AUTH label, else the default). The token stays
// inside this test process; it is never logged.
func liveToken(t *testing.T) string {
	t.Helper()
	if tok := os.Getenv("AGENT_VERCEL_IT_TOKEN"); tok != "" {
		return tok
	}
	store, err := credential.New()
	if err != nil {
		return ""
	}
	creds, err := store.Load()
	if err != nil {
		return ""
	}
	label := os.Getenv("AGENT_VERCEL_IT_AUTH")
	if label == "" {
		label = creds.DefaultAuth
	}
	for _, a := range creds.Auths {
		if a.Label == label && !credential.IsPlaceholder(a.Secret) {
			return a.Secret
		}
	}
	return ""
}

func liveClient(t *testing.T) *Client {
	t.Helper()
	tok := liveToken(t)
	if tok == "" {
		t.Skip("set AGENT_VERCEL_IT_TOKEN (or a stored credential via AGENT_VERCEL_IT_AUTH) to run live integration tests")
	}
	c, err := New(Config{Token: tok, Scope: os.Getenv("AGENT_VERCEL_IT_SCOPE")})
	if err != nil {
		t.Fatalf("client: %v", err)
	}
	return c
}

func liveCtx(t *testing.T) context.Context {
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	t.Cleanup(cancel)
	return ctx
}

func TestLiveUserShape(t *testing.T) {
	c := liveClient(t)
	u, err := c.GetUser(liveCtx(t))
	if err != nil {
		t.Fatalf("GetUser: %v", err)
	}
	if u.ID == "" || u.Username == "" {
		t.Fatal("GET /v2/user missing id/username our mapper relies on")
	}
}

func TestLiveTeamsShape(t *testing.T) {
	c := liveClient(t)
	teams, err := c.ListTeams(liveCtx(t))
	if err != nil {
		t.Fatalf("ListTeams: %v", err)
	}
	for _, tm := range teams {
		if tm.ID == "" || tm.Slug == "" {
			t.Fatal("a team is missing id/slug")
		}
	}
}

func TestLiveDeploymentShape(t *testing.T) {
	c := liveClient(t)
	ctx := liveCtx(t)
	q := url.Values{}
	q.Set("limit", "5")
	items, _, err := c.ListDeployments(ctx, q)
	if err != nil {
		t.Fatalf("ListDeployments: %v", err)
	}
	if len(items) == 0 {
		t.Skip("no deployments in this scope to validate")
	}
	var firstID string
	for _, raw := range items {
		var d struct {
			UID        string `json:"uid"`
			ID         string `json:"id"`
			State      string `json:"state"`
			ReadyState string `json:"readyState"`
			Created    int64  `json:"created"`
			CreatedAt  int64  `json:"createdAt"`
		}
		if err := json.Unmarshal(raw, &d); err != nil {
			t.Fatalf("decode deployment: %v", err)
		}
		id := d.UID
		if id == "" {
			id = d.ID
		}
		if id == "" {
			t.Fatal("deployment list item has neither uid nor id")
		}
		if d.State == "" && d.ReadyState == "" {
			t.Fatal("deployment list item has neither state nor readyState")
		}
		if d.Created == 0 && d.CreatedAt == 0 {
			t.Fatal("deployment list item has neither created nor createdAt")
		}
		if firstID == "" {
			firstID = id
		}
	}

	// v13 single-get returns `id` (not `uid`) — the assumption compactDeployment encodes.
	raw, err := c.GetDeployment(ctx, firstID)
	if err != nil {
		t.Fatalf("GetDeployment: %v", err)
	}
	var single struct {
		ID  string `json:"id"`
		UID string `json:"uid"`
	}
	_ = json.Unmarshal(raw, &single)
	if single.ID == "" && single.UID == "" {
		t.Fatal("v13 GetDeployment missing id/uid")
	}
}

func TestLiveProjectAndEnvShape(t *testing.T) {
	c := liveClient(t)
	ctx := liveCtx(t)
	q := url.Values{}
	q.Set("limit", "5")
	items, _, err := c.ListProjects(ctx, q)
	if err != nil {
		t.Fatalf("ListProjects: %v", err)
	}
	if len(items) == 0 {
		t.Skip("no projects in this scope to validate")
	}
	var projectID string
	for _, raw := range items {
		var p struct {
			ID   string `json:"id"`
			Name string `json:"name"`
		}
		if err := json.Unmarshal(raw, &p); err != nil {
			t.Fatalf("decode project: %v", err)
		}
		if p.ID == "" || p.Name == "" {
			t.Fatal("project missing id/name")
		}
		projectID = p.ID
	}

	// env endpoint must unwrap to a list of {key,target,type}; values not asserted.
	envs, err := c.ProjectEnv(ctx, projectID, url.Values{})
	if err != nil {
		t.Fatalf("ProjectEnv: %v", err)
	}
	for _, raw := range envs {
		var e struct {
			Key    string   `json:"key"`
			Target []string `json:"target"`
			Type   string   `json:"type"`
		}
		if err := json.Unmarshal(raw, &e); err != nil {
			t.Fatalf("decode env: %v", err)
		}
		if e.Key == "" {
			t.Fatal("env var missing key")
		}
	}
}

func TestLiveDomainShape(t *testing.T) {
	c := liveClient(t)
	items, _, err := c.ListDomains(liveCtx(t), url.Values{})
	if err != nil {
		t.Fatalf("ListDomains: %v", err)
	}
	for _, raw := range items {
		var d struct {
			Name string `json:"name"`
		}
		if err := json.Unmarshal(raw, &d); err != nil {
			t.Fatalf("decode domain: %v", err)
		}
		if d.Name == "" {
			t.Fatal("domain missing name")
		}
	}
}
