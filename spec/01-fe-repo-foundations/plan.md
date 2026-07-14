# Plan — Repo foundations

Spec: [spec.md](spec.md)

## Approach

### Taskfile

A single root `Taskfile.yml` (Task v3 schema) with four targets:
`doctor`, `build` (depends on `build:tui`, which runs `go build -o
bin/cloudtui{{exeExt}} ./cmd/tui` from `tui/`), `run:tui` (`go run
./cmd/tui`), `test` (depends on `test:tui`, `go test ./...`). Split into
a top-level target plus a `:tui`-suffixed one so a second module
(`mq-proxy`, added later) could join as another dependency without
renaming anything. Task's own command interpreter (`mvdan/sh`) runs
identical command text on every OS, so none of the four needed a
`.sh`/`.ps1` pair.

### Workflow rules

`CLAUDE.md` gained a "Feature & bugfix workflow" section: spec → plan →
tasks, one file per stage, written only once its predecessor is
approved; every task in the breakdown needs its own approval before
implementation; every change needs unit tests, with an explicit
"genuinely untestable" carve-out for things like declarative config with
no logic (used immediately by the Taskfile itself).

### Retroactive compliance

Two existing specs (Taskfile, tui scaffold) predated the rule above.
Rather than rewrite their content, each got a one-line "predates
workflow update" annotation plus new, clearly-labeled-as-retroactive
plan/task files documenting the approach actually taken. The scaffold's
tests were written in-package (`package app`, `package views`) since the
behavior worth covering (`switchTo`, `onGlobalKey`, `onPromptDone`,
`Name()`/`Title()`) is only reachable through unexported fields —
adding exported test hooks just to enable external tests would have been
a bigger, unrequested change. The Taskfile itself got no backfilled
tests, by design, per the carve-out above.

## Files touched

- `Taskfile.yml` (new)
- `CLAUDE.md` (modified — workflow section)
- `tui/internal/ui/views/views_test.go` (new, retroactive)
- `tui/internal/app/app_test.go` (new, retroactive)

## Key decisions / trade-offs

- Task over Make, specifically because Make isn't native on Windows and
  the project's hard cross-platform constraint rules it out.
- Per-task manual approval (not just per-stage) — deliberately slower,
  in exchange for nothing landing without being reviewed as it's built.
- Tests live in-package rather than adding exported hooks — avoids
  widening the public surface of `internal/app`/`internal/ui/views` just
  to make external testing possible.

## Testing

- Taskfile: no tests — genuinely untestable per the carve-out (no
  branching logic, declarative config only).
- Workflow rules: process/documentation, not code — nothing to unit
  test; the historical record itself (this condensation) is the closest
  thing to a compliance check.
- Retroactive scaffold tests: see `02-fe-tui-shell-and-starting-features`
  for what they actually cover, since the scaffold's behavior lives
  there now.
