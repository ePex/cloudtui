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
