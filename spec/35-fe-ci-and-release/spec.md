# Spec — FE 35: CI and first release via GitHub Actions

Date: 2026-08-08

## Background

The project has been built entirely through local `task build`/`task
test` runs so far — no CI, no tags, no releases. `CLAUDE.md` already
states a cross-platform requirement ("every developer on Windows,
Linux, or macOS must be able to build, run, and test the application
with the same commands") and a `Taskfile.yml` that already expresses
every relevant command (`task test`, `task build:tui`, ...), but
nothing currently verifies that requirement automatically, and there's
no way for someone outside the repo to get a runnable binary without a
Go toolchain.

## Problem

Two related gaps: (1) no automated check that changes actually build
and pass tests — cross-platform breakage would only be caught if
someone happens to test on the affected OS by hand; (2) no way to ship
a "first release" of cloudtui — a tagged version with downloadable
binaries.

## Decisions (confirmed)

1. **Two separate GitHub Actions workflows**: a CI workflow (runs on
   every push/PR) and a release workflow (runs on pushing a version
   tag). Kept separate rather than one workflow with conditional
   branches — clearer to read, and a release run is a fundamentally
   different kind of event (produces public artifacts) from a CI run.
2. **CI runs both `tui` and `mq-proxy` build/test across Linux, macOS,
   and Windows.** For `tui` this exercises the cross-platform
   requirement directly. For `mq-proxy`, getting there requires a
   prerequisite fix: `Taskfile.yml`'s `test:proxy`/`build:proxy`/
   `run:proxy` tasks currently hardcode `./gradlew`, a Unix shell
   script — `mq-proxy/gradlew.bat` already exists but the Taskfile
   never picks it, unlike `build:tui`'s `{{exeExt}}` branching. On
   native Windows this would fail to launch at all (no shebang
   interpreter), which would be the task invocation breaking, not a
   real signal about `mq-proxy`'s code. This slice fixes those three
   tasks to pick `gradlew.bat` on Windows (same `{{if eq OS
   "windows"}}...{{end}}` pattern already used elsewhere in the
   Taskfile), then adds `mq-proxy` to the same three-OS CI matrix as
   `tui`. `build:proxy`'s `podman build` container-image step is
   **not** part of the CI matrix — it's an artifact-publishing concern
   (and out of scope per decision 5 below), not a "does the code build"
   check; CI only needs `./gradlew`'s build/test, cross-platform.
3. **CI uses the existing `Taskfile.yml` commands** (`task test:tui`,
   `task test:proxy`, `task build:tui`), not ad hoc `go build`/`go
   test` invocations in the workflow YAML — so "what CI runs" and
   "what a developer runs locally" never drift apart, which is the
   whole point of having a task runner per `CLAUDE.md`.
4. **Release is tag-triggered** (pushing `v*.*.*`, e.g. `v0.1.0`) and
   uses **GoReleaser** to cross-compile the `tui` binary for
   linux/darwin/windows × amd64/arm64, package as archives, generate
   checksums, and publish a GitHub Release with an auto-generated
   changelog grouped by Conventional Commit type (`feat`/`fix`/...) —
   which the repo already uses, per `CLAUDE.md`. Chosen over a
   hand-rolled per-OS build matrix + manual asset upload: far less
   custom workflow YAML to maintain, and cross-compilation/checksums/
   changelog generation are exactly GoReleaser's job.
5. **Only `tui` is released as binaries this slice.** `mq-proxy` is a
   supporting dev service (used for local proxy-backend testing), not
   the product described in `README.md` — it isn't packaged or
   published as part of this release. Could get its own container-image
   release pipeline later if wanted, but that's a separate decision.
6. **First version is `v0.1.0`**, matching the project's own "Early
   development" status note in `README.md`. Tagging and pushing that
   first tag is a deliberate, explicit action taken after this
   pipeline is built and verified — not something automated by this
   feature itself.

## Proposed scope for this slice

- `Taskfile.yml`: make `test:proxy`/`build:proxy`/`run:proxy` pick
  `gradlew.bat` on Windows and `./gradlew` elsewhere (the `build:proxy`
  task's `podman build` line stays as-is — not part of the CI matrix,
  see decision 2).
- `.github/workflows/ci.yml`: triggers on push to `main` and on pull
  requests; matrix over `ubuntu-latest`/`macos-latest`/`windows-latest`,
  running both the `tui` job (`task test:tui`, `task build:tui`) and
  the `mq-proxy` job (`./gradlew test`, i.e. `build:proxy` minus the
  `podman build` step) on all three.
- `.github/workflows/release.yml`: triggers on pushing a `v*.*.*` tag;
  runs GoReleaser against a `tui`-scoped config to build, archive, and
  publish to GitHub Releases.
- `tui/.goreleaser.yaml` (or repo-root, scoped to build from `tui/` —
  exact placement decided in `plan.md` after checking GoReleaser's
  actual config shape for a non-root Go module).
- `README.md` gets a short "Installing a release" or "Download"
  section once binaries actually exist to point at.

## Out of scope (this slice)

- Releasing/publishing `mq-proxy` (container image, package registry,
  etc.).
- Homebrew/Scoop/apt packaging, or any install-script convenience
  layer beyond "download the archive for your OS from the Release
  page."
- Code coverage reporting, linting beyond what already exists
  (`gofmt`), or other CI quality gates not already implied by
  `CLAUDE.md`'s definition of done.
- Actually cutting the `v0.1.0` tag — that's a manual step after this
  pipeline is reviewed and working, done with explicit confirmation
  (pushing a tag is a public, hard-to-reverse action).
