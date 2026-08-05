# Tasks — Bugfix 11: Queues list does not scroll to top on navigation

Plan: [plan.md](plan.md)

1. [x] **Reset selection in `repaint`** — add `qv.table.Select(1, 0)` at
   the end of `repaint` in `queues.go`, guarded by `GetRowCount() > 1`.
