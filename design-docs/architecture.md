# Architecture: package layout and boundaries

Package boundaries mirror the `agent-*` family (`agent-slack` especially). The
command layer is the only place cobra and process I/O appear; everything below
is plain Go with dependency-injected seams so tests run without network or
Keychain.

```
agent-vercel/
├── cmd/agent-vercel/main.go      # entry point; injects build version → cli.Execute
├── internal/
│   ├── cli/                      # cobra command tree (the only I/O layer)
│   │   ├── root.go               # GlobalFlags, Execute, error rendering, helpers
│   │   ├── usage.go              # `usage` overview
│   │   ├── auth.go               # `auth` group   (credential / secret axis)
│   │   ├── scope.go              # `scope` group  (scope axis)
│   │   ├── deployment.go         # `deployment` group   (planned)
│   │   ├── project.go            # `project` group      (planned)
│   │   ├── env.go                # `env` group          (planned)
│   │   ├── domain.go             # `domain` group       (planned)
│   │   ├── alias.go              # `alias` group        (planned)
│   │   └── api.go                # `api call` escape hatch (planned)
│   ├── credential/              # auth + scope store, Keychain boundary
│   ├── vercel/                  # REST client (DI transport, retry, mapping) — planned
│   ├── errors/                  # APIError{error, hint, fixable_by}
│   └── output/                  # JSON/YAML/NDJSON writers, error/notice rendering
├── design-docs/
└── skills/agent-vercel/         # SKILL.md + references/ (ships with the CLI)
```

## Command layer (`internal/cli`)

- `Execute(version)` builds the root command, runs it, and **renders any error
  exactly once** as structured JSON on stderr (cobra's own error/usage printing
  is silenced). This single funnel covers RunE bodies, `PersistentPreRunE`
  checks, flag-parse errors, and unknown-subcommand handlers — so no error path
  can leak unstructured text or be swallowed.
- `GlobalFlags` carries the persistent flags. The two that matter:
  `--auth <label>` (which credential) and `--scope <team>` (which team) — the
  two axes, kept independent.
- Each domain registers via `register<Domain>(root, g)`: it builds a parent
  command whose bare `RunE` calls `handleUnknownSubcommand` (structured "valid
  subcommands" error), then adds children.
- `printSingle` / `output.NewNDJSONWriter` are the only ways results reach
  stdout, keeping the output contract in one place.

## Credential store (`internal/credential`) — the security boundary

- `Store` reads/writes the credentials file and the backing `Keychain`.
- **Two axes, one store.** `Auth{Label, Type, Secret, UserID, Username}` is the
  secret axis (with a `Type` discriminator, currently `token`); `Scope{ID, Slug,
  Name}` (cached) plus `DefaultScope` is the scope axis. `DefaultAuth` and
  `DefaultScope` resolve the per-invocation defaults.
- **`Keychain` interface** (`Get/Set/Delete/Available`) is injected:
  `securityKeychain` (macOS `security` CLI) on darwin, `noopKeychain` elsewhere
  (falls back to the 0600 file), `MemoryKeychain` in tests. The Keychain account
  key is `<type>:<label>` (e.g. `token:default`).
- **`Load`** hydrates each secret from the Keychain into memory (for the
  Authorization header). **`Save`** pushes secrets to the Keychain and writes a
  `__KEYCHAIN__` placeholder to the file — only for secrets the Keychain accepted,
  so a failed `Set` never loses the secret.
- **`SecretStatuses`** is the read path `auth list` uses: it probes the Keychain
  and reports `keychain`/`file`/`missing` **without returning any secret
  material**. There is no symmetric "get secret" — that asymmetry *is* the
  boundary. Tested in `credential_test.go` (secret never in file, never in
  serialized status, 0600 perms, Keychain round-trip).

## REST client (`internal/vercel`) — planned

- A `Doer` interface (`Do(*http.Request) (*http.Response, error)`) is the
  injected seam; tests pass a fixture server / stub.
- `Client` holds base URL (`https://api.vercel.com`, overridable via
  `--base-url`), the active token, and the active scope. Every request gets
  `Authorization: Bearer <token>` and, when scoped, `?teamId=` / `?slug=`.
- Cross-cutting: pagination helper (timestamp-cursor → `next_cursor`), 429/5xx
  retry with capped exponential backoff (honoring `Retry-After`), and an error
  mapper turning Vercel's `{error:{code,message}}` + HTTP status into the
  `fixable_by` taxonomy.
- Typed compact mappers live next to each domain command; `--full` bypasses them
  and prints the raw payload.

## Output & errors

- `internal/output`: `Print` (JSON/YAML, null-pruned), `NDJSONWriter`
  (`WriteItem` / `WriteMetaLine` for `@`-prefixed trailers), `WriteError`,
  `WriteNotice`. HTML escaping is off so URLs/queries render literally.
- `internal/errors`: `APIError{Message, Hint, FixableBy, Cause}` with
  `New`/`Newf`/`Wrap`/`WithHint`. The client and command layers attach the right
  `fixable_by` and a command-naming hint.

## Testing strategy

- `credential`: in-memory Keychain + temp file; assert the secret never reaches
  disk or serialized output.
- `vercel` (planned): fixture HTTP server returning recorded Vercel payloads;
  assert retry/backoff, scope params, pagination, and error mapping.
- `cli` (planned): execute the root command against a stubbed client; assert
  exact NDJSON / JSON / error-JSON shapes (the agent-visible contract).
