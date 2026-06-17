# agent-vercel

Vercel CLI for AI agents — a token-efficient, structured-output tool for
**triaging and (carefully) acting on** Vercel from an LLM agent. It belongs to
the `agent-*` CLI family (`agent-slack`, `agent-stripe`, `agent-postmark`, `lin`,
…), sharing their conventions, output contract, and credential handling.

> **Status:** feature-complete. The full command surface (auth, scope,
> deployment, project, env, domain, alias, billing, api, config) is implemented
> and tested against a fixture Vercel server (`internal/mockvercel`). See
> [`design-docs/`](design-docs/) for design decisions.

## Why this and not the `vercel` CLI

The `vercel` CLI is a deploy-loop tool scoped to one linked project, with a thin
read surface. The triage questions an agent gets — *which deploy is live across
these projects, why did this build fail, what env var is missing in prod, what
DNS record is missing* — are read-oriented, cross-project, historical, or about
access state, and live in the Vercel **REST API**. agent-vercel talks to
`api.vercel.com` directly and is read-default, cross-project, and history-aware.
See [`design-docs/cli-design.md`](design-docs/cli-design.md) § "Why not just use
the Vercel CLI".

## Credential vs scope (the core model)

One Vercel credential reaches many **teams**. So the two are separate axes:

- `auth` — manages the credential (the secret; currently an access token, with a
  `type` discriminator for future kinds). Stored in the macOS Keychain;
  **never printed, and there is no command to read it back out.**
- `scope` — lists/selects which team (account) to act on. Not a secret.

Per call: `--auth <label>` picks the credential, `--scope <team>` picks the team.

## Why Go

A single static binary, fast startup (matters for per-call agent invocation), and
alignment with the rest of the `agent-*` family.

## Installation

```bash
go install github.com/shhac/agent-vercel/cmd/agent-vercel@latest   # dev-stamped
# or build a version-stamped binary:
git clone https://github.com/shhac/agent-vercel.git && cd agent-vercel && make build
```

## Getting started

```bash
export VERCEL_TOKEN=...                  # create at vercel.com/account/tokens
agent-vercel auth add personal           # store it in the Keychain (label optional; default "default")
agent-vercel auth test                   # verify (GET /v2/user)
agent-vercel scope list                  # teams this credential can reach
agent-vercel scope set-default acme       # default scope
agent-vercel usage                       # LLM-oriented overview
```

## Command surface

| Domain | Commands |
|---|---|
| `auth` | `add`, `list` (`ls`), `test`, `set-default`, `remove`, `import-cli` |
| `scope` | `list` (`ls`), `current`, `set-default` |
| `deployment` | `list`, `get`, `current`, `logs`, `runtime-logs`, `promote`*, `rollback`*, `cancel`*, `redeploy`* |
| `project` | `list`, `get` |
| `env` | `list`, `diff`, `get`, `set`*, `rm`*, `pull` |
| `domain` | `list`, `get`, `inspect`, `records`, `verify`*, `add`*, `rm`*, `cert` |
| `alias` | `list`, `set`*, `rm`*, `bypass`* |
| `billing` | `charges` (`--by service\|project`) |
| `api` | `call <METHOD> <path>` (raw escape hatch) |
| `config` | `get`, `set`, `list`, `unset` |

\* destructive / state-changing — requires `--yes`.

## Development

```bash
make test             # go test ./... -count=1 (offline; uses the mock server)
make vet
make lint             # golangci-lint
make dev ARGS="usage"
make test-integration # opt-in live API shape checks (read-only; needs a token)
```

## License

MIT — see [LICENSE](LICENSE).
