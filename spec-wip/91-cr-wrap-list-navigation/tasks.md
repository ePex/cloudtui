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
8. [x] Wire wrap-around into `tui/internal/view/codepipelinelist.go`.
9. [x] Wire wrap-around into `tui/internal/view/codepipelinedetail.go`.
10. [x] Wire wrap-around into `tui/internal/view/datadoglogs.go` (results
    table only — service/env filter dropdowns are untouched).
11. [x] `verify-live`: drove the real TUI (tmux) against a real ActiveMQ
    broker, covering the queues list and message browser. Found and fixed
    a real bug in the process: the `W` handler toggled `wrapNav` but never
    refreshed the top bar's context panel, so the `wrap: on/off` hint
    stayed stale until some other action happened to rebuild it — fixed
    by adding the same "rebuild lines from Shortcuts() + host.SetContextHint"
    pattern the codebase already uses elsewhere (e.g. queues.go's `M`/`c`
    handlers) to all 9 views' `W` case. After the fix, verified: (1) wrap
    off by default on both views: pass. (2) `W` then bottom-row `j` wraps
    to the top row: pass — confirmed via message-ID identity (bottom row's
    message opened after wrap-down matched the top row's message ID), not
    just visual inspection. (3) top-row `k` wraps to the bottom row: pass
    — confirmed the same way (filtered queues list to 5 items, top-row `k`
    opened the alphabetically-last one). (4) context hint shows
    "wrap: on"/"wrap: off" live: pass, after the fix above. (5) toggling
    wrap on queues left messages' wrap independently off: pass — observed
    directly (queues had wrap on; opening messages showed wrap off).
    Cleanup: removed the disposable `cr91.wrap.verify` test queue, the
    scratch `tui/config.yaml` created for this session (none existed
    before), and the scratch binary/tmux session.
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
