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

## deployment

| Command | Key flags |
|---|---|
| `deployment list` | `--project --state --target --custom-env --branch --sha --user --since --until --limit --cursor --all` |
| `deployment get <id\|url>` | |
| `deployment checks <id\|url>` | `--blocking --failed`; CI/integration checks on the deploy (name, status, conclusion, blocking) — what is blocking or failing it |
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
| `project get <id\|name>` | settings, framework, latest prod deployment |
| `project crons <id\|name>` | cron jobs the project runs (path + schedule) and whether crons are enabled |

## env

| Command | Key flags |
|---|---|
| `env list <project>` | `--environment --git-branch --decrypt --custom-env` |
| `env diff <project>` | `--environments a,b` (default production,preview) |
| `env get <project> <key>` | `--environment --decrypt` |
| `env set <project> <key> <value>` * | `--environment --git-branch` |
| `env rm <project> <key>` * | `--environment` |
| `env pull <project>` | `--environment` (default development), `--out` (default .env), `--git-branch` — write decrypted vars to a dotenv file |

Env vars are the *application's* config (legitimately readable with `--decrypt`),
distinct from the agent-vercel access token (never readable).

## domain

| Command | Key flags |
|---|---|
| `domain list` | `--limit` |
| `domain get <domain>` | verification challenges, verified state |
| `domain inspect <domain>` | nameserver / config check, misconfig reasons |
| `domain records list <domain>` | list DNS records |
| `domain records add <domain> <type> <name> <value>` * | `--ttl`; add a DNS record |
| `domain records rm <domain> <record-id>` * | remove a DNS record |
| `domain verify <domain> --project <p>` * | |
| `domain add <project> <domain>` * | `--redirect --git-branch` |
| `domain rm <project> <domain>` * | |
| `domain cert <id>` | cert expiry / autoRenew / cns |

## alias

| Command | Key flags |
|---|---|
| `alias list <deployment>` | the deployment's aliases; surfaces `protection_bypass` (why a preview 401s) |
| `alias set <deployment> <alias>` * | repoint |
| `alias rm <alias>` * | |
| `alias bypass <alias\|id>` * | `--ttl`, `--revoke <secret>`, `--regenerate` — mint/revoke a shareable bypass link for a gated preview |

## billing

| Command | Notes |
|---|---|
| `billing charges` | `--from`/`--to` (date, RFC3339, or duration like 30d; default last 30d), `--by service\|project` to aggregate billed cost — "what's driving spend". `GET /v1/billing/charges` (FOCUS) |

## api (escape hatch)

| Command | Notes |
|---|---|
| `api call <METHOD> <path>` | `--query 'k=v&…'`, `--body <json\|->`. GET ungated; non-GET needs `--yes` |

## config

| Command | Notes |
|---|---|
| `config get\|set\|list\|unset` | persisted defaults for `format`, `max-body-chars`, `timeout` (precedence: flag > config > built-in). Auth/scope defaults use `auth`/`scope set-default`, not config |
