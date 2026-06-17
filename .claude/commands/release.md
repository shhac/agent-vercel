---
description: Build, release, and publish to Homebrew
argument-hint: <patch|minor|major>
---

# Release

Perform a full release: version bump, build, GitHub release, and Homebrew tap update.

## Arguments

- `$ARGUMENTS` — version bump type: `patch`, `minor`, or `major`

## Instructions

You are performing a release of the `agent-vercel` CLI (Go). Follow these steps exactly.

### Pre-flight

1. Confirm `$ARGUMENTS` is exactly `patch`, `minor`, or `major`. If not, stop and ask.
2. Confirm the working tree is clean (`git status --short`). If not, stop and ask.
3. Confirm the current branch is `main`, a GitHub remote named `origin` exists
   (`git remote -v`; if not, stop and ask the user to create the repo), and
   `main` is up to date with `origin/main`.
4. Run `make test`, `go vet ./...`, `make lint`, and the cross-builds
   (`GOOS=windows go build ./...`, `GOOS=linux go build ./...`). `agent-vercel`
   is a pure-Go static binary (no cgo), so the cross-builds must stay green. If
   anything fails, stop and fix.
5. Determine the current version from the latest git tag
   (`git describe --tags --abbrev=0`). If no tag exists, start at `0.1.0`.

### Step 1: Version bump, tag, and push

Calculate the new version by bumping the current tag:

```bash
current=$(git describe --tags --abbrev=0 2>/dev/null | sed 's/^v//' || echo "0.0.0")
IFS='.' read -r major minor patch <<< "$current"
```

Apply the bump type ($ARGUMENTS):
- `patch`: increment patch
- `minor`: increment minor, reset patch to 0
- `major`: increment major, reset minor and patch to 0

Then tag and push:

```bash
git tag "v${new_version}"
git push origin main "v${new_version}"
```

### Step 2: Build

This repo has **no committed GoReleaser config** — the normal path is a manual
build. (The command supports GoReleaser if you add a `.goreleaser.yml`:
`goreleaser release --clean` then skip to Step 4.)

```bash
rm -rf dist/
mkdir -p dist
GOOS=darwin GOARCH=arm64 go build -ldflags="-s -w -X main.version=${new_version}" -o "dist/agent-vercel-darwin-arm64" ./cmd/agent-vercel
GOOS=darwin GOARCH=amd64 go build -ldflags="-s -w -X main.version=${new_version}" -o "dist/agent-vercel-darwin-amd64" ./cmd/agent-vercel
GOOS=linux GOARCH=amd64 go build -ldflags="-s -w -X main.version=${new_version}" -o "dist/agent-vercel-linux-amd64" ./cmd/agent-vercel
GOOS=linux GOARCH=arm64 go build -ldflags="-s -w -X main.version=${new_version}" -o "dist/agent-vercel-linux-arm64" ./cmd/agent-vercel
GOOS=windows GOARCH=amd64 go build -ldflags="-s -w -X main.version=${new_version}" -o "dist/agent-vercel-windows-amd64.exe" ./cmd/agent-vercel

cd dist
for bin in agent-vercel-darwin-arm64 agent-vercel-darwin-amd64 agent-vercel-linux-amd64 agent-vercel-linux-arm64; do
  tar czf "${bin}.tar.gz" "$bin"
done
zip agent-vercel-windows-amd64.zip agent-vercel-windows-amd64.exe
shasum -a 256 *.tar.gz *.zip > checksums-sha256.txt
cd ..
```

Smoke-test the native binary (auth-free — proves the binary loads and the
command tree wired up correctly):

```bash
./dist/agent-vercel-darwin-arm64 --version
./dist/agent-vercel-darwin-arm64 usage
```

### Step 3: Create GitHub release

If GoReleaser handled it, skip this step. Otherwise:

```bash
prev_tag=$(git tag --sort=-v:refname | head -2 | tail -1)
notes=$(git log --pretty=format:"- %s" "${prev_tag}..v${new_version}" --no-merges | grep -v "^- v[0-9]" || true)

gh release create "v${new_version}" dist/*.tar.gz dist/*.zip dist/checksums-sha256.txt \
  --title "v${new_version}" \
  --notes "$notes"
```

Verify: `gh release view "v${new_version}"`

### Step 4: Update Homebrew tap

The Homebrew formula lives in `../homebrew-tap` relative to this repo's root.
Create or update `../homebrew-tap/Formula/agent-vercel.rb` using the sibling
agent formula pattern (`../homebrew-tap/Formula/agent-sql.rb` is the seed
reference), with:

- Class name: `AgentVercel`
- desc: `"Vercel CLI for AI agents"`
- homepage: `https://github.com/shhac/agent-vercel`
- version, URLs (use `v${new_version}`), and SHA256 values from
  `dist/checksums-sha256.txt` (Go binaries use `amd64`, not `x64`)
- in `install`, after `bin.install`, install shell completions:
  `generate_completions_from_executable(bin/"agent-vercel", "completion")`
- test assertions for `agent-vercel --version` and `agent-vercel usage`

Then commit and push the tap (this repo lives outside `~/projects/`, so plain
git only — no Graphite):

```bash
cd ../homebrew-tap
git add Formula/agent-vercel.rb
git commit -m "agent-vercel ${new_version}"
git push
cd -
```

**IMPORTANT:** Always `cd` back to the `agent-vercel` repo after updating the tap.

### Step 5: Report

Show the user:

- New version number
- GitHub release URL
- Homebrew tap commit (if applicable)
- `brew install shhac/tap/agent-vercel` (new users)
- `brew upgrade shhac/tap/agent-vercel` (existing users)
