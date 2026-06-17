---
name: agent-vercel
description: |
  Vercel CLI for AI agents: triage deployments and builds across projects, read
  build and runtime logs, see what is live in production (incl. rolling
  releases), diff environment variables across environments, inspect domains /
  DNS / certs, manage aliases, and call the raw Vercel REST API. Read-default;
  state-changing actions are gated behind --yes.
when_to_use: |
  Use when the user asks to read or act on Vercel: why a deployment failed,
  which deploy is live in prod, list recent/failed deployments (across
  projects), fetch build or runtime logs, compare env vars between production
  and preview, check why a domain or SSL cert is misconfigured, inspect/repoint
  an alias, or promote/rollback a deployment.
allowed-tools: Bash(agent-vercel *) Read
---

# agent-vercel

JSON in, JSON out, no interactivity. Lists are NDJSON (one object per line, then
`{"@pagination":…}` when more pages exist); single resources are pretty JSON.
Errors are JSON on stderr with `fixable_by: agent|human|retry` and a `hint`.

Safety: read freely (`list`, `get`, `current`, `logs`, `runtime-logs`, `diff`,
`inspect`, `records`, `cert`). Do not promote, rollback, cancel, redeploy,
change env vars, add/remove/verify domains, or change aliases unless the user
explicitly asked — those require `--yes`.

## Credential vs scope (read this first)

One Vercel access **token** reaches many **teams**. They are separate axes:

- `auth` manages the credential (the **secret**; currently an access token) —
  kept in the macOS Keychain, never printed. There is no command to read it back
  out; never ask the user to paste a token into chat, and don't try to retrieve
  one.
- `scope` selects which team (account) to act on — not a secret.

Pick per command: `--auth <label>` (which credential) and
`--scope <team-slug|id>` (which team; omit for the default / personal account).

## Setup (once)

```bash
export VERCEL_TOKEN=...                    # human creates one at vercel.com/account/tokens
agent-vercel auth add --label personal     # stores it in the Keychain
agent-vercel auth test                     # verify (GET /v2/user)
agent-vercel scope list                    # teams this credential can reach
agent-vercel scope set-default acme        # default scope for later calls
```

## Triage (reading)

```bash
agent-vercel deployment list --state ERROR --target production --since 24h   # failed prod deploys
agent-vercel deployment list --project my-app --limit 10
agent-vercel deployment current my-app                  # what is live in prod (+ rolling release)
agent-vercel deployment get dpl_…                       # one deployment, compact
agent-vercel deployment logs dpl_… --status 5xx         # build logs, filtered
agent-vercel deployment runtime-logs dpl_… --level error --path /api
agent-vercel env diff my-app                            # prod-vs-preview env var diff
agent-vercel domain inspect example.com                 # missing DNS record / cert state
agent-vercel alias list dpl_…                           # aliases + protection state
```

`deployment list` is **cross-project** and filterable — the main thing the
`vercel` CLI cannot do. Pass `--scope <team>` to look at another team.

## Acting (gated — only when asked)

```bash
agent-vercel deployment promote dpl_… --yes     # repoint prod to this deployment
agent-vercel deployment rollback dpl_… --yes
agent-vercel env set my-app KEY value --environment production --yes
agent-vercel alias set dpl_… app.example.com --yes
```

Without `--yes`, a gated command returns a description of what would happen —
show that to the user before retrying with `--yes`.

## Escape hatch

```bash
agent-vercel api call GET /v6/deployments --query 'state=ERROR&limit=5'
agent-vercel api call POST /v2/deployments/dpl_…/aliases --body '{"alias":"x"}' --yes
```

## More detail

- [references/commands.md](references/commands.md) — full command map, flags, and which are `--yes`-gated
- [references/output.md](references/output.md) — NDJSON + meta-line contract, compact vs `--full`, timestamps, pagination

Live docs from the binary: `agent-vercel usage` is the overview;
`agent-vercel <domain> usage` has per-domain detail.
