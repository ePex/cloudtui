# Preview/message text wrapping

Date: 2026-08-19

## What

Add a `W` toggle to the views whose table has a long, truncated
free-text column, so the full text is readable in place instead of only
via the detail view:

- Message browser (`messages.go`) — the **PREVIEW** column (first ~80
  chars of the message body).
- CloudWatch Logs search (`logsearch.go`) — the **MESSAGE** column.
- Datadog Logs (`datadoglogs.go`) — the **MESSAGE** column.

Supersedes spec-origin/91 (`cr/91-wrap-list-navigation`, PR #48), which
misread "wrapping" as wrap-around cursor navigation at the top/bottom of
a list — closed unmerged, nothing from it ships.

## Why

Today the PREVIEW/MESSAGE column is a single truncated line (`"..."` cut
off at the column's rendered width). Seeing the full text means leaving
the list entirely (`Enter` → detail view), which loses your place in a
list you might be scanning multiple items in — e.g. skimming several
queue messages or log lines in a row to find the one you want.

## Proposed behavior

- `W` toggles wrapping on/off for that view's table, per-session (not
  persisted), off by default — same shape as spec-origin/91's toggle,
  minus the part that was wrong. Independent per view (three separate
  toggles, one per view above).
- When on: each row whose free-text column doesn't fit on one line is
  word-wrapped into as many additional **continuation rows** as needed,
  directly below it. The item's other columns (ID, timestamp, etc.) only
  appear on the first row; continuation rows show just the wrapped text
  in the same column.
- Continuation rows are marked non-selectable (`SetSelectable(false)`,
  the same mechanism the header row already uses) — `tview.Table`'s
  built-in up/down cursor movement already skips non-selectable cells
  automatically, so `j`/`k`/arrow navigation keeps landing only on real
  items with no extra code, and existing row→item-index math just needs
  to go through a mapping instead of assuming `row-1 == item index`.
- Wrap width is a **fixed constant** (not the table's live rendered
  column width): wrapping to the actual on-screen width would need to
  recompute on every terminal resize, which `tview.Table` gives no hook
  for short of reacting to every `Draw()` call. A fixed width (exact
  value TBD in plan.md, something in the 80–100 char range) is simpler,
  resize-safe, and still solves "I can't read the message" — it just
  isn't perfectly edge-to-edge on a very wide terminal.
- Toggling re-triggers whatever repaint the view already has (same
  repaint path used for load/filter/reload) — no new redraw plumbing.

## Scope

- `tui/internal/view/messages.go`, `logsearch.go`, `datadoglogs.go`.
- Likely a small shared word-wrap helper (plain-string wrapping, no new
  dependency — standard library has no built-in word-wrap).
- Unit tests for the wrap helper and for each view's row→item mapping
  with wrapping on (multi-row items, marking/selection still resolves to
  the right item in messages.go specifically, since it's the one view
  here with mark/select-by-row state).

## Out of scope

- Any other column/view (queue names, pipeline names, secret/param
  names) — those aren't the long free-text columns this was about.
- Reacting to terminal resize by re-wrapping at the new width.
- Persisting the toggle.
- The wrap-around navigation feature from spec-origin/91 — abandoned.

## Open questions for plan.md

- Exact fixed wrap width.
- Whether `Message Detail`/message-marking interactions need any
  adjustment beyond the row→item index mapping (e.g. does `SetSelectedFunc`
  need to look up the item differently when the cursor is, by
  construction, always on a primary row already).
