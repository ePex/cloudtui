package ui

import (
	"testing"

	"github.com/gdamore/tcell/v2"
	"github.com/rivo/tview"
)

// newTestTable builds a table with a non-selectable header row (row 0)
// and dataRows selectable data rows below it, mirroring how every list
// view in this app sets up its table.
func newTestTable(dataRows int) *tview.Table {
	table := tview.NewTable()
	table.SetSelectable(true, false)
	table.SetCell(0, 0, tview.NewTableCell("HEADER").SetSelectable(false))
	for i := 0; i < dataRows; i++ {
		table.SetCell(i+1, 0, tview.NewTableCell("row"))
	}
	return table
}

func downEvent() *tcell.EventKey { return tcell.NewEventKey(tcell.KeyDown, 0, tcell.ModNone) }
func upEvent() *tcell.EventKey   { return tcell.NewEventKey(tcell.KeyUp, 0, tcell.ModNone) }

func TestTableWrap_DisabledClamps(t *testing.T) {
	table := newTestTable(3)
	table.Select(1, 0)
	var w TableWrap

	if got := w.HandleNav(table, 1, upEvent()); got == nil {
		t.Fatalf("HandleNav() = nil, want a forwarded event when wrap is disabled")
	}
	if row, _ := table.GetSelection(); row != 1 {
		t.Fatalf("HandleNav() should not move selection itself, got row %d", row)
	}
}

func TestTableWrap_EnabledWrapsAtBottomEdge(t *testing.T) {
	table := newTestTable(3)
	table.Select(3, 0) // last data row
	var w TableWrap
	w.Toggle()

	if got := w.HandleNav(table, 1, downEvent()); got != nil {
		t.Fatalf("HandleNav() = %v, want nil (fully handled) when wrapping", got)
	}
	if row, _ := table.GetSelection(); row != 1 {
		t.Fatalf("selection = %d, want 1 (wrapped to first data row)", row)
	}
}

func TestTableWrap_EnabledWrapsAtTopEdge(t *testing.T) {
	table := newTestTable(3)
	table.Select(1, 0) // first data row
	var w TableWrap
	w.Toggle()

	if got := w.HandleNav(table, 1, upEvent()); got != nil {
		t.Fatalf("HandleNav() = %v, want nil (fully handled) when wrapping", got)
	}
	if row, _ := table.GetSelection(); row != 3 {
		t.Fatalf("selection = %d, want 3 (wrapped to last data row)", row)
	}
}

func TestTableWrap_EnabledMidListMovesNormally(t *testing.T) {
	table := newTestTable(3)
	table.Select(2, 0)
	var w TableWrap
	w.Toggle()

	got := w.HandleNav(table, 1, downEvent())
	if got == nil {
		t.Fatalf("HandleNav() = nil, want a forwarded event for a mid-list move")
	}
	if got.Key() != tcell.KeyDown {
		t.Fatalf("HandleNav() key = %v, want KeyDown", got.Key())
	}
	if row, _ := table.GetSelection(); row != 2 {
		t.Fatalf("HandleNav() should not move selection itself, got row %d", row)
	}
}

func TestTableWrap_EmptyTableNoOp(t *testing.T) {
	table := newTestTable(0)
	var w TableWrap
	w.Toggle()

	if got := w.HandleNav(table, 1, downEvent()); got == nil {
		t.Fatalf("HandleNav() = nil, want a forwarded event on an empty table")
	}
	if got := w.HandleNav(table, 1, upEvent()); got == nil {
		t.Fatalf("HandleNav() = nil, want a forwarded event on an empty table")
	}
}

func TestTableWrap_SingleRowStaysPut(t *testing.T) {
	table := newTestTable(1)
	table.Select(1, 0)
	var w TableWrap
	w.Toggle()

	if got := w.HandleNav(table, 1, downEvent()); got != nil {
		t.Fatalf("HandleNav() = %v, want nil (wraps to itself)", got)
	}
	if row, _ := table.GetSelection(); row != 1 {
		t.Fatalf("selection = %d, want 1", row)
	}

	if got := w.HandleNav(table, 1, upEvent()); got != nil {
		t.Fatalf("HandleNav() = %v, want nil (wraps to itself)", got)
	}
	if row, _ := table.GetSelection(); row != 1 {
		t.Fatalf("selection = %d, want 1", row)
	}
}

func TestTableWrap_ToggleAndEnabled(t *testing.T) {
	var w TableWrap
	if w.Enabled() {
		t.Fatalf("Enabled() = true, want false for zero value")
	}
	w.Toggle()
	if !w.Enabled() {
		t.Fatalf("Enabled() = false, want true after Toggle()")
	}
	w.Toggle()
	if w.Enabled() {
		t.Fatalf("Enabled() = true, want false after second Toggle()")
	}
}

func TestTableWrap_JKAliasesNormalizeToArrowKeys(t *testing.T) {
	table := newTestTable(3)
	table.Select(2, 0)
	var w TableWrap

	jEvent := tcell.NewEventKey(tcell.KeyRune, 'j', tcell.ModNone)
	got := w.HandleNav(table, 1, jEvent)
	if got == nil || got.Key() != tcell.KeyDown {
		t.Fatalf("HandleNav('j') = %v, want a KeyDown event", got)
	}

	kEvent := tcell.NewEventKey(tcell.KeyRune, 'k', tcell.ModNone)
	got = w.HandleNav(table, 1, kEvent)
	if got == nil || got.Key() != tcell.KeyUp {
		t.Fatalf("HandleNav('k') = %v, want a KeyUp event", got)
	}
}
