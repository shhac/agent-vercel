# agent-vercel output contract

## Shapes

- **Lists → NDJSON**: one JSON object per line. When more pages exist, a final
  meta line `{"@pagination":{"has_more":true,"next_cursor":"<cursor>"}}`. Pass
  that value back via `--cursor <next_cursor>` to fetch the next page, or pass
  `--all` to follow every page automatically (capped; if the cap is hit a final
  `@pagination` cursor is still emitted so you can resume). With
  `--format json|yaml` on a list command, output is wrapped in a single envelope
  document `{"data":[…], "@pagination":…}` instead of NDJSON lines.
- **Single resources → NDJSON** (one line). `--format json` gives the pretty
  object; `--format yaml` gives YAML. (Was pretty JSON before Phase 3.)
- **Get (single + multi).** `get <id>...` takes one or more ids and returns one
  result per id, in input order. Default output is NDJSON: one line per id —
  the record, or `{"@unresolved":{"id","reason","fixable_by","hint"?}}` for an
  id that couldn't be resolved (e.g. not found / bad id). `--format json|yaml`
  collapses to one `{"data":[…], "@unresolved":[…]}` envelope. A single
  `get <id>` is just the one-element case (NDJSON one line by default). Item-
  level misses stay on stdout and exit 0; only a command-level failure (auth,
  network) goes to stderr with exit 1 and empty stdout.

  Vercel-specific multi-get shapes:
  - `env get <project> <key>...` — project scope is fixed first, then 1..N keys
    are looked up against that project's vars; one NDJSON record (or
    `@unresolved`) per key, in input order.
  - `domain cert get <id>...` — 1..N cert ids → one record or `@unresolved`
    per id.
  - `config get <key>...` — 1..N local config keys → one record or `@unresolved`
    per key.
  - `scope member get` and `env shared get` stay single-arg (one id, not 1..N),
    but still emit NDJSON one line by default (`--format json|yaml` for the
    object). Their `--full` path is raw pretty-JSON passthrough, like `api get`.
- **Confirmations** (writes) are JSON objects too (e.g. `{"removed":"…"}`).

## Meta lines (`@`-prefixed, trailing)

- `@pagination` — `{has_more, next_cursor}`.
- `@referenced_projects` — `{prj_…: {id, name}}` so cross-project rows needn't be
  re-resolved.
- `@unresolved` — interleaved control line `{"@unresolved":{"id","reason","fixable_by","hint"?}}` for each id that couldn't be resolved in a `get`; appears in position in the NDJSON stream, or collected into the `"@unresolved":[…]` array in the `--format json|yaml` envelope.

## Compact vs `--full`

Compact projections are the default and the big token win. `--full` returns the
raw Vercel payload (huge: project settings, attribution, checks).

- deployment: `id, name, project_id, state, target, ready_substate, url,
  inspector_url, branch, sha, commit_message, creator, created, error_code,
  error_message, oom, checks, custom_environment` plus build-triage fields:
  `build_skipped, first_branch_deployment, queued (concurrent_builds|system_builds),
  error_step, error_link, state_reason, source, queue_wait_ms, build_duration_ms`
- project: `id, name, framework, node_version, repo, production_branch,
  latest_prod_deployment, updated` plus build config
  (`root_directory, output_directory, build_command, install_command,
  ignore_command`) and `paused`
- env var: `id, key, target[], type, git_branch, comment` (+ `value` with
  `--decrypt`)
- domain: `name, apex, verified, verification[], redirect, intended_nameservers`
  (`domain inspect` adds `configured_by, accepted_challenges, recommended_ipv4,
  recommended_cname`)

`queue_wait_ms`/`build_duration_ms` are durations in milliseconds (not
timestamps). New domains follow the same contract: `firewall`/`project routes`/
`deployment routes`/`domain transfer` print compact summaries (raw under
`--full`); `billing consumption` and `drains list` are NDJSON lists.

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

Compact projections also withhold other non-token secrets: `drains` omits the
delivery URL (it can embed a destination token — exposed only under `--full`),
and `project protection` reports `automation_bypass: true/false`, never the
bypass secret.
