# Behavior reference: Vercel API handling the implementation relies on

The Vercel-side facts the implementation depends on. Endpoints verified against
the OpenAPI spec (`openapi.vercel.sh`) as of 2026-06.

## Auth and scope

- **Bearer token.** `Authorization: Bearer <token>` against
  `https://api.vercel.com`. Tokens are created at vercel.com/account/tokens; one
  token reaches every team the user belongs to (subject to token scope).
- **Scope is a query param.** Almost every endpoint accepts `teamId` **or**
  `slug` to act within a team. Omitting both acts on the personal account. We
  send `teamId` when we have the id (from cached `scope list`), else `slug`.
- `GET /v2/user` identifies the token's owner (used by `auth test`/`whoami`).
- `GET /v2/teams` lists reachable teams (used by `scope list`, cached to id/slug).

## Deployments

- `GET /v6/deployments` — the workhorse. Filters: `projectId`/`projectIds`,
  `state` (`BUILDING,ERROR,INITIALIZING,QUEUED,READY,CANCELED`), `target`
  (`production`), `branch`, `sha`, `users`, `app`, `since`/`until` (JS ms),
  `rollbackCandidate`, `limit`. Paginates by `pagination.next` (ms cursor).
- Each row carries `uid`, `state`/`readyState`, `readySubstate`
  (`STAGED`/`ROLLING`/`PROMOTED` — tells you if it has seen prod traffic),
  `target`, `inspectorUrl`, `errorCode`/`errorMessage`, `meta` (git),
  `creator`, timing fields (`createdAt`, `buildingAt`, `ready`).
- `GET /v6/deployments/{id}/files` — source file tree (rarely needed).
- "What is live in prod": `GET /v6/deployments?target=production&state=READY&limit=1`
  combined with the rolling-release endpoint below.

### Rolling releases

- `GET /v1/projects/{idOrName}/rolling-release` — returns `state`
  (`ACTIVE`/`COMPLETE`/`ABORTED`), `currentDeployment` vs `canaryDeployment`,
  `activeStage`/`nextStage`, `stages[]` with `targetPercentage`, and
  `currentCanaryPercentage`. This is how `deployment current` reports the exact
  live-vs-canary split during a rollout — there is no `vercel` CLI equivalent.
- Promotion/restore of staged **routing rules**:
  `POST /v1/projects/{id}/routes/versions` with `action: promote|restore|discard`.

## Logs (two distinct endpoints — do not conflate)

- **Build** logs: `GET /v3/deployments/{idOrUrl}/events`. Event `type` includes
  `stdout`, `stderr`, `command`, `exit`, `deployment-state`,
  `middleware-invocation`, `edge-function-invocation`, `fatal`. Filters:
  `statusCode` (e.g. `5xx`), `direction`, `since`/`until`, `limit` (`-1` = all,
  not just tail). `proxy` payloads carry `vercelCache` (HIT/MISS/STALE/…) and
  `wafAction` — useful for cache/WAF triage and absent from the CLI.
- **Runtime** logs: `GET /v1/projects/{projectId}/deployments/{deploymentId}/runtime-logs`.
  Structured: `level` (trace…fatal), `source`
  (edge-function/edge-middleware/serverless/request), `responseStatusCode`,
  `requestMethod`, `requestPath`, `timestampInMs`, `messageTruncated`.

## Environment variables

- `GET /v10/projects/{idOrName}/env` — per-project vars. `decrypt=true` returns
  values. Each var carries `target[]` (`production`/`preview`/`development`),
  `gitBranch`, `type` (`encrypted`/`sensitive`/`system`/`plain`/`secret`),
  `comment`, `customEnvironmentIds`. `gitBranch=` filters preview vars by branch.
- `env diff` is client-side: pull once with `decrypt`, group by `key`, compare
  the value/presence across each var's `target` set. No API endpoint does this —
  it is the diff the `vercel` CLI makes you do by hand.
- Shared (team-level) vars: `GET /v1/env` (and `/v1/env/{id}` for one decrypted
  value). Note these are the *application's* config, not our auth token — they
  are legitimately retrievable; only the agent-vercel access token is not.
- `GET /v9/projects/{id}/custom-environments` — custom envs (id/slug, branch
  matcher, domains) for resolving `--custom-env`.

## Domains, DNS, certs

- `GET /v5/domains` — account domains; `GET /v9/projects/{id}/domains/{domain}` —
  project domain with `verified` + a `verification[]` array giving the **exact**
  `{type, domain, value, reason}` challenge to satisfy.
- `POST /v9/projects/{id}/domains/{domain}/verify` — verify; precise 400 reasons
  ("no TXT record", "TXT does not match", "verifying for another project").
- `GET /v5/domains/{domain}/records` — DNS records; `POST /v2/domains/{d}/records`
  to create. `GET /v8/certs/{id}` — cert `expiresAt`, `autoRenew`, `cns`.
- `POST /v10/projects/{id}/domains` — add a domain (`gitBranch` or `redirect`).
  Add fails with a clear reason if the latest prod deployment wasn't successful.

## Aliases & deployment protection

- `GET /v2/deployments/{id}/aliases` — aliases for a deployment; each carries a
  `protectionBypass` object (the access state behind a 401-ing preview).
- `POST /v2/deployments/{id}/aliases` — assign an alias (auto-detaches it from
  the previous deployment; this is the promotion primitive). 400s clearly when
  the cert isn't ready or the deployment isn't `READY`.
- `PATCH /aliases/{id}/protection-bypass` — manage shareable-link bypass (TTL,
  revoke, regenerate). (Possible later `alias bypass` subcommand.)

## Error model

- Errors are `{error: {code, message}}` with an HTTP status. Mapping to
  `fixable_by`: 400/404/422 → `agent`; 401/403 → `human`; 402 (missing payment)
  → `human` (pointed hint); 429 + 5xx + network → `retry`.
- 429 carries `Retry-After`; the client honors it, otherwise capped exponential
  backoff (~30s cap), matching the family.

## Timestamps & pagination

- Vercel timestamps are **epoch milliseconds**. Compact projections convert to
  RFC3339; `--full` keeps the raw ms.
- List endpoints paginate by a millisecond cursor (`pagination.next`/`prev`).
  We normalize `next` into the family `@pagination.next_cursor`; passing it back
  becomes the `until`/`from` param on the next page.

## `vercel login` token import (optional)

- The `vercel` CLI stores its session token in
  `~/.local/share/com.vercel.cli/auth.json` (XDG data dir; platform-specific).
  `auth import-cli` may read it as a convenience, mirroring agent-slack's
  `import-desktop`. Caveat: it is an OAuth session token (may rotate); a
  dashboard-minted access token is the more durable path. We never shell out to
  `vercel` to perform work.
