# Tasks — Bugfix 11: Queues list does not scroll to top on navigation

Plan: [plan.md](plan.md)

1. [x] **Reset selection in `repaint`** — add `qv.table.Select(1, 0)` at
   the end of `repaint` in `queues.go`, guarded by `GetRowCount() > 1`.
2. [x] **Reset scroll offset too** — add `qv.table.SetOffset(0, 0)` next to
   `Select(1, 0)` to clear `tview.Table`'s `trackEnd` latch, which otherwise
   scrolls a long initial list to the bottom. See plan.md addendum.
3. [x] **Regression test** — `TestQueuesViewRepaintScrollsToTopWithManyRows`
   in `queues_test.go`.
