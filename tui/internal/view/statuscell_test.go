package view

import (
	"testing"

	"github.com/gdamore/tcell/v2"
	"github.com/rivo/tview"
)

func TestClearTableBodyRemovesAllRowsButHeader(t *testing.T) {
	table := tview.NewTable()
	table.SetCell(0, 0, tview.NewTableCell("HEADER"))
	table.SetCell(1, 0, tview.NewTableCell("a"))
	table.SetCell(2, 0, tview.NewTableCell("b"))
	table.SetCell(3, 0, tview.NewTableCell("c"))

	clearTableBody(table)

	if got := table.GetRowCount(); got != 1 {
		t.Errorf("GetRowCount() after clearTableBody() = %d, want 1 (header only)", got)
	}
	if got := table.GetCell(0, 0).Text; got != "HEADER" {
		t.Errorf("header cell = %q, want %q (untouched)", got, "HEADER")
	}
}

func TestClearTableBodyNoOpOnHeaderOnlyTable(t *testing.T) {
	table := tview.NewTable()
	table.SetCell(0, 0, tview.NewTableCell("HEADER"))

	clearTableBody(table)

	if got := table.GetRowCount(); got != 1 {
		t.Errorf("GetRowCount() after clearTableBody() = %d, want 1", got)
	}
}

func TestShowStatusCellWritesTextColorAndTitle(t *testing.T) {
	table := tview.NewTable()
	table.SetCell(0, 0, tview.NewTableCell("HEADER"))
	table.SetCell(1, 0, tview.NewTableCell("stale"))

	clearStateCalled := false
	showStatusCell(table, 1, "Error: boom", tcell.ColorRed, " My View ", func() {
		clearStateCalled = true
	})

	if !clearStateCalled {
		t.Error("clearState was not called")
	}
	if got := table.GetRowCount(); got != 2 {
		t.Fatalf("GetRowCount() = %d, want 2 (header + 1 status row)", got)
	}
	cell := table.GetCell(1, 1)
	if cell.Text != "Error: boom" {
		t.Errorf("cell(1,1) text = %q, want %q", cell.Text, "Error: boom")
	}
	fg, _, _ := cell.Style.Decompose()
	if fg != tcell.ColorRed {
		t.Errorf("cell(1,1) color = %v, want %v", fg, tcell.ColorRed)
	}
	if got := table.GetTitle(); got != " My View " {
		t.Errorf("GetTitle() = %q, want %q", got, " My View ")
	}
	// The old row-0 status cell (column 0) must have been cleared away
	// by clearTableBody before the new one was written at column 1.
	if got := table.GetCell(1, 0); got != nil && got.Text == "stale" {
		t.Errorf("cell(1,0) = %q, want the stale row cleared", got.Text)
	}
}

// TestShowStatusCellClearsStateBeforeWritingCell confirms clearState
// runs before the table is touched — matters if a future caller's
// closure ever needs to read table state before it's cleared, even
// though none of the current callers do.
func TestShowStatusCellClearsStateBeforeWritingCell(t *testing.T) {
	table := tview.NewTable()
	table.SetCell(0, 0, tview.NewTableCell("HEADER"))

	var rowCountAtClearState int
	showStatusCell(table, 0, "msg", tcell.ColorBlue, " Title ", func() {
		rowCountAtClearState = table.GetRowCount()
	})

	if rowCountAtClearState != 1 {
		t.Errorf("GetRowCount() during clearState = %d, want 1 (table untouched until after clearState runs)", rowCountAtClearState)
	}
}
