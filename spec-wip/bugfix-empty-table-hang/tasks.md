# Tasks: arrow keys in an emptied table hang the TUI

Each task needs its own approval before it starts, and is pushed once
done. Live checks run with an isolated HOME and a separate tmux server
(`TMUX_TMPDIR` set to a short path), never against the real
`~/.cloudtui`.

1. [x] **`ui.ClampTableSelection`** (plan "Approach").
   `internal/ui/table.go` + `table_test.go`. Table-driven cases:
   selection beyond the rows → last row; beyond the columns → last
   column; header-only table with a stale row → row 0; inside the
   content → unchanged; empty table → unchanged. Plus the hang itself:
   a header-only table with a stale selection, ↓ and ↑ through its
   `InputHandler` in a goroutine with a 2 s timeout. It times out
   without the clamp and returns with it (mutation-checked). Not wired
   into the app yet.
   *Done (2026-10-10):* the hang test sends ↓, ↑, `j`, `k`, PgDn, PgUp,
   Home and End. With the clamp disabled, ↓, ↑, `j`, `k` and PgDn each
   time out after 2 s (the test fails instead of hanging), and the clamp
   cases fail too. PgUp, Home and End return even without it, so
   spec.md now lists only the keys that hang. With the clamp,
   everything passes.

2. [ ] **Wire it into `onGlobalKey`** (plan "Approach").
   `internal/app/app.go`: clamp the focused `*tview.Table` right after
   the progress modal's key check. `app_test.go`: with a stale-selection
   table focused, the key passes through unchanged and the selection is
   back inside the content.
   *Live check (tmux, isolated HOME, local broker):* open a queue with
   messages, move down a few rows, go back, open an empty queue, then
   ↓, ↑, PgDn, Home: no hang, CPU idle, `h` still goes home. Queue
   list: move down a few rows, filter to no match, the same keys: no
   hang; clear the filter and navigate a full list normally (the
   cursor still skips the header row).

3. [ ] **Regression run.** `task test`, `task test:tui:race`, smoke test
   (isolated), and the live checks from task 2 in one session. Remove
   scratch queues.
