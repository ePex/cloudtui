# List navigation: wrap-around toggle

Date: 2026-08-19

## What

Change how cursor navigation behaves at the top/bottom edge of every
list-style `tview.Table` view in the app: queues, message browser, log
groups, CloudWatch log search results, SSM parameters, Secrets Manager,
CodePipeline list, CodePipeline detail, and Datadog logs.

Today, `tview.Table`'s built-in navigation (driven by arrow keys, and by
`j`/`k` which each view remaps to `KeyDown`/`KeyUp` in its own
`SetInputCapture`) simply clamps at row 0 (first data row, since row 0
is always the header) and the last row — pressing `k` on the first row
or `j`/down on the last row does nothing.

## Why

The user asked for wrap-around navigation ("like the queue message
browser, the logs etc") — pressing down on the last row jumps to the
first, and up on the first row jumps to the last, which is faster than
scrolling back manually on long lists (many messages, many log
groups/events). Not everyone wants this always on, so it's a per-view
runtime toggle rather than a persisted setting.

## Proposed behavior

- Each of the 9 list views gains a `W` keybinding that toggles wrap
  mode on/off for that view's table, for the current session only (not
  persisted to config, not carried over between app restarts or
  between different tables — toggling wrap on the message browser
  doesn't affect the queues list).
- Default is **off** — matches today's clamped behavior until a user
  opts in.
- When wrap is on: pressing `k`/Up on the topmost data row (row 1,
  since row 0 is the header) moves the cursor to the last row; pressing
  `j`/Down on the last row moves it to the first data row.
- When wrap is off: unchanged clamped behavior (today's default).
- `W` was chosen (rather than lowercase `w`) because `w` is already
  bound to "toggle watch" (desktop notifications) on the CodePipeline
  list and detail views (spec/20) — using `W` everywhere keeps one
  consistent key across all 9 views instead of a different key on two
  of them.
- Each view's `Shortcuts()` (context-panel hint) gains an entry for
  `W`, and the entry's description reflects current state, e.g. `wrap:
  off` / `wrap: on`, so the toggle's current state is visible without
  needing a separate status-bar message.

## Scope

- A shared helper (new, likely in `internal/ui/`) that wraps
  `tview.Table` up/down navigation with wrap-at-edge behavior, given
  the table's current row count and header-row offset — used by all 9
  views instead of each duplicating the edge logic.
- The 9 views listed above: wire up `W` in `SetInputCapture`, apply the
  shared helper to their `j`/`k`/arrow handling, and update
  `Shortcuts()`.
- Unit tests for the shared wrap-navigation helper (edge cases: single
  row, empty table, wrap on/off at both edges).

## Out of scope

- Persisting the toggle to config/settings.
- Wrapping any other kind of navigable UI (dropdowns, `tview.List`-based
  dialogs/pickers, filter-input history, etc.) — only the 9 table-based
  list views above.
- Changing what `j`/`k`/arrows do when wrap is off (stays clamped, as
  today).

## Decisions

- Scope: all 9 list-style `tview.Table` views app-wide (not just queue
  browser + logs).
- Persistence: none — a per-view, per-session toggle, not a Settings
  entry.
- Key: `W` (uppercase), uniformly across all 9 views.
- Default state: off.

## Open questions for plan.md

- Exact shape/location of the shared helper (a small package-level
  function taking `(table *tview.Table, headerRows int, key
  tcell.Key) *tcell.EventKey`, vs. a small struct holding the per-view
  wrap-enabled bool) — plan.md should settle the concrete API so it's
  easy to wire into all 9 `SetInputCapture` closures consistently.
- Whether the wrap-enabled bool lives on each view struct (9 new
  fields) or the helper is a small stateful type each view embeds/holds
  one instance of.
