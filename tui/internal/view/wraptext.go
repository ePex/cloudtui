package view

import (
	"strings"

	"github.com/gdamore/tcell/v2"
	"github.com/rivo/tview"
)

// previewWrapWidth is the fixed line width used to word-wrap a table's
// free-text column when wrapping is enabled. Fixed rather than derived
// from the table's live rendered column width, since the latter would
// need recomputing on every terminal resize — tview.Table gives no hook
// for that short of reacting to every Draw() call. A fixed width is
// simpler and resize-safe; it isn't perfectly edge-to-edge on a very wide
// terminal, but the upstream text this wraps is already capped (queue
// message previews at 80 chars, log message previews at 200 via
// logEventPreview), so it never produces more than a handful of lines.
const previewWrapWidth = 80

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
