package views

import (
	"fmt"

	"github.com/gdamore/tcell/v2"
	"github.com/rivo/tview"

	"github.com/ePex/cloudtui/tui/internal/ui"
)

// ViewInfo describes a single view entry on the home dashboard.
type ViewInfo struct {
	Name        string
	Description string
}

// HomeView is the app's landing screen: a table listing available views with
// their names and descriptions.
type HomeView struct {
	table *tview.Table
	infos []ViewInfo
}

var _ ui.View = (*HomeView)(nil)

func (h *HomeView) Name() string               { return "home" }
func (h *HomeView) Title() string              { return "Home" }
func (h *HomeView) Primitive() tview.Primitive { return h.table }

// NewHome constructs the home dashboard. labelColor and textColor are
// tview/tcell color strings from the active palette (e.g. "#e0af68").
func NewHome(infos []ViewInfo, labelColor, textColor string) *HomeView {
	table := tview.NewTable()
	table.SetBorder(true).SetTitle(" Home ")
	RepaintHomeTable(table, infos, labelColor, textColor)
	return &HomeView{table: table, infos: infos}
}

// RepaintHomeTable rewrites all cells of table from infos with the given label
// and text colors. Called on construction and again by reapplyTheme when the
// palette changes at runtime.
func RepaintHomeTable(table *tview.Table, infos []ViewInfo, labelColor, textColor string) {
	table.Clear()
	lc := tcell.GetColor(labelColor)
	tc := tcell.GetColor(textColor)
	for row, info := range infos {
		table.SetCell(row, 0,
			tview.NewTableCell(fmt.Sprintf("  %-12s", info.Name)).
				SetTextColor(lc).
				SetSelectable(false))
		table.SetCell(row, 1,
			tview.NewTableCell(info.Description).
				SetTextColor(tc).
				SetSelectable(false))
	}
}
