# Easy install: script, Homebrew, Scoop, apt, go install

Date: 2026-08-26

## Prerequisite: config file moves to the user's home directory

`config.LoadDefault`/`SaveDefault` currently resolve to `config.yaml` in
the **current working directory** — fine for dev (`task run:tui` always
runs with `dir: tui`, so it resolves to `tui/config.yaml`), but wrong for
a binary installed via any of the five methods below: run `cloudtui` from
an arbitrary directory and it either can't find its config or, worse,
writes a fresh `config.yaml` into whatever directory happened to be
current. The log file already gets this right
(`~/.cloudtui/cloudtui.log`, spec/06) — config needs the same treatment
for the same reason, and needs to land *before* the install methods
below, since otherwise every one of them ships a binary with this bug.

- New default: `~/.cloudtui/config.yaml`, alongside the existing
  `~/.cloudtui/cloudtui.log`. `LoadDefault`/`SaveDefault` change to
  resolve here unconditionally — no cwd fallback, no env var override
  (matching the log file's own precedent of a single fixed location).
- **One-time migration**: if `~/.cloudtui/config.yaml` doesn't exist yet
  but a `config.yaml` is found in the current working directory, it's
  copied into place (logged via `slog`) rather than silently ignored —
  this preserves existing dev setups' connections/favorites/theme
  without forcing anyone to redo setup by hand. Only ever copies once:
  after that, `~/.cloudtui/config.yaml` is authoritative and the cwd
  copy is never consulted again.
- `tui/CLAUDE.md`/README dev-setup instructions updated: the
  `config.example.yaml` template now gets copied to
  `~/.cloudtui/config.yaml`, not `tui/config.yaml`.

## What

Five ways to get `cloudtui`, all sourced from the release artifacts
GoReleaser already produces (spec/02) — no new build pipeline, just more
consumers of it:

1. **One-line install script** — macOS/Linux (`scripts/install.sh` via
   `curl -fsSL ... | sh`) and Windows (`scripts/install.ps1` via
   `irm ... | iex`). Detects OS/arch, resolves latest (or a pinned)
   release tag, downloads the matching archive, verifies it against that
   release's `checksums.txt`, extracts the binary into a per-user
   directory (no `sudo`/admin), prints a `PATH` hint if needed.
2. **Homebrew** — `brew install ePex/tap/cloudtui` (macOS + Linux).
   GoReleaser auto-generates and pushes a formula to a new tap repo,
   `ePex/homebrew-tap`, on every release.
3. **Scoop** — `scoop bucket add ePex https://github.com/ePex/scoop-bucket`
   then `scoop install cloudtui` (Windows). Same mechanism, pushing a
   manifest to a new bucket repo, `ePex/scoop-bucket`.
4. **apt** — `.deb`/`.rpm`/`.apk` packages built by GoReleaser's `nfpm`
   integration, published to a real hosted repo via
   [Gemfury](https://gemfury.com) (free tier for open source), so
   `apt-get install cloudtui` works after adding the repo once. Not
   built for this task: creating and configuring the Gemfury account
   itself (see Data & config below) — that's a one-time manual step for
   the user; this task wires the automated push once the account and its
   API token exist.
5. **`go install`** — `go install github.com/ePex/cloudtui/tui/cmd/tui@latest`.
   Already works today (confirmed: `tui/go.mod`'s module path is
   `github.com/ePex/cloudtui/tui`) — this task only documents it, no
   code/pipeline change.

`README.md`'s "Installing a release" section is restructured to present
all of these, with the install script and `go install` as the two
"just works, no extra setup" options and Homebrew/Scoop/apt as the
"add this once, then `install`/`upgrade` forever" options. Manual
archive download from the Releases page stays documented as the
fallback that always works.

## Why

Today, "installing a release" means manually picking the right archive
off the Releases page and unpacking it. That's the gap the previous,
narrower version of this spec addressed with just the install script.
The user then asked to also support the package-manager-native flows
(Homebrew/Scoop/apt) and `go install`, since between them they cover
effectively every way a developer expects to install a CLI tool on
their platform of choice, not just the lowest-common-denominator script.

## Scope

- The config-file relocation described above (`~/.cloudtui/config.yaml`
  + one-time migration), landing first since the install methods below
  depend on it.
- `scripts/install.sh` (macOS/Linux, amd64/arm64) and `scripts/install.ps1`
  (Windows, amd64/arm64) as previously specced: mandatory checksum
  verification against the release's own `checksums.txt`, version
  pinning via an env var (default: latest), per-user install directory,
  no automatic `PATH`/shell-rc/registry edits — the script prints
  instructions instead.
- `.goreleaser.yaml` gains:
  - a `brews:` block targeting a new `ePex/homebrew-tap` repo.
  - a `scoops:` block targeting a new `ePex/scoop-bucket` repo.
  - an `nfpms:` block producing `.deb`/`.rpm`/`.apk` packages for
    linux/amd64 and linux/arm64.
  - a publish step pushing those packages to Gemfury (exact mechanism —
    GoReleaser's built-in `furies:` publisher if available for the
    pinned GoReleaser version, else a manual `curl`-based push step —
    to be confirmed in `plan.md`).
- Two new GitHub repos created (`ePex/homebrew-tap`, `ePex/scoop-bucket`),
  empty/public, for GoReleaser to push into.
- New GitHub Actions secrets the release workflow needs (added by the
  user, not generated by an agent — these are credentials):
  - a personal access token with `repo` scope covering the tap/bucket
    repos, for GoReleaser's cross-repo pushes (Homebrew/Scoop tokens
    are typically separate secrets, e.g. `HOMEBREW_TAP_GITHUB_TOKEN` /
    `SCOOP_BUCKET_GITHUB_TOKEN`, or one shared token if scoped broadly
    enough — confirmed in `plan.md`).
  - a Gemfury API token/push URL for the apt/rpm publish step.
- README rewrite of "Installing a release" covering all five methods.
- `spec/02-ci-and-release/spec.md` updated to reflect all of the above
  as shipped, replacing its current "no install scripts/packaging"
  out-of-scope note. `spec/01-repo-and-tui-shell/spec.md`'s theming
  section (which currently says `config.yaml`, implicitly cwd-relative)
  updated to name the new `~/.cloudtui/config.yaml` location.

## Out of scope

- Auto-updating an already-installed binary (`cloudtui upgrade` or
  similar) — install-only.
- Signing packages (GPG-signed `.deb`s, notarized macOS binaries,
  Windows code-signing) — none of the five methods above require it to
  function; can be a later hardening pass.
- Any change to what GoReleaser cross-compiles or the existing
  linux/darwin/windows × amd64/arm64 archive matrix (spec/02) — this
  only adds more distribution channels for the same builds.
- Creating the Gemfury account itself, or generating its API token —
  the user does this manually; this task only wires the automated push
  once the token exists as a secret.
