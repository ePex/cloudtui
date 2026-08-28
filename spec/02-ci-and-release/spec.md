# CI and release

_Condensed from spec/35-fe-ci-and-release — see that folder for the incremental history._

## Purpose

Automated cross-platform build/test verification on every push/PR, and a tag-triggered release pipeline that publishes the `cloudtui` binary via a GitHub Release plus several easy-install channels (see Installing below).

## Behavior

Two separate GitHub Actions workflows (kept separate rather than one conditional workflow — a release run produces public artifacts, which is a fundamentally different kind of event from a CI run):

### CI workflow (`.github/workflows/ci.yml`)

- Triggers on push to `main` and on pull requests.
- Matrix over `ubuntu-latest` / `macos-latest` / `windows-latest`.
- Runs both:
  - the **`tui` job**: `task test:tui`, `task build:tui` — exercises the cross-platform requirement directly.
  - the **`mq-proxy` job**: `task test:proxy` (`./gradlew test`) then `task build:proxy:jar` (`./gradlew bootJar`) — the JAR-only build, not the full `task build:proxy` (which additionally depends on `build:proxy:jar` and then runs `podman build` to produce a container image, an artifact-publishing concern, not a build check).
- Uses the repo's own `Taskfile.yml` commands (`task test:tui`, `task test:proxy`, `task build:tui`), never ad hoc `go build`/`go test` invocations — so "what CI runs" and "what a developer runs locally" never drift apart.
- `Taskfile.yml`'s `test:proxy`/`build:proxy`/`run:proxy` tasks pick `mq-proxy/gradlew.bat` on Windows and `./gradlew` elsewhere (`{{if eq OS "windows"}}...{{end}}`, the same pattern `build:tui` already uses for `{{exeExt}}`) — without this, those tasks would fail to launch at all on native Windows (no shebang interpreter for the Unix `gradlew` script).
- A separate `shellcheck` job (ubuntu-latest only) lints `scripts/install.sh` — the one install script written in POSIX `sh` rather than Go, so it needs its own lint step outside `task test:tui`. `scripts/install.ps1` (PowerShell) has no equivalent CI lint step; verified manually instead (see Installing below).

### Release workflow (`.github/workflows/release.yml`)

- Triggers on pushing a `v*.*.*` tag (e.g. `v0.1.0`).
- Uses **GoReleaser** (`.goreleaser.yaml` at repo root) to cross-compile the `tui` binary for linux/darwin/windows × amd64/arm64, package as archives, generate checksums, and publish a GitHub Release with an auto-generated changelog grouped by Conventional Commit type (`feat`/`fix`/...).
- Only `tui` is released as binaries — `mq-proxy` is a supporting dev service (used for local proxy-backend testing), not the shipped product, and is not packaged or published by this pipeline.
- Cutting a release tag is a manual, explicit action (pushing a tag is a public, hard-to-reverse action) — never automated by CI itself.
- On the same tag push, also publishes a Homebrew cask to `ePex/homebrew-tap` (`homebrew_casks:` in `.goreleaser.yaml` — not the classic `brews:` formula mechanism, which is hard-deprecated as of GoReleaser v2.16) and a Scoop manifest to `ePex/scoop-bucket` (`scoops:`). Both are separate, empty-except-for-an-initial-commit GitHub repos that GoReleaser pushes a generated file into on every release, authenticated via a repo-scoped PAT each (`HOMEBREW_TAP_GITHUB_TOKEN`, `SCOOP_BUCKET_GITHUB_TOKEN` — two separate secrets rather than one shared token, so each can stay scoped to just its one target repo).

### Installing

Beyond the manual archive download above, four other ways to get `cloudtui`, all sourced from the same release artifacts:

- **Install script** — `scripts/install.sh` (macOS/Linux, POSIX `sh`) and `scripts/install.ps1` (Windows, PowerShell 5.1+), run via `curl -fsSL ... | sh` / `irm ... | iex`. Detect OS/arch, resolve the latest (or a `CLOUDTUI_VERSION`-pinned) release tag, download the matching archive, **verify it against that release's `checksums.txt` before extracting** (non-optional — this is a `curl | sh`-style flow, so skipping verification would be a real regression, not a nice-to-have), and install into a per-user directory (`$HOME/.local/bin` / `%LOCALAPPDATA%\cloudtui\bin`, both overridable via `CLOUDTUI_INSTALL_DIR`) — no `sudo`/admin rights, no automatic `PATH`/shell-rc/registry edits (a hint is printed instead; silently rewriting a user's shell config was deliberately avoided).
- **Homebrew** — `brew install --cask ePex/tap/cloudtui` (macOS + Linux).
- **Scoop** — `scoop bucket add ePex https://github.com/ePex/scoop-bucket` then `scoop install cloudtui` (Windows).
- **`go install`** — `go install github.com/ePex/cloudtui/tui/cmd/cloudtui@latest`. Works because of `tui/go.mod`'s module path; the entrypoint package is `tui/cmd/cloudtui/` (renamed from `tui/cmd/tui/` specifically so the installed binary is named `cloudtui`, matching every other method — Go otherwise names it after the last path segment, found by actually running the command before shipping it as a documented fact).

### Repository settings

- GitHub's "Automatically delete head branches" is enabled on the repo, so merging a PR (`gh pr merge` or the GitHub UI) deletes its remote branch without a separate step. The corresponding local branch still needs a manual `git branch -d` — see the root `CLAUDE.md` workflow's merge-and-clean-up step.

## Licensing

The repo is MIT-licensed (`LICENSE` at repo root) — compatible with its
dependencies (`tview`: MIT, `tcell`: Apache-2.0, `aws-sdk-go-v2`:
Apache-2.0). `README.md` links it under a `## License` section and
documents how to install a release (download the platform archive from
the Releases page) versus building from source (`task run:tui`).

## Data & config

- `.github/workflows/ci.yml` — CI workflow.
- `.github/workflows/release.yml` — release workflow.
- `.goreleaser.yaml` (repo root, scoped to build from `tui/`) — GoReleaser config; `homebrew_casks:`/`scoops:` blocks target `ePex/homebrew-tap`/`ePex/scoop-bucket`.
- `Taskfile.yml` — `test:tui`, `build:tui`, `test:proxy`, `build:proxy:jar`, `build:proxy`, `run:proxy` tasks; the last four branch on OS for the Gradle wrapper.
- `LICENSE` — MIT License text, copyright Philipp Holz.
- `scripts/install.sh`, `scripts/install.ps1` — install scripts, see Installing above.
- `HOMEBREW_TAP_GITHUB_TOKEN`, `SCOOP_BUCKET_GITHUB_TOKEN` — repo secrets on `ePex/cloudtui`, each a PAT scoped to just its one target repo.

## Implementation notes

Confirmed present at repo root: `.github/workflows/ci.yml`, `.github/workflows/release.yml`, `.goreleaser.yaml`.

## Out of scope (by design)

- Releasing/publishing `mq-proxy` (container image, package registry, etc.) — not part of this pipeline.
- apt/rpm/apk packaging.
- Auto-updating an already-installed binary (a `cloudtui upgrade` subcommand or similar) — every install method above is install-only.
- Code coverage reporting or linting beyond `gofmt`/`go vet` (already implied by `CLAUDE.md`'s definition of done) plus `shellcheck` on `scripts/install.sh`.
