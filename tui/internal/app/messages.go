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

// messagesView shows the messages currently in a specific queue. It is not a
// registered ui.View (no switchTo / home dashboard entry); it is opened
// exclusively via App.openMessages and returns to "queues" on Esc/Backspace.
type messagesView struct {
	table     *tview.Table
	app       *App
	queueName string
	msgs      []queue.Message // sorted snapshot, index 0 = row 1
}

func (mv *messagesView) Shortcuts() []ui.Shortcut {
	return []ui.Shortcut{
		{Key: "r", Description: "refresh"},
		{Key: "Esc", Description: "back"},
	}
}

// newMessagesView constructs the messages view. The queue name is set later
// via app.openMessages before the view is shown.
func newMessagesView(a *App) *messagesView {
	table := tview.NewTable()
	table.SetBorder(true).SetTitle(" Messages ")
	table.SetSelectable(true, false)
	table.SetFixed(1, 0)

	mv := &messagesView{table: table, app: a}
	mv.setHeader()

	table.SetInputCapture(func(event *tcell.EventKey) *tcell.EventKey {
		switch {
		case event.Rune() == 'r':
			mv.load()
			return nil
		case event.Rune() == 'j':
			return tcell.NewEventKey(tcell.KeyDown, 0, tcell.ModNone)
		case event.Rune() == 'k':
			return tcell.NewEventKey(tcell.KeyUp, 0, tcell.ModNone)
		case event.Key() == tcell.KeyEscape, event.Key() == tcell.KeyBackspace, event.Key() == tcell.KeyBackspace2:
			a.switchTo("queues")
			return nil
		}
		return event
	})

	return mv
}

func (mv *messagesView) setHeader() {
	p := mv.app.cfg.Colors
	bg := tcell.GetColor(p.Label)
	fg := tcell.GetColor(p.Background)

	for i, label := range []string{"ID", "TYPE", "CORR.ID", "TIMESTAMP", "PREVIEW"} {
		mv.table.SetCell(0, i,
			tview.NewTableCell(label).
				SetTextColor(fg).
				SetBackgroundColor(bg).
				SetSelectable(false).
				SetExpansion(1).
				SetAlign(tview.AlignCenter))
	}
}

func (mv *messagesView) load() {
	queueName := mv.queueName
	go func() {
		msgs, err := mv.app.backend.BrowseMessages(context.Background(), queueName)
		mv.app.tv.QueueUpdateDraw(func() {
			if err != nil {
				slog.Error("messages: failed to browse messages", "queue", queueName, "error", err)
				mv.showError(err)
				return
			}
			mv.repaint(msgs)
		})
	}()
}

func (mv *messagesView) repaint(msgs []queue.Message) {
	// Sort descending by timestamp (newest first).
	sort.Slice(msgs, func(i, j int) bool {
		return msgs[i].Timestamp.After(msgs[j].Timestamp)
	})
	mv.msgs = msgs

	for mv.table.GetRowCount() > 1 {
		mv.table.RemoveRow(mv.table.GetRowCount() - 1)
	}

	p := mv.app.cfg.Colors
	idColor := tcell.GetColor(p.Value)
	tsColor := tcell.GetColor(p.Label)
	textColor := tcell.GetColor(p.Text)

	for i, m := range msgs {
		row := i + 1
		mv.table.SetCell(row, 0, tview.NewTableCell(m.ID).SetTextColor(idColor).SetExpansion(2))
		mv.table.SetCell(row, 1, tview.NewTableCell(m.JMSType).SetTextColor(tsColor).SetExpansion(1))
		mv.table.SetCell(row, 2, tview.NewTableCell(m.CorrelationID).SetTextColor(idColor).SetExpansion(2))
		mv.table.SetCell(row, 3, tview.NewTableCell(m.Timestamp.Local().Format("2006-01-02 15:04:05")).SetTextColor(tsColor).SetExpansion(1))
		mv.table.SetCell(row, 4, tview.NewTableCell(m.Preview).SetTextColor(textColor).SetExpansion(3))
	}
}

func (mv *messagesView) showError(err error) {
	for mv.table.GetRowCount() > 1 {
		mv.table.RemoveRow(mv.table.GetRowCount() - 1)
	}
	mv.table.SetCell(1, 0,
		tview.NewTableCell(fmt.Sprintf("Error: %v", err)).
			SetTextColor(tcell.ColorRed).
			SetExpansion(3),
	)
}
