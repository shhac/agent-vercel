# agent-vercel command reference

`*` = destructive / state-changing — requires `--yes`.

## Global flags

- `--auth <label>` — which stored credential to use.
- `--scope <team-slug|id>` / `-s` — which team to act on (omit = personal/default).
- `--format <json|yaml|jsonl>` / `-f` — override output format.
- `--full` — raw API payload instead of the compact projection.
- `--max-body-chars <n>` — truncate long log/body fields (`0` = per-command default, `-1` = unlimited).
- `--timeout <ms>` / `-t`, `--debug` / `-d`.
- list commands also take `--cursor <next_cursor>` (page from a prior `@pagination`) and `--all` (follow all pages, capped).
- deployment targets accept a `dpl_…` id, a bare host (`web-abc.vercel.app`), or a full URL (`https://web-abc.vercel.app/path`).
- `agent-vercel usage --json` emits a machine-readable command catalog (domains, subcommands, descriptions).

## auth (credential — the secret)

| Command | Notes |
|---|---|
| `auth add [label]` | stores a token in the Keychain under `label` (default "default"); `--form` prompts via OS dialog, else reads `$VERCEL_TOKEN`; verifies it (GET /v2/user) and records the username; never echoes the secret |
| `auth list` (`ls`) | label, type, default, username, `secret_status` (keychain/file/missing) — never the secret |
| `auth test` (`whoami`) | verifies via `GET /v2/user` |
| `auth set-default <label>` | |
| `auth remove <label>` | also deletes the Keychain entry |
| `auth import-cli [label]` | optional: import a token from `vercel login` |

There is no command that prints the secret. That is intentional.

## scope

| Command | Notes |
|---|---|
| `scope list` (`ls`) | teams the credential can reach (`GET /v2/teams`) |
| `scope current` | active scope + default credential |
| `scope set-default <slug>` | empty arg → personal account |
| `scope member list` | `--limit --cursor --all`; members of the active team scope — uid, username, email, role, confirmed. Needs a team scope (not the personal account) |
| `scope member get <id\|email\|username>` | one member, matched client-side |

## deployment

| Command | Key flags |
|---|---|
| `deployment list` | `--project --state --target --custom-env --branch --sha --user --since --until --limit --cursor --all` |
| `deployment get <id\|url>` | compact projection also carries build-triage fields: `build_skipped`, `first_branch_deployment`, `queued`, `error_step`/`error_link`/`state_reason`, `source`, `queue_wait_ms`, `build_duration_ms` |
| `deployment checks <id\|url>` | `--blocking --failed`; CI/integration checks on the deploy (name, status, conclusion, blocking) — what is blocking or failing it |
| `deployment routes <id\|url>` | the compiled routing the deploy runs (redirects/rewrites/headers) — redirect-loop triage |
| `deployment current <project>` | live prod deployment + rolling-release; `--custom-env <slug>` shows newest READY in a custom env |
| `deployment logs <id\|url>` | `--status --since --until --limit --max-body-chars` (build logs) |
| `deployment runtime-logs <id\|url>` | `--level --status --path --limit`; a bounded live tail — collects logs for the `--timeout` window (default 6s) then returns |
| `deployment promote <id>` * | repoint prod |
| `deployment rollback <id>` * | |
| `deployment cancel <id>` * | cancel in-progress build |
| `deployment redeploy <id>` * | |

`--state`: `BUILDING,ERROR,INITIALIZING,QUEUED,READY,CANCELED`.
`--target`: usually `production`. `--since/--until`: durations (`24h`, `7d`) or dates.

## project

| Command | Key flags |
|---|---|
| `project list` | `--limit --search` |
| `project get <id\|name>` | settings, framework, latest prod deployment; also build config (`root_directory`/`output_directory`/`build_command`/`install_command`/`ignore_command`) and `paused` |
| `project crons <id\|name>` | cron jobs the project runs (path + schedule) and whether crons are enabled |
| `project custom-environments <id\|name>` (`custom-envs`) | the project's custom deployment environments — slug, type, branch binding, domains; discovery counterpart to `--custom-env` |
| `project protection <id\|name>` | deployment protection — which gate is on (Vercel Authentication / Password / Trusted IPs) and scope, plus whether an automation bypass exists (never the secret). Answers "why is my preview/prod URL 401-ing?" |
| `project routes <id\|name>` | `--diff`; authored CDN routing rules (redirects/rewrites/headers) with staged-vs-live diff; `GET /v1/projects/{id}/routes`. Spec-plausible, not live-validated |

## env

| Command | Key flags |
|---|---|
| `env list <project>` | `--environment --git-branch --decrypt --custom-env` |
| `env diff <project>` | `--environments a,b` (default production,preview) |
| `env get <project> <key>...` | `--environment --decrypt`; 1..N keys, project scope is fixed; one NDJSON record or `@unresolved` per key in input order |
| `env set <project> <key> <value>` * | `--environment --git-branch` |
| `env rm <project> <key>` * | `--environment` |
| `env pull <project>` | `--environment` (default development), `--out` (default .env), `--git-branch` — write decrypted vars to a dotenv file |
| `env shared list` | `--decrypt`; the team's shared env vars (defined once, linked into many projects) — key, type, target, linked projects. Distinct from per-project `env list` |
| `env shared get <key\|id>` | `--decrypt`; one shared var by key or id |

Env vars are the *application's* config (legitimately readable with `--decrypt`),
distinct from the agent-vercel access token (never readable).

## domain

| Command | Key flags |
|---|---|
| `domain list` | `--cursor`, `--all` |
| `domain get <domain>` | verification challenges, verified state |
| `domain inspect <domain>` | nameserver / config check, misconfig reasons; plus SSL/ACME readiness (`configured_by`, `accepted_challenges`, `recommended_ipv4`/`recommended_cname`) |
| `domain records list <domain>` | list DNS records |
| `domain records add <domain> <type> <name> <value>` * | `--ttl`; add a DNS record |
| `domain records rm <domain> <record-id>` * | remove a DNS record |
| `domain verify <domain> --project <p>` * | |
| `domain add <project> <domain>` * | `--redirect --git-branch` |
| `domain rm <project> <domain>` * | |
| `domain cert get <id>...` | cert expiry / autoRenew / cns; 1..N cert ids |
| `domain cert list` | `--expiring <days>` filters to certs expiring within N days (0 = expired); bulk renewal triage. Spec-plausible, not live-validated — falls back to `domain cert get <id>` if the list 404s |
| `domain projects <domain>` | the projects an apex is attached to (verified/redirect state) — wrong-project / conflict triage; `GET /v1/domains/{d}/project-domains`. Spec-plausible, not live-validated |
| `domain transfer <domain>` | registrar registration/transfer status ("why is my transfer stuck"); `GET /v1/registrar/domains/{d}/transfer`. Spec-plausible, not live-validated |

## alias

| Command | Key flags |
|---|---|
| `alias list <deployment>` | the deployment's aliases; surfaces `protection_bypass` (why a preview 401s) |
| `alias set <deployment> <alias>` * | repoint |
| `alias rm <alias>` * | |
| `alias bypass <alias\|id>` * | `--ttl`, `--revoke <secret>`, `--regenerate` — mint/revoke a shareable bypass link for a gated preview |

## firewall

Read-only Vercel Firewall (WAF) inspection; all take a `<project>` and are
spec-plausible, not live-validated. `--full` returns the raw API object.

| Command | Notes |
|---|---|
| `firewall config <project>` | active WAF config — enabled state, active custom rules, IP-rule count, active managed rulesets; `GET /v1/security/firewall/config/active` |
| `firewall attack-status <project>` | `--since <days>`; active-attack / DDoS anomalies; `GET /v1/security/firewall/attack-status` |
| `firewall bypass <project>` | system-bypass rules (sources allowed to skip the WAF), printed raw; `GET /v1/security/firewall/bypass` |

## cache

| Command | Notes |
|---|---|
| `cache purge <project>` * | `--tag <t>` (repeatable, max 16); invalidate CDN/runtime/data-cache entries by tag (background revalidate); `POST /v1/edge-cache/invalidate-by-tags`. Requires at least one `--tag` |

## billing

| Command | Notes |
|---|---|
| `billing charges` | `--from`/`--to` (date, RFC3339, or duration like 30d; default last 30d), `--by service\|project\|region` to aggregate billed cost — "what's driving spend". `GET /v1/billing/charges` (FOCUS) |
| `billing consumption` | `--from`/`--to`; consumed quantity per service (volume + unit, not just $) — "what resource is the spike". Reuses the FOCUS charges payload |

## webhook

| Command | Notes |
|---|---|
| `webhook list` | `--project <id>` to filter; the scope's webhooks — url, subscribed events, target projects. "Is the deploy notification / CI integration wired up" |

## drains

Where observability data (log/trace/analytics/speed-insights) is shipped — the
only REST handle on it. Spec-plausible, not live-validated.

| Command | Notes |
|---|---|
| `drains list` | `--project <id>` to filter; the scope's drains — id, name, status, disabled, schema types, project count. Flags failing/disabled drains. The delivery URL is omitted from compact (can embed a token) — use `--full`. `GET /v1/drains` |
| `drains get <id>` | one drain by id; `GET /v1/drains/{id}` |

## edge-config (`edge`)

| Command | Notes |
|---|---|
| `edge-config list` | the scope's Edge Configs — id, slug, item count, size |
| `edge-config items <id>` | the key/value items in one Edge Config (non-secret config: feature flags, redirects, maintenance toggles) |

## api (escape hatch)

| Command | Notes |
|---|---|
| `api call <METHOD> <path>` | `--query 'k=v&…'`, `--body <json\|->`. GET ungated; non-GET needs `--yes` |

## config

| Command | Notes |
|---|---|
| `config get <key>...` | 1..N config keys; NDJSON one record (or `@unresolved`) per key in input order |
| `config set\|list\|unset` | persisted defaults for `format`, `max-body-chars`, `timeout` (precedence: flag > config > built-in). Auth/scope defaults use `auth`/`scope set-default`, not config |
