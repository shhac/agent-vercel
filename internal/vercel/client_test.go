package vercel

import (
	"context"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync/atomic"
	"testing"
	"time"

	agenterrors "github.com/shhac/agent-vercel/internal/errors"
	"github.com/shhac/agent-vercel/internal/mockvercel"
)

func zeroBackoff(int) time.Duration { return 0 }

func mustClient(t *testing.T, cfg Config) *Client {
	t.Helper()
	if cfg.Backoff == nil {
		cfg.Backoff = zeroBackoff
	}
	c, err := New(cfg)
	if err != nil {
		t.Fatalf("New: %v", err)
	}
	return c
}

func TestGetUserAgainstMock(t *testing.T) {
	srv := httptest.NewServer(mockvercel.New())
	defer srv.Close()
	c := mustClient(t, Config{BaseURL: srv.URL, Token: "tok"})

	u, err := c.GetUser(context.Background())
	if err != nil {
		t.Fatalf("GetUser: %v", err)
	}
	if u.Username != "acme-bot" {
		t.Fatalf("username = %q; want acme-bot", u.Username)
	}
}

func TestListTeamsAgainstMock(t *testing.T) {
	srv := httptest.NewServer(mockvercel.New())
	defer srv.Close()
	c := mustClient(t, Config{BaseURL: srv.URL, Token: "tok"})

	teams, err := c.ListTeams(context.Background())
	if err != nil {
		t.Fatalf("ListTeams: %v", err)
	}
	if len(teams) != 2 || teams[0].Slug != "acme" {
		t.Fatalf("teams = %+v; want acme first", teams)
	}
}

func TestRequestCarriesAuthAndScope(t *testing.T) {
	cases := []struct {
		scope     string
		wantQuery string
	}{
		{"acme", "slug=acme"},
		{"team_abc", "teamId=team_abc"},
		{"", ""},
	}
	for _, tc := range cases {
		var gotAuth, gotQuery string
		srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			gotAuth = r.Header.Get("Authorization")
			gotQuery = r.URL.RawQuery
			_, _ = w.Write([]byte(`{"user":{"username":"x"}}`))
		}))
		c := mustClient(t, Config{BaseURL: srv.URL, Token: "tok", Scope: tc.scope})
		if _, err := c.GetUser(context.Background()); err != nil {
			t.Fatalf("scope %q: %v", tc.scope, err)
		}
		srv.Close()
		if gotAuth != "Bearer tok" {
			t.Fatalf("scope %q: auth header = %q; want 'Bearer tok'", tc.scope, gotAuth)
		}
		if tc.wantQuery == "" {
			if gotQuery != "" {
				t.Fatalf("scope %q: query = %q; want empty", tc.scope, gotQuery)
			}
		} else if !strings.Contains(gotQuery, tc.wantQuery) {
			t.Fatalf("scope %q: query = %q; want contains %q", tc.scope, gotQuery, tc.wantQuery)
		}
	}
}

func TestErrorMapping(t *testing.T) {
	cases := []struct {
		status int
		want   agenterrors.FixableBy
	}{
		{400, agenterrors.FixableByAgent},
		{401, agenterrors.FixableByHuman},
		{402, agenterrors.FixableByHuman},
		{403, agenterrors.FixableByHuman},
		{404, agenterrors.FixableByAgent},
		{422, agenterrors.FixableByAgent},
		{429, agenterrors.FixableByRetry},
		{500, agenterrors.FixableByRetry},
	}
	for _, tc := range cases {
		srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
			w.WriteHeader(tc.status)
			_, _ = w.Write([]byte(`{"error":{"code":"oops","message":"boom"}}`))
		}))
		// Default retries with zero backoff: retry-class statuses still resolve
		// to the same APIError, just after a few (instant) attempts.
		c := mustClient(t, Config{BaseURL: srv.URL, Token: "tok"})
		_, err := c.Get(context.Background(), "/v2/user", nil)
		srv.Close()
		var aerr *agenterrors.APIError
		if !agenterrors.As(err, &aerr) {
			t.Fatalf("status %d: error is not *APIError: %v", tc.status, err)
		}
		if aerr.FixableBy != tc.want {
			t.Fatalf("status %d: fixable_by = %q; want %q", tc.status, aerr.FixableBy, tc.want)
		}
		if !strings.Contains(aerr.Message, "boom") {
			t.Fatalf("status %d: message = %q; want it to include the API message", tc.status, aerr.Message)
		}
	}
}

func TestRetriesTransientThenSucceeds(t *testing.T) {
	var hits int32
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		if atomic.AddInt32(&hits, 1) == 1 {
			w.Header().Set("Retry-After", "0")
			w.WriteHeader(http.StatusTooManyRequests)
			_, _ = w.Write([]byte(`{"error":{"code":"rate","message":"slow down"}}`))
			return
		}
		_, _ = w.Write([]byte(`{"user":{"username":"acme-bot"}}`))
	}))
	defer srv.Close()
	c := mustClient(t, Config{BaseURL: srv.URL, Token: "tok", MaxRetries: 3})

	u, err := c.GetUser(context.Background())
	if err != nil {
		t.Fatalf("GetUser after retry: %v", err)
	}
	if u.Username != "acme-bot" {
		t.Fatalf("username = %q; want acme-bot", u.Username)
	}
	if got := atomic.LoadInt32(&hits); got != 2 {
		t.Fatalf("hits = %d; want 2 (one 429, one success)", got)
	}
}

func TestEmptyTokenIsForbiddenByMock(t *testing.T) {
	srv := httptest.NewServer(mockvercel.New())
	defer srv.Close()
	c := mustClient(t, Config{BaseURL: srv.URL, Token: ""})

	_, err := c.GetUser(context.Background())
	var aerr *agenterrors.APIError
	if !agenterrors.As(err, &aerr) || aerr.FixableBy != agenterrors.FixableByHuman {
		t.Fatalf("empty token: want human-fixable APIError, got %v", err)
	}
}
