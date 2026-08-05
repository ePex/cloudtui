package app

import (
	"context"
	"fmt"
	"log/slog"

	"github.com/gdamore/tcell/v2"
	"github.com/rivo/tview"

	"github.com/ePex/cloudtui/tui/internal/queue"
	"github.com/ePex/cloudtui/tui/internal/ui"
)

// queuesView is the Queues screen: a bordered tview.Table showing Name,
// Pending messages, and Consumer count for each queue on the broker.
type queuesView struct {
	table   *tview.Table
	app     *App
	backend queue.Backend
}

var _ ui.View = (*queuesView)(nil)
var _ ui.Shortcuttable = (*queuesView)(nil)

func (qv *queuesView) Name() string               { return "queues" }
func (qv *queuesView) Title() string              { return "Queues" }
func (qv *queuesView) Primitive() tview.Primitive { return qv.table }

func (qv *queuesView) Shortcuts() []ui.Shortcut {
	return []ui.Shortcut{
		{Key: "r", Description: "refresh"},
	}
}

// newQueuesView constructs the queues view backed by b.
func newQueuesView(a *App, b queue.Backend) *queuesView {
	table := tview.NewTable()
	table.SetBorder(true).SetTitle(" Queues ")
	table.SetSelectable(false, false)

	qv := &queuesView{table: table, app: a, backend: b}
	qv.setHeader()

	table.SetInputCapture(func(event *tcell.EventKey) *tcell.EventKey {
		if event.Rune() == 'r' {
			qv.load()
			return nil
		}
		return event
	})

	return qv
}

// Activate reloads the queue list. Called by switchTo each time the queues
// view becomes active.
func (qv *queuesView) Activate() {
	qv.load()
}

func (qv *queuesView) setHeader() {
	cols := []string{"Name", "Pending", "Consumers"}
	p := qv.app.cfg.Colors
	for i, label := range cols {
		cell := tview.NewTableCell(label).
			SetTextColor(tcell.GetColor(p.Label)).
			SetSelectable(false).
			SetExpansion(1)
		qv.table.SetCell(0, i, cell)
	}
}

// load fetches queues from the backend in a goroutine and repaints via
// QueueUpdateDraw so the update lands on the tview event loop.
func (qv *queuesView) load() {
	go func() {
		summaries, err := qv.backend.List(context.Background())
		qv.app.tv.QueueUpdateDraw(func() {
			if err != nil {
				slog.Error("queues: failed to list queues", "error", err)
				qv.showError(err)
				return
			}
			qv.repaint(summaries)
		})
	}()
}

func (qv *queuesView) repaint(summaries []queue.Summary) {
	// Clear all rows except the header.
	for qv.table.GetRowCount() > 1 {
		qv.table.RemoveRow(qv.table.GetRowCount() - 1)
	}

	p := qv.app.cfg.Colors
	textColor := tcell.GetColor(p.Text)

	for i, s := range summaries {
		row := i + 1
		qv.table.SetCell(row, 0, tview.NewTableCell(s.Name).SetTextColor(textColor).SetExpansion(1))
		qv.table.SetCell(row, 1, tview.NewTableCell(fmt.Sprintf("%d", s.PendingCount)).SetTextColor(textColor).SetExpansion(1))
		qv.table.SetCell(row, 2, tview.NewTableCell(fmt.Sprintf("%d", s.ConsumerCount)).SetTextColor(textColor).SetExpansion(1))
	}
}

func (qv *queuesView) showError(err error) {
	// Clear data rows and show error in first data cell.
	for qv.table.GetRowCount() > 1 {
		qv.table.RemoveRow(qv.table.GetRowCount() - 1)
	}
	qv.table.SetCell(1, 0,
		tview.NewTableCell(fmt.Sprintf("Error: %v", err)).
			SetTextColor(tcell.ColorRed).
			SetExpansion(3),
	)
}
