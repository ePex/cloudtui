# Tasks: list navigation wrap-around toggle

Each task is implemented and pushed only after being separately approved.

1. [x] Add `ui.TableWrap` (`tui/internal/ui/tablewrap.go`) with `Enabled()`,
   `Toggle()`, and `HandleNav(table, headerRows, event)`, plus
   `tablewrap_test.go` covering: wrap disabled, wrap enabled at bottom
   edge, wrap enabled at top edge, wrap enabled mid-list (normal
   single-step move), empty table (header row only), single-data-row
   table (up/down both stay put).
2. [x] Wire wrap-around into `tui/internal/view/queues.go`: `wrapNav`
   field, guard clause, `W` case, `Shortcuts()` entry, remove dead `j`/`k`
   cases.
3. [x] Wire wrap-around into `tui/internal/view/messages.go` (same
   shape as task 2, adapted to its boolean-expression switch style).
4. [x] Wire wrap-around into `tui/internal/view/logs.go`.
5. [x] Wire wrap-around into `tui/internal/view/logsearch.go`.
6. [x] Wire wrap-around into `tui/internal/view/ssmparams.go`.
7. [x] Wire wrap-around into `tui/internal/view/secrets.go`.
8. [ ] Wire wrap-around into `tui/internal/view/codepipelinelist.go`.
9. [ ] Wire wrap-around into `tui/internal/view/codepipelinedetail.go`.
10. [ ] Wire wrap-around into `tui/internal/view/datadoglogs.go` (results
    table only — service/env filter dropdowns are untouched).
11. [ ] `verify-live`: drive the real TUI against a real broker
    (`verify-live` skill) covering the queues list and message browser —
    confirm `W` toggles the context-panel hint, wrap is off by default,
    wrapping works at both edges once enabled, and toggling wrap on one
    view doesn't affect another. Record what was checked here.
12. [ ] Manual check of the remaining 7 views (logs, log search, SSM
    parameters, Secrets Manager, CodePipeline list/detail, Datadog logs)
    against whatever backend is configured locally — same wrap-toggle
    checks as task 11, adapted to each view's data. Record what was
    checked here.
13. [ ] Merge-back: fold a short note into each touched view's existing
    `spec/<area>/spec.md` (07, 08, 17, 18, 20, plus SSM/Secrets areas)
    describing the `W` wrap toggle as end-state behavior; delete
    `spec-wip/91-cr-wrap-list-navigation/`; push; mark the PR ready for
    review (no longer draft).
