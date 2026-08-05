package app

import (
	"context"
	"fmt"
	"log/slog"
	"sort"

	"github.com/gdamore/tcell/v2"
	"github.com/rivo/tview"

	"github.com/ePex/cloudtui/tui/internal/queue"
	"github.com/ePex/cloudtui/tui/internal/ui"
)

// queuesView is the Queues screen: a bordered tview.Table showing Name,
// Pending, Consumers, Enqueued, and Dequeued for each queue on the broker.
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
	table.SetSelectable(true, false)
	table.SetFixed(1, 0) // keep header row visible when scrolling

	qv := &queuesView{table: table, app: a, backend: b}
	qv.setHeader()

	table.SetInputCapture(func(event *tcell.EventKey) *tcell.EventKey {
		switch event.Rune() {
		case 'r':
			qv.load()
			return nil
		case 'j':
			return tcell.NewEventKey(tcell.KeyDown, 0, tcell.ModNone)
		case 'k':
			return tcell.NewEventKey(tcell.KeyUp, 0, tcell.ModNone)
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
	p := qv.app.cfg.Colors
	bg := tcell.GetColor(p.Label)
	fg := tcell.GetColor(p.Background)

	headers := []string{"NAME ▲", "PENDING", "CONSUMERS", "ENQUEUED", "DEQUEUED"}
	for i, label := range headers {
		qv.table.SetCell(0, i,
			tview.NewTableCell(label).
				SetTextColor(fg).
				SetBackgroundColor(bg).
				SetSelectable(false).
				SetExpansion(1).
				SetAlign(tview.AlignCenter))
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
	// Sort ascending by name.
	sort.Slice(summaries, func(i, j int) bool {
		return summaries[i].Name < summaries[j].Name
	})

	// Clear data rows (keep header at row 0).
	for qv.table.GetRowCount() > 1 {
		qv.table.RemoveRow(qv.table.GetRowCount() - 1)
	}

	p := qv.app.cfg.Colors
	nameColor := tcell.GetColor(p.Value)
	textColor := tcell.GetColor(p.Text)
	accentColor := tcell.GetColor(p.Accent)

	for i, s := range summaries {
		row := i + 1

		pendingColor := textColor
		if s.PendingCount > 0 {
			pendingColor = accentColor
		}

		consumerColor := textColor
		if s.ConsumerCount == 0 {
			consumerColor = accentColor
		}

		qv.table.SetCell(row, 0, tview.NewTableCell(s.Name).SetTextColor(nameColor).SetExpansion(2))
		qv.table.SetCell(row, 1, tview.NewTableCell(fmt.Sprintf("%d", s.PendingCount)).SetTextColor(pendingColor).SetExpansion(1).SetAlign(tview.AlignRight))
		qv.table.SetCell(row, 2, tview.NewTableCell(fmt.Sprintf("%d", s.ConsumerCount)).SetTextColor(consumerColor).SetExpansion(1).SetAlign(tview.AlignRight))
		qv.table.SetCell(row, 3, tview.NewTableCell(fmt.Sprintf("%d", s.EnqueueCount)).SetTextColor(textColor).SetExpansion(1).SetAlign(tview.AlignRight))
		qv.table.SetCell(row, 4, tview.NewTableCell(fmt.Sprintf("%d", s.DequeueCount)).SetTextColor(textColor).SetExpansion(1).SetAlign(tview.AlignRight))
	}
}

func (qv *queuesView) showError(err error) {
	for qv.table.GetRowCount() > 1 {
		qv.table.RemoveRow(qv.table.GetRowCount() - 1)
	}
	qv.table.SetCell(1, 0,
		tview.NewTableCell(fmt.Sprintf("Error: %v", err)).
			SetTextColor(tcell.ColorRed).
			SetExpansion(5),
	)
}
