# Tasks — tui app scaffold (retroactive)

Plan: [plan.md](plan.md)

> **Retroactive.** Written 2026-07-10, after the change was already
> implemented and committed — see
> [2026-07-10_bugfix-01-retroactive-workflow-compliance/spec.md](../2026-07-10_bugfix-01-retroactive-workflow-compliance/spec.md).
> Tasks 1–6 were already done at commit time. Tasks 7–8 are the test
> backfill this retroactive-compliance pass adds; they are tracked (and
> individually approved) as tasks 4–5 of
> [2026-07-10_bugfix-01-retroactive-workflow-compliance/tasks.md](../2026-07-10_bugfix-01-retroactive-workflow-compliance/tasks.md) —
> status here mirrors that file rather than duplicating approval.

1. [x] Initialize the Go module (`github.com/ePex/cloudtui/tui`) with
   `tview`/`tcell` dependencies.
   Status: done.
2. [x] Add `cmd/tui/main.go` entrypoint.
   Status: done.
3. [x] Add `internal/ui/view.go` (`View` interface).
   Status: done.
4. [x] Add `internal/ui/views/placeholder.go` (shared placeholder type).
   Status: done.
5. [x] Add `internal/ui/views/{secrets,params,queues}.go` constructors.
   Status: done.
6. [x] Add `internal/app/app.go` (Flex shell, global key capture, command
   routing via `switchTo`/`onPromptDone`).
   Status: done.
7. [x] Backfill `internal/ui/views/views_test.go`.
   Status: done (see task 4 of the 2026-07-10 bugfix tasks file).
8. [x] Backfill `internal/app/app_test.go`.
   Status: done (see task 5 of the 2026-07-10 bugfix tasks file).
