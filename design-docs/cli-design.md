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
| Exact missing DNS record + cert state for a domain | ~ partial | `domain inspect <d>` / `records` / `cert` → `/v9/projects/{id}/domains/{d}`, `/v5/domains/{d}/records`, `/v8/certs/{id}` |
| Why is this preview 401-ing (deployment protection)? | ✗ | alias `protectionBypass` state → `/v2/deployments/{id}/aliases` |
| Re-point prod / promote / rollback as an explicit, gated action | ~ `promote`/`rollback` exist, thin | `deployment promote\|rollback <id> --yes`, `alias set <id> <alias> --yes` |

The throughline: the triage-grade data lives in the **REST API**; the `vercel`
binary is for shipping. agent-vercel is read-default, cross-project, history- and
access-aware — and never needs the `vercel` binary at runtime.

Caveat documented honestly: **usage / billing / Web-Analytics** have no clean
REST query (they live behind the dashboard and Log Drains). agent-vercel does not
pretend to answer "what's driving my usage spike" in v1; a `log-drain` domain is
a possible later addition. This is called out so the tool never implies coverage
it lacks.

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
| `auth add` | `--label` (default), `--form` | | `--form` prompts via OS dialog, else reads `VERCEL_TOKEN`; stores in Keychain; never echoes |
| `auth list` (`ls`) | | | label, type, default, username, `secret_status` (keychain/file/missing) — never the secret |
| `auth test` (`whoami`) | | | `GET /v2/user`; resolves + caches username |
| `auth set-default <label>` | | | |
| `auth remove <label>` | | | deletes Keychain entry too |
| `auth import-cli` | | | optional: read a token from `vercel login` (`~/.local/share/com.vercel.cli/auth.json`) |
| `scope list` (`ls`) | | | `GET /v2/teams` (live; names/slugs are used directly, no resolution cache) |
| `scope current` | | | active scope + default credential |
| `scope set-default <slug>` | | | empty arg → personal account |
| `deployment list` | `--project`, `--state`, `--target`, `--branch`, `--sha`, `--user`, `--since`, `--until`, `--limit`, `--app` | | NDJSON; org/scope-wide; `GET /v6/deployments` |
| `deployment get <id\|url>` | | | compact: state, target, creator, commit, urls, timings, `errorCode` |
| `deployment current <project>` | | | live prod deployment + rolling-release state |
| `deployment logs <id\|url>` | `--since`, `--until`, `--status`, `--limit`, `--max-body-chars` | | build events; `GET /v3/deployments/{id}/events` |
| `deployment runtime-logs <id\|url>` | `--level`, `--status`, `--path`, `--max-body-chars` | | `GET /v1/projects/{id}/deployments/{id}/runtime-logs` |
| `deployment promote <id>` | | `--yes` | repoint prod to this deployment |
| `deployment rollback <id>` | | `--yes` | |
| `deployment cancel <id>` | | `--yes` | cancel an in-progress build |
| `deployment redeploy <id>` | | `--yes` | |
| `project list` | `--limit`, `--search` | | NDJSON, compact |
| `project get <id\|name>` | | | settings, framework, latest prod deployment |
| `env list <project>` | `--environment`, `--git-branch`, `--decrypt`, `--custom-env` | | NDJSON; `GET /v10/projects/{id}/env` |
| `env diff <project>` | `--environments a,b` (default production,preview) | | the killer feature: which keys differ / are missing per env |
| `env get <project> <key>` | `--environment`, `--decrypt` | | |
| `env set <project> <key> <value>` | `--environment`, `--git-branch` | `--yes` | |
| `env rm <project> <key>` | `--environment` | `--yes` | |
| `domain list` | `--limit` | | account domains; `GET /v5/domains` |
| `domain get <domain>` | | | verification challenges, redirect, verified state |
| `domain inspect <domain>` | | | config check: intended vs actual nameservers, misconfig reasons |
| `domain records <domain>` | `--limit` | | DNS records; `GET /v5/domains/{d}/records` |
| `domain verify <domain> --project <p>` | | `--yes` | `POST /v9/projects/{id}/domains/{d}/verify` |
| `domain add <project> <domain>` | `--redirect`, `--git-branch` | `--yes` | |
| `domain rm <project> <domain>` | | `--yes` | |
| `domain cert <id>` | | | `GET /v8/certs/{id}` (expiry, autoRenew, cns) |
| `alias list <deployment>` | | | `GET /v2/deployments/{id}/aliases`; surfaces `protectionBypass` |
| `alias set <deployment> <alias>` | | `--yes` | `POST /v2/deployments/{id}/aliases` |
| `alias rm <alias>` | | `--yes` | |
| `api call <METHOD> <path>` | `--query`, `--body <json\|->` | `--yes` if non-GET | raw REST escape hatch |
| `config get/set/list/unset` | | | persists settings in `config.json` |
| `usage`, `<domain> usage` | | | self-docs |

## Mutation gating (`--yes`)

**Decision: gate anything that changes remote state.** Vercel actions are
production-facing (promoting a deployment shifts live traffic; removing an env
var can break a build), so the bar is lower than agent-slack's "destructive
only".

Gated: `deployment promote|rollback|cancel|redeploy`, `env set|rm`,
`domain add|rm|verify`, `alias set|rm`, and `api call` with a non-`GET` method.

Without `--yes`, a gated command returns `fixable_by: human` describing exactly
what would happen, and a hint with the rerun command including `--yes`.

Ungated: every read (`list`, `get`, `current`, `logs`, `runtime-logs`, `diff`,
`inspect`, `records`, `cert`), plus `auth`/`scope` local management and `api
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
    error_code, error_message`
  - project: `id, name, framework, latest_prod_deployment, updated (RFC3339)`
  - env var: `id, key, target[], type, git_branch, comment` (+ `value` only with
    `--decrypt`)
  - domain: `name, apex, verified, verification[], redirect, intended_nameservers`
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
- **No usage/billing/analytics** answers in v1 (no clean REST query; see the
  caveat above). Possible later `log-drain` domain.
- **No self-update** command; distribution is brew / `go install`.
- **No interactive terminal anything.** Native OS dialog (`auth add --form`)
  is the one sanctioned human interaction, for secret entry only.
