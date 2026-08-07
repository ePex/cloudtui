# Spec — FE 24: multi-select on the messages list

Date: 2026-08-08

## Background

The messages view (`tui/internal/app/messages.go`, opened from a queue in
the Queues list) already supports deleting/moving a *single* message from
the message-detail view (`d`/`m` there), and purging an entire queue (`p`).
There was no way to act on an arbitrary subset of messages — you either
did it one at a time via the detail view, or all-or-nothing via purge.

## Problem

No way to select several specific messages in a queue and delete or move
just those, without going through the detail view once per message.

## Solution

Multi-select on the messages list table, independent of the cursor:

- `space` — toggle the mark on the message under the cursor, then advance
  the cursor (so repeated presses mark a run of messages quickly).
- `a` — mark every message that has an ID.
- `n` — clear all marks.
- `d` — delete all marked messages (single confirm dialog for the whole
  batch, e.g. `Delete 3 marked message(s) from "orders"?`).
- `m` — open the move picker once; on target selection, move all marked
  messages there.

A new narrow first column shows a checkmark (`✓`) for marked rows, blank
otherwise.

## Scope

### In scope

- Marking state (`marked map[string]bool`, keyed by message ID) on
  `messagesView`, plus the five keybindings above.
- Marker column in the table.
- Bulk delete and bulk move, each tolerating partial failure (one bad
  message doesn't abort the rest of the batch; the status bar reports
  how many of the batch actually succeeded).
- Messages without an ID (limited-info Jolokia responses — see FE 13/14)
  can't be marked, same restriction the existing single delete/move
  already have.
- Unit tests for the marking logic and the "no marks → no-op" guards on
  `d`/`m`.

### Out of scope

- Marks surviving a reload — `repaint()` always clears them, since a
  refreshed list may reorder or drop messages and a mark keyed by a
  message ID that no longer matches its original row would be confusing.
- Multi-select on any other table in the app (queues list, connection
  manager, move picker).
- A "select range" gesture (e.g. shift+click or visual-mode-style range
  marking) — only per-row toggle and mark-all.

## Design notes

- **Marker glyph**: cell text `"[x]"`/`"[ ]"` was the first attempt, but
  `tview.Table` always interprets `[...]` in cell text as a color/region
  tag (no per-cell opt-out), so `"[x]"` was silently swallowed and never
  rendered — a real display bug, not a cosmetic choice. Switched to `"✓"`
  (marked) / `" "` (unmarked), which have no tag syntax to collide with.
- **No-marks behavior for `d`/`m`**: no-op with a status bar hint
  ("No messages marked (press space to mark)"), not a fallback to acting
  on the cursor row. The user's own wording ("marks the message, then 'd'
  deletes the selected message(s)") ties `d`/`m` to the marked set, and a
  silent fallback to the cursor row on a destructive action risks
  deleting/moving something the user didn't intend to touch.
- **Bulk delete/move run in a goroutine**: single-item delete/move
  elsewhere in the app (`message_detail.go`) call the backend synchronously
  on the UI goroutine; that's fine for one HTTP call but would visibly
  freeze the UI for a batch of many. Bulk operations here are wrapped in
  `go func() { ... }` like the existing purge flow.

## Files touched

| File | Change |
|---|---|
| `tui/internal/app/messages.go` | marking state, marker column, 5 new keybindings, bulk delete/move |
| `tui/internal/app/messages_test.go` | tests for marking logic and no-marks guards; updated column count/header tests |

## Definition of done

1. `go build ./...`, `go vet ./...`, `go test ./...` pass in `tui/`.
2. Verified live against a real broker: marked all messages in a test
   queue, deleted them (batch delete confirmed and reloaded to empty);
   seeded more, marked all, moved them to a disposable second queue
   (created via JMX `addQueue` for the test, removed afterward via
   `removeQueue`) and confirmed they landed there.
