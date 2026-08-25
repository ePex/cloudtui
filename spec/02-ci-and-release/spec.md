# CI and release

_Condensed from spec/35-fe-ci-and-release — see that folder for the incremental history._

## Purpose

Automated cross-platform build/test verification on every push/PR, and a tag-triggered release pipeline that publishes downloadable `tui` binaries.

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

### Release workflow (`.github/workflows/release.yml`)

- Triggers on pushing a `v*.*.*` tag (e.g. `v0.1.0`).
- Uses **GoReleaser** (`.goreleaser.yaml` at repo root) to cross-compile the `tui` binary for linux/darwin/windows × amd64/arm64, package as archives, generate checksums, and publish a GitHub Release with an auto-generated changelog grouped by Conventional Commit type (`feat`/`fix`/...).
- Only `tui` is released as binaries — `mq-proxy` is a supporting dev service (used for local proxy-backend testing), not the shipped product, and is not packaged or published by this pipeline.
- Cutting a release tag is a manual, explicit action (pushing a tag is a public, hard-to-reverse action) — never automated by CI itself.

### Repository settings

- GitHub's "Automatically delete head branches" is enabled on the repo, so merging a PR (`gh pr merge` or the GitHub UI) deletes its remote branch without a separate step. The corresponding local branch still needs a manual `git branch -d` — see the root `CLAUDE.md` workflow's merge-and-clean-up step.

## Data & config

- `.github/workflows/ci.yml` — CI workflow.
- `.github/workflows/release.yml` — release workflow.
- `.goreleaser.yaml` (repo root, scoped to build from `tui/`) — GoReleaser config.
- `Taskfile.yml` — `test:tui`, `build:tui`, `test:proxy`, `build:proxy:jar`, `build:proxy`, `run:proxy` tasks; the last four branch on OS for the Gradle wrapper.

## Implementation notes

Confirmed present at repo root: `.github/workflows/ci.yml`, `.github/workflows/release.yml`, `.goreleaser.yaml`.

## Out of scope (by design)

- Releasing/publishing `mq-proxy` (container image, package registry, etc.) — not part of this pipeline.
- Homebrew/Scoop/apt packaging or install scripts beyond "download the archive for your OS from the Release page."
- Code coverage reporting or linting beyond `gofmt`/`go vet` (already implied by `CLAUDE.md`'s definition of done).
