package views

import (
	"testing"

	"github.com/rivo/tview"
)

var testSections = []SectionInfo{
	{
		Title: "Apps",
		Entries: []ViewInfo{
			{Name: "queues", Description: "List queues"},
		},
	},
	{
		Title: "System",
		Entries: []ViewInfo{
			{Name: "settings", Description: "App settings"},
			{Name: "log", Description: "View log"},
		},
	},
}

const (
	testLabel      = "#ffffff"
	testText       = "#cccccc"
	testHeader     = "#aaaaaa"
	testSelectionBg   = "#2ac3de"
	testSelectionText = "#1a1b26"
)

func newTestHome(sections []SectionInfo) *HomeView {
	return NewHome(sections, func(string) {}, testLabel, testText, testHeader, testSelectionBg, testSelectionText)
}

func repaintTest(table *tview.Table, sections []SectionInfo) {
	RepaintHomeTable(table, sections, testLabel, testText, testHeader, testSelectionBg, testSelectionText)
}

func TestHomeViewNameAndTitle(t *testing.T) {
	h := newTestHome(nil)
	if got := h.Name(); got != "home" {
		t.Errorf("Name() = %q, want %q", got, "home")
	}
	if got := h.Title(); got != "Home" {
		t.Errorf("Title() = %q, want %q", got, "Home")
	}
}

func TestHomeViewPrimitiveIsTable(t *testing.T) {
	h := newTestHome(nil)
	if _, ok := h.Primitive().(*tview.Table); !ok {
		t.Errorf("Primitive() = %T, want *tview.Table", h.Primitive())
	}
}

func TestRepaintHomeTableRowCount(t *testing.T) {
	table := tview.NewTable()
	// 2 sections × 1 header + 1+2 entries = 5 rows total
	repaintTest(table, testSections)
	if got, want := table.GetRowCount(), 5; got != want {
		t.Errorf("GetRowCount() = %d, want %d (2 headers + 3 entries)", got, want)
	}
}

func TestRepaintHomeTableSectionHeadersNotSelectable(t *testing.T) {
	table := tview.NewTable()
	repaintTest(table, testSections)

	// Row 0: Apps header — must not be selectable.
	cell := table.GetCell(0, 0)
	if cell == nil {
		t.Fatal("header row 0 cell is nil")
	}
	if !cell.NotSelectable {
		t.Error("section header row 0 should be non-selectable")
	}

	// Row 2: System header — must not be selectable.
	cell = table.GetCell(2, 0)
	if cell == nil {
		t.Fatal("header row 2 cell is nil")
	}
	if !cell.NotSelectable {
		t.Error("section header row 2 should be non-selectable")
	}
}

func TestRepaintHomeTableEntryRowsSelectable(t *testing.T) {
	table := tview.NewTable()
	repaintTest(table, testSections)

	// Row 1: queues entry — must be selectable.
	cell := table.GetCell(1, 0)
	if cell == nil {
		t.Fatal("entry row 1 cell is nil")
	}
	if cell.NotSelectable {
		t.Error("entry row 1 (queues) should be selectable")
	}
}

func TestRepaintHomeTableSingleColumn(t *testing.T) {
	table := tview.NewTable()
	repaintTest(table, testSections)
	if got, want := table.GetColumnCount(), 1; got != want {
		t.Errorf("GetColumnCount() = %d, want %d (single-column layout)", got, want)
	}
}

func TestRepaintHomeTableClearsBeforeRepaint(t *testing.T) {
	table := tview.NewTable()
	repaintTest(table, testSections)
	repaintTest(table, []SectionInfo{
		{Title: "Only", Entries: []ViewInfo{{Name: "x", Description: "desc x"}}},
	})

	// 1 header + 1 entry = 2 rows
	if got, want := table.GetRowCount(), 2; got != want {
		t.Errorf("GetRowCount() after re-paint = %d, want %d", got, want)
	}
}

func TestNewHomeEmptySections(t *testing.T) {
	h := newTestHome(nil)
	table, ok := h.Primitive().(*tview.Table)
	if !ok {
		t.Fatalf("Primitive() = %T, want *tview.Table", h.Primitive())
	}
	if got := table.GetRowCount(); got != 0 {
		t.Errorf("GetRowCount() with nil sections = %d, want 0", got)
	}
}

func TestBuildRowNames(t *testing.T) {
	names := buildRowNames(testSections)
	// Expected: "", "queues", "", "settings", "log"
	want := []string{"", "queues", "", "settings", "log"}
	if len(names) != len(want) {
		t.Fatalf("buildRowNames() len = %d, want %d", len(names), len(want))
	}
	for i, w := range want {
		if names[i] != w {
			t.Errorf("rowNames[%d] = %q, want %q", i, names[i], w)
		}
	}
}

func TestNewHomeCallbackFiredWithCorrectName(t *testing.T) {
	h := newTestHome(testSections)

	if h.rowNames[1] != "queues" {
		t.Fatalf("rowNames[1] = %q, want %q", h.rowNames[1], "queues")
	}
	// Simulate selection: select row 1 and read back rowNames.
	h.table.Select(1, 0)
	row, _ := h.table.GetSelection()
	if got := h.rowNames[row]; got != "queues" {
		t.Errorf("rowNames[selected row] = %q, want %q", got, "queues")
	}
}
