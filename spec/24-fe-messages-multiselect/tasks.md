# Tasks — FE 24: multi-select on the messages list

Plan: [plan.md](plan.md)

1. [x] Add `marked` field, marker column, and `markerCell`/
   `refreshMarkerColumn`/`markedIDs` helpers to `messages.go`.
2. [x] Add `toggleMark`, `markAll`, `clearMarks` and wire `space`/`a`/`n`.
3. [x] Add `deleteMarked` (confirm once, bulk `RemoveMessage`, reload) and
   wire `d`.
4. [x] Add `moveMarked` (single move-picker, bulk `MoveMessage`, reload)
   and wire `m`.
5. [x] Update `Shortcuts()` to list the 5 new keys.
6. [x] Fix marker glyph after discovering `"[x]"` gets swallowed by
   `tview.Table`'s tag parsing — switched to `"✓"`/`" "`.
7. [x] Update `messages_test.go` (header/column count for the new column)
   and add tests for the marking logic and the no-marks guards.
8. [x] `go build ./...`, `go vet ./...`, `go test ./...`.
9. [x] Manual verification against a live broker: bulk delete (14
   messages) and bulk move (3 messages, to a disposable queue created and
   removed for the test) both confirmed working end-to-end.
