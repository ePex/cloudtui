package views

import (
	"testing"

	"github.com/rivo/tview"
)

func TestHomeViewNameAndTitle(t *testing.T) {
	h := NewHome(nil, "#ffffff", "#cccccc")
	if got := h.Name(); got != "home" {
		t.Errorf("Name() = %q, want %q", got, "home")
	}
	if got := h.Title(); got != "Home" {
		t.Errorf("Title() = %q, want %q", got, "Home")
	}
}

func TestHomeViewPrimitiveIsTable(t *testing.T) {
	h := NewHome(nil, "#ffffff", "#cccccc")
	if _, ok := h.Primitive().(*tview.Table); !ok {
		t.Errorf("Primitive() = %T, want *tview.Table", h.Primitive())
	}
}

func TestRepaintHomeTableSetsRows(t *testing.T) {
	table := tview.NewTable()
	infos := []ViewInfo{
		{Name: "home", Description: "Landing page"},
		{Name: "settings", Description: "App settings"},
	}
	RepaintHomeTable(table, infos, "#ffffff", "#cccccc")

	if got, want := table.GetRowCount(), 2; got != want {
		t.Errorf("GetRowCount() = %d, want %d", got, want)
	}
	if got := table.GetCell(0, 1).Text; got != "Landing page" {
		t.Errorf("cell(0,1).Text = %q, want %q", got, "Landing page")
	}
	if got := table.GetCell(1, 1).Text; got != "App settings" {
		t.Errorf("cell(1,1).Text = %q, want %q", got, "App settings")
	}
}

func TestRepaintHomeTableClearsBeforeRepaint(t *testing.T) {
	table := tview.NewTable()
	RepaintHomeTable(table, []ViewInfo{{Name: "a", Description: "desc a"}}, "#fff", "#ccc")
	RepaintHomeTable(table, []ViewInfo{
		{Name: "x", Description: "desc x"},
		{Name: "y", Description: "desc y"},
	}, "#fff", "#ccc")

	if got, want := table.GetRowCount(), 2; got != want {
		t.Errorf("GetRowCount() after re-paint = %d, want %d", got, want)
	}
}

func TestNewHomeEmptyInfos(t *testing.T) {
	h := NewHome(nil, "#ffffff", "#cccccc")
	table, ok := h.Primitive().(*tview.Table)
	if !ok {
		t.Fatalf("Primitive() = %T, want *tview.Table", h.Primitive())
	}
	if got := table.GetRowCount(); got != 0 {
		t.Errorf("GetRowCount() with nil infos = %d, want 0", got)
	}
}
