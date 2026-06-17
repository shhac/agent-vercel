// Package mockvercel is an in-process fixture of the Vercel REST API for tests
// and manual exercising of agent-vercel without real network access. It serves a
// small set of endpoints with canned data and enforces Bearer auth, mirroring
// the real API's error envelope {error:{code,message}} so client error-mapping
// can be exercised end-to-end.
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
	User           User
	Teams          []Team
	Deployments    []map[string]any
	Projects       []map[string]any
	RollingRelease map[string]any
	BuildEvents    []map[string]any
	RuntimeLogs    []map[string]any
	Env            []map[string]any
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
				"inspectorUrl": "https://vercel.com/acme/web/ready", "created": int64(1716206800000),
				"creator": map[string]any{"username": "acme-bot", "email": "bot@acme.com"},
				"meta":    map[string]any{"githubCommitRef": "main", "githubCommitSha": "abc123", "githubCommitMessage": "ship it"},
			},
			{
				"uid": "dpl_err", "name": "web", "projectId": "prj_web",
				"url": "web-err.vercel.app", "state": "ERROR", "readyState": "ERROR",
				"target": "production", "errorCode": "BUILD_FAILED",
				"errorMessage": "Command \"next build\" exited with 1",
				"inspectorUrl": "https://vercel.com/acme/web/err", "created": int64(1716206500000),
				"creator": map[string]any{"username": "dev", "email": "dev@acme.com"},
				"meta":    map[string]any{"githubCommitRef": "fix/build", "githubCommitSha": "def456", "githubCommitMessage": "wip"},
			},
		},
		Projects: []map[string]any{
			{
				"id": "prj_web", "name": "web", "framework": "nextjs",
				"updatedAt": int64(1716206800000),
				"latestDeployments": []any{map[string]any{
					"uid": "dpl_ready", "url": "web-ready.vercel.app", "readyState": "READY", "target": "production",
				}},
			},
			{
				"id": "prj_api", "name": "api", "framework": "go",
				"updatedAt": int64(1716100000000),
			},
		},
		RollingRelease: map[string]any{
			"rollingRelease": map[string]any{
				"state": "ACTIVE",
				"currentDeployment": map[string]any{
					"id": "dpl_ready", "url": "web-ready.vercel.app", "readyState": "READY", "target": "production",
				},
				"canaryDeployment": map[string]any{
					"id": "dpl_canary", "url": "web-canary.vercel.app", "readyState": "READY", "target": "production",
				},
				"currentCanaryPercentage": 25,
			},
		},
		BuildEvents: []map[string]any{
			{"type": "stdout", "created": int64(1716206500000), "payload": map[string]any{"text": "Running \"next build\""}},
			{"type": "stderr", "created": int64(1716206501000), "payload": map[string]any{"text": "Error: build failed", "statusCode": 500}},
			{"type": "exit", "created": int64(1716206502000), "payload": map[string]any{"text": "Command exited with 1"}},
		},
		RuntimeLogs: []map[string]any{
			{"level": "info", "source": "serverless", "message": "GET /api/health 200", "timestampInMs": int64(1716206600000), "requestMethod": "GET", "requestPath": "/api/health", "responseStatusCode": 200},
			{"level": "error", "source": "serverless", "message": "GET /api/users 500 boom", "timestampInMs": int64(1716206601000), "requestMethod": "GET", "requestPath": "/api/users", "responseStatusCode": 500},
		},
		Env: []map[string]any{
			{"id": "env_shared", "key": "KEY_SHARED", "target": []any{"production", "preview"}, "type": "encrypted", "value": "shared-val"},
			{"id": "env_apiprod", "key": "API_URL", "target": []any{"production"}, "type": "plain", "value": "https://prod.example.com"},
			{"id": "env_apiprev", "key": "API_URL", "target": []any{"preview"}, "type": "plain", "value": "https://preview.example.com"},
			{"id": "env_onlyprod", "key": "ONLY_PROD", "target": []any{"production"}, "type": "encrypted", "value": "p"},
			{"id": "env_onlyprev", "key": "ONLY_PREVIEW", "target": []any{"preview"}, "type": "encrypted", "value": "v"},
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

	mux.HandleFunc("GET /v2/user", requireBearer(func(w http.ResponseWriter, _ *http.Request) {
		writeJSON(w, http.StatusOK, map[string]any{"user": o.User})
	}))
	mux.HandleFunc("GET /v2/teams", requireBearer(func(w http.ResponseWriter, _ *http.Request) {
		writeJSON(w, http.StatusOK, map[string]any{
			"teams":      o.Teams,
			"pagination": map[string]any{"count": len(o.Teams)},
		})
	}))

	mux.HandleFunc("GET /v6/deployments", requireBearer(func(w http.ResponseWriter, r *http.Request) {
		items := o.Deployments
		if state := r.URL.Query().Get("state"); state != "" {
			items = filterMaps(items, "state", state)
		}
		if target := r.URL.Query().Get("target"); target != "" {
			items = filterMaps(items, "target", target)
		}
		writeJSON(w, http.StatusOK, map[string]any{
			"deployments": items,
			"pagination":  map[string]any{"count": len(items)},
		})
	}))
	mux.HandleFunc("GET /v13/deployments/{id}", requireBearer(func(w http.ResponseWriter, r *http.Request) {
		id := r.PathValue("id")
		for _, d := range o.Deployments {
			if d["uid"] == id || d["url"] == id {
				single := cloneMap(d)
				single["id"] = d["uid"]
				writeJSON(w, http.StatusOK, single)
				return
			}
		}
		writeErr(w, http.StatusNotFound, "not_found", "deployment not found: "+id)
	}))

	mux.HandleFunc("GET /v9/projects", requireBearer(func(w http.ResponseWriter, _ *http.Request) {
		writeJSON(w, http.StatusOK, map[string]any{
			"projects":   o.Projects,
			"pagination": map[string]any{"count": len(o.Projects)},
		})
	}))
	mux.HandleFunc("GET /v9/projects/{idOrName}", requireBearer(func(w http.ResponseWriter, r *http.Request) {
		key := r.PathValue("idOrName")
		for _, p := range o.Projects {
			if p["id"] == key || p["name"] == key {
				writeJSON(w, http.StatusOK, p)
				return
			}
		}
		writeErr(w, http.StatusNotFound, "not_found", "project not found: "+key)
	}))
	mux.HandleFunc("GET /v1/projects/{idOrName}/rolling-release", requireBearer(func(w http.ResponseWriter, _ *http.Request) {
		writeJSON(w, http.StatusOK, o.RollingRelease)
	}))

	mux.HandleFunc("GET /v3/deployments/{id}/events", requireBearer(func(w http.ResponseWriter, _ *http.Request) {
		writeJSON(w, http.StatusOK, o.BuildEvents)
	}))
	mux.HandleFunc("GET /v1/projects/{projectId}/deployments/{deploymentId}/runtime-logs", requireBearer(func(w http.ResponseWriter, _ *http.Request) {
		writeJSON(w, http.StatusOK, o.RuntimeLogs)
	}))
	mux.HandleFunc("GET /v10/projects/{idOrName}/env", requireBearer(func(w http.ResponseWriter, r *http.Request) {
		decrypt := r.URL.Query().Get("decrypt") == "true"
		envs := make([]map[string]any, 0, len(o.Env))
		for _, e := range o.Env {
			ev := cloneMap(e)
			if !decrypt {
				delete(ev, "value")
			}
			envs = append(envs, ev)
		}
		writeJSON(w, http.StatusOK, map[string]any{"envs": envs})
	}))

	return mux
}

func filterMaps(items []map[string]any, key, want string) []map[string]any {
	out := make([]map[string]any, 0, len(items))
	for _, m := range items {
		if v, ok := m[key].(string); ok && v == want {
			out = append(out, m)
		}
	}
	return out
}

func cloneMap(m map[string]any) map[string]any {
	out := make(map[string]any, len(m)+1)
	for k, v := range m {
		out[k] = v
	}
	return out
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
