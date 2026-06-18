// Package mockvercel is an in-process fixture of the Vercel REST API for tests
// and manual exercising of agent-vercel without real network access. It serves a
// small set of endpoints with canned data and enforces Bearer auth, mirroring
// the real API's error envelope {error:{code,message}} so client error-mapping
// can be exercised end-to-end.
package mockvercel

import (
	"encoding/json"
	"net/http"
	"sort"
	"strconv"
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
	User               User
	Teams              []Team
	TeamMembers        []map[string]any
	Deployments        []map[string]any
	Projects           []map[string]any
	RollingRelease     map[string]any
	ProjectCrons       map[string]any
	CustomEnvironments []map[string]any
	DeploymentChecks   []map[string]any
	BuildEvents        []map[string]any
	RuntimeLogs        []map[string]any
	Env                []map[string]any
	SharedEnv          []map[string]any
	Domains            []map[string]any
	DomainConfig       map[string]any
	DomainRecords      []map[string]any
	Certs              map[string]map[string]any
	Aliases            []map[string]any
	Charges            []map[string]any
	Webhooks           []map[string]any
	EdgeConfigs        []map[string]any
	EdgeConfigItems    map[string][]map[string]any
	// RuntimeLogsHang, when set, makes the runtime-logs handler hold the
	// connection open after emitting its lines — simulating Vercel's
	// open-ended stream so the client's bounded-window read can be tested.
	RuntimeLogsHang bool
}

// Option mutates Options.
type Option func(*Options)

// WithEnv overrides the fixture environment variables.
func WithEnv(env []map[string]any) Option { return func(o *Options) { o.Env = env } }

// WithDeployments overrides the fixture deployments.
func WithDeployments(d []map[string]any) Option { return func(o *Options) { o.Deployments = d } }

// WithDeploymentChecks overrides the fixture deployment checks.
func WithDeploymentChecks(c []map[string]any) Option {
	return func(o *Options) { o.DeploymentChecks = c }
}

// WithProjectCrons overrides the fixture project crons payload.
func WithProjectCrons(c map[string]any) Option { return func(o *Options) { o.ProjectCrons = c } }

// WithCustomEnvironments overrides the fixture custom environments.
func WithCustomEnvironments(e []map[string]any) Option {
	return func(o *Options) { o.CustomEnvironments = e }
}

// WithWebhooks overrides the fixture webhooks.
func WithWebhooks(w []map[string]any) Option { return func(o *Options) { o.Webhooks = w } }

// WithEdgeConfigs overrides the fixture Edge Configs.
func WithEdgeConfigs(e []map[string]any) Option { return func(o *Options) { o.EdgeConfigs = e } }

// WithTeamMembers overrides the fixture team members.
func WithTeamMembers(m []map[string]any) Option { return func(o *Options) { o.TeamMembers = m } }

// WithSharedEnv overrides the fixture team-level shared env vars.
func WithSharedEnv(e []map[string]any) Option { return func(o *Options) { o.SharedEnv = e } }

// WithRuntimeLogsHang makes the runtime-logs endpoint hold the connection open
// after emitting its lines, simulating Vercel's open-ended log stream.
func WithRuntimeLogsHang() Option { return func(o *Options) { o.RuntimeLogsHang = true } }

func defaults() *Options {
	return &Options{
		User: User{ID: "usr_mock", Username: "acme-bot", Email: "bot@acme.com", Name: "Acme Bot"},
		Teams: []Team{
			{ID: "team_abc", Slug: "acme", Name: "Acme Inc"},
			{ID: "team_xyz", Slug: "side", Name: "Side Project"},
		},
		TeamMembers: []map[string]any{
			{"uid": "usr_owner", "username": "acme-bot", "email": "bot@acme.com", "role": "OWNER", "confirmed": true, "createdAt": int64(1700000000000)},
			{"uid": "usr_dev", "username": "dev", "email": "dev@acme.com", "role": "MEMBER", "confirmed": true, "createdAt": int64(1710000000000)},
			{"uid": "usr_pending", "username": "newbie", "email": "newbie@acme.com", "role": "MEMBER", "confirmed": false, "createdAt": int64(1716000000000)},
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
				"checksConclusion": "failed", "oomReport": "out-of-memory",
				"creator": map[string]any{"username": "dev", "email": "dev@acme.com"},
				"meta":    map[string]any{"githubCommitRef": "fix/build", "githubCommitSha": "def456", "githubCommitMessage": "wip"},
			},
		},
		Projects: []map[string]any{
			{
				"id": "prj_web", "name": "web", "framework": "nextjs", "nodeVersion": "20.x",
				"link":      map[string]any{"org": "acme", "repo": "web", "type": "github", "productionBranch": "main"},
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
		ProjectCrons: map[string]any{
			"crons": map[string]any{
				"enabledAt":    int64(1716200000000),
				"disabledAt":   nil,
				"updatedAt":    int64(1716206800000),
				"deploymentId": "dpl_ready",
				"definitions": []any{
					map[string]any{"host": "web-ready.vercel.app", "path": "/api/cron/sync", "schedule": "0 5 * * *"},
					map[string]any{"host": "web-ready.vercel.app", "path": "/api/cron/digest", "schedule": "*/15 * * * *"},
				},
			},
		},
		CustomEnvironments: []map[string]any{
			{
				"id": "env_staging", "slug": "staging", "type": "preview",
				"description":   "Staging environment",
				"branchMatcher": map[string]any{"type": "startsWith", "pattern": "release/"},
				"domains":       []any{map[string]any{"name": "staging.example.com"}},
				"createdAt":     int64(1716100000000), "updatedAt": int64(1716206800000),
			},
			{
				"id": "env_qa", "slug": "qa", "type": "preview",
				"branchMatcher": map[string]any{"type": "equals", "pattern": "qa"},
				"createdAt":     int64(1716050000000), "updatedAt": int64(1716060000000),
			},
		},
		DeploymentChecks: []map[string]any{
			{"id": "check_lint", "name": "Lint", "status": "completed", "conclusion": "succeeded", "blocking": true, "integrationId": "icfg_lint", "startedAt": int64(1716206500000), "completedAt": int64(1716206520000)},
			{"id": "check_e2e", "name": "E2E", "status": "completed", "conclusion": "failed", "blocking": true, "integrationId": "icfg_e2e", "detailsUrl": "https://ci.example.com/runs/1", "rerequestable": true, "startedAt": int64(1716206500000), "completedAt": int64(1716206590000)},
			{"id": "check_perf", "name": "Lighthouse", "status": "running", "conclusion": "", "blocking": false, "integrationId": "icfg_perf", "startedAt": int64(1716206500000)},
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
		SharedEnv: []map[string]any{
			{"id": "env_shared_db", "key": "DATABASE_URL", "type": "encrypted", "target": []any{"production", "preview"}, "projectId": []any{"prj_web", "prj_api"}, "value": "postgres://shared", "createdAt": int64(1716000000000), "updatedAt": int64(1716206800000)},
			{"id": "env_shared_flag", "key": "FEATURE_X", "type": "plain", "target": []any{"production"}, "projectId": []any{"prj_web"}, "value": "on", "createdAt": int64(1716010000000), "updatedAt": int64(1716010000000)},
		},
		Domains: []map[string]any{
			{
				"name": "example.com", "verified": true, "serviceType": "external",
				"nameservers":         []any{"ns1.registrar.com", "ns2.registrar.com"},
				"intendedNameservers": []any{"ns1.vercel-dns.com", "ns2.vercel-dns.com"},
				"expiresAt":           int64(1763200000000), "renew": true,
			},
		},
		DomainConfig: map[string]any{
			"misconfigured":      true,
			"configuredBy":       nil,
			"acceptedChallenges": []any{},
		},
		DomainRecords: []map[string]any{
			{"id": "rec_1", "type": "A", "name": "", "value": "76.76.21.21", "ttl": 60},
			{"id": "rec_2", "type": "CNAME", "name": "www", "value": "cname.vercel-dns.com", "ttl": 60},
		},
		Certs: map[string]map[string]any{
			// cert_1 expires in the past (an already-expired cert, for --expiring triage)…
			"cert_1": {"id": "cert_1", "createdAt": int64(1716000000000), "expiresAt": int64(1763200000000), "autoRenew": true, "cns": []any{"example.com", "www.example.com"}},
			// …cert_2 expires far in the future.
			"cert_2": {"id": "cert_2", "createdAt": int64(1716000000000), "expiresAt": int64(2000000000000), "autoRenew": true, "cns": []any{"app.example.com"}},
		},
		Aliases: []map[string]any{
			{"uid": "alias_1", "alias": "example.com", "created": "2026-05-01T10:00:00.000Z"},
			{"uid": "alias_2", "alias": "web-ready.vercel.app", "created": "2026-05-01T10:00:00.000Z", "protectionBypass": map[string]any{"scope": "shareable-link"}},
		},
		Charges: []map[string]any{
			{"ServiceName": "Functions", "ChargeCategory": "Usage", "BilledCost": 12.50, "BillingCurrency": "USD", "ConsumedQuantity": 1000000.0, "ConsumedUnit": "invocations", "ChargePeriodStart": "2026-06-01T00:00:00Z", "ChargePeriodEnd": "2026-06-02T00:00:00Z", "Tags": map[string]any{"ProjectName": "web", "ProjectId": "prj_web"}},
			{"ServiceName": "Bandwidth", "ChargeCategory": "Usage", "BilledCost": 40.00, "BillingCurrency": "USD", "ConsumedQuantity": 200.0, "ConsumedUnit": "GB", "ChargePeriodStart": "2026-06-01T00:00:00Z", "ChargePeriodEnd": "2026-06-02T00:00:00Z", "Tags": map[string]any{"ProjectName": "web", "ProjectId": "prj_web"}},
			{"ServiceName": "Functions", "ChargeCategory": "Usage", "BilledCost": 3.00, "BillingCurrency": "USD", "ConsumedQuantity": 50000.0, "ConsumedUnit": "invocations", "ChargePeriodStart": "2026-06-01T00:00:00Z", "ChargePeriodEnd": "2026-06-02T00:00:00Z", "Tags": map[string]any{"ProjectName": "api", "ProjectId": "prj_api"}},
		},
		Webhooks: []map[string]any{
			{"id": "hook_deploys", "url": "https://hooks.example.com/vercel", "events": []any{"deployment.created", "deployment.succeeded", "deployment.error"}, "projectIds": []any{"prj_web"}, "createdAt": int64(1716000000000), "updatedAt": int64(1716206800000)},
			{"id": "hook_all", "url": "https://hooks.example.com/audit", "events": []any{"deployment.error"}, "createdAt": int64(1716010000000), "updatedAt": int64(1716010000000)},
		},
		EdgeConfigs: []map[string]any{
			{"id": "ecfg_flags", "slug": "flags", "itemCount": 2, "sizeInBytes": 128, "createdAt": int64(1716000000000), "updatedAt": int64(1716206800000)},
			{"id": "ecfg_redirects", "slug": "redirects", "itemCount": 0, "createdAt": int64(1716010000000), "updatedAt": int64(1716010000000)},
		},
		EdgeConfigItems: map[string][]map[string]any{
			"ecfg_flags": {
				{"key": "maintenance_mode", "value": false},
				{"key": "new_checkout", "value": map[string]any{"enabled": true, "rollout": 25}},
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

	mux.HandleFunc("GET /v2/user", requireBearer(func(w http.ResponseWriter, _ *http.Request) {
		writeJSON(w, http.StatusOK, map[string]any{"user": o.User})
	}))
	mux.HandleFunc("GET /v2/teams/{id}/members", requireBearer(func(w http.ResponseWriter, _ *http.Request) {
		writeJSON(w, http.StatusOK, map[string]any{
			"members":    o.TeamMembers,
			"pagination": map[string]any{"count": len(o.TeamMembers)},
		})
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
		page, next := pageByCreated(items, r.URL.Query().Get("until"), r.URL.Query().Get("limit"))
		pag := map[string]any{"count": len(page)}
		if next != nil {
			pag["next"] = *next
		}
		writeJSON(w, http.StatusOK, map[string]any{"deployments": page, "pagination": pag})
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
	mux.HandleFunc("GET /v1/deployments/{id}/checks", requireBearer(func(w http.ResponseWriter, _ *http.Request) {
		writeJSON(w, http.StatusOK, map[string]any{"checks": o.DeploymentChecks})
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
	mux.HandleFunc("GET /v1/projects/{idOrName}/crons", requireBearer(func(w http.ResponseWriter, _ *http.Request) {
		writeJSON(w, http.StatusOK, o.ProjectCrons)
	}))
	mux.HandleFunc("GET /v9/projects/{idOrName}/custom-environments", requireBearer(func(w http.ResponseWriter, _ *http.Request) {
		writeJSON(w, http.StatusOK, map[string]any{"environments": o.CustomEnvironments})
	}))

	mux.HandleFunc("GET /v3/deployments/{id}/events", requireBearer(func(w http.ResponseWriter, _ *http.Request) {
		writeJSON(w, http.StatusOK, o.BuildEvents)
	}))
	mux.HandleFunc("GET /v1/projects/{projectId}/deployments/{deploymentId}/runtime-logs", requireBearer(func(w http.ResponseWriter, r *http.Request) {
		// Vercel serves runtime logs as an open-ended NDJSON stream: emit the
		// buffered lines, flushing each, then (in hang mode) hold the connection
		// open until the client gives up — exercising the bounded-window read.
		w.Header().Set("Content-Type", "application/x-ndjson")
		flusher, _ := w.(http.Flusher)
		for _, lg := range o.RuntimeLogs {
			b, _ := json.Marshal(lg)
			_, _ = w.Write(append(b, '\n'))
			if flusher != nil {
				flusher.Flush()
			}
		}
		if o.RuntimeLogsHang {
			<-r.Context().Done() // never close on our own; the client's window must
		}
	}))
	mux.HandleFunc("GET /v1/env", requireBearer(func(w http.ResponseWriter, _ *http.Request) {
		writeJSON(w, http.StatusOK, map[string]any{"envs": o.SharedEnv})
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

	mux.HandleFunc("GET /v5/domains", requireBearer(func(w http.ResponseWriter, _ *http.Request) {
		writeJSON(w, http.StatusOK, map[string]any{
			"domains":    o.Domains,
			"pagination": map[string]any{"count": len(o.Domains)},
		})
	}))
	mux.HandleFunc("GET /v5/domains/{domain}", requireBearer(func(w http.ResponseWriter, r *http.Request) {
		name := r.PathValue("domain")
		for _, d := range o.Domains {
			if d["name"] == name {
				writeJSON(w, http.StatusOK, map[string]any{"domain": d})
				return
			}
		}
		writeErr(w, http.StatusNotFound, "not_found", "domain not found: "+name)
	}))
	mux.HandleFunc("GET /v6/domains/{domain}/config", requireBearer(func(w http.ResponseWriter, _ *http.Request) {
		writeJSON(w, http.StatusOK, o.DomainConfig)
	}))
	mux.HandleFunc("GET /v5/domains/{domain}/records", requireBearer(func(w http.ResponseWriter, _ *http.Request) {
		writeJSON(w, http.StatusOK, map[string]any{
			"records":    o.DomainRecords,
			"pagination": map[string]any{"count": len(o.DomainRecords)},
		})
	}))
	mux.HandleFunc("GET /v9/certs", requireBearer(func(w http.ResponseWriter, _ *http.Request) {
		ids := make([]string, 0, len(o.Certs))
		for id := range o.Certs {
			ids = append(ids, id)
		}
		sort.Strings(ids)
		certs := make([]map[string]any, 0, len(ids))
		for _, id := range ids {
			certs = append(certs, o.Certs[id])
		}
		writeJSON(w, http.StatusOK, map[string]any{"certs": certs})
	}))
	mux.HandleFunc("GET /v8/certs/{id}", requireBearer(func(w http.ResponseWriter, r *http.Request) {
		id := r.PathValue("id")
		if c, ok := o.Certs[id]; ok {
			writeJSON(w, http.StatusOK, c)
			return
		}
		writeErr(w, http.StatusNotFound, "not_found", "cert not found: "+id)
	}))
	mux.HandleFunc("GET /v2/deployments/{id}/aliases", requireBearer(func(w http.ResponseWriter, _ *http.Request) {
		writeJSON(w, http.StatusOK, map[string]any{
			"aliases":    o.Aliases,
			"pagination": map[string]any{"count": len(o.Aliases)},
		})
	}))

	// --- writes (M6) ---
	mux.HandleFunc("PATCH /v12/deployments/{id}/cancel", requireBearer(func(w http.ResponseWriter, r *http.Request) {
		writeJSON(w, http.StatusOK, map[string]any{"id": r.PathValue("id"), "state": "CANCELED"})
	}))
	mux.HandleFunc("POST /v10/projects/{projectId}/promote/{deploymentId}", requireBearer(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusAccepted)
	}))
	mux.HandleFunc("POST /v1/projects/{projectId}/rollback/{deploymentId}", requireBearer(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusCreated)
	}))
	mux.HandleFunc("POST /v13/deployments", requireBearer(func(w http.ResponseWriter, _ *http.Request) {
		writeJSON(w, http.StatusOK, map[string]any{"id": "dpl_new", "url": "web-new.vercel.app", "readyState": "QUEUED"})
	}))
	mux.HandleFunc("POST /v10/projects/{idOrName}/env", requireBearer(func(w http.ResponseWriter, r *http.Request) {
		// Echo the posted body so tests can assert the wire payload (target/type).
		var body map[string]any
		_ = json.NewDecoder(r.Body).Decode(&body)
		if body == nil {
			body = map[string]any{}
		}
		body["id"] = "env_new"
		writeJSON(w, http.StatusOK, map[string]any{"created": body})
	}))
	mux.HandleFunc("DELETE /v9/projects/{idOrName}/env/{id}", requireBearer(func(w http.ResponseWriter, _ *http.Request) {
		writeJSON(w, http.StatusOK, map[string]any{})
	}))
	mux.HandleFunc("POST /v10/projects/{idOrName}/domains", requireBearer(func(w http.ResponseWriter, _ *http.Request) {
		writeJSON(w, http.StatusOK, map[string]any{"name": "new.example.com", "verified": false,
			"verification": []any{map[string]any{"type": "TXT", "domain": "_vercel.new.example.com", "value": "vc-domain-verify=...", "reason": "pending"}}})
	}))
	mux.HandleFunc("DELETE /v9/projects/{idOrName}/domains/{domain}", requireBearer(func(w http.ResponseWriter, _ *http.Request) {
		writeJSON(w, http.StatusOK, map[string]any{})
	}))
	mux.HandleFunc("POST /v9/projects/{idOrName}/domains/{domain}/verify", requireBearer(func(w http.ResponseWriter, r *http.Request) {
		writeJSON(w, http.StatusOK, map[string]any{"name": r.PathValue("domain"), "verified": true})
	}))
	mux.HandleFunc("POST /v2/domains/{domain}/records", requireBearer(func(w http.ResponseWriter, r *http.Request) {
		var body map[string]any
		_ = json.NewDecoder(r.Body).Decode(&body)
		writeJSON(w, http.StatusOK, map[string]any{"uid": "rec_new", "type": body["type"], "name": body["name"]})
	}))
	mux.HandleFunc("DELETE /v2/domains/{domain}/records/{recordId}", requireBearer(func(w http.ResponseWriter, _ *http.Request) {
		writeJSON(w, http.StatusOK, map[string]any{})
	}))
	mux.HandleFunc("POST /v2/deployments/{id}/aliases", requireBearer(func(w http.ResponseWriter, _ *http.Request) {
		writeJSON(w, http.StatusOK, map[string]any{"uid": "alias_new", "alias": "app.example.com"})
	}))
	mux.HandleFunc("DELETE /v2/aliases/{id}", requireBearer(func(w http.ResponseWriter, _ *http.Request) {
		writeJSON(w, http.StatusOK, map[string]any{"status": "SUCCESS"})
	}))
	mux.HandleFunc("GET /v1/webhooks", requireBearer(func(w http.ResponseWriter, r *http.Request) {
		if pid := r.URL.Query().Get("projectId"); pid != "" {
			filtered := make([]map[string]any, 0, len(o.Webhooks))
			for _, wh := range o.Webhooks {
				if ids, ok := wh["projectIds"].([]any); ok {
					for _, id := range ids {
						if id == pid {
							filtered = append(filtered, wh)
							break
						}
					}
				}
			}
			writeJSON(w, http.StatusOK, filtered)
			return
		}
		writeJSON(w, http.StatusOK, o.Webhooks)
	}))
	mux.HandleFunc("GET /v1/edge-config", requireBearer(func(w http.ResponseWriter, _ *http.Request) {
		writeJSON(w, http.StatusOK, o.EdgeConfigs)
	}))
	mux.HandleFunc("GET /v1/edge-config/{id}/items", requireBearer(func(w http.ResponseWriter, r *http.Request) {
		writeJSON(w, http.StatusOK, o.EdgeConfigItems[r.PathValue("id")])
	}))
	mux.HandleFunc("GET /v1/billing/charges", requireBearer(func(w http.ResponseWriter, _ *http.Request) {
		// FOCUS charges are JSONL; emit one object per line.
		w.Header().Set("Content-Type", "application/x-ndjson")
		for _, c := range o.Charges {
			b, _ := json.Marshal(c)
			_, _ = w.Write(append(b, '\n'))
		}
	}))
	mux.HandleFunc("PATCH /aliases/{id}/protection-bypass", requireBearer(func(w http.ResponseWriter, r *http.Request) {
		var body map[string]any
		_ = json.NewDecoder(r.Body).Decode(&body)
		if _, ok := body["revoke"]; ok {
			writeJSON(w, http.StatusOK, map[string]any{"revoked": true})
			return
		}
		writeJSON(w, http.StatusOK, map[string]any{
			"protectionBypass": map[string]any{"scope": "shareable-link", "secret": "vc-bypass-abc123"},
		})
	}))

	return mux
}

// pageByCreated emulates Vercel's timestamp-cursor pagination: items sorted
// newest-first, filtered to created < until, then the first `limit` returned
// with a `next` cursor (the last item's created) when more remain.
func pageByCreated(items []map[string]any, untilStr, limitStr string) ([]map[string]any, *int64) {
	sorted := append([]map[string]any(nil), items...)
	sort.Slice(sorted, func(i, j int) bool { return asInt64(sorted[i]["created"]) > asInt64(sorted[j]["created"]) })
	if until, err := strconv.ParseInt(untilStr, 10, 64); err == nil && untilStr != "" {
		kept := make([]map[string]any, 0, len(sorted))
		for _, m := range sorted {
			if asInt64(m["created"]) < until {
				kept = append(kept, m)
			}
		}
		sorted = kept
	}
	if limit, err := strconv.Atoi(limitStr); err == nil && limit > 0 && limit < len(sorted) {
		page := sorted[:limit]
		next := asInt64(page[len(page)-1]["created"])
		return page, &next
	}
	return sorted, nil
}

func asInt64(v any) int64 {
	switch n := v.(type) {
	case int64:
		return n
	case int:
		return int64(n)
	case float64:
		return int64(n)
	}
	return 0
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
