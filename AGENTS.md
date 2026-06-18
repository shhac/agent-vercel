# agent-vercel

Vercel CLI for AI agents. Go + cobra.

## Project Rules

- Lists default to NDJSON; single resources default to JSON.
- Errors are JSON on stderr with `fixable_by` (`agent` | `human` | `retry`) and a
  `hint`. Never exit with an unstructured error.
- Never print the access token. The token lives in the macOS Keychain; the
  credentials file holds only non-secret metadata plus a `__KEYCHAIN__`
  placeholder. There is deliberately no command that reads the token back out.
- A credential (the access token) and a scope (which team to act on) are
  **separate axes**: one credential reaches many teams. `auth` manages the
  secret; `scope`/`--scope` selects the account context. See
  `design-docs/cli-design.md`.
- Prefer read-only commands. Destructive or state-changing mutations
  (`deployment promote|rollback|cancel|redeploy`, `env set|rm`,
  `domain add|rm|verify`, `domain records add|rm`, `alias set|rm|bypass`,
  `cache purge`, and `api call` with a non-GET method) must require `--yes` and
  return a human-fixable JSON error without it.
- Keep log/body output truncatable (`--max-body-chars`); omit bulky payloads
  from list output by default, restore with `--full`.
- Keep Vercel HTTP logic dependency-injected so tests run without real network
  access; back CLI contract tests with a fixture server.

## Verification

```bash
GOCACHE=$(pwd)/.cache/go-build go test ./... -count=1
GOCACHE=$(pwd)/.cache/go-build go vet ./...
golangci-lint run ./...
```

Live API shape checks are opt-in (build tag `integration`), so the default test
run stays offline/green. To run them against real Vercel (read-only):

```bash
make test-integration   # uses $AGENT_VERCEL_IT_TOKEN, or a stored credential
                        # via $AGENT_VERCEL_IT_AUTH (+ optional $AGENT_VERCEL_IT_SCOPE)
```

## Keeping docs in sync

When you add, remove, or change a command, a flag, or its `--yes` gating, update
the docs that don't regenerate themselves:

- **Auto-generated — nothing to do.** Per-domain `usage`
  (`agent-vercel <domain> usage`), the `usage --json` catalog, and `--help` all
  derive from the cobra tree (`Use` + `Short`), so fixing a command's `Short` or
  flag help updates them for free — see `printDomainUsage` / `printCommandCatalog`
  in `internal/cli/usage.go`.
- **Hand-maintained — the one drift point.** The `usageOverview` const in
  `internal/cli/usage.go`: the CORE DOMAINS list and its `*` `--yes` markers.
  Update it whenever subcommands or gating change; nothing enforces it.
- **Skill** — `skills/agent-vercel/SKILL.md` (lean, common-path triage) plus
  `skills/agent-vercel/references/{commands,output}.md` (full surface). Use
  incremental disclosure: keep `SKILL.md` short and push detail into the
  references. Mirror user-facing changes into the `README` command-surface table.
- **design-docs/** — capture the *why* of a decision so it survives the diff:
  command-surface and gating choices in `cli-design.md`; Vercel API behavior the
  implementation relies on (quirks, undocumented shapes) in
  `behavior-reference.md`. Don't let a new gating call or output-shape change land
  with the rationale only in a commit message.

## References

The full design and command surface live in `design-docs/`:

- `initial-design.md` — goals, credential/scope model, output contract.
- `cli-design.md` — command surface, gating, output, LLM-first decisions.
- `architecture.md` — package layout and boundaries.
- `behavior-reference.md` — Vercel API handling the implementation relies on.
