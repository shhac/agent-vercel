# Behavior reference: Vercel API handling the implementation relies on

The Vercel-side facts the implementation depends on. Endpoints verified against
the OpenAPI spec (`openapi.vercel.sh`) as of 2026-06.

## Endpoint availability: spec ≠ live (validate before relying)

The published OpenAPI spec lists endpoints that **404 on the real
`api.vercel.com`**. Confirmed 404 live (with a valid token/scope) despite being
in the spec: `GET /v4/aliases` (alias list), `GET /v3/events` (audit log),
`GET /v1/security/firewall/events`. Treat the spec as a candidate list, not
ground truth — wire a new endpoint behind the `integration` test harness and
probe it live before shipping. (We shipped scope-wide `alias list` from the spec
and had to revert it when it 404'd live.)

### Spec-documented, shipped without live validation

These endpoints back commands that shipped from the OpenAPI spec but were
validated only against fixtures — no live response confirmed (no fixture
traffic / no role-scoped token / read deemed safe behind the same caveat as
`/v9/certs` and the billing-charges shape). Each is wired behind the
`integration` harness; treat the field shapes as spec-trusted, not
live-confirmed, until probed.

- **Firewall (WAF) reads** — `GET /v1/security/firewall/config/active` (active
  config: custom rules, IP blocklist, managed rulesets, bot/attack state),
  `GET /v1/security/firewall/attack-status` (active-attack/DDoS anomalies over
  `since` days, default 1), `GET /v1/security/firewall/bypass` (system bypass
  rules). All take `projectId`. **Note** the sibling
  `GET /v1/security/firewall/events` is confirmed 404 live (above), so these
  three are especially suspect — validate before relying. Compact projections
  are defensive (decode-and-omit on shape mismatch); `--full` returns the raw
  object.
- **Cache purge** — `POST /v1/edge-cache/invalidate-by-tags?projectIdOrName=…`,
  body `{tags:[…]}` (≤16 tags), marks CDN/runtime/data-cache entries stale for
  background revalidation. `--yes`-gated. Success body unconfirmed; the command
  synthesizes `{purged, project}` when the response body is empty (Vercel
  returns 200 with no body in the documented case).
- **Observability drains** — `GET /v1/drains` (log/trace/analytics/
  speed_insights exports; payload may be a bare array or wrapped under
  `drains`), `GET /v1/drains/{id}`. The compact projection omits the delivery
  URL (it can embed a destination token).
- **Project routes** — `GET /v1/projects/{id}/routes`: authored CDN routing
  rules (redirects/rewrites/headers) + a version block (staging vs live);
  `diff=true` returns the staged-vs-production diff.
- **Reverse domain map** — `GET /v1/domains/{domain}/project-domains`: every
  project domain on an apex (projectId, redirect, verified); payload may be a
  bare array or wrapped under `domains`.
- **Domain transfer status** — `GET /v1/registrar/domains/{domain}/transfer`:
  registration/transfer status.

Commands that reuse already-live endpoints (not spec-only): `project protection`
reads protection fields off `GET /v9/projects/{id}`; `deployment routes` reads
the `routes[]` off `GET /v13/deployments/{id}`; `billing consumption` reuses
`GET /v1/billing/charges`.

## Auth and scope

- **Bearer token.** `Authorization: Bearer <token>` against
  `https://api.vercel.com`. Tokens are created at vercel.com/account/tokens; one
  token reaches every team the user belongs to (subject to token scope).
- **Scope is a query param.** Almost every endpoint accepts `teamId` **or**
  `slug` to act within a team. Omitting both acts on the personal account. We
  send `teamId` when the scope is a `team_…` id, else `slug`.
- `GET /v2/user` identifies the token's owner (used by `auth test`/`whoami`).
- `GET /v2/teams` lists reachable teams (used by `scope list`).

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

## Billing / usage

- `GET /v1/billing/charges?from=&to=` — FOCUS v1.3 charges as **JSONL** (one
  record per line), 1-day granularity, range ≤ 1 year. Record fields are
  PascalCase: `BilledCost`, `BillingCurrency`, `ChargeCategory`,
  `ConsumedQuantity`/`ConsumedUnit`, `ServiceName`, `ChargePeriodStart/End`,
  and `Tags` (carries `ProjectName`/`ProjectId`). Needs a billing-role token.
- **Validation status:** spec-only. Live validation could not reach billing data
  (no billing-role token), so the field shapes above are trusted to the OpenAPI
  spec, not confirmed against a live response.

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
