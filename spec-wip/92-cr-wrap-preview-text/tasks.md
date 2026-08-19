# Tasks: preview/message text wrapping

Each task is implemented and pushed only after being separately approved.

1. [x] Add `tui/internal/view/wraptext.go` (new): `wrapText(s string,
   width int) []string` (greedy word-wrap, hard-break on an
   over-length single word, always returns ≥1 element) and
   `setContinuationRow(table, rowIdx, numCols, textCol int, text
   string, textColor tcell.Color)` (non-selectable row, every column
   explicitly set). `previewWrapWidth = 80` const. Plus
   `wraptext_test.go` covering: empty string, fits-on-one-line,
   exact-width boundary, multi-word wrap, single over-length word,
   leading/trailing whitespace, and `setContinuationRow`'s cell
   text/selectability on a real `tview.Table`.
2. [x] Wire wrap into `tui/internal/view/messages.go`: `wrap bool`
   field; extract row-building into `renderRows()` (wrap-aware,
   builds `rowToIdx`/`idxToRow`, preserves current item selection);
   `repaint()` calls `renderRows()` in place of its inline loop, keeps
   its own scroll-reset; `w` case (toggle + `renderRows()` +
   context-hint rebuild, matching `queues.go`'s `M`/`c` pattern);
   `Shortcuts()` entry; replace `idx := row - 1` in
   `SetSelectedFunc`/`targetIDs()`/`toggleMark()` with `rowToIdx`
   lookups; `refreshMarkerColumn()` uses `idxToRow`; `toggleMark()`'s
   cursor-advance forwards a synthetic `KeyDown` through
   `table.InputHandler()` instead of raw `Select(row+1, 0)`. Tests:
   wrap toggle produces expected continuation rows; `j`/`k` skip
   continuation rows (drives the real `SetInputCapture` closure);
   `Enter` opens the right message after an earlier item wrapped;
   marks survive toggling wrap; `toggleMark`'s advance lands on the
   next item's primary row when the current item wrapped; `w` in
   `Shortcuts()`; `fakeViewHost.contextHint` reflects `wrap: on/off`
   after a simulated `w` keypress.
3. [x] Bump `ui.ShortcutPanelRows` 11→12 (`tui/internal/ui/topbar.go`,
   doc comment included) since task 2 makes messages.go's
   `Shortcuts()` 12 entries; update `topbar_test.go`'s two
   literal-11 assertions to 12.
4. [ ] Wire wrap into `tui/internal/view/logsearch.go`: `wrap bool`
   field; extract `renderRows()` (wrap-aware, builds `rowToIdx`,
   preserves current item selection); `repaint()` calls it, keeps its
   own scroll-reset; `w` case + context-hint rebuild; `Shortcuts()`
   entry; `SetSelectedFunc` uses `rowToIdx`. Tests mirroring task 2's
   (minus marking, which doesn't apply here).
5. [ ] Wire wrap into `tui/internal/view/datadoglogs.go`: same shape
   as task 4.
6. [ ] `verify-live`: drive the real TUI against a real broker,
   toggling `w` in the message browser — confirm continuation rows
   render correctly, `j`/`k` skip them, marking still works and
   survives the toggle, and the context-panel hint reflects state.
   Not strictly required by `tui/CLAUDE.md` (rendering-only, no
   backend behavior change) but cheap insurance given CR 91's history.
   Record what was checked here.
7. [ ] Merge-back: fold the `w` wrap toggle into
   `spec/08-message-browser-and-detail/spec.md`,
   `spec/17-aws-cloudwatch-logs/spec.md`, and
   `spec/18-datadog-logs/spec.md` as end-state behavior; delete
   `spec-wip/92-cr-wrap-preview-text/`; push; mark the PR ready for
   review (no longer draft).
