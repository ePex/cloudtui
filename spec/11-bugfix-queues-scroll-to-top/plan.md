# Plan — Bugfix 11: Queues list does not scroll to top on navigation

## Root cause

`repaint` in `queues.go` clears and redraws all data rows but never calls
`table.Select(...)` to reset the cursor. After the first load, the cursor
stays at the row it happened to end on.

## Fix

At the end of `repaint`, after all rows are written, call:

```go
if qv.table.GetRowCount() > 1 {
    qv.table.Select(1, 0)
}
```

The guard prevents a panic when the filtered list is empty (row count = 1,
only the header).

## Files touched

- `tui/internal/app/queues.go` — add `Select(1, 0)` at end of `repaint`

## Addendum: initial load with a long list (2026-08-08)

`Select(1, 0)` alone turned out not to be sufficient. When the queue list is
long enough to overflow the visible area, `tview.Table`'s internal `trackEnd`
flag (meant to auto-scroll a table like a tailed log when rows are appended)
latches `true` during the *first* draw of the table — while it's still empty,
header-only, right after `switchTo("queues")` and before the async `load()`
call returns. `Select` never clears `trackEnd`, so the repaint that follows
with the real data stays latched to the bottom instead of the top.

Fix: call `qv.table.SetOffset(0, 0)` alongside `Select(1, 0)` — `SetOffset`
explicitly clears `trackEnd`, unlike `Select`. Regression test:
`TestQueuesViewRepaintScrollsToTopWithManyRows` in `queues_test.go`, which
draws the table once empty (reproducing the latch) before repainting with
enough rows to overflow a small `SetRect` height, then asserts the offset is
back to 0.
