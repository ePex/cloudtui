package view

import (
	"fmt"
	"strings"

	"github.com/gdamore/tcell/v2"
	"github.com/rivo/tview"
)

// dynamicWrapWidth computes the wrap width for a table's free-text
// column from the table's actual current rendered width
// (table.GetInnerRect()) minus otherColumnsWidth — the caller's best
// estimate of every other column's width (their known MaxWidth caps, or
// a fixed content width for an uncapped one like Timestamp) plus a
// small margin for inter-column spacing.
//
// Earlier attempts used a fixed constant instead (first 80, then 70) —
// found live (CR 92) that any single fixed number is eventually wrong:
// too wide and it's a no-op (an exactly-that-length preview never
// wraps, so tview just keeps clipping it as before); too narrow and it
// wraps more aggressively than the column actually needs. Worse, a
// fixed width can still exceed the *real* space left for the free-text
// column once every other column's actual width is accounted for —
// tview then silently re-clips individual wrapped lines with its own
// "…" on top of the intentional line breaks, a confusing
// double-truncation. Deriving the width from the table's live geometry
// instead tracks the actual available space directly.
//
// This can only be as accurate as GetInnerRect() is at the moment
// renderRows() calls it, which needs the table to have been laid out by
// its parent (e.g. a Flex) at least once — in practice this is already
// true by the time a user can press the wrap key at all, since the view
// must already be visible (and thus already laid out) to receive that
// keypress. Since renderRows() only re-runs on a reload or on toggling
// wrap itself, the computed width can go stale relative to a manual
// terminal resize until the next one of those — an accepted trade-off,
// not a bug: tview.Table gives no hook to react to every resize short
// of every Draw() call. The result is clamped to a minimum of 20 so a
// pathologically narrow table (or one that hasn't been laid out yet,
// whose un-laid-out tview.Box defaults to a small placeholder size)
// never computes something unreadably tiny or negative.
func dynamicWrapWidth(table *tview.Table, otherColumnsWidth int) int {
	_, _, tableWidth, _ := table.GetInnerRect()
	width := tableWidth - otherColumnsWidth
	if width < 20 {
		width = 20
	}
	return width
}

// maxWrapLines caps how many lines a single item's wrapped text can
// expand to (see wrapMultilineText) — without this, one very long or
// heavily multi-line message (a large stack trace, a JSON blob) could
// produce dozens of continuation rows and bury every other item in the
// list.
//
// 50, not 15: found live (CR 92) against real CloudWatch data that a
// single HTTP request/response log entry (headers plus a JSON body) is
// routinely 20-40+ lines on its own — 15 hit the "… N more line(s)"
// indicator on completely ordinary logging, not just pathological
// cases. 50 covers that comfortably while still bounding a truly
// extreme outlier (a multi-KB stack trace or blob).
const maxWrapLines = 50

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
