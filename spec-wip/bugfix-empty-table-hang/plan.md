# Plan: arrow keys in an emptied table hang the TUI

Implements `spec.md` in this folder.

## Why the loop can't end

tview v0.42 `Table.InputHandler`, for ↓ (↑ is the mirror image):
`selectedRow++` (clamped to the last row), then `forward(final)`; if
that finds nothing selectable, `backwards(previousSelection)`. Both
walk the cells, wrapping around inside `0…rowCount-1`, until they hit
a selectable cell or their final cell. With nothing selectable, only
the final cell can stop them, and `backwards`'s final cell is the
previous selection. When that selection lies outside the content
(`row >= rowCount`, or `column > lastColumn`), it can never be
reached. If the selection is inside the content, every final cell is
inside it too, and the walk reaches it within one lap. So keeping the
selection inside the content is enough, for every navigation key
(↑/↓, `j`/`k`, PgUp/PgDn, Home/End, ←/→).

## Approach: clamp in the app's key handler, before the table sees the key

- **`ui.ClampTableSelection(t *tview.Table)`** (new,
  `internal/ui/table.go`): if the table has rows and its selection lies
  outside them, `Select` the nearest cell that exists:
  `min(row, rowCount-1)`, `min(column, columnCount-1)`. That's the last
  remaining row when the content shrank, or the header when it emptied.
  A selection inside the content is left alone, and so is an empty
  table (`rowCount == 0`), where tview already returns early.
- **`App.onGlobalKey`** (`internal/app/app.go`) calls it for the
  focused primitive when it's a `*tview.Table`, right after the
  progress modal's key check and before anything else. Every key goes
  through `onGlobalKey` first (it's the application's input capture),
  and every table in the app is focused directly, so this one call
  covers all nine views' tables, the AWS profile picker and the Home
  table, plus any table added later.
- `Select` only fires `SetSelectionChangedFunc`, which no table here
  uses (`SetSelectedFunc`, fired by Enter, is untouched), so the clamp
  has no side effects beyond moving the selection.

Rejected alternatives:
- **Fix each view's repaint** (reset the selection after `Clear`):
  eleven places, and the next table would bring the bug back.
- **Fork or patch tview:** heavier than the bug, and against the
  repo's no-vendoring stance on tview workarounds (see spec/01's
  gotchas). An upstream issue is out of scope (spec.md).
- **Swallow navigation keys on tables with no selectable cell:** more
  logic (scanning for selectable cells on every key) for the same
  effect.

## Files

- `tui/internal/ui/table.go` (new) + `table_test.go`.
- `tui/internal/app/app.go` (`onGlobalKey`) + `app_test.go`.
- `spec/01-repo-and-tui-shell/spec.md`: a "Navigation" bullet on the
  clamp, and a "Notable gotchas" entry on the tview loop.
- `tui/CLAUDE.md` → "tview gotchas": the same trap, briefly.

## Testing

- **`ClampTableSelection`**, table-driven: selection beyond the rows
  → last row; beyond the columns → last column; header-only table with
  a stale row → row 0; inside the content → unchanged; empty table →
  unchanged.
- **The hang itself:** a header-only table with a stale selection,
  then ↓ and ↑ through its `InputHandler`, run in a goroutine with a
  2 s timeout so a regression fails instead of hanging the test run.
  Without the clamp it times out; with it, it returns. Mutation check:
  remove the clamp and confirm the test fails.
- **`onGlobalKey`:** with a stale-selection table focused, the key
  passes through unchanged and the table's selection is now inside
  its content.
- **Live (tmux, isolated HOME):** the two repros from spec.md (empty
  queue after a fuller one; queue filter with no match), each with ↓,
  ↑, PgDn and Home: no hang, CPU idle, the app keeps responding; then
  normal navigation in a full list still works, including the rows
  after the header.
