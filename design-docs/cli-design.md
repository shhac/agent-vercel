# CLI design: command surface, output, and LLM-first decisions

agent-vercel's command surface, output contract, and LLM-first decisions,
following `lin` and `agent-slack` for conventions (result formats, error hints,
lazy data pulls, Keychain handling).

## Principles

1. **LLM-only.** No interactive prompts, no browser opening, no editors, no
   CI-mode special cases. If a feature exists for a human at a keyboard, it is
   out of scope.
2. **Token economy.** Compact projections by default; raw payloads behind
   `--full`; log/body truncation with explicit markers; `--counts-only` /
   `--limit` where applicable.
3. **Chainability.** Every output carries the ids the next command needs
   (`deployment_id`, `project_id`, `alias`, `domain`).
4. **Structured errors always.** JSON on stderr with `fixable_by` and a hint
   that names the exact follow-up command. Never a bare message.
5. **Cross-project by default.** `deployment list` is org/scope-wide and
   filterable — the single biggest thing the `vercel` CLI cannot do.

## Why not just use the Vercel CLI

This section validates the tool against "why does this need to exist". Each row
is a triage job an agent is plausibly asked to do; the `vercel` CLI column is the
honest state of that tool.

| Triage job | `vercel` CLI | agent-vercel (REST) |
|---|---|---|
| List failed prod deploys across **many** projects, filtered by author/branch/sha/time | ✗ single linked project only | `deployment list --state ERROR --target production --project … --user … --since …` → `GET /v6/deployments` |
| "What is live in prod **right now**", incl. rolling-release canary % | ~ infer from `vercel ls` | `deployment current <project>` → `/v6/deployments` + `/v1/projects/{id}/rolling-release` |
| Historical **build** logs for a past failed deploy, filtered to 5xx | ✗ tail/stream, short window | `deployment logs <id> --status 5xx --limit -1` → `/v3/deployments/{id}/events` |
| **Runtime** logs by level/path/status | ~ weak filtering | `deployment runtime-logs <id> --level error --path /api` → `/v1/.../runtime-logs` |
| **Diff** env vars prod-vs-preview; find the missing one | ✗ pull each, diff by hand | `env diff <project>` → `/v10/projects/{id}/env?decrypt=true` (each var carries `target`) |
| Exact missing DNS record + cert state for a domain | ~ partial | `domain inspect <d>` / `records` / `cert get` → `/v9/projects/{id}/domains/{d}`, `/v5/domains/{d}/records`, `/v8/certs/{id}` |
| Why is this preview 401-ing (deployment protection)? | ✗ | `project protection <id>` (SSO/password/trusted-ips/bypass off `GET /v9/projects/{id}`); alias `protectionBypass` state → `/v2/deployments/{id}/aliases` |
| Is this project under attack / what WAF rules are active? | ✗ | `firewall config\|attack-status\|bypass <project>` → `/v1/security/firewall/...` |
| Why did the bill spike — which resource (units, not $) and which region? | ~ cost only | `billing consumption`, `billing charges --by region` → `/v1/billing/charges` (FOCUS) |
| Purge stale edge cache by tag after a bad deploy | ✗ | `cache purge <project> --tag <t> --yes` → `POST /v1/edge-cache/invalidate-by-tags` |
| Where is this account's log/trace/analytics data drained? | ✗ | `drains list\|get` → `/v1/drains` |
| What routes/rewrites does this project (or deploy) actually serve? | ~ | `project routes <id> --diff` / `deployment routes <id>` → `/v1/projects/{id}/routes`, `/v13/deployments/{id}` |
| Which projects is this domain on / is it mid-transfer? | ✗ | `domain projects` / `domain transfer <domain>` → `/v1/domains/{d}/project-domains`, `/v1/registrar/domains/{d}/transfer` |
| Re-point prod / promote / rollback as an explicit, gated action | ~ `promote`/`rollback` exist, thin | `deployment promote\|rollback <id> --yes`, `alias set <id> <alias> --yes` |

The throughline: the triage-grade data lives in the **REST API**; the `vercel`
binary is for shipping. agent-vercel is read-default, cross-project, history- and
access-aware — and never needs the `vercel` binary at runtime.

Partially addressed by **`billing charges`** (`GET /v1/billing/charges`, FOCUS
format): per-service / per-project billed cost and consumed quantity over a date
range, with `--by` aggregation — this answers "what's driving spend". (Its FOCUS
field shapes — `BilledCost`, `ServiceName`, `Tags.ProjectName`, … — are validated
against the OpenAPI spec only; live validation couldn't reach billing data, as
it needs a billing-role token we didn't have. Treat as spec-validated, not
live-validated.) The
remaining honest gap is **request-level observability** (error rates, traffic,
Web Analytics): those have no clean REST query (dashboard + Drains only), so
agent-vercel does not claim to answer "what's my error rate". The `drains` domain
(`drains list/get`) now surfaces *which* drains are configured and whether one is
failing delivery — the connective tissue for that data leaving the platform — but
reading the drained request-level data itself stays out of scope.

## Global flags

`--scope/-s`, `--auth`, `--format/-f`, `--timeout/-t`, `--debug/-d`, `--full`,
`--max-body-chars`, `--base-url` (hidden) are persistent.

- `--scope <team-slug|id>` — which team to act on (`""` = personal). Maps to the
  `teamId`/`slug` query param on every request.
- `--auth <label>` — which stored credential to authenticate with.

## Command tree

`*` = destructive / state-changing, requires `--yes`.

| Command | Key flags | Gate | Notes |
|---|---|---|---|
| `auth add [label]` | `--form` | | label positional (default "default"); `--form` prompts via OS dialog, else reads `VERCEL_TOKEN`; verifies (best-effort GET /v2/user) and records username; stores in Keychain; never echoes |
| `auth list` (`ls`) | | | label, type, default, username, `secret_status` (keychain/file/missing) — never the secret |
| `auth test` (`whoami`) | | | `GET /v2/user`; resolves + caches username |
| `auth set-default <label>` | | | |
| `auth remove <label>` | | | deletes Keychain entry too |
| `auth import-cli [label]` | | | optional: read a token from `vercel login` (`~/.local/share/com.vercel.cli/auth.json`) |
| `scope list` (`ls`) | | | `GET /v2/teams` (live; names/slugs are used directly, no resolution cache) |
| `scope current` | | | active scope + default credential |
| `scope set-default <slug>` | | | empty arg → personal account |
| `scope member list` | `--limit`, `--cursor`, `--all` | | NDJSON; members of the active team scope — uid, username, email, role, confirmed; `GET /v2/teams/{id}/members`. The scope slug is resolved to a team id first; the personal account has no members |
| `scope member get <id\|email\|username>` | | | one member matched client-side (the endpoint has no per-member GET) |
| `deployment list` | `--project`, `--state`, `--target`, `--custom-env`, `--branch`, `--sha`, `--user`, `--since`, `--until`, `--limit`, `--cursor`, `--all` | | NDJSON; org/scope-wide; `GET /v6/deployments`. `--custom-env` filters client-side (the API has no custom-env param) |
| `deployment get <id\|url>` | | | compact: state, target, creator, commit, urls, timings, `errorCode`; plus build-triage fields surfaced from the same payload — `build_skipped`, `first_branch_deployment`, `queued`, `error_step`/`error_link`/`state_reason`, `source`, and derived `queue_wait_ms`/`build_duration_ms` |
| `deployment checks <id\|url>` | `--blocking`, `--failed` | | NDJSON; CI/integration checks on the deploy — name, status, conclusion, blocking; `GET /v1/deployments/{id}/checks`. Filters are client-side. Answers "what's blocking / what failed" |
| `deployment routes <id\|url>` | | | the compiled routing the deploy runs (redirects/rewrites/headers), read off the `routes[]` of `GET /v13/deployments/{id}` — redirect-loop triage |
| `deployment current <project>` | `--custom-env` | | live prod deployment + rolling-release state; `--custom-env` shows the newest READY deploy in a custom environment |
| `deployment logs <id\|url>` | `--since`, `--until`, `--status`, `--limit`, `--max-body-chars` | | build events; `GET /v3/deployments/{id}/events` |
| `deployment runtime-logs <id\|url>` | `--level`, `--status`, `--path`, `--limit`, `--max-body-chars` | | bounded live tail: collects for the `--timeout` window (default 6s) then returns; `GET /v1/projects/{id}/deployments/{id}/runtime-logs` (NDJSON stream) |
| `deployment promote <id>` | | `--yes` | repoint prod to this deployment |
| `deployment rollback <id>` | | `--yes` | |
| `deployment cancel <id>` | | `--yes` | cancel an in-progress build |
| `deployment redeploy <id>` | | `--yes` | |
| `project list` | `--limit`, `--search` | | NDJSON, compact |
| `project get <id\|name>` | | | settings, framework, latest prod deployment; plus build config (`root_directory`/`output_directory`/`build`+`install`/`ignore` command) and `paused`, surfaced from the same payload |
| `project crons <id\|name>` | | | cron jobs the project runs (path + schedule) and whether crons are enabled; `GET /v1/projects/{id}/crons`. Spec-validated, not live-validated — projection shape may need adjusting against live data |
| `project custom-environments <id\|name>` (`custom-envs`) | | | NDJSON; the project's custom deployment environments — slug, type, branch binding, domains; `GET /v9/projects/{id}/custom-environments`. The discovery counterpart to the `--custom-env` filters on `deployment`/`env` |
| `project protection <id\|name>` | | | deployment-protection posture — which gate is on (Vercel Authentication / Password / Trusted IPs) + scope, and whether an automation bypass exists (never the secret); reads `ssoProtection`/`passwordProtection`/`trustedIps`/`protectionBypass` off `GET /v9/projects/{id}`. Answers "why is my preview/prod URL 401-ing?" |
| `project routes <id\|name>` | `--diff` | | authored CDN routing rules (redirects/rewrites/headers) + version (staging vs live); `--diff` = staged-vs-production. `GET /v1/projects/{id}/routes`. Spec-plausible, not live-validated |
| `env list <project>` | `--environment`, `--git-branch`, `--decrypt`, `--custom-env` | | NDJSON; `GET /v10/projects/{id}/env` |
| `env diff <project>` | `--environments a,b` (default production,preview) | | the killer feature: which keys differ / are missing per env |
| `env get <project> <key>` | `--environment`, `--decrypt` | | |
| `env set <project> <key> <value>` | `--environment`, `--git-branch` | `--yes` | |
| `env rm <project> <key>` | `--environment` | `--yes` | |
| `env pull <project>` | `--environment` (default development), `--out` (default .env), `--git-branch` | | writes decrypted vars to a 0600 dotenv file |
| `env shared list` | `--decrypt` | | NDJSON; the team's shared env vars (defined once, linked into many projects) — key, type, target[], linked projects; values withheld unless `--decrypt`. `GET /v1/env` (team-scoped). Distinct from the per-project `env list`. Spec-validated, not live-validated |
| `env shared get <key\|id>` | `--decrypt` | | one shared var matched by key or id |
| `domain list` | `--limit` | | account domains; `GET /v5/domains` |
| `domain get <domain>` | | | verification challenges, redirect, verified state |
| `domain inspect <domain>` | | | config check: intended vs actual nameservers, misconfig reasons; plus SSL/ACME readiness (`configured_by`, `accepted_challenges`, `recommended_ipv4`/`recommended_cname`) folded in from the same `/config` payload |
| `domain records list/add/rm <domain> …` | `--ttl` (add) | `--yes` (add/rm) | list (`GET /v5/domains/{d}/records`), add (`POST /v2/...`), rm (`DELETE`) — closes the inspect→fix loop |
| `domain verify <domain> --project <p>` | | `--yes` | `POST /v9/projects/{id}/domains/{d}/verify` |
| `domain add <project> <domain>` | `--redirect`, `--git-branch` | `--yes` | |
| `domain rm <project> <domain>` | | `--yes` | |
| `domain cert get <id>` | | | `GET /v8/certs/{id}` (expiry, autoRenew, cns) |
| `domain cert list` | `--expiring <days>` | | NDJSON; the scope's certs for bulk expiry/renewal triage; `--expiring` filters to certs expiring within N days (0 = already expired). `GET /v9/certs`. **Spec-plausible, not live-validated** — Vercel may not expose a scope-wide certs list (cf. the absent scope-wide alias list); if it 404s, use `domain cert get <id>` per-id |
| `domain projects <domain>` | | | NDJSON; the projects an apex is attached to (verified/redirect state) — wrong-project / conflict triage; `GET /v1/domains/{d}/project-domains`. Spec-plausible, not live-validated |
| `domain transfer <domain>` | | | registrar registration/transfer status — the "why is my transfer stuck" signal; `GET /v1/registrar/domains/{d}/transfer`. Spec-plausible, not live-validated |
| `alias list <deployment>` | `--cursor`, `--all` | | `GET /v2/deployments/{id}/aliases`; surfaces `protection_bypass`. (No scope-wide alias-list endpoint exists — `/v4/aliases` is in the OpenAPI spec but 404s live.) |
| `alias set <deployment> <alias>` | | `--yes` | `POST /v2/deployments/{id}/aliases` |
| `alias rm <alias>` | | `--yes` | |
| `alias bypass <alias\|id>` | `--ttl`, `--revoke`, `--regenerate` | `--yes` | `PATCH /aliases/{id}/protection-bypass`; mint/revoke a shareable link to a 401-ing preview |
| `billing charges` | `--from`, `--to`, `--by service\|project\|region` | | `GET /v1/billing/charges` (FOCUS, JSONL); `--by` aggregates billed cost — "what's driving spend" |
| `billing consumption` | `--from`, `--to` | | consumed quantity per service (volume + unit, not just $) — "what resource is the spike"; reuses the FOCUS charges payload |
| `firewall config <project>` | | | active WAF config — enabled, active custom rules, IP-rule count, active managed rulesets; `GET /v1/security/firewall/config/active`. Spec-documented, not live-validated |
| `firewall attack-status <project>` | `--since <days>` | | active-attack / DDoS anomalies; `GET /v1/security/firewall/attack-status`. Spec-documented, not live-validated |
| `firewall bypass <project>` | | | system-bypass rules (raw); `GET /v1/security/firewall/bypass`. Spec-documented, not live-validated (the sibling `/firewall/events` 404s live) |
| `cache purge <project>` | `--tag <t>` (repeatable, max 16) | `--yes` | invalidate CDN/runtime/data-cache entries by tag (background revalidate); `POST /v1/edge-cache/invalidate-by-tags`. **State-changing** |
| `webhook list` | `--project` | | NDJSON; the scope's webhooks — endpoint url, subscribed events, target projects; `GET /v1/webhooks`. Answers "is the deploy notification / CI integration wired up, and for what events" |
| `drains list` | `--project` | | NDJSON; the scope's observability drains (log/trace/analytics/speed-insights) — id, name, status, types, project count; flags failing/disabled drains. Delivery URL omitted from compact (can embed a token). `GET /v1/drains`. Spec-documented, not live-validated |
| `drains get <id>` | | | one drain by id; `GET /v1/drains/{id}`. Spec-documented, not live-validated |
| `edge-config list` (`edge`) | | | NDJSON; the scope's Edge Configs — id, slug, item count, size; `GET /v1/edge-config` |
| `edge-config items <id>` | | | NDJSON; the key/value items in one Edge Config (non-secret config: feature flags, redirects); `GET /v1/edge-config/{id}/items` |
| `api call <METHOD> <path>` | `--query`, `--body <json\|->` | `--yes` if non-GET | raw REST escape hatch |
| `config get/set/list/unset` | | | persisted defaults for `format`/`max-body-chars`/`timeout` (flag > config > built-in), applied in PersistentPreRunE; unknown keys and `auth`/`scope` are rejected (those use set-default) |
| `usage`, `<domain> usage` | | | self-docs |

**New-domain placement.** `firewall`, `cache`, and `drains` are top-level
domains rather than subcommands of `project`: each is a distinct object family
with its own endpoint namespace (`/v1/security/firewall/*`, `/v1/edge-cache/*`,
`/v1/drains`) and its own triage job, so nesting them under `project` would bury
a rich surface a level deeper for no gain (and force three-level paths like
`project firewall config`). Deployment-protection, by contrast, *is* a project
setting, so `project protection` stays a `project` subcommand and the
"why-is-this-blocked" story spans `firewall` + `project protection`, linked here
rather than in the tree. Several reads are marked **spec-documented, not
live-validated** — wired straight from the OpenAPI spec and validated only
against fixtures, following the same caveat as `/v9/certs`; see
`behavior-reference.md`.

## Mutation gating (`--yes`)

**Decision: gate anything that changes remote state.** Vercel actions are
production-facing (promoting a deployment shifts live traffic; removing an env
var can break a build), so the bar is lower than agent-slack's "destructive
only".

Gated: `deployment promote|rollback|cancel|redeploy`, `env set|rm`,
`domain add|rm|verify`, `domain records add|rm`, `alias set|rm|bypass`,
`cache purge`, and `api call` with a non-`GET` method.

`cache purge` is the one mutation in the new surface — it changes what live
traffic sees (stale cache), so it gates like any other state change and refuses
without at least one `--tag`.

Without `--yes`, a gated command returns `fixable_by: human` describing exactly
what would happen, and a hint with the rerun command including `--yes`.

Ungated: every read (`list`, `get`, `current`, `logs`, `runtime-logs`, `diff`,
`inspect`, `records`, `cert get/list`, `routes`, `protection`, `consumption`, and
the `firewall`/`drains` reads), plus `auth`/`scope` local management and `api
call` GETs.

## Output contract

- **Lists → NDJSON** (one object per line), trailing
  `{"@pagination":{"has_more":true,"next_cursor":"…"}}` when more exist. Vercel
  paginates by timestamp cursor (`pagination.next`), normalized into
  `next_cursor`.
- **Single resources → pretty JSON.** `--format json|yaml|jsonl` overrides.
- **Compact projections by default; `--full` returns the raw API payload.** The
  raw deployment object is huge (project settings, attribution, checks); the
  compact projection is the biggest token win.
  - deployment: `id, name, project_id, state, target, ready_substate, url,
    inspector_url, branch, sha, commit_message, creator, created (RFC3339),
    error_code, error_message` (+ build-triage fields: `build_skipped`,
    `first_branch_deployment`, `queued`, `error_step`/`error_link`/`state_reason`,
    `source`, `queue_wait_ms`, `build_duration_ms`)
  - project: `id, name, framework, latest_prod_deployment, updated (RFC3339)`
    (+ build config `root_directory`/`output_directory`/`build`+`install`/`ignore`
    command, and `paused`)
  - env var: `id, key, target[], type, git_branch, comment` (+ `value` only with
    `--decrypt`)
  - domain: `name, apex, verified, verification[], redirect, intended_nameservers`
    (`domain inspect` adds `configured_by`, `accepted_challenges`,
    `recommended_ipv4`/`recommended_cname`)
- **Timestamps:** Vercel returns epoch-ms numbers; compact projections emit
  RFC3339 strings. `--full` keeps the raw ms.
- **Truncation:** `--max-body-chars` (default ~4000 for logs; `-1` unlimited);
  truncated content ends with `\n…`.
- All confirmations are JSON.

## Meta-lines

NDJSON trailers use `@`-prefixed keys, family convention:

- `@pagination` — `{has_more, next_cursor}` when more pages exist.
- `@referenced_projects` — `{prj_…: {id, name}}` when rows reference projects by
  id (e.g. a cross-project `deployment list`), so the agent needn't re-resolve.
- `@unresolved` — `[string]` when a batch `get` had args that didn't resolve.

## Errors and hints

`{error, fixable_by, hint?}` on stderr, exit code 1. Conventions from `lin`:

- Hints name the exact next command: `run 'agent-vercel auth add'`, `pass
  --yes to promote`, `run 'agent-vercel scope list' to see scopes`.
- Vercel returns `{error:{code,message}}` with an HTTP status; the client maps:
  400/404/422 → `agent`; 401/403 → `human`; 402 (payment) → `human` with a
  pointed hint; 429/5xx/network → `retry`.
- Ambiguous `--auth` selectors enumerate the candidate labels.
- Unknown subcommands return a structured `fixable_by: agent` error listing the
  valid subcommands, not bare cobra help.

## `api call` escape hatch

`agent-vercel api call GET /v6/deployments --query 'state=ERROR&limit=5'` or
`api call POST /v2/deployments/{id}/aliases --body '<json|->' --yes`. Posts to
any Vercel REST endpoint with the stored credential and active scope, printing
the raw response. GET is ungated; non-GET requires `--yes` (REST mutations are
real even though the caller typed the method). This is `lin api query` /
`agent-slack api call` translated to Vercel's REST model.

## usage system

- `agent-vercel usage`: ~1k-token overview — domains, the auth/scope split,
  target syntax, id formats, pagination, truncation, error contract, gating,
  setup. (Implemented in the scaffold.)
- `agent-vercel <domain> usage` (deployment, env, domain, …): per-domain detail
  pages written for an LLM reader.
- Ship `skills/agent-vercel/SKILL.md` in-repo, kept in sync with the surface.

## Out of scope (decisions)

- **No `vercel`-binary wrapping** at runtime (only the optional `auth
  import-cli` reads its auth file).
- **No resolution cache.** Unlike `agent-slack` (opaque Slack IDs must be
  resolved to names to be readable), Vercel project names and team slugs are
  first-class API identifiers the endpoints accept directly — like `lin`, there
  is no id-resolution step to cache. (`config` persists ordinary settings.)
- **No request-level observability** (error rates, traffic, Web Analytics): no
  clean REST query (dashboard + Drains only). Billing/usage *cost* and *volume*
  are covered by `billing charges`/`billing consumption`; the `drains` domain
  lists *which* drains are configured (the only REST handle on this data
  leaving the platform), but reading the drained request-level data itself stays
  out of scope.
- **No self-update** command; distribution is brew / `go install`.
- **No interactive terminal anything.** Native OS dialog (`auth add --form`)
  is the one sanctioned human interaction, for secret entry only.
