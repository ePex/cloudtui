# Tasks — FE 35: CI and first release via GitHub Actions

Plan: [plan.md](plan.md)

Each task needs separate approval before it's implemented — see
`CLAUDE.md`.

1. [x] `Taskfile.yml`: fix `test:proxy`/`run:proxy` to pick a
   Windows-appropriate gradlew invocation; split `build:proxy` into
   `build:proxy` (unchanged behavior: jar + podman image) and a new
   podman-free `build:proxy:jar`. Verified locally on this machine
   (macOS, `./gradlew` branch): `task test:proxy` and `task
   build:proxy:jar` succeed, `task --list` parses cleanly. **The
   Windows branch itself took three real CI runs to actually get
   right** — see task 8's writeup; the final form is `cmd /c
   gradlew.bat`, not the `gradlew.bat`/`.\gradlew.bat`/`./gradlew.bat`
   variants tried first. Local macOS testing could never have caught
   this — it's exactly the class of bug cross-platform CI exists to
   catch, and finding it live is itself proof the CI matrix is doing
   its job.
2. [x] `.github/workflows/ci.yml`: `tui` and `mq-proxy` jobs, each
   matrixed over `ubuntu-latest`/`macos-latest`/`windows-latest`
   (`fail-fast: false`); `tui` runs `task test:tui` + `task build:tui`
   via `actions/setup-go` (`go-version-file: tui/go.mod`) +
   `go-task/setup-task`; `mq-proxy` runs `task test:proxy` + `task
   build:proxy:jar` via `actions/setup-java` (temurin, 21) +
   `go-task/setup-task`. Installed `actionlint` (via Homebrew) and ran
   it against the file — no findings.
3. [x] `tui/cmd/tui/main.go`: a package-level `var version = "dev"`
   and a `--version`/`-v` check at the top of `main()` that prints
   `cloudtui <version>` and exits before normal startup. Manual check
   only (thin entrypoint code, no existing test file for `cmd/tui`).
   Verified: `go run ./cmd/tui --version`/`-v` both print `cloudtui
   dev`; building with `-ldflags "-X main.version=0.1.0"` (exactly
   what GoReleaser will do) produces `cloudtui 0.1.0`.
4. [x] `.goreleaser.yaml` (repo root): `version: 2`; `before.hooks`
   running `go test ./...` in `tui/`; `builds` (`dir: tui`, `main:
   ./cmd/tui`, `binary: cloudtui`, `CGO_ENABLED=0`, linux/darwin/
   windows × amd64/arm64, `ldflags` injecting `main.version`);
   `archives` (tar.gz, zip on Windows); `checksum`; `changelog`
   (grouped by Conventional Commit type, matching this repo's commit
   convention). **Course correction found during real validation**:
   the top-level `before.hooks` field only accepts plain strings, not
   the `cmd`/`dir` object form (confirmed via `goreleaser jsonschema`)
   — that richer form is only valid under `builds[].hooks.pre`, and
   attaching it there would have re-run `go test` once per of the 6
   build targets. Fixed by using a single shell-wrapped string instead:
   `sh -c "cd tui && go test ./..."`, which runs exactly once.
   Installed `goreleaser` (via Homebrew) and validated for real:
   `goreleaser check` passes, and `goreleaser release --snapshot
   --clean` actually built all 6 linux/darwin/windows × amd64/arm64
   archives, ran the pre-hook once, computed checksums, and — spot
   check — extracting the darwin/arm64 tarball and running
   `./cloudtui --version` printed `cloudtui 0.0.0-SNAPSHOT-<commit>`,
   confirming the ldflags version injection works end to end. `dist/`
   output removed afterward (already gitignored).
5. [x] `.github/workflows/release.yml`: triggers on `v*.*.*` tags;
   `actions/checkout` (`fetch-depth: 0`) + `actions/setup-go` +
   `goreleaser/goreleaser-action` (`args: release --clean`), gated by
   `permissions: contents: write` and the automatic `GITHUB_TOKEN`.
   `actionlint` reports no findings. Not exercised against a real tag
   yet — that's explicitly deferred (task 8 / spec.md decision 6).
6. [x] `README.md`: short "Installing a release" section pointing at
   the GitHub Releases page, noting the per-OS/arch archive naming.
7. [x] `task test`, `task build` (both still pass locally after the
   `Taskfile.yml` changes), `go build ./...`/`go vet ./...`/`go test
   ./...` in `tui/` — all pass, `gofmt` clean.
8. [x] Push this work (per explicit go-ahead) and verify the CI
   workflow actually goes green on all 6 real matrix jobs on GitHub.
   This absolutely was not a rubber-stamp verification — it caught a
   real, three-round bug that no amount of local (macOS) testing or
   config review would have surfaced:

   - **Push 1** (`68f2925`, the feature itself): 5/6 jobs passed
     immediately. `mq-proxy (windows-latest)` failed in ~1s on `task
     test:proxy` — too fast to be Gradle actually starting, so
     something failed before exec even began.
   - **Push 2** (`1e3a1fc`): tried `./gradlew.bat` (matching the Unix
     branch's `./gradlew` pattern). Failed with a literal Windows
     `cmd.exe` error: `'.' is not recognized as an internal or
     external command, operable program or batch file.` — `gh auth
     login` was set up at this point specifically to pull this log
     (the public unauthenticated API only exposes job/step status, not
     log content).
   - **Push 3** (`4aa955a`): tried `.\gradlew.bat` (backslash, the
     native Windows relative-path form) reasoning the forward slash
     was the problem. Failed differently: `".gradlew.bat": executable
     file not found in $PATH` — Task's embedded shell (`mvdan.cc/sh`)
     treats `\` as a POSIX escape character, so `.\g` collapsed to
     `.g`, silently eating the path separator entirely before exec
     ever ran.
   - **Push 4** (`80673d0`, the actual fix): `cmd /c gradlew.bat` —
     sidesteps the ambiguity entirely. `mvdan.cc/sh` execs `cmd` via a
     normal unambiguous `PATH` lookup and passes `gradlew.bat` as a
     bare argument, which `cmd.exe` resolves via its own
     current-directory search (no slash needed or wanted). **All 6
     matrix jobs green**: `tui` and `mq-proxy` × `ubuntu-latest`/
     `macos-latest`/`windows-latest`.

   Confirmed via `gh run view --log-failed` for the real failure text
   at each step (not guessed from symptoms) and `gh run watch
   --exit-status` for the final green run. Not exercised: pushing a
   `v0.1.0` tag — stays a separate, explicit step per spec.md
   decision 6.

   **Noted, not fixed** (cosmetic, non-blocking): `actions/setup-go`'s
   dependency cache warns `Restore cache failed: Dependencies file is
   not found ... Supported file pattern: go.mod` on every OS, because
   `go.mod` lives at `tui/go.mod`, not the repo root the cache action
   scans by default. Doesn't fail the build — just means Go module
   downloads aren't cached between CI runs yet. Also noted:
   `actions/setup-java@v4` is deprecated in favor of `v5`. Both are
   minor follow-ups, not required for this feature to be done.
