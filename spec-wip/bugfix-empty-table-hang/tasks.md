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

2. [x] **Wire it into `onGlobalKey`** (plan "Approach").
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
   *Done (2026-10-10):* the app test fails with the clamp call removed
   (`selected row = 4, want 0`) and passes with it. Live (tmux,
   scratch HOME, local broker, scratch queues `hangfull` with 30
   messages and `hangempty`): opened `hangfull`, moved down 6 rows,
   back, opened `hangempty`, then ↓ ↑ `j` `k` PgDn Home End: no hang,
   CPU 0.0 %, `h` went home. Queue list: moved down 4 rows, filtered to
   `zzznomatch`, the same keys: CPU 0.0 %. Cleared the filter, then
   Home ↓ ↑ Enter opened the first queue, not the header. Scratch
   queues removed.

3. [x] **Regression run.** `task test`, `task test:tui:race`, smoke test
   (isolated), and the live checks from task 2 in one session. Remove
   scratch queues.
   *Done (2026-10-10):* `task test` (Go, mq-proxy, 78 web tests) and
   `task test:tui:race` pass; the smoke test passes (scratch HOME, own
   tmux server). Both repros again in one session with a fresh build:
   empty queue after scrolling a full one, ↓ ↑ `j` `k` PgDn Home End →
   CPU 0.0 %, `h` went home; queue filter with no match, same keys →
   CPU 0.0 %; cleared filter, Home ↓ ↑ Enter → the first queue opened.
   Scratch queues removed.
