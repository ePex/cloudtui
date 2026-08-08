# Tasks — FE 35: CI and first release via GitHub Actions

Plan: [plan.md](plan.md)

Each task needs separate approval before it's implemented — see
`CLAUDE.md`.

1. [x] `Taskfile.yml`: fix `test:proxy`/`run:proxy` to pick
   `gradlew.bat` on Windows (`{{if eq OS "windows"}}...{{end}}`); split
   `build:proxy` into `build:proxy` (unchanged behavior: jar + podman
   image) and a new podman-free `build:proxy:jar`. Verify locally on
   this machine: `task test:proxy` and `task build:proxy:jar` still
   succeed. Confirmed both pass locally on macOS (`./gradlew` branch
   taken); `task --list` still parses cleanly.
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
8. [ ] Push this work (branch/PR, per your go-ahead) and verify the CI
   workflow actually goes green on all 6 real matrix jobs on GitHub —
   this is the live-verification step for this feature, since a CI
   config that only "looks right" isn't proven until it's actually run
   by GitHub Actions. **Does not** include pushing a `v0.1.0` tag —
   that stays a separate, explicit step per spec.md decision 6.
