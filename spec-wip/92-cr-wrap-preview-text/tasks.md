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
4. [x] Wire wrap into `tui/internal/view/logsearch.go`: `wrap bool`
   field; extract `renderRows()` (wrap-aware, builds `rowToIdx`,
   preserves current item selection); `repaint()` calls it, keeps its
   own scroll-reset; `w` case + context-hint rebuild; `Shortcuts()`
   entry; `SetSelectedFunc` uses `rowToIdx`. Tests mirroring task 2's
   (minus marking, which doesn't apply here).
5. [x] Wire wrap into `tui/internal/view/datadoglogs.go`: same shape
   as task 4.
6. [x] `verify-live`: drove the real TUI (tmux) against a real ActiveMQ
   broker, message browser only (CloudWatch/Datadog need AWS SSO/
   Datadog credentials this environment doesn't have — same gap CR 91
   hit; the wrap mechanism is shared and already covered by
   production-closure unit tests for those two views).

   Found and fixed a real design bug in the process: `previewWrapWidth`
   was set to 80, matching `queue.Message.Preview`'s own 80-char
   upstream cap — but the messages table's rendered PREVIEW column is
   well under 80 chars in practice (it competes with 5 other columns;
   measured ~76 in a 160-column terminal, narrower at typical widths).
   Since `wrapText` only wraps text *longer* than the width, an exactly
   80-char preview against an 80-wide wrap never triggered — toggling
   `w` flipped the context hint to "on" but the row looked identical,
   still silently clipped by `tview` itself, defeating the feature's
   purpose. Fixed by dropping `previewWrapWidth` to 40 (user's call,
   asked directly rather than silently repicking a number) — reliably
   wraps even in a narrow column. Updated the three views' "produces
   continuation rows" tests to assert `rowCount > 2` instead of a
   width-specific exact count, and the two log views' "selected func"
   tests to locate the second item's row via `rowToIdx` instead of a
   hardcoded row number — both were accidentally coupled to the old
   width's exact wrap-line count.

   After the fix, verified in the message browser: wrap off by default
   (pass); `w` toggle wraps the full 80-char preview across multiple
   rows, revealing text `tview` was clipping before (pass); `j`
   navigation with a wrapped item present doesn't misbehave (pass);
   `space` places the mark glyph on the correct (primary) row, not a
   continuation row (pass); toggling `w` off returns to a single
   truncated line, and the mark from before the toggle is still there
   (pass, confirms marks-survive-toggle live not just in the unit
   test). Cleanup: removed the disposable `cr92.wrap.verify` test
   queue, the scratch `tui/config.yaml` (none existed before), and the
   scratch binary/tmux session.
7. [ ] Merge-back: fold the `w` wrap toggle into
   `spec/08-message-browser-and-detail/spec.md`,
   `spec/17-aws-cloudwatch-logs/spec.md`, and
   `spec/18-datadog-logs/spec.md` as end-state behavior; delete
   `spec-wip/92-cr-wrap-preview-text/`; push; mark the PR ready for
   review (no longer draft).
