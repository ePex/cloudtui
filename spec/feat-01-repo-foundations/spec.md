# Repo foundations: cross-platform tooling and workflow

Date: 2026-07-07 to 2026-07-10 (condensed 2026-07-14 from two originally
separate entries: the Taskfile feature and the retroactive
workflow-compliance bugfix).

## Feature

Lay down the repo's cross-platform build/run/test tooling and the
collaboration rules every feature/bugfix since has followed: a Task-based
workflow (not Make, since Make isn't native on Windows) and the spec →
plan → tasks gating documented in `CLAUDE.md`.

## What

- **`Taskfile.yml`** at the repo root: `doctor` (checks `go`/`java`/`task`
  are on `PATH`), `build`/`build:tui` (`go build`, using Task's
  `{{exeExt}}` template function so the same command text produces
  `cloudtui.exe` on Windows and `cloudtui` elsewhere), `run:tui`, `test`/
  `test:tui`.
- **`CLAUDE.md`'s "Feature & bugfix workflow"**: every non-trivial change
  gets a spec, then an implementation plan, then a task breakdown — each
  its own file, each requiring explicit approval before the next stage
  starts; every task in the breakdown needs its own approval before
  being implemented; every change ships with unit tests unless something
  is genuinely untestable (and that has to be stated explicitly, not
  silently skipped).
- **Retroactive compliance pass.** The workflow rules above were written
  after the repo's first two changes (the Taskfile itself, and the tui's
  initial app scaffold — see `feat-02-tui-shell-and-starting-features`)
  already existed without plans, task files, or tests. Rather than leave
  the historical record contradicting the process everything after it is
  held to, both got retroactive plan/task files (clearly labeled as
  written after the fact) and, for the scaffold, its first unit tests.
  The Taskfile itself stayed untested by design — declarative Task
  config with no branching logic falls under the "genuinely untestable"
  carve-out the workflow rule itself anticipates.

## Why

Every developer on Windows, Linux, or macOS needs to build/run/test the
project with identical commands, and Docker can only ever be optional
convenience — Task is the one runner that satisfies both. Separately, a
lightweight but real gating process (spec/plan/tasks, tests mandatory)
keeps a small, fast-moving repo from accumulating undocumented,
untested changes — worth having from the very start rather than
retrofitting once bad habits set in, which is exactly what the
retroactive pass had to do for the two changes that predated it.

## Scope

- `Taskfile.yml`: `doctor`, `build`/`build:tui`, `run:tui`, `test`/
  `test:tui`.
- `CLAUDE.md`: the "Feature & bugfix workflow" section, the `spec/`
  naming convention, and the "every change ships with tests" rule.
- Backfilled `tui/internal/ui/views/views_test.go` and
  `tui/internal/app/app_test.go` for the pre-existing scaffold code.

## Out of scope

- `mq-proxy`'s own Taskfile targets — added later, once that module
  existed (see `feat-03-mq-proxy`).
- Any production-code changes as part of the retroactive-compliance
  pass — tests only.

## A living document

Unlike the other buckets, this one documents process and tooling that's
expected to keep evolving — new Taskfile targets, workflow tweaks, and
so on. Future changes to these conventions should be tracked as their
own `chg-NN-<slug>` entries referencing back here, rather than by
rewriting this spec after the fact.
