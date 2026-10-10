package ui

import "github.com/rivo/tview"

// ClampTableSelection moves t's selection back inside its content when
// the content shrank below it: to the last row and column that exist
// (the header, once the table holds nothing else). A selection inside
// the content, or an empty table, is left alone.
//
// It works around a tview v0.42.0 hang: a table keeps its selection
// when its content is cleared and refilled, and the arrow keys walk the
// cells, wrapping around, until they reach either a selectable cell or
// the previous selection. In a table with nothing selectable (only its
// header) and a selection past its last row, neither ever comes, and the
// UI goroutine spins at 100 % CPU. With the selection inside the
// content, every walk ends within one lap.
//
// Select only fires SetSelectionChangedFunc, not SetSelectedFunc (Enter),
// so moving the selection here has no other effect.
func ClampTableSelection(t *tview.Table) {
	rows, cols := t.GetRowCount(), t.GetColumnCount()
	if rows == 0 || cols == 0 {
		return
	}
	row, col := t.GetSelection()
	if row < rows && col < cols {
		return
	}
	t.Select(min(row, rows-1), min(col, cols-1))
}
