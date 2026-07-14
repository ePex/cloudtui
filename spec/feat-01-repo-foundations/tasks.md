# Tasks — Repo foundations

Plan: [plan.md](plan.md)

Condensed from the original Taskfile and retroactive-workflow-compliance
task files; every item below was already implemented before this
condensation, so all are checked.

1. [x] **`Taskfile.yml`** — `doctor`, `build`/`build:tui`, `run:tui`,
   `test`/`test:tui` targets.
2. [x] **`CLAUDE.md`: "Feature & bugfix workflow"** — spec/plan/tasks
   gating, per-task approval, mandatory-unit-tests rule with a
   genuinely-untestable carve-out.
3. [x] **Retroactive annotations** — one-line "predates workflow update"
   notes on the two pre-existing specs (Taskfile, tui scaffold).
4. [x] **Retroactive plan/tasks files** for both pre-existing specs,
   labeled as written after the fact.
5. [x] **Backfilled unit tests** for the tui scaffold —
   `internal/ui/views/views_test.go` (constructor `Name()`/`Title()`)
   and `internal/app/app_test.go` (view registration, `switchTo`,
   `onGlobalKey`, `onPromptDone`).
6. [x] **Verify** — `task doctor`/`build`/`test` all succeed; `gofmt`/
   `go vet` clean.
