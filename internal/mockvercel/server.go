// Package mockvercel is an in-process fixture of the Vercel REST API for tests
// and manual exercising of agent-vercel without real network access. It serves a
// small set of endpoints (user, teams, deployments) with canned data and
// enforces Bearer auth, mirroring the real API's error envelope
// {error:{code,message}} so client error-mapping can be exercised end-to-end.
package mockvercel

import (
	"encoding/json"
	"net/http"
	"strings"
)

// User and Team are the fixture shapes. Exported so callers can override them.
type User struct {
	ID       string `json:"id"`
	Username string `json:"username"`
	Email    string `json:"email"`
	Name     string `json:"name"`
}

type Team struct {
	ID   string `json:"id"`
	Slug string `json:"slug"`
	Name string `json:"name"`
}

// Options configures the fixtures served by the handler.
type Options struct {
	User        User
	Teams       []Team
	Deployments []map[string]any
}

// Option mutates Options.
type Option func(*Options)

// WithUser overrides the fixture user.
func WithUser(u User) Option { return func(o *Options) { o.User = u } }

// WithTeams overrides the fixture team list.
func WithTeams(t []Team) Option { return func(o *Options) { o.Teams = t } }

func defaults() *Options {
	return &Options{
		User: User{ID: "usr_mock", Username: "acme-bot", Email: "bot@acme.com", Name: "Acme Bot"},
		Teams: []Team{
			{ID: "team_abc", Slug: "acme", Name: "Acme Inc"},
			{ID: "team_xyz", Slug: "side", Name: "Side Project"},
		},
		Deployments: []map[string]any{
			{
				"uid": "dpl_ready", "name": "web", "projectId": "prj_web",
				"url": "web-ready.vercel.app", "state": "READY", "readyState": "READY",
				"target": "production", "readySubstate": "PROMOTED",
				"inspectorUrl": "https://vercel.com/acme/web/ready", "created": 1716206800000,
				"meta": map[string]any{"githubCommitRef": "main", "githubCommitSha": "abc123"},
			},
			{
				"uid": "dpl_err", "name": "web", "projectId": "prj_web",
				"url": "web-err.vercel.app", "state": "ERROR", "readyState": "ERROR",
				"target": "production", "errorCode": "BUILD_FAILED",
				"errorMessage": "Command \"next build\" exited with 1",
				"inspectorUrl": "https://vercel.com/acme/web/err", "created": 1716206500000,
				"meta": map[string]any{"githubCommitRef": "fix/build", "githubCommitSha": "def456"},
			},
		},
	}
}

// New returns an http.Handler serving the fixture API.
func New(opts ...Option) http.Handler {
	o := defaults()
	for _, f := range opts {
		f(o)
	}
	mux := http.NewServeMux()
	mux.HandleFunc("/v2/user", requireBearer(func(w http.ResponseWriter, _ *http.Request) {
		writeJSON(w, http.StatusOK, map[string]any{"user": o.User})
	}))
	mux.HandleFunc("/v2/teams", requireBearer(func(w http.ResponseWriter, _ *http.Request) {
		writeJSON(w, http.StatusOK, map[string]any{
			"teams":      o.Teams,
			"pagination": map[string]any{"count": len(o.Teams)},
		})
	}))
	mux.HandleFunc("/v6/deployments", requireBearer(func(w http.ResponseWriter, _ *http.Request) {
		writeJSON(w, http.StatusOK, map[string]any{
			"deployments": o.Deployments,
			"pagination":  map[string]any{"count": len(o.Deployments)},
		})
	}))
	return mux
}

func requireBearer(next http.HandlerFunc) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		tok := strings.TrimSpace(strings.TrimPrefix(r.Header.Get("Authorization"), "Bearer "))
		if tok == "" || !strings.HasPrefix(r.Header.Get("Authorization"), "Bearer ") {
			writeErr(w, http.StatusForbidden, "forbidden", "Not authorized: missing or empty Bearer token")
			return
		}
		next(w, r)
	}
}

func writeJSON(w http.ResponseWriter, code int, body any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(code)
	_ = json.NewEncoder(w).Encode(body)
}

func writeErr(w http.ResponseWriter, code int, errCode, msg string) {
	writeJSON(w, code, map[string]any{"error": map[string]any{"code": errCode, "message": msg}})
}
