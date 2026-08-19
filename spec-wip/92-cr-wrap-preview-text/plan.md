# Plan: preview/message text wrapping

## Approach

`tview.Table` renders exactly one screen line per row with no built-in
multi-line cell support (confirmed by reading `rivo/tview@v0.42.0`'s
`table.go`: `Draw()` prints each cell on a single `rowY`). Wrapping is
implemented by inserting extra **continuation rows** directly below an
item's primary row — one per wrapped line beyond the first — with every
column in a continuation row marked `SetSelectable(false)`, same as the
header row (row 0) already is.

This isn't a new mechanism: `tview.Table`'s own up/down navigation
(confirmed by reading the `down()`/`up()`/`forward()`/`backwards()`
closures in `InputHandler()`) already skips non-selectable cells when
scanning for the next row to land on — the same logic that lets the
header row be skipped today. So `j`/`k`/arrow navigation, `Enter`
(`SetSelectedFunc`), and the header-skip all keep working with zero new
navigation code — continuation rows are simply invisible to the cursor.

What each view needs, instead, is a **row→item index mapping**, since
row number no longer equals `item index + 1` once some items span
multiple rows. Three call sites currently assume `idx := row - 1`:
`SetSelectedFunc` (all three views), plus `targetIDs()`/`toggleMark()`
(messages.go only, for marking).

## Shared helper — `tui/internal/view/wraptext.go` (new)

```go
// wrapText greedily word-wraps s into lines of at most width runes,
// breaking on whitespace; a single word longer than width is
// hard-broken. Always returns at least one element (a single "" for an
// empty s), so callers can unconditionally treat lines[0] as the
// primary row's text and lines[1:] as continuation rows.
func wrapText(s string, width int) []string

// setContinuationRow writes a non-selectable row at rowIdx: text in
// column textCol, blank (also non-selectable) everywhere else — every
// column must be explicitly set non-selectable, since an unset
// tview.TableCell defaults to selectable (NotSelectable's zero value is
// false), same reason every column of the header row is set
// individually today (see each view's setHeader()).
func setContinuationRow(table *tview.Table, rowIdx, numCols, textCol int, text string, textColor tcell.Color)
```

`previewWrapWidth = 80` (package-level const in the same file) — a
fixed width, not the table's live rendered column width (per spec.md's
resize-safety reasoning). 80 doubles as messages.go's own `Preview`
field cap, so that column typically renders as 1–2 wrapped lines;
CloudWatch/Datadog's 200-char `logEventPreview` cap yields at most
~3. No separate "max continuation lines" safety cap is needed — these
upstream caps already bound it.

No new dependency — plain string wrapping, standard library only.

## Per-view changes

Each of `messages.go`/`logsearch.go`/`datadoglogs.go` gets:

1. A `wrap bool` field (zero value = off, matching the spec's
   default-off requirement).
2. `w` in `SetInputCapture`: flips `wrap`, re-renders (see below),
   rebuilds the context-panel hint the same way `queues.go`'s `M`/`c`
   handlers already do (`rebuild lines from Shortcuts() +
   host.SetContextHint(...)`) — CR 91's `verify-live` pass found this
   exact omission live last time; this time it's designed in from the
   start **and** covered by a unit test asserting on
   `fakeViewHost.contextHint` after simulating the keypress, so it
   doesn't need a live pass to catch a regression.
3. A `Shortcuts()` entry: `{Key: "w", Description: "wrap: on/off"}`
   (live state, same pattern as the description above).
4. **Row building split out of the existing reload path.** Each view's
   `repaint()` already does more than render rows — `messages.go`'s
   also re-filters (quick search), re-sorts, and unconditionally clears
   `mv.marked` (confirmed: `applyQuickSearch` already calls
   `mv.repaint(mv.allMsgs)` on every keystroke, so marks already don't
   survive a filter change today — that's existing, accepted behavior,
   not something this CR touches). Reusing that full path for the wrap
   toggle would silently wipe the user's marks on a purely cosmetic
   toggle, which — unlike the quick-search case — has no logical reason
   to invalidate them (a plausible real flow: mark a few messages, then
   press `w` to read full previews before deciding what to delete).
   So each view's row-drawing loop is extracted into its own
   `renderRows()`, which:
   - Clears and rebuilds the table body from the view's *current* data
     slice (`mv.msgs` / `sv.results` / `dv.results`) — same data, no
     re-filter/re-sort/marks-reset.
   - Is wrap-aware: for each item, writes the primary row, then (if
     `wrap` is on and the free-text column doesn't fit) one
     `setContinuationRow` per extra wrapped line.
   - Builds `rowToIdx []int` (row → item index; index 0 unused, a
     placeholder for the header row) as a view field, replacing every
     `idx := row - 1` call site with `rowToIdx[row]` (bounds-checked
     the same way the old code already was).
   - messages.go additionally builds `idxToRow []int` (item → primary
     row), since `refreshMarkerColumn()` needs to place each item's
     marker glyph on its primary row, not at `i+1`.
   - Preserves the current selection *by item index*: reads the
     currently-selected row's item index via the *old* `rowToIdx`
     before clearing, rebuilds, then re-selects that item's new primary
     row via the *new* `idxToRow`/`rowToIdx` (clamped if the table is
     now empty). Item order/set doesn't change from toggling wrap
     alone, so index-based (not ID-based) preservation is sufficient
     and simpler.
   - `repaint()` (messages.go) / `repaint()` (logsearch.go,
     datadoglogs.go) call `renderRows()` at the end of their existing
     body, in place of the inline loop they have today, then still do
     their own `Select(1, 0)` + `SetOffset(0, 0)` reset — a genuine
     reload/filter-change should still jump to the top, matching
     today's behavior exactly. Only the `w` handler calls
     `renderRows()` directly, skipping that reset.
5. `toggleMark()`'s (messages.go only) cursor-advance changes from the
   raw `mv.table.Select(row+1, 0)` to forwarding a synthetic `KeyDown`
   event through `mv.table.InputHandler()(...)`, the same forwarding
   pattern the search-input's Up/Down handling already uses elsewhere
   in this file. Reusing `tview.Table`'s own `down()` logic (verified
   in `table.go`) means it automatically lands on the next *selectable*
   row — skipping any continuation rows the current item wrapped
   into — for free, instead of reimplementing "find the next primary
   row" by hand.

## `ShortcutPanelRows`

messages.go's `Shortcuts()` is at 11 entries today (`r`, `p`, `c`, `/`,
`f`, `Esc`, `space`, `a`, `n`, `d`, `m`); adding `w` makes 12.
`ui.ShortcutPanelRows` (`tui/internal/ui/topbar.go`) is currently 11 —
its own doc comment says to bump it whenever a view's `Shortcuts()`
grows past it, so this CR bumps it to 12 (and updates `topbar_test.go`'s
two literal-11 assertions to 12, same fix CR 91 needed and had already
verified was the sanctioned approach). logsearch.go/datadoglogs.go stay
well under the limit.

## Files touched

- `tui/internal/view/wraptext.go` (new) + `wraptext_test.go` (new).
- `tui/internal/view/messages.go` + `messages_test.go`.
- `tui/internal/view/logsearch.go` + `logsearch_test.go`.
- `tui/internal/view/datadoglogs.go` + `datadoglogs_test.go`.
- `tui/internal/ui/topbar.go` + `topbar_test.go` (the `ShortcutPanelRows`
  bump).
- `spec/08-message-browser-and-detail`, `spec/17-aws-cloudwatch-logs`,
  `spec/18-datadog-logs` — merge-back.

## Key decisions & trade-offs

- **Continuation rows via `SetSelectable(false)`, not a custom
  navigation layer**: CR 91 built a whole `ui.TableWrap` type for
  something `tview.Table` already had a hook for
  (`NotSelectable`-skipping). This CR leans on that existing mechanism
  instead of adding new navigation code — smaller, and can't drift out
  of sync with `tview`'s own up/down behavior (`g`/`G`/Home/End/PageUp/
  PageDown all go through the same `forward()`/`backwards()` scan, so
  those keys also correctly skip continuation rows with no extra code).
- **Fixed wrap width over live column width**: stated in spec.md;
  reaffirmed here now that the upstream 80/200-char caps are confirmed,
  which is what makes a fixed width sufficient rather than a
  compromise — the caps mean even a "wrong" width still shows the
  (short) full text within 1–3 lines.
- **Row building split into `renderRows()`**: the alternative (reuse
  the existing full `repaint()`/`applyQuickSearch`-style reload for the
  toggle) is simpler code but silently drops marks and resets scroll on
  a toggle that has no logical reason to do either — judged not
  acceptable UX for a cosmetic toggle, worth the extra split.
- **Index-based, not ID-based, selection preservation**: simpler than
  ID lookups, and valid because nothing about item order/set changes
  when only `wrap` flips — filtering/sorting/marks-reset are exactly
  the parts `renderRows()` deliberately does *not* redo.

## Testing

- `wraptext_test.go`: `wrapText` — empty string, fits-on-one-line,
  exact-width boundary, multi-word wrap, single word longer than
  width (hard break), leading/trailing whitespace. `setContinuationRow`
  — cell text/selectability on a real `tview.Table`.
- Per view: wrap toggle produces the expected continuation rows (row
  count grows, continuation row cells carry the right wrapped text and
  are non-selectable); `j`/`k` navigation skips continuation rows
  end-to-end (drives the real `SetInputCapture` closure, not a mock);
  `Enter`/`SetSelectedFunc` opens the right item when the cursor is on
  an item after an earlier one wrapped into multiple rows (proves
  `rowToIdx` offsets correctly); `Shortcuts()` includes `w`;
  `fakeViewHost.contextHint` reflects `wrap: on`/`wrap: off` after a
  simulated `w` keypress (this is the test that would have caught CR
  91's live-only-found bug).
- messages.go additionally: marks survive toggling wrap (mark a
  message, toggle wrap on, confirm still marked); `toggleMark`'s
  cursor-advance lands on the next item's primary row (not a
  continuation row) when the current item wrapped into multiple lines;
  `targetIDs()` resolves the right message with wrap on.
- Manual: `verify-live` isn't strictly required by `tui/CLAUDE.md`'s
  rule (this doesn't touch queue/message/connection *backend*
  behavior, only rendering) but is worth doing anyway given CR 91's
  history of a real bug only surfacing live — a quick pass toggling `w`
  in the message browser against a real broker, confirming the
  continuation rows render and read correctly, is cheap insurance.
