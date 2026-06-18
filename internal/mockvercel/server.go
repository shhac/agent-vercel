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
	mux.HandleFunc("POST /v10/projects/{idOrName}/domains", requireBearer(func(w http.ResponseWriter, r *http.Request) {
		var body map[string]any
		_ = json.NewDecoder(r.Body).Decode(&body)
		resp := map[string]any{"name": "new.example.com", "verified": false,
			"verification": []any{map[string]any{"type": "TXT", "domain": "_vercel.new.example.com", "value": "vc-domain-verify=...", "reason": "pending"}}}
		// Reflect the optional posted fields so a test can assert body shaping.
		if v, ok := body["redirect"]; ok {
			resp["redirect"] = v
		}
		if v, ok := body["gitBranch"]; ok {
			resp["gitBranch"] = v
		}
		writeJSON(w, http.StatusOK, resp)
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
