package views

import (
	"fmt"
	"strings"

	"github.com/gdamore/tcell/v2"
	"github.com/rivo/tview"

	"github.com/ePex/cloudtui/tui/internal/ui"
)

// ViewInfo describes a single view entry on the home dashboard.
type ViewInfo struct {
	Name        string
	Description string
}

// SectionInfo groups related view entries under a named section.
type SectionInfo struct {
	Title   string
	Entries []ViewInfo
}

// HomeView is the app's landing screen: a sectioned, keyboard-navigatable
// table that lets the user select and activate any registered view.
type HomeView struct {
	table    *tview.Table
	sections []SectionInfo
	rowNames []string // row index → view name; "" for section header rows
}

var _ ui.View = (*HomeView)(nil)

func (h *HomeView) Name() string               { return "home" }
func (h *HomeView) Title() string              { return "Home" }
func (h *HomeView) Primitive() tview.Primitive { return h.table }

// NewHome constructs the home dashboard. onSelect is called with the view name
// when the user presses Enter on an entry. Color parameters are tview/tcell
// color strings from the active palette.
func NewHome(sections []SectionInfo, onSelect func(name string), labelColor, textColor, headerColor, selectionBg, selectionText string) *HomeView {
	table := tview.NewTable()
	table.SetBorder(true).SetTitle(" Home ")
	table.SetSelectable(true, false)

	h := &HomeView{table: table, sections: sections}
	RepaintHomeTable(table, sections, labelColor, textColor, headerColor, selectionBg, selectionText)
	h.rowNames = buildRowNames(sections)

	// j/k forwarded to ↓/↑ so tview's built-in cursor logic (including
	// skipping non-selectable header rows) applies unchanged.
	table.SetInputCapture(func(event *tcell.EventKey) *tcell.EventKey {
		switch event.Rune() {
		case 'j':
			return tcell.NewEventKey(tcell.KeyDown, 0, tcell.ModNone)
		case 'k':
			return tcell.NewEventKey(tcell.KeyUp, 0, tcell.ModNone)
		}
		return event
	})

	table.SetSelectedFunc(func(row, _ int) {
		if row < len(h.rowNames) && h.rowNames[row] != "" {
			onSelect(h.rowNames[row])
		}
	})

	return h
}

// RepaintHomeTable rewrites all cells of table from sections with the given
// colors and reapplies the selection style. Called on construction and again
// by reapplyTheme when the palette changes at runtime.
func RepaintHomeTable(table *tview.Table, sections []SectionInfo, labelColor, textColor, headerColor, selectionBg, selectionText string) {
	table.Clear()
	table.SetSelectedStyle(
		tcell.StyleDefault.
			Background(tcell.GetColor(selectionBg)).
			Foreground(tcell.GetColor(selectionText)),
	)

	hc := tcell.GetColor(headerColor)

	row := 0
	for _, sec := range sections {
		// Section header: single cell, non-selectable. A very long dash string
		// is clipped by tview to the exact cell width, so the line always fills
		// to the right edge of the table regardless of terminal width.
		header := "─── " + sec.Title + " " + strings.Repeat("─", 200)
		table.SetCell(row, 0,
			tview.NewTableCell(header).
				SetTextColor(hc).
				SetSelectable(false).
				SetExpansion(1))
		row++

		for _, entry := range sec.Entries {
			// Single cell with dynamic color tags: name in label color,
			// description in text color. SetSelectedStyle overrides both
			// uniformly when the row is selected.
			text := fmt.Sprintf("[%s]  %-18s[-]  [%s]%s[-]", labelColor, entry.Name, textColor, entry.Description)
			table.SetCell(row, 0,
				tview.NewTableCell(text).
					SetExpansion(1))
			row++
		}
	}
}

// buildRowNames returns a slice mapping each row index to a view name.
// Section header rows map to "".
func buildRowNames(sections []SectionInfo) []string {
	var names []string
	for _, sec := range sections {
		names = append(names, "") // header row
		for _, entry := range sec.Entries {
			names = append(names, entry.Name)
		}
	}
	return names
}
