# agent-vercel command reference

`*` = destructive / state-changing — requires `--yes`.

## Global flags

- `--auth <label>` — which stored credential to use.
- `--scope <team-slug|id>` / `-s` — which team to act on (omit = personal/default).
- `--format <json|yaml|jsonl>` / `-f` — override output format.
- `--full` — raw API payload instead of the compact projection.
- `--max-body-chars <n>` — truncate long log/body fields (`-1` = unlimited).
- `--timeout <ms>` / `-t`, `--debug` / `-d`.
- list commands also take `--cursor <next_cursor>` (page from a prior `@pagination`) and `--all` (follow all pages, capped).

## auth (credential — the secret)

| Command | Notes |
|---|---|
| `auth add --label <l>` | stores a token in the Keychain; `--form` prompts via OS dialog, else reads `$VERCEL_TOKEN`; never echoes it |
| `auth list` (`ls`) | label, type, default, username, `secret_status` (keychain/file/missing) — never the secret |
| `auth test` (`whoami`) | verifies via `GET /v2/user` |
| `auth set-default <label>` | |
| `auth remove <label>` | also deletes the Keychain entry |
| `auth import-cli` | optional: import a token from `vercel login` |

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
| `deployment list` | `--project --state --target --branch --sha --user --since --until --limit` |
| `deployment get <id\|url>` | |
| `deployment current <project>` | live prod deployment + rolling-release state |
| `deployment logs <id\|url>` | `--status --since --until --limit --max-body-chars` (build logs) |
| `deployment runtime-logs <id\|url>` | `--level --status --path` (runtime logs) |
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

## env

| Command | Key flags |
|---|---|
| `env list <project>` | `--environment --git-branch --decrypt --custom-env` |
| `env diff <project>` | `--environments a,b` (default production,preview) |
| `env get <project> <key>` | `--environment --decrypt` |
| `env set <project> <key> <value>` * | `--environment --git-branch` |
| `env rm <project> <key>` * | `--environment` |

Env vars are the *application's* config (legitimately readable with `--decrypt`),
distinct from the agent-vercel access token (never readable).

## domain

| Command | Key flags |
|---|---|
| `domain list` | `--limit` |
| `domain get <domain>` | verification challenges, verified state |
| `domain inspect <domain>` | nameserver / config check, misconfig reasons |
| `domain records <domain>` | DNS records |
| `domain verify <domain> --project <p>` * | |
| `domain add <project> <domain>` * | `--redirect --git-branch` |
| `domain rm <project> <domain>` * | |
| `domain cert <id>` | cert expiry / autoRenew / cns |

## alias

| Command | Key flags |
|---|---|
| `alias list <deployment>` | surfaces `protectionBypass` (why a preview 401s) |
| `alias set <deployment> <alias>` * | repoint |
| `alias rm <alias>` * | |

## api (escape hatch)

| Command | Notes |
|---|---|
| `api call <METHOD> <path>` | `--query 'k=v&…'`, `--body <json\|->`. GET ungated; non-GET needs `--yes` |

## config

| Command | Notes |
|---|---|
| `config get\|set\|list\|unset` | persists ordinary settings in config.json |
