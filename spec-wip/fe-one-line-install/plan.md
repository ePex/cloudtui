# Implementation plan

## Order of work

1. Config relocation (prerequisite — everything else assumes it).
2. Install script (`install.sh` + `install.ps1`).
3. Homebrew.
4. Scoop.
5. apt/rpm via nfpm + Gemfury.
6. `go install` doc note (no code).
7. README rewrite covering all five.
8. Merge-back into `spec/01` and `spec/02`.

Each numbered item below is a separate `tasks.md` step (or a few), reviewed
independently — this plan just fixes the approach per item so review at
the task level is fast.

## 1. Config relocation

`tui/internal/config/config.go`:

- New exported `DefaultPath() (string, error)` — resolves
  `filepath.Join(home, ".cloudtui", "config.yaml")` via
  `os.UserHomeDir()`. Single source of truth for the path; `LoadDefault`,
  `SaveDefault`, and `cmd/tui/main.go`'s startup log line all call it
  instead of each hardcoding `"config.yaml"`.
- New unexported `migrateLegacyConfig(legacyPath, newPath string) error`:
  no-op if `newPath` already exists or `legacyPath` doesn't; otherwise
  reads `legacyPath`, `os.MkdirAll`s `newPath`'s directory (`0o755`),
  writes the bytes to `newPath` (`0o644`, matching `Save`'s existing
  permission choice — not a security hardening pass), logs via
  `slog.Info("config: migrated legacy config.yaml", "from", ..., "to",
  ...)`. Takes explicit paths (not cwd-implicit) specifically so it's
  unit-testable with `t.TempDir()` paths, no `t.Chdir()`/cwd trickery
  needed.
- `LoadDefault()` becomes: resolve `DefaultPath()`, call
  `migrateLegacyConfig("config.yaml", path)` (the literal relative
  string — this is the one place cwd-relative lookup still happens,
  deliberately, as the migration source), log+swallow a migration
  error (never block loading over a failed migration), then
  `Load(path)`.
- `SaveDefault(cfg)` becomes: resolve `DefaultPath()`, `os.MkdirAll` its
  directory, `Save(path, cfg)`.
- No cwd fallback after migration, no env var override — matches
  `openLogFile()`'s existing single-fixed-location precedent
  (`cmd/tui/main.go`).

`tui/cmd/tui/main.go`: replace `filepath.Abs("config.yaml")` with
`config.DefaultPath()` for the startup log line (fall back to a
placeholder string on error rather than failing startup over a log
line).

Tests (`tui/internal/config/config_test.go`):

- `TestLoadDefaultFallsBackWhenAbsent` (existing) needs updating — it
  currently depends on there being no `config.yaml` in the *test's real
  cwd*, which no longer matches what `LoadDefault` reads. Rewritten to
  isolate `$HOME`/`%USERPROFILE%` via `t.Setenv` (both, unconditionally —
  setting the one `os.UserHomeDir()` doesn't consult on the current OS is
  harmless) pointed at a fresh `t.TempDir()`.
- New: `TestDefaultPath` — asserts the joined path shape given a fake
  home dir.
- New: `TestMigrateLegacyConfigCopiesWhenDestAbsent`,
  `TestMigrateLegacyConfigNoopWhenDestExists`,
  `TestMigrateLegacyConfigNoopWhenSourceAbsent` — directly against
  `migrateLegacyConfig` with `t.TempDir()` paths, no env vars needed.
- New: `TestLoadDefaultMigratesOnFirstRun` — end-to-end via `t.Setenv`
  (home) + `t.Chdir()` (Go 1.24+, available here per `go 1.26.4` in
  `go.mod`) into a temp dir containing a `config.yaml`, confirms it
  lands at the resolved `DefaultPath()`.
- New: `TestSaveDefaultWritesUnderHomeConfigDir`.

No behavior change for `Load(path)`/`Save(path, cfg)` themselves (the
explicit-path functions) — only the `*Default` wrappers move.

## 2. Install scripts

`scripts/install.sh` (POSIX `sh`, not bash-specific — macOS ships an old
bash but a modern-enough `/bin/sh`):

- `set -eu` (no `pipefail` — not POSIX; the one pipeline that matters,
  the curl-to-tar extraction, is written to avoid needing it).
- OS detection: `uname -s` → `Linux`/`Darwin`. Arch: `uname -m`, mapped
  (`x86_64`→`amd64`, `aarch64`/`arm64`→`arm64`) to match GoReleaser's
  `{{ .Arch }}` naming (spec/02).
- Version: `${CLOUDTUI_VERSION:-latest}`. `latest` resolves via GitHub's
  `releases/latest` redirect (`curl -fsSLI -o /dev/null -w '%{url_effective}'
  https://github.com/ePex/cloudtui/releases/latest`, parse the trailing
  tag) rather than the API, to avoid unauthenticated rate-limiting on the
  API endpoint.
- Downloads `cloudtui_<version>_<os>_<arch>.tar.gz` and that release's
  `checksums.txt` into a `mktemp -d` scratch dir; verifies with
  `sha256sum -c` (falls back to `shasum -a 256 -c` on macOS, which lacks
  `sha256sum` by default) filtered to just the one relevant line —
  **hard failure, no `--insecure`-style bypass flag**, matching the
  spec's "verification is mandatory" line.
- Extracts to `${CLOUDTUI_INSTALL_DIR:-$HOME/.local/bin}`, `mkdir -p`
  first. No `sudo` anywhere in the script.
- Prints a one-line `PATH` hint (checks `$PATH` for the install dir
  first, only prints if actually missing).
- Cleans up the scratch dir on exit (`trap ... EXIT`).

`scripts/install.ps1` (PowerShell 5.1+ compatible, so it works on
stock Windows without requiring PowerShell 7):

- `$ErrorActionPreference = 'Stop'`.
- Arch via `[System.Runtime.InteropServices.RuntimeInformation]::ProcessArchitecture`
  (`X64`→`amd64`, `Arm64`→`arm64`).
- Same latest/pinned version resolution against the `releases/latest`
  redirect (`Invoke-WebRequest -MaximumRedirection 0` and read the
  `Location` header, to avoid following into the HTML page).
- Downloads the `.zip` + `checksums.txt`, verifies with
  `Get-FileHash -Algorithm SHA256`, hard-fails on mismatch.
- Extracts via `Expand-Archive` to
  `${env:CLOUDTUI_INSTALL_DIR}` or default
  `$env:LOCALAPPDATA\cloudtui\bin`.
- Prints a `PATH` hint (checks `$env:PATH`), no automatic
  `[Environment]::SetEnvironmentVariable` — matches the "no automatic
  PATH edits" scope decision.

Testing: shell/PowerShell scripts aren't Go, so no `go test` coverage.
CI gets a `shellcheck` lint step for `install.sh` (ubuntu runners ship
`shellcheck` preinstalled — confirmed locally too). `install.ps1` has no
equivalent lint step added (avoiding a new CI dependency for one script)
— covered by manual verification instead. `tasks.md` will call out
running both scripts by hand (a real `CLOUDTUI_VERSION` pinned to the
current latest tag, on at least macOS locally; Linux/Windows execution
noted as best-effort manual checks, not blocking, since this session
only has direct access to macOS) — this is exactly the "where behavior
can't be fully unit tested" case `CLAUDE.md`'s testing section expects
explicit manual steps for.

## 3. Homebrew

`.goreleaser.yaml` gains:

```yaml
brews:
  - name: cloudtui
    repository:
      owner: ePex
      name: homebrew-tap
      token: "{{ .Env.HOMEBREW_TAP_GITHUB_TOKEN }}"
    homepage: "https://github.com/ePex/cloudtui"
    description: "A terminal UI for managing cloud resources."
    license: "MIT"
    install: |
      bin.install "cloudtui"
    test: |
      system "#{bin}/cloudtui", "--version"
```

New empty public repo `ePex/homebrew-tap` (created via `gh repo create`).
New repo secret `HOMEBREW_TAP_GITHUB_TOKEN` on `ePex/cloudtui` — a
classic PAT (or fine-grained PAT scoped to just that one repo) with
`repo` (or fine-grained `contents: write`) access to `homebrew-tap`,
since the release workflow's own `GITHUB_TOKEN` can't push to a
*different* repository. **The user creates this token and adds the
secret** (`gh secret set HOMEBREW_TAP_GITHUB_TOKEN --repo ePex/cloudtui`)
— not something an agent should generate.

## 4. Scoop

Same shape, `.goreleaser.yaml`:

```yaml
scoops:
  - name: cloudtui
    repository:
      owner: ePex
      name: scoop-bucket
      token: "{{ .Env.SCOOP_BUCKET_GITHUB_TOKEN }}"
    homepage: "https://github.com/ePex/cloudtui"
    description: "A terminal UI for managing cloud resources."
    license: "MIT"
```

New empty public repo `ePex/scoop-bucket`, new secret
`SCOOP_BUCKET_GITHUB_TOKEN` — same reasoning and same "user adds this,
not an agent" rule as Homebrew's token. (Kept as two separate tokens
rather than one shared PAT, so each can be scoped to exactly one repo if
using fine-grained PATs — narrower blast radius than one token valid for
both.)

## 5. apt/rpm via nfpm + Gemfury

`.goreleaser.yaml` gains an `nfpms:` block (linux only — nfpm doesn't
package for darwin/windows):

```yaml
nfpms:
  - id: nfpm
    ids: [tui]
    package_name: cloudtui
    formats: [deb, rpm, apk]
    maintainer: "Philipp Holz"
    homepage: "https://github.com/ePex/cloudtui"
    description: "A terminal UI for managing cloud resources."
    license: "MIT"
    bindir: /usr/bin
```

(Maintainer field is name-only, no email — matches the `LICENSE`
copyright line and keeps every public-facing credit in the repo
consistent: `LICENSE`, this field, and the Homebrew/Scoop metadata below
all read "Philipp Holz" with nothing else.)

GoReleaser OSS (confirmed: `.github/workflows/release.yml` pins
`distribution: goreleaser`, not `goreleaser-pro`) has no built-in Gemfury
integration — that's a Pro-only pipe. Publishing uses the OSS
`publishers:` extension instead, one custom command per `.deb`/`.rpm`/
`.apk` artifact:

```yaml
publishers:
  - name: gemfury
    ids: [nfpm]
    cmd: curl -sf -F package=@{{ .ArtifactPath }} https://{{ .Env.FURY_PUSH_TOKEN }}@push.fury.io/{{ .Env.FURY_ACCOUNT }}/
```

New secrets: `FURY_PUSH_TOKEN` (Gemfury push token) and a plain (not
necessarily secret, but kept alongside for convenience) `FURY_ACCOUNT`
repo variable/secret for the Gemfury account/org name. **User creates
the Gemfury account and generates the token** — out of scope for this
task per the spec, wiring only starts once both exist.

Once pushed, end-user setup is Gemfury's standard one-time repo-add
(documented in README): add Gemfury's apt source + GPG key, then
`apt-get update && apt-get install cloudtui`. rpm/apk get the equivalent
Gemfury-hosted yum/apk repo add step. Exact one-liners come from
Gemfury's own per-account repo page once it exists — `tasks.md`'s
relevant task can't be fully written until the account exists, so that
task's exact copy will be filled in during implementation, confirmed
against the real account.

## 6. `go install`

No code/pipeline change — `tui/go.mod`'s module path
(`github.com/ePex/cloudtui/tui`) already makes
`go install github.com/ePex/cloudtui/tui/cmd/tui@latest` work today.
Documented in the README rewrite only.

## 7. README rewrite

"Installing a release" section restructured into the five options,
roughly: install script and `go install` first (zero setup), then
Homebrew/Scoop/apt (one-time add, then `install`/`upgrade` going
forward), manual archive download kept last as the fallback that always
works regardless of platform/tooling.

## 8. Merge-back

- `spec/02-ci-and-release/spec.md`: replace the "no install
  scripts/packaging" out-of-scope line with the five methods actually
  shipped (script, Homebrew, Scoop, apt/rpm/apk via Gemfury, go
  install), the new `.goreleaser.yaml` blocks, the two new repos, and
  the new secrets (naming them, not their values).
- `spec/01-repo-and-tui-shell/spec.md`: update the theming section's
  `config.yaml` reference to name `~/.cloudtui/config.yaml` and mention
  the one-time legacy migration.

## Key trade-offs

- **No env var override for the config path, no cwd fallback after
  migration** — one location, matching the log file's existing
  precedent; simpler than supporting multiple resolution strategies for
  a single-user desktop-style app.
- **Gemfury over a self-hosted apt repo** — self-hosting (e.g. a GitHub
  Pages-hosted repo with `reprepro`/`aptly` + a signing key managed in
  CI) would avoid a third-party dependency but is meaningfully more
  moving parts (key management, repo metadata regeneration) for a small
  OSS project; Gemfury's free OSS tier trades a third-party dependency
  for near-zero ongoing maintenance.
- **Two separate tap/bucket tokens, not one shared PAT** — narrower
  blast radius if fine-grained PATs are used (each scoped to exactly one
  repo) at the cost of one extra secret to create.
- **No automatic PATH/registry edits in either install script** —
  consistent with the spec's explicit call-out that silently rewriting
  shell rc files or the Windows registry is the kind of surprising
  side effect this project avoids; a printed instruction is enough.
