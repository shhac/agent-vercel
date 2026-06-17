# agent-vercel: initial design

agent-vercel's initial design, following the `agent-*` CLI family conventions
(`agent-slack`, `agent-stripe`, `agent-postmark`, `lin`, …): a single static Go
binary that an LLM agent invokes per-call, with structured output, structured
errors, and Keychain-first secret handling.

## Why this tool exists

Vercel's own `vercel` CLI is a **deploy-loop** tool: it is scoped to the one
project linked in the current directory, and its read surface is thin
(streaming `vercel logs`, a flat `vercel env ls`, `vercel inspect`). The
questions an agent is actually asked during triage are **read-oriented,
cross-project, historical, or about live-traffic / access state** — and those
are exactly the things the `vercel` CLI does poorly or not at all, while the
Vercel **REST API** answers them in one filtered call.

Representative triage questions and where each is served today:

| Question | Vercel CLI | Vercel REST API |
|---|---|---|
| Which deployment is live in prod right now (incl. rolling release)? | inference from `ls` | `GET /v6/deployments?target=production&state=READY`, `GET /v1/projects/{id}/rolling-release` |
| All failed prod deploys across N projects, by author, last 24h | not possible (single-project) | `GET /v6/deployments` with `projectIds`/`state`/`users`/`since` filters |
| Why did this build fail (historical, filterable)? | tail-only, short window | `GET /v3/deployments/{id}/events` (`statusCode`, `since/until`, `limit=-1`) |
| Why is a function 5xx-ing / cache missing? | weak filtering | `GET /v1/projects/{id}/deployments/{id}/runtime-logs` (level, status, path) |
| Prod-vs-preview env var diff; which var is missing | pull each env, diff by hand | `GET /v10/projects/{id}/env?decrypt=true` (each var carries its `target`) |
| Exact missing DNS record / cert state for a domain | partial | `GET /v9/projects/{id}/domains/{domain}` (+ `verification[]`), `/v5/domains/{d}/records`, `/v8/certs/{id}` |
| Why is this preview 401-ing? | none | alias `protectionBypass` state |

So agent-vercel is **not a thin wrapper over the `vercel` binary** — it talks to
`api.vercel.com` directly, and its job is triage and safe action, token-efficient
for an LLM reader. See `cli-design.md` § "Why not just use the Vercel CLI" for
the full validation.

## Goals

1. Single static binary, fast cold start (agents invoke per-call).
2. Output and error contract identical to the rest of the `agent-*` family.
3. Correct, well-tested **read** paths first, then carefully-gated writes.
4. Keychain-first secret handling; **nothing sensitive ever in output**.
5. Cross-project / org-wide reads as a first-class capability (the CLI's biggest
   gap).

## Credential model: auth and scope are separate axes

This is the central design decision and the place agent-vercel deliberately
diverges from agent-slack's shape.

- In Slack, each **workspace has its own token** — one token ↔ one workspace, so
  `--workspace` selects both credential and target at once.
- In Vercel, **one access token reaches many teams**. The token belongs to a
  user; the team is a per-request **scope** parameter (`teamId`/`slug`), not a
  property of the credential.

So we model two orthogonal axes, named for what they do:

- **`auth`** — manages the **credential** (currently a Vercel access token,
  carried with a `type` discriminator so other kinds can be added later). This is
  the secret half. Stored in the macOS Keychain; never printed.
- **`scope`** — lists and selects the **team (account) scope** to act on. This is
  not a secret — just slugs/ids. The personal account is the implicit default
  scope.

Per-invocation selection uses two global flags:

- `--auth <label>` picks **which stored credential** (most users have one).
- `--scope <team-slug|id>` picks **which team** to act on (`""` = personal).

Resolution order:

- auth: `--auth` flag → `VERCEL_TOKEN` env (raw token, Vercel's own var) →
  stored default credential.
- scope: `--scope` flag → `VERCEL_SCOPE` / `VERCEL_TEAM_ID` env → stored default
  team → personal account.

## Secret handling (the most important property)

- The access token is written **straight to the macOS Keychain** (service
  `app.paulie.agent-vercel`, account `token:<label>`), following the family
  reverse-DNS convention.
- The credentials file (`~/.config/agent-vercel/credentials.json`, XDG, 0600)
  holds **only non-secret metadata** — label, default flags, resolved username,
  cached team list — with a `__KEYCHAIN__` placeholder where the token would be.
- **There is deliberately no command that reads the token back out.** `auth
  list` reports *where* each secret lives (`keychain` / `file` / `missing`) via a
  Keychain probe that never returns secret material (`SecretStatuses`). The token
  leaves the Keychain only to populate an `Authorization: Bearer` header inside
  the binary. An agent driving this tool cannot exfiltrate the token through it.
- Token entry that avoids the agent's conversation: `auth add` reads
  `VERCEL_TOKEN` from the environment (set by the human out-of-band); a native OS
  dialog path (`--form`, zenity, family precedent agent-slack/agent-posthog) is
  planned so a human can paste a token without it transiting chat.

## Command surface (overview)

Full surface, flags, and gating live in `cli-design.md`. Domains:

- **auth**: `add`, `list` (`ls`), `test` (`whoami`), `set-default`, `remove`,
  `import-cli`
- **scope**: `list` (`ls`), `current`, `set-default`
- **deployment**: `list`, `get`, `logs`, `runtime-logs`, `current`, `promote`,
  `rollback`, `cancel`, `redeploy`
- **project**: `list`, `get`
- **env**: `list`, `diff`, `get`, `set`, `rm`
- **domain**: `list`, `get`, `inspect`, `records`, `verify`, `add`, `rm`, `cert`
- **alias**: `list`, `set`, `rm`
- **api**: `call <METHOD> <path>` (raw REST escape hatch)
- **config**: `get`, `set`, `list`, `unset`
- **cache**: `info`, `warm`, `purge`
- **usage** / `<domain> usage`

### Targets

- A `<deployment>` target is a deployment id (`dpl_…`), a deployment URL
  (`my-app-abc123.vercel.app`), or an alias URL.
- A `<project>` target is a project id (`prj_…`) or a project name.
- A `<domain>` is a hostname (`example.com`, `www.example.com`).

## Output contract

- Lists → NDJSON, trailing `{"@pagination": {...}}` line when more pages exist.
- Single resources → pretty JSON.
- Compact projections by default; `--full` restores the raw API payload.
- Vercel timestamps are epoch-ms; compact projections surface RFC3339 strings
  (LLM-friendly), with raw ms available under `--full`.
- `--max-body-chars` truncates long log/body fields with a `\n…` marker.
- Errors → JSON on stderr: `{error, fixable_by, hint?}`.
  - `agent`: bad args/flags/targets (400/404/422).
  - `human`: auth, permissions, missing token, payment required (401/403/402).
  - `retry`: 429, 5xx, network.

## Safety

- Destructive / state-changing mutations require `--yes` and otherwise return a
  `fixable_by: human` error describing exactly what would happen:
  `deployment promote|rollback|cancel|redeploy`, `env set|rm`,
  `domain add|rm|verify`, `alias set|rm`.
- `api call` is the explicit power tool: `GET` is ungated; non-`GET` requires
  `--yes` (the caller typed the method, but REST mutations are real).
- Read commands (`list`, `get`, `logs`, `diff`, `inspect`, …) are ungated.

## Build order / layering

1. **Scaffold + contract** (this commit): root, global flags, output, errors,
   credential/Keychain (auth + scope), `auth`/`scope` local commands, usage, CI,
   design-docs, skill.
2. **API client** (`internal/vercel`): DI transport, Bearer auth, scope query
   params, pagination, 429/5xx retry+backoff, error mapping. Unlocks
   `auth test`/`whoami` and live `scope list`.
3. **Read slice A**: `deployment list/get/current`, `project list/get`.
4. **Read slice B**: `deployment logs/runtime-logs`, `env list/diff/get`.
5. **Read slice C**: `domain list/get/inspect/records/cert`, `alias list`.
6. **Writes** (behind `--yes`): `deployment promote/rollback/cancel/redeploy`,
   `env set/rm`, `domain add/rm/verify`, `alias set/rm`.
7. **Escape hatch + ergonomics**: `api call`, `cache`, `config`, per-domain
   `usage`, `auth import-cli`.

## Decisions

- **Talk to the REST API, not the `vercel` binary.** The triage value is in data
  the binary doesn't expose; wrapping it would inherit its single-project,
  thin-read limitations. (See the table above.)
- **Auth and scope are separate command domains.** Reusing one
  `--workspace`-style selector would misrepresent Vercel's one-token-many-teams
  model and make cross-team reads awkward.
- **No `vercel`-CLI dependency at runtime.** `auth import-cli` *optionally*
  reads a token the user already minted via `vercel login`
  (`~/.local/share/com.vercel.cli/auth.json`) as a convenience import path —
  mirroring agent-slack's `import-desktop` — but the tool never shells out to
  `vercel` to do its work.
- **Keychain naming follows the family.** Service `app.paulie.agent-vercel`;
  config dir is the plain `~/.config/agent-vercel` (XDG) — agent-slack's
  reverse-DNS config dir was only to dodge a TS-tool filename collision that
  doesn't exist here.
- **LLM-only.** No interactive prompts, editors, browser opening, or CI-mode
  special cases. Native OS dialogs for secret entry (`auth add --form`) are the
  one sanctioned human interaction.
