# Plan — FE 24: multi-select on the messages list

## Data model

```go
type messagesView struct {
    // ...existing fields...
    marked map[string]bool // message IDs currently marked
}
```

Keyed by message ID (not row index), since `repaint()` re-sorts by
timestamp on every load — a row-index key would silently point at the
wrong message after a reorder. `repaint()` resets `marked` to an empty map
every time, for the same reason (see spec.md's "out of scope").

## Table layout

Marker column inserted at index 0, header `""`, shifting the existing five
columns (ID, TYPE, CORR.ID, TIMESTAMP, PREVIEW) to indices 1–5.

`markerCell(marked bool) *tview.TableCell` returns `"✓"` (accent color) or
`" "` (text color), centered. See spec.md's design note on why `"[x]"`
doesn't work.

`refreshMarkerColumn()` redraws just column 0 from the current `marked` map
— used by `toggleMark`/`markAll`/`clearMarks` so they don't need a full
`repaint()` (no re-fetch, no re-sort, marks untouched).

## Keybindings (added to the table's existing `SetInputCapture`)

| Key | Handler | Behavior |
|---|---|---|
| `space` | `toggleMark` | Flip mark on cursor row, advance cursor. No-op (with status message) if the row has no ID. |
| `a` | `markAll` | Mark every message with an ID; status reports how many, and how many were skipped for lacking one. |
| `n` | `clearMarks` | Empty the marked set; no-op if already empty. |
| `d` | `deleteMarked` | Confirm once (wording depends on marks vs. fallback), then `RemoveMessage` per target ID in a goroutine, reload. No-op only if there's truly nothing to act on. |
| `m` | `moveMarked` | `showMovePicker` once, then `MoveMessage` per target ID to the chosen target in a goroutine, reload. No-op only if there's truly nothing to act on. |

`markedIDs()` returns marked IDs in the table's current display order
(iterates `mv.msgs`, not the map, since Go map iteration order is random).

`targetIDs()` returns `markedIDs()` if non-empty, else the single ID of the
message under the cursor (or `nil` if that row has no ID, or the list is
empty) — this is what `deleteMarked`/`moveMarked` actually operate on, per
explicit follow-up direction that `d`/`m` should act on the cursor row when
nothing is marked, not no-op. Marks always take priority over the cursor
when both exist.

## Bulk operation error handling

Both `deleteMarked` and `moveMarked` loop over all marked IDs even if one
fails (`RemoveMessage`/`MoveMessage` return an `error` per call) — logged via
`slog.Error`, counted, and summarized in the final status bar message
(`"Deleted 3/4 marked message(s); 1 failed"` style) rather than aborting the
whole batch on the first error.

## Testing

`messages_test.go`:

- Updated `TestMessagesViewHeaderLabels` / `TestMessagesViewColumnCount` for
  the new marker column (6 columns now).
- New: shortcut list includes the 5 new keys; `toggleMark` marks/unmarks
  and advances the cursor; `toggleMark`/`markAll` skip ID-less messages;
  `clearMarks` empties the set; `markedIDs` matches display order;
  `repaint` clears marks; `deleteMarked`/`moveMarked` are no-ops (don't
  open the confirm dialog / move picker) when nothing is marked.

The confirmed-delete/confirmed-move paths that actually call the backend
aren't unit tested — there's no existing precedent for driving the
confirm-dialog "Yes" path or the move-picker selection in this codebase's
tests (checked: `queues_test.go`'s fake backend stubs for
`PurgeQueue`/`RemoveMessage`/`MoveMessage` are used for unrelated tests,
not to exercise those confirm flows). Verified manually instead, against a
live broker — see spec.md, Definition of done #2.

## No new dependencies
