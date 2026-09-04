package view

import (
	"github.com/gdamore/tcell/v2"
	"github.com/rivo/tview"
)

// clearTableBody removes every row except the header (row 0) — the
// shared clear-loop behind repaint()/showError()/showStatus() (and,
// in CodePipelineDetailView, Open()/Render() too) across every
// table-backed AWS view in this package.
func clearTableBody(table *tview.Table) {
	for table.GetRowCount() > 1 {
		table.RemoveRow(table.GetRowCount() - 1)
	}
}

// showStatusCell renders a single-row, single-column status/error
// message into table — the shared shape behind SSMParamsView,
// SecretsView, LogsView, CodePipelineListView, and
// CodePipelineDetailView's showError/showStatus: null the view's own
// cached data (clearState), clear the table body, write one cell at
// (1, col) in color with text, then set the table's title.
func showStatusCell(table *tview.Table, col int, text string, color tcell.Color, title string, clearState func()) {
	clearState()
	clearTableBody(table)
	table.SetCell(1, col,
		tview.NewTableCell(text).
			SetTextColor(color).
			SetExpansion(3),
	)
	table.SetTitle(title)
}
