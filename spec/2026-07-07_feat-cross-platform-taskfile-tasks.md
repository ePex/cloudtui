# Tasks — Cross-platform Taskfile (retroactive)

Plan: [2026-07-07_feat-cross-platform-taskfile-plan.md](2026-07-07_feat-cross-platform-taskfile-plan.md)

> **Retroactive.** Written 2026-07-10, after the change was already
> implemented and committed — see
> [2026-07-10_bugfix-retroactive-workflow-compliance.md](2026-07-10_bugfix-retroactive-workflow-compliance.md).
> All steps below were already done at commit time; nothing here is
> pending.

1. Add `doctor` target (`go version`, `java -version`, `task --version`).
   Status: done.
2. Add `build` (depends on `build:tui`) and `build:tui`
   (`go build -o bin/cloudtui{{exeExt}} ./cmd/tui`, run from `tui/`).
   Status: done.
3. Add `run:tui` (`go run ./cmd/tui`, run from `tui/`).
   Status: done.
4. Add `test` (depends on `test:tui`) and `test:tui` (`go test ./...`, run
   from `tui/`).
   Status: done.
5. **Testing:** not applicable. `Taskfile.yml` is declarative config with
   no branching logic — CLAUDE.md's "genuinely untestable" carve-out
   applies, so no unit tests exist or are being backfilled for this
   change.
   Status: done (documented, nothing to write).
