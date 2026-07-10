# Plan — Retroactive workflow compliance for pre-existing specs

Spec: [2026-07-10_bugfix-retroactive-workflow-compliance.md](2026-07-10_bugfix-retroactive-workflow-compliance.md)

## Approach

1. **Annotate** both existing specs with a one-line "Predates workflow
   update" note pointing at this bugfix's spec — no other edits to their
   content.
2. **Retroactive plan/tasks files**, one pair per existing spec, clearly
   labeled as written after the fact (not backdated as if they existed
   before implementation):
   - Taskfile pair: documents the (simple) approach taken and explicitly
     invokes CLAUDE.md's "genuinely untestable" carve-out — declarative
     Task config, no branching logic, no tests to backfill.
   - tui scaffold pair: documents the actual design decisions (module
     layout, `View` interface, placeholder pattern, command routing) as
     the plan, and a task list where the original implementation steps
     are marked done/historical, plus new, not-yet-done tasks for the test
     backfill — those are the only tasks in this whole change that
     actually need per-task approval before execution, since they're the
     only steps not already implemented.
3. **Backfill unit tests** for the tui scaffold (the new tasks from step
   2), in package (white-box) so unexported fields (`views`, `pages`,
   `prompt`) are reachable without adding test-only exports:
   - `tui/internal/ui/views/views_test.go`: table-driven check that
     `NewSecrets`/`NewParams`/`NewQueues` return the expected `Name()`/
     `Title()`, plus a `placeholder.Primitive()` sanity check (non-nil,
     bordered `*tview.TextView`).
   - `tui/internal/app/app_test.go`: `New()` registers all three views
     with `secrets` active by default; `switchTo` switches on a known
     name and no-ops on an unknown one; `onGlobalKey` focuses the prompt
     on `:` when unfocused, passes through other keys, and passes through
     everything when the prompt already has focus; `onPromptDone` stops
     the app on `q`/`quit`, switches view on a known name, leaves the
     current view unchanged on an unknown command, and returns focus to
     `pages` in every case except when the key isn't Enter (no-op) —
     matching the actual `switch`/`defer` logic in `app.go`.

## Files touched

- `spec/2026-07-07_feat-cross-platform-taskfile.md` (append note)
- `spec/2026-07-07_feat-tui-app-scaffold.md` (append note)
- `spec/2026-07-07_feat-cross-platform-taskfile-plan.md` (new)
- `spec/2026-07-07_feat-cross-platform-taskfile-tasks.md` (new)
- `spec/2026-07-07_feat-tui-app-scaffold-plan.md` (new)
- `spec/2026-07-07_feat-tui-app-scaffold-tasks.md` (new)
- `tui/internal/ui/views/views_test.go` (new)
- `tui/internal/app/app_test.go` (new)

No production code changes anywhere in this bugfix.

## Key decisions / trade-offs

- Retroactive plan/tasks files are explicitly labeled as written after the
  fact, so `spec/` doesn't imply a false history of when planning
  happened.
- Tests live in-package (`package app`, `package views`) rather than
  `_test` external packages, since the behavior worth covering
  (`switchTo`, `onGlobalKey`, `onPromptDone`) is only reachable through
  unexported fields/methods — adding exported test hooks just to make
  external tests possible would be a bigger, unrequested change to
  `app.go`.
- `tcell.EventKey` values needed for `onGlobalKey`/prompt tests are
  constructed directly with `tcell.NewEventKey(...)` — no fake terminal or
  screen needed since these are pure input-handling functions, not
  rendering.
- Taskfile gets no backfilled tests at all, by design — CLAUDE.md already
  anticipates this case ("if something is genuinely untestable... say so
  explicitly instead of skipping silently"), so the retroactive tasks file
  for it says so instead of manufacturing a shell-based test harness that
  wasn't asked for.
