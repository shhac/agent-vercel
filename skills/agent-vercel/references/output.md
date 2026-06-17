# agent-vercel output contract

## Shapes

- **Lists → NDJSON**: one JSON object per line. When more pages exist, a final
  meta line `{"@pagination":{"has_more":true,"next_cursor":"<cursor>"}}`. Pass
  that value back via `--cursor <next_cursor>` to fetch the next page, or pass
  `--all` to follow every page automatically (capped; if the cap is hit a final
  `@pagination` cursor is still emitted so you can resume).
- **Single resources → pretty JSON.** Override with `--format json|yaml|jsonl`.
- **Confirmations** (writes) are JSON objects too (e.g. `{"removed":"…"}`).

## Meta lines (`@`-prefixed, trailing)

- `@pagination` — `{has_more, next_cursor}`.
- `@referenced_projects` — `{prj_…: {id, name}}` so cross-project rows needn't be
  re-resolved.
- `@unresolved` — `[…]` args that didn't resolve in a batch `get`.

## Compact vs `--full`

Compact projections are the default and the big token win. `--full` returns the
raw Vercel payload (huge: project settings, attribution, checks).

- deployment: `id, name, project_id, state, target, ready_substate, url,
  inspector_url, branch, sha, commit_message, creator, created, error_code,
  error_message, oom, checks, custom_environment`
- project: `id, name, framework, node_version, repo, production_branch,
  latest_prod_deployment, updated`
- env var: `id, key, target[], type, git_branch, comment` (+ `value` with
  `--decrypt`)
- domain: `name, apex, verified, verification[], redirect, intended_nameservers`

## Timestamps

Vercel returns epoch-milliseconds. Compact projections emit RFC3339 strings;
`--full` keeps the raw ms.

## Truncation

`--max-body-chars` (default ~4000 for log commands; `-1` = unlimited). Truncated
content ends with `\n…`.

## Errors

`{error, fixable_by, hint?}` on stderr, exit code 1.

- `agent` — bad args/flags/target (fix the call): 400/404/422.
- `human` — auth / permission / missing token / payment required: 401/403/402.
- `retry` — transient (429/5xx/network); back off and retry.

The `hint` names the exact next command when there is one.

## Secrets

The access token is never in any output. `auth list` reports only where each
secret lives (`keychain`/`file`/`missing`), never its value. Application env vars
(`env …`) are different — they are the app's own config and are readable with
`--decrypt`.
