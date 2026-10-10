package ui

import (
	"testing"
	"time"

	"github.com/gdamore/tcell/v2"
	"github.com/rivo/tview"
)

// tableWithRows returns a table like the app's lists: a fixed, not
// selectable header row, then dataRows rows of cols cells, rows
// selectable.
func tableWithRows(dataRows, cols int) *tview.Table {
	t := tview.NewTable().SetSelectable(true, false).SetFixed(1, 0)
	fill(t, dataRows, cols)
	return t
}

// fill replaces t's content with a header and dataRows data rows,
// keeping its selection, the way the views repaint after a reload.
func fill(t *tview.Table, dataRows, cols int) {
	t.Clear()
	for c := 0; c < cols; c++ {
		t.SetCell(0, c, tview.NewTableCell("H").SetSelectable(false))
		for r := 1; r <= dataRows; r++ {
			t.SetCell(r, c, tview.NewTableCell("x"))
		}
	}
}

func TestClampTableSelection(t *testing.T) {
	tests := []struct {
		name             string
		dataRows, cols   int
		selRow, selCol   int
		wantRow, wantCol int
	}{
		{"beyond the rows", 3, 4, 7, 0, 3, 0},
		{"beyond the columns", 3, 4, 2, 9, 2, 3},
		{"header only, stale row", 0, 4, 5, 0, 0, 0},
		{"inside the content", 3, 4, 2, 1, 2, 1},
		{"on the header", 3, 4, 0, 0, 0, 0},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			table := tableWithRows(tt.dataRows, tt.cols)
			table.Select(tt.selRow, tt.selCol)
			ClampTableSelection(table)
			if row, col := table.GetSelection(); row != tt.wantRow || col != tt.wantCol {
				t.Errorf("selection = (%d, %d), want (%d, %d)", row, col, tt.wantRow, tt.wantCol)
			}
		})
	}
}

func TestClampTableSelectionLeavesAnEmptyTableAlone(t *testing.T) {
	table := tview.NewTable().SetSelectable(true, false)
	table.Select(4, 2)
	ClampTableSelection(table)
	if row, col := table.GetSelection(); row != 4 || col != 2 {
		t.Errorf("selection = (%d, %d), want (4, 2) unchanged", row, col)
	}
}

// TestNavigationInAnEmptiedTableEnds reproduces the hang: a table whose
// selection was a few rows down, refilled with only its header, then a
// navigation key through tview's own InputHandler. Without the clamp the
// key never returns (tview v0.42 loops forever); a timeout turns that into
// a failure instead of a stuck test run.
func TestNavigationInAnEmptiedTableEnds(t *testing.T) {
	keys := []struct {
		name string
		ev   *tcell.EventKey
	}{
		{"down", tcell.NewEventKey(tcell.KeyDown, 0, tcell.ModNone)},
		{"up", tcell.NewEventKey(tcell.KeyUp, 0, tcell.ModNone)},
		{"j", tcell.NewEventKey(tcell.KeyRune, 'j', tcell.ModNone)},
		{"k", tcell.NewEventKey(tcell.KeyRune, 'k', tcell.ModNone)},
		{"page down", tcell.NewEventKey(tcell.KeyPgDn, 0, tcell.ModNone)},
		{"page up", tcell.NewEventKey(tcell.KeyPgUp, 0, tcell.ModNone)},
		{"home", tcell.NewEventKey(tcell.KeyHome, 0, tcell.ModNone)},
		{"end", tcell.NewEventKey(tcell.KeyEnd, 0, tcell.ModNone)},
	}
	for _, k := range keys {
		t.Run(k.name, func(t *testing.T) {
			table := tableWithRows(5, 4)
			table.Select(4, 0)
			fill(table, 0, 4)

			ClampTableSelection(table)

			done := make(chan struct{})
			go func() {
				table.InputHandler()(k.ev, func(tview.Primitive) {})
				close(done)
			}()
			select {
			case <-done:
			case <-time.After(2 * time.Second):
				t.Fatalf("%s in an emptied table didn't return within 2 s (tview's navigation loop)", k.name)
			}
		})
	}
}
