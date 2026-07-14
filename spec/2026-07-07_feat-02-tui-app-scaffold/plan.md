# Plan — tui app scaffold (retroactive)

Spec: [spec.md](spec.md)

> **Retroactive.** Written 2026-07-10, after the change was already
> implemented and committed, to bring this spec into line with the
> plan/tasks-file rule added to `CLAUDE.md`'s "Feature & bugfix workflow".
> See
> [2026-07-10_bugfix-01-retroactive-workflow-compliance/spec.md](../2026-07-10_bugfix-01-retroactive-workflow-compliance/spec.md).
> It documents the design actually built, not a plan written in advance.

## Approach

Lay down the k9s-style shell CLAUDE.md's architecture section calls for
(`tui/` — Go, tview/tcell, command prompt, resource views, non-blocking
UI), with no AWS wiring yet:

- Initialize the Go module (`github.com/ePex/cloudtui/tui`, matching the
  origin remote) with `tview`/`tcell` as the only dependencies.
- Define a small `ui.View` interface (`Name`, `Title`, `Primitive`) so the
  app shell can register and switch between resource views without
  knowing their concrete type.
- Implement one generic `placeholder` type in `internal/ui/views` that
  renders a bordered, titled "not yet implemented" `TextView`, and three
  thin constructors (`NewSecrets`, `NewParams`, `NewQueues`) that
  configure it for each of the three resource domains — avoids repeating
  the same rendering code three times before any of them has real
  behavior.
- Implement `internal/app.App`: a `tview.Flex` (header + one-line command
  prompt + `Pages`), global key capture that focuses the prompt on `:`,
  and prompt `SetDoneFunc` that routes Enter to either `q`/`quit` (stop)
  or a view-name lookup (`switchTo`), with Escape/anything else just
  returning focus to `Pages`.
- `cmd/tui/main.go` builds an `App` and runs it, printing to stderr and
  exiting 1 on error.

## Files/modules touched

- `go.mod` / `go.sum` (new)
- `cmd/tui/main.go` (new)
- `internal/ui/view.go` (new)
- `internal/ui/views/placeholder.go` (new)
- `internal/ui/views/{secrets,params,queues}.go` (new)
- `internal/app/app.go` (new)

## Key decisions / trade-offs

- Placeholders share one `placeholder` struct/rendering path instead of
  three near-duplicate view types, since all three need identical
  behavior until their real AWS-backed implementations land.
- No UI code makes AWS calls directly, per CLAUDE.md — that's a later
  service-wrapper layer; these views intentionally do nothing but render.
- Command routing (`switchTo`) does a linear scan over the (three-element)
  view slice rather than a map, since the list is small and fixed at
  startup.

## Testing

No unit tests were written at commit time — this is the gap this
retroactive-compliance pass backfills. Testable surface identified for the
backfill (implemented as tasks 4–5 of
[2026-07-10_bugfix-01-retroactive-workflow-compliance/tasks.md](../2026-07-10_bugfix-01-retroactive-workflow-compliance/tasks.md)):

- `internal/ui/views`: each constructor's `Name()`/`Title()`.
- `internal/app`: default active view on `New()`, `switchTo` routing,
  `onGlobalKey` prompt-focus behavior, `onPromptDone` command handling.

No AWS/rendering integration tests are in scope — there's no AWS code yet,
and tview rendering itself isn't exercised (only the pure routing/input
logic around it).
