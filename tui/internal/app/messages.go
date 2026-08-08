package app

import (
	"context"
	"fmt"
	"log/slog"
	"sort"
	"strings"

	"github.com/gdamore/tcell/v2"
	"github.com/rivo/tview"

	"github.com/ePex/cloudtui/tui/internal/queue"
	"github.com/ePex/cloudtui/tui/internal/ui"
)

// messagesView shows the messages currently in a specific queue. It is not a
// registered ui.View (no switchTo / home dashboard entry); it is opened
// exclusively via App.openMessages and returns to "queues" on Esc/Backspace.
//
// Messages can be multi-selected ("marked", tracked in marked by message ID)
// independently of the table's cursor: space toggles the mark on the row
// under the cursor, 'a' marks all, 'n' clears all marks, and 'd'/'m' act on
// the marked set (delete / move). If nothing is marked, 'd'/'m' fall back to
// the single message under the cursor, so they also work as a single-item
// shortcut without requiring an explicit mark first. Marks are cleared on
// every reload (load), since a refreshed list may reorder or drop messages.
type messagesView struct {
	table     *tview.Table
	app       *App
	queueName string
	msgs      []queue.Message // sorted snapshot, index 0 = row 1
	marked    map[string]bool // message IDs currently marked
}

func (mv *messagesView) Shortcuts() []ui.Shortcut {
	return []ui.Shortcut{
		{Key: "space", Description: "mark"},
		{Key: "a", Description: "mark all"},
		{Key: "n", Description: "clear marks"},
		{Key: "d", Description: "delete marked/current"},
		{Key: "m", Description: "move marked/current"},
		{Key: "r", Description: "refresh"},
		{Key: "p", Description: "purge"},
		{Key: "c", Description: "create message"},
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
		case event.Rune() == ' ':
			mv.toggleMark()
			return nil
		case event.Rune() == 'a':
			mv.markAll()
			return nil
		case event.Rune() == 'n':
			mv.clearMarks()
			return nil
		case event.Rune() == 'd':
			mv.deleteMarked()
			return nil
		case event.Rune() == 'm':
			mv.moveMarked()
			return nil
		case event.Rune() == 'r':
			mv.load()
			return nil
		case event.Rune() == 'c':
			name := mv.queueName
			a.showSendMessage(name, func() {
				a.tv.SetFocus(mv.table)
				lines := make([]string, 0, len(mv.Shortcuts()))
				for _, sc := range mv.Shortcuts() {
					lines = append(lines, fmt.Sprintf("[%s]<%s>[-] %s", a.cfg.Colors.Accent, sc.Key, sc.Description))
				}
				a.contextPanel.SetText(strings.Join(lines, "\n"))
			})
			return nil
		case event.Rune() == 'p':
			name := mv.queueName
			a.showConfirm(fmt.Sprintf("Purge %q? All messages will be deleted.", name), func() {
				go func() {
					err := a.backend.PurgeQueue(context.Background(), name)
					a.tv.QueueUpdateDraw(func() {
						if err != nil {
							slog.Error("messages: purge failed", "queue", name, "error", err)
							mv.showError(err)
							return
						}
						mv.load()
					})
				}()
			})
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

	for i, label := range []string{"", "ID", "TYPE", "CORR.ID", "TIMESTAMP", "PREVIEW"} {
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
			if len(msgs) > 0 && msgs[0].ID == "" {
				mv.app.statusBar.SetText("[yellow]Note: limited message info — individual move/delete unavailable[-]")
			}
		})
	}()
}

func (mv *messagesView) repaint(msgs []queue.Message) {
	// Sort descending by timestamp (newest first).
	sort.Slice(msgs, func(i, j int) bool {
		return msgs[i].Timestamp.After(msgs[j].Timestamp)
	})
	mv.msgs = msgs
	mv.marked = map[string]bool{} // a reload may reorder/drop messages; marks don't survive it

	for mv.table.GetRowCount() > 1 {
		mv.table.RemoveRow(mv.table.GetRowCount() - 1)
	}

	p := mv.app.cfg.Colors
	idColor := tcell.GetColor(p.Value)
	tsColor := tcell.GetColor(p.Label)
	textColor := tcell.GetColor(p.Text)

	for i, m := range msgs {
		row := i + 1
		mv.table.SetCell(row, 0, mv.markerCell(false))
		mv.table.SetCell(row, 1, tview.NewTableCell(m.ID).SetTextColor(idColor).SetExpansion(2))
		mv.table.SetCell(row, 2, tview.NewTableCell(m.JMSType).SetTextColor(tsColor).SetExpansion(1))
		mv.table.SetCell(row, 3, tview.NewTableCell(m.CorrelationID).SetTextColor(idColor).SetExpansion(2))
		mv.table.SetCell(row, 4, tview.NewTableCell(m.Timestamp.Local().Format("2006-01-02 15:04:05")).SetTextColor(tsColor).SetExpansion(1))
		mv.table.SetCell(row, 5, tview.NewTableCell(m.Preview).SetTextColor(textColor).SetExpansion(3))
	}

	if mv.table.GetRowCount() > 1 {
		mv.table.Select(1, 0)
		// Select alone isn't enough — see queues.go's repaint for why
		// SetOffset is also needed (tview.Table's "track end" auto-scroll
		// latches on during the table's first, still-empty draw).
		mv.table.SetOffset(0, 0)
	}
}

// markerCell builds the checkbox cell shown in the marker column. Plain "[x]"
// text doesn't work here: tview.Table always interprets "[...]" in cell text
// as a color/region tag, so it gets silently swallowed instead of displayed.
func (mv *messagesView) markerCell(marked bool) *tview.TableCell {
	p := mv.app.cfg.Colors
	text, color := " ", tcell.GetColor(p.Text)
	if marked {
		text, color = "✓", tcell.GetColor(p.Accent)
	}
	return tview.NewTableCell(text).SetTextColor(color).SetAlign(tview.AlignCenter)
}

// refreshMarkerColumn redraws column 0 to reflect the current marked set,
// without re-fetching or resorting messages.
func (mv *messagesView) refreshMarkerColumn() {
	for i, m := range mv.msgs {
		mv.table.SetCell(i+1, 0, mv.markerCell(mv.marked[m.ID]))
	}
}

// markedIDs returns the IDs of currently marked messages, in the table's
// current display order.
func (mv *messagesView) markedIDs() []string {
	ids := make([]string, 0, len(mv.marked))
	for _, m := range mv.msgs {
		if mv.marked[m.ID] {
			ids = append(ids, m.ID)
		}
	}
	return ids
}

// targetIDs returns the marked message IDs, or — if nothing is marked — the
// single message under the cursor (if it has an ID), so 'd'/'m' also work as
// a single-item shortcut without requiring an explicit mark first.
func (mv *messagesView) targetIDs() []string {
	if ids := mv.markedIDs(); len(ids) > 0 {
		return ids
	}
	row, _ := mv.table.GetSelection()
	idx := row - 1
	if idx < 0 || idx >= len(mv.msgs) || mv.msgs[idx].ID == "" {
		return nil
	}
	return []string{mv.msgs[idx].ID}
}

// toggleMark flips the mark on the row under the cursor and advances the
// cursor, so repeated space presses mark a run of messages quickly. Messages
// without an ID (limited-info mode) can't be marked, matching the existing
// restriction on individual move/delete.
func (mv *messagesView) toggleMark() {
	row, _ := mv.table.GetSelection()
	idx := row - 1
	if idx < 0 || idx >= len(mv.msgs) {
		return
	}
	m := mv.msgs[idx]
	if m.ID == "" {
		mv.app.statusBar.SetText("[yellow]Cannot mark: message ID unavailable[-]")
		return
	}
	if mv.marked == nil {
		mv.marked = map[string]bool{}
	}
	if mv.marked[m.ID] {
		delete(mv.marked, m.ID)
	} else {
		mv.marked[m.ID] = true
	}
	mv.refreshMarkerColumn()
	if row < mv.table.GetRowCount()-1 {
		mv.table.Select(row+1, 0)
	}
}

// markAll marks every message that has an ID.
func (mv *messagesView) markAll() {
	if mv.marked == nil {
		mv.marked = map[string]bool{}
	}
	skipped := 0
	for _, m := range mv.msgs {
		if m.ID == "" {
			skipped++
			continue
		}
		mv.marked[m.ID] = true
	}
	mv.refreshMarkerColumn()
	if skipped > 0 {
		mv.app.statusBar.SetText(fmt.Sprintf("Marked %d message(s); %d skipped (no ID)", len(mv.marked), skipped))
	} else {
		mv.app.statusBar.SetText(fmt.Sprintf("Marked %d message(s)", len(mv.marked)))
	}
}

// clearMarks deselects every marked message.
func (mv *messagesView) clearMarks() {
	if len(mv.marked) == 0 {
		return
	}
	mv.marked = map[string]bool{}
	mv.refreshMarkerColumn()
	mv.app.statusBar.SetText("Cleared marks")
}

// deleteMarked confirms and deletes every marked message, or — if nothing is
// marked — the single message under the cursor. Each deletion is
// independent, so one failure doesn't stop the rest; the status bar reports
// how many of the batch actually succeeded.
func (mv *messagesView) deleteMarked() {
	ids := mv.targetIDs()
	if len(ids) == 0 {
		mv.app.statusBar.SetText("[yellow]No message marked or selected[-]")
		return
	}
	a := mv.app
	queueName := mv.queueName
	question := fmt.Sprintf("Delete message from %q?", queueName)
	if len(mv.markedIDs()) > 0 {
		question = fmt.Sprintf("Delete %d marked message(s) from %q?", len(ids), queueName)
	}
	a.showConfirm(question, func() {
		go func() {
			failed := 0
			for _, id := range ids {
				if err := a.backend.RemoveMessage(context.Background(), queueName, id); err != nil {
					slog.Error("messages: bulk delete failed", "queue", queueName, "id", id, "error", err)
					failed++
				}
			}
			a.tv.QueueUpdateDraw(func() {
				switch {
				case failed > 0:
					a.statusBar.SetText(fmt.Sprintf("[red]Deleted %d/%d message(s); %d failed[-]", len(ids)-failed, len(ids), failed))
				case len(ids) == 1:
					a.statusBar.SetText("Deleted message")
				default:
					a.statusBar.SetText(fmt.Sprintf("Deleted %d message(s)", len(ids)))
				}
				mv.load()
			})
		}()
	})
}

// moveMarked opens the move picker once and, on target selection, moves
// every marked message there, or — if nothing is marked — the single
// message under the cursor. As with deleteMarked, one failure doesn't stop
// the rest.
func (mv *messagesView) moveMarked() {
	ids := mv.targetIDs()
	if len(ids) == 0 {
		mv.app.statusBar.SetText("[yellow]No message marked or selected[-]")
		return
	}
	a := mv.app
	srcQueue := mv.queueName
	restore := func() {
		a.tv.SetFocus(mv.table)
		lines := make([]string, 0, len(mv.Shortcuts()))
		for _, sc := range mv.Shortcuts() {
			lines = append(lines, fmt.Sprintf("[%s]<%s>[-] %s", a.cfg.Colors.Accent, sc.Key, sc.Description))
		}
		a.contextPanel.SetText(strings.Join(lines, "\n"))
	}
	a.showMovePicker(srcQueue, func(target string) {
		go func() {
			failed := 0
			for _, id := range ids {
				if err := a.backend.MoveMessage(context.Background(), srcQueue, id, target); err != nil {
					slog.Error("messages: bulk move failed", "src", srcQueue, "dst", target, "id", id, "error", err)
					failed++
				}
			}
			a.tv.QueueUpdateDraw(func() {
				switch {
				case failed > 0:
					a.statusBar.SetText(fmt.Sprintf("[red]Moved %d/%d message(s) to %q; %d failed[-]", len(ids)-failed, len(ids), target, failed))
				case len(ids) == 1:
					a.statusBar.SetText(fmt.Sprintf("Moved message to %q", target))
				default:
					a.statusBar.SetText(fmt.Sprintf("Moved %d message(s) to %q", len(ids), target))
				}
				restore()
				mv.load()
			})
		}()
	}, restore)
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
