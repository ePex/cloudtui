# FE 09 — Queue list filtering

Date: 2026-08-06

## What and why

The queue list can grow large. Users need a way to narrow it down by name
without leaving the view. This feature adds a live filter: pressing `/`
opens an inline input; typing filters the visible rows by queue name
(case-insensitive substring match). The filter persists when the user
navigates away and returns.

## User flow

1. User is in the Queues view.
2. User presses **`/`** — a filter input appears (e.g. in the table title
   or below it).
3. User types a substring — the table rows update live on every keystroke
   to show only queues whose name contains the typed string (case-insensitive).
4. User presses **Enter**, **Esc**, or **↑/↓** — the input closes; the filter
   stays active. Pressing ↑/↓ additionally moves the table selection in that
   direction immediately.
5. The active filter is visible at all times while in the Queues view
   (e.g. title shows `Queues [filter: foo]`).
6. User presses **`/`** again to edit the current filter, or clears it by
   pressing **`/`** and then deleting all text before confirming.
7. When the user switches to another view and returns, the filter is still
   applied (rows re-filtered from the latest data load).

## Scope

**In scope:**
- `/` hotkey in the Queues view opens an inline `tview.InputField` for filter
  input; Enter/Esc confirms and closes it.
- Case-insensitive substring match on queue name.
- Persistent filter: stored on `queuesView`; reapplied after every data
  refresh and on every `Activate()` call.
- Title reflects active filter: `" Queues "` → `" Queues [foo] "` (no `filter:` prefix).
- Shortcut entry added to the context panel: `</> filter`.
- Clearing the filter (empty string confirmed) removes all filtering and
  restores the original title.

**Out of scope:**
- Filtering by columns other than name.
- Regex or glob filter syntax.
- Filtering in the messages view.
