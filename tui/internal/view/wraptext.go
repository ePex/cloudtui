package view

import (
	"fmt"
	"strings"

	"github.com/gdamore/tcell/v2"
	"github.com/rivo/tview"
)

// previewWrapWidth is the fixed line width used to word-wrap a table's
// free-text column when wrapping is enabled. Fixed rather than derived
// from the table's live rendered column width, since the latter would
// need recomputing on every terminal resize — tview.Table gives no hook
// for that short of reacting to every Draw() call. A fixed width is
// simpler and resize-safe; the upstream text this wraps is already
// capped (queue message previews at 80 chars, log message previews at
// 200 via logEventPreview), so it never produces more than a handful of
// lines.
//
// 70, not 80: found live (verify-live, CR 92) that 80 was a no-op for
// messages.go's PREVIEW column specifically — with every column setting
// its own Expansion(1) (the bug messageColumns/logSearchColumns/
// datadogLogsColumns fixed), PREVIEW's actual rendered width was well
// under 80 in practice (measured ~76 in a 160-column terminal, narrower
// still at more typical widths), so an exactly 80-char preview against
// an 80-wide wrap never wrapped — tview kept silently clipping it
// exactly as before, toggle or not. Once PREVIEW/MESSAGE's own
// Expansion was raised well above every other column's (see the
// column-weight tables in messages.go/logsearch.go/datadoglogs.go),
// the rendered column got wide enough on its own that 70 is a better
// balance than the narrower 40 first tried: still helps on a narrow
// terminal, wraps less "unnecessarily" now that the column usually
// has more room by default. Either way this is a fixed width, so it's
// never perfectly matched to the live column — that's the accepted
// resize-safety trade-off, not a bug.
const previewWrapWidth = 70

// maxWrapLines caps how many lines a single item's wrapped text can
// expand to (see wrapMultilineText) — without this, one very long or
// heavily multi-line message (a large stack trace, a JSON blob) could
// produce dozens of continuation rows and bury every other item in the
// list. 15 is generous enough to show a genuinely useful chunk of a
// multi-line message while still leaving most of the visible table for
// other items.
const maxWrapLines = 15

// wrapText greedily word-wraps s into lines of at most width runes,
// breaking on whitespace. A single word longer than width is hard-broken
// across multiple lines. Always returns at least one element — []string{""}
// for an empty (or all-whitespace) s — so callers can unconditionally
// treat lines[0] as the primary row's text and lines[1:] as continuation
// rows without a length check.
func wrapText(s string, width int) []string {
	if width <= 0 {
		return []string{s}
	}

	fields := strings.Fields(s)
	if len(fields) == 0 {
		return []string{""}
	}

	var lines []string
	var cur []rune

	for _, f := range fields {
		word := []rune(f)

		if len(word) > width {
			if len(cur) > 0 {
				lines = append(lines, string(cur))
				cur = nil
			}
			for len(word) > width {
				lines = append(lines, string(word[:width]))
				word = word[width:]
			}
			cur = word
			continue
		}

		switch {
		case len(cur) == 0:
			cur = word
		case len(cur)+1+len(word) <= width:
			cur = append(cur, ' ')
			cur = append(cur, word...)
		default:
			lines = append(lines, string(cur))
			cur = word
		}
	}
	if len(cur) > 0 {
		lines = append(lines, string(cur))
	}

	return lines
}

// wrapMultilineText word-wraps s to width, respecting s's own existing
// line breaks — each of s's original lines is word-wrapped
// independently, rather than s being flattened into one continuous
// paragraph and re-flowed. This matters for genuinely multi-line
// content (a stack trace, a formatted payload): flattening it would
// jumble separate lines together, losing exactly the structure that
// makes it readable. \r\n is normalized to \n first.
//
// Returns at most maxLines lines (maxLines <= 0 means unbounded): if
// wrapping would produce more, the result is truncated to maxLines-1
// real lines plus a final "… N more line(s)" indicator, rather than
// silently cutting off content with no sign anything was omitted.
func wrapMultilineText(s string, width, maxLines int) []string {
	rawLines := strings.Split(strings.ReplaceAll(s, "\r\n", "\n"), "\n")
	var all []string
	for _, rl := range rawLines {
		all = append(all, wrapText(rl, width)...)
	}
	if maxLines <= 0 || len(all) <= maxLines {
		return all
	}
	kept := all[:maxLines-1]
	remaining := len(all) - len(kept)
	return append(kept, fmt.Sprintf("… %d more line(s) — see detail for full text", remaining))
}

// setContinuationRow writes a non-selectable row at rowIdx: text in
// column textCol (colored textColor, with textExpansion matching the
// primary row's expansion for that column so the column's rendered
// width stays consistent even if a screen's visible rows happen to be
// all continuation rows of one very tall item), blank everywhere else.
// Every column is set explicitly non-selectable — an unset
// tview.TableCell defaults to selectable (NotSelectable's zero value is
// false) — the same reason every header row column is set individually.
// tview.Table's own up/down navigation already skips non-selectable
// cells (the same mechanism the header row relies on), so continuation
// rows are simply invisible to cursor movement with no further code.
func setContinuationRow(table *tview.Table, rowIdx, numCols, textCol int, text string, textColor tcell.Color, textExpansion int) {
	for col := 0; col < numCols; col++ {
		cell := tview.NewTableCell("").SetSelectable(false)
		if col == textCol {
			cell.SetText(text).SetTextColor(textColor).SetExpansion(textExpansion)
		}
		table.SetCell(rowIdx, col, cell)
	}
}
