package view

import (
	"context"
	"fmt"
	"log/slog"
	"sort"
	"strings"

	"github.com/gdamore/tcell/v2"
	"github.com/rivo/tview"

	"github.com/ePex/cloudtui/tui/internal/config"
	"github.com/ePex/cloudtui/tui/internal/dialog"
	"github.com/ePex/cloudtui/tui/internal/queue"
	"github.com/ePex/cloudtui/tui/internal/ui"
)

// QueuesView is the Queues screen: a bordered tview.Table showing Name,
// Pending, Consumers, Enqueued, and Dequeued for each queue on the broker.
var queueColumns = []string{"NAME", "PENDING", "CONSUMERS", "ENQUEUED", "DEQUEUED"}

type QueuesView struct {
	table         *tview.Table
	filterInput   *tview.InputField
	flex          *tview.Flex
	host          ui.ViewHost
	backend       queue.Backend
	confirm       *dialog.ConfirmDialog
	movePicker    *dialog.MovePicker
	sendMessage   *dialog.SendMessageOverlay
	jmsTypePrompt *dialog.JMSTypePrompt
	filter        string          // active filter (empty = no filter)
	allSummaries  []queue.Summary // full unfiltered list from last load
	sortCol       int             // 0=NAME,1=PENDING,2=CONSUMERS,3=ENQUEUED,4=DEQUEUED
	sortAsc       bool            // true = ascending
}

var _ ui.View = (*QueuesView)(nil)
var _ ui.Shortcuttable = (*QueuesView)(nil)
var _ ui.Themeable = (*QueuesView)(nil)

// ApplyPalette recolors the queues view for a live theme switch.
func (qv *QueuesView) ApplyPalette(p config.Palette) {
	bg := tcell.GetColor(p.Background)
	qv.table.SetBackgroundColor(bg)
	qv.table.SetBorderColor(tcell.GetColor(p.ViewColor("queues")))
	qv.table.SetTitleColor(tcell.GetColor(p.ViewColor("queues")))
	qv.filterInput.SetLabelColor(tcell.GetColor(p.Label))
	qv.filterInput.SetFieldBackgroundColor(tcell.GetColor(p.SelectionBg))
	qv.filterInput.SetFieldTextColor(tcell.GetColor(p.SelectionText))
}

func (qv *QueuesView) Name() string               { return "queues" }
func (qv *QueuesView) Title() string              { return "Queues" }
func (qv *QueuesView) Primitive() tview.Primitive { return qv.flex }
func (qv *QueuesView) Table() *tview.Table        { return qv.table }
func (qv *QueuesView) FilterInputs() []tview.Primitive {
	return []tview.Primitive{qv.filterInput}
}

// SetBackend swaps the queue.Backend queries are made against — used by
// App.switchConnection when the active connection changes.
func (qv *QueuesView) SetBackend(b queue.Backend) { qv.backend = b }

func (qv *QueuesView) Shortcuts() []ui.Shortcut {
	return []ui.Shortcut{
		{Key: "r", Description: "refresh"},
		{Key: "p", Description: "purge"},
		{Key: "M", Description: "move queue"},
		{Key: "c", Description: "create message"},
		{Key: "/", Description: "filter"},
		{Key: "o/O", Description: "sort col/dir"},
	}
}

// NewQueuesView constructs the queues view backed by b.
func NewQueuesView(a ui.ViewHost, b queue.Backend, confirm *dialog.ConfirmDialog, movePicker *dialog.MovePicker, sendMessage *dialog.SendMessageOverlay, jmsTypePrompt *dialog.JMSTypePrompt, onSelect func(queueName string)) *QueuesView {
	table := tview.NewTable()
	table.SetBorder(true).SetTitle(" Queues ")
	table.SetSelectable(true, false)
	table.SetFixed(1, 0) // keep header row visible when scrolling

	p := a.Config().Colors
	filterInput := tview.NewInputField()
	filterInput.SetLabel(" / filter: ")
	filterInput.SetLabelColor(tcell.GetColor(p.Label))
	filterInput.SetFieldBackgroundColor(tcell.GetColor(p.SelectionBg))
	filterInput.SetFieldTextColor(tcell.GetColor(p.SelectionText))

	flex := tview.NewFlex().SetDirection(tview.FlexRow).
		AddItem(table, 0, 1, true).
		AddItem(filterInput, 1, 0, false)

	qv := &QueuesView{table: table, filterInput: filterInput, flex: flex, host: a, backend: b, confirm: confirm, movePicker: movePicker, sendMessage: sendMessage, jmsTypePrompt: jmsTypePrompt, sortAsc: true}
	qv.setHeader()

	filterInput.SetChangedFunc(func(text string) {
		qv.applyFilter(text)
	})
	filterInput.SetDoneFunc(func(_ tcell.Key) {
		qv.applyFilter(qv.filterInput.GetText())
		qv.host.SetFocus(qv.table)
	})
	filterInput.SetInputCapture(func(event *tcell.EventKey) *tcell.EventKey {
		if event.Key() == tcell.KeyUp || event.Key() == tcell.KeyDown {
			qv.applyFilter(qv.filterInput.GetText())
			qv.host.SetFocus(qv.table)
			qv.table.InputHandler()(event, func(tview.Primitive) {})
			return nil
		}
		return event
	})

	table.SetInputCapture(func(event *tcell.EventKey) *tcell.EventKey {
		switch event.Rune() {
		case 'r':
			qv.Load()
			return nil
		case 'j':
			return tcell.NewEventKey(tcell.KeyDown, 0, tcell.ModNone)
		case 'k':
			return tcell.NewEventKey(tcell.KeyUp, 0, tcell.ModNone)
		case '/':
			qv.filterInput.SetText(qv.filter)
			qv.host.SetFocus(qv.filterInput)
			return nil
		case 'o':
			qv.sortCol = (qv.sortCol + 1) % len(queueColumns)
			qv.repaint(qv.allSummaries)
			return nil
		case 'O':
			qv.sortAsc = !qv.sortAsc
			qv.repaint(qv.allSummaries)
			return nil
		case 'p':
			row, _ := qv.table.GetSelection()
			cell := qv.table.GetCell(row, 0)
			if cell == nil || cell.Text == "" || row == 0 {
				return nil
			}
			name := cell.Text
			qv.jmsTypePrompt.Show("Purge", name, func(jmsType string) {
				qv.confirmPurge(name, jmsType)
			}, qv.restoreShortcuts)
			return nil
		case 'M':
			row, _ := qv.table.GetSelection()
			cell := qv.table.GetCell(row, 0)
			if cell == nil || cell.Text == "" || row == 0 {
				return nil
			}
			srcQueue := cell.Text
			qv.jmsTypePrompt.Show("Move All", srcQueue, func(jmsType string) {
				qv.pickMoveAllTarget(srcQueue, jmsType)
			}, qv.restoreShortcuts)
			return nil
		case 'c':
			row, _ := qv.table.GetSelection()
			cell := qv.table.GetCell(row, 0)
			if cell == nil || cell.Text == "" || row == 0 {
				return nil
			}
			name := cell.Text
			qv.sendMessage.Show(name, qv.restoreShortcuts)
			return nil
		}
		return event
	})

	table.SetSelectedFunc(func(row, _ int) {
		cell := qv.table.GetCell(row, 0)
		if cell == nil || cell.Text == "" {
			return
		}
		onSelect(cell.Text)
	})

	return qv
}

// restoreShortcuts returns focus to the queue table and rewrites the
// context panel's shortcut hints — the common "an overlay opened from
// here just closed" cleanup shared by every overlay this view opens
// (send message, the JMS Type filter prompt, the move picker).
func (qv *QueuesView) restoreShortcuts() {
	qv.host.SetFocus(qv.table)
	lines := make([]string, 0, len(qv.Shortcuts()))
	for _, sc := range qv.Shortcuts() {
		lines = append(lines, fmt.Sprintf("[%s]<%s>[-] %s", qv.host.Config().Colors.Accent, sc.Key, sc.Description))
	}
	qv.host.SetContextHint(strings.Join(lines, "\n"))
}

// confirmPurge shows the purge confirmation dialog for name, wording it
// according to jmsType (empty = every message), and on confirmation
// performs the purge via doPurge.
func (qv *QueuesView) confirmPurge(name, jmsType string) {
	question := fmt.Sprintf("Purge %q? All messages will be deleted.", name)
	if jmsType != "" {
		question = fmt.Sprintf("Purge %q? All %s messages will be deleted.", name, jmsType)
	}
	qv.confirm.Show(question, func() {
		go func() {
			err := qv.doPurge(context.Background(), name, jmsType)
			qv.host.QueueUpdateDraw(func() {
				if err != nil {
					slog.Error("queues: purge failed", "queue", name, "jmsType", jmsType, "error", err)
					qv.showError(err)
					return
				}
				qv.Load()
			})
		}()
	})
}

// doPurge dispatches to the backend call matching jmsType: PurgeQueue
// (the existing fast, native path) when empty, or DeleteMessages with a
// JMSType filter otherwise — see spec/09's "Preserving the existing
// unfiltered path" for why these stay two different backend calls
// rather than DeleteMessages with an always-empty-filter fallback.
func (qv *QueuesView) doPurge(ctx context.Context, name, jmsType string) error {
	if jmsType == "" {
		return qv.backend.PurgeQueue(ctx, name)
	}
	_, err := qv.backend.DeleteMessages(ctx, name, queue.MessageFilter{JMSType: jmsType})
	return err
}

// pickMoveAllTarget shows the move-picker for srcQueue, and on target
// selection performs the move via doMoveAll.
func (qv *QueuesView) pickMoveAllTarget(srcQueue, jmsType string) {
	qv.movePicker.Show(srcQueue, func(target string) {
		count, err := qv.doMoveAll(context.Background(), srcQueue, target, jmsType)
		qv.host.QueueUpdateDraw(func() {
			if err != nil {
				slog.Error("queues: move all failed", "src", srcQueue, "dst", target, "jmsType", jmsType, "error", err)
				qv.showError(err)
				return
			}
			qv.host.SetStatus(fmt.Sprintf("Moved %d message(s) from %q to %q", count, srcQueue, target))
			qv.Load()
		})
	}, qv.restoreShortcuts)
}

// doMoveAll dispatches to the backend call matching jmsType:
// MoveAllMessages (the existing fast, native path) when empty, or
// MoveMessages with a JMSType filter otherwise — same reasoning as
// doPurge above.
func (qv *QueuesView) doMoveAll(ctx context.Context, srcQueue, target, jmsType string) (int, error) {
	if jmsType == "" {
		return qv.backend.MoveAllMessages(ctx, srcQueue, target)
	}
	return qv.backend.MoveMessages(ctx, srcQueue, target, queue.MessageFilter{JMSType: jmsType})
}

// Activate reloads the queue list. Called by SwitchTo each time the queues
// view becomes active.
func (qv *QueuesView) Activate() {
	qv.Load()
}

func (qv *QueuesView) setHeader() {
	p := qv.host.Config().Colors
	bg := tcell.GetColor(p.Label)
	fg := tcell.GetColor(p.Background)

	for i, col := range queueColumns {
		label := col
		if i == qv.sortCol {
			if qv.sortAsc {
				label += " ▲"
			} else {
				label += " ▼"
			}
		}
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
func (qv *QueuesView) Load() {
	go func() {
		summaries, err := qv.backend.List(context.Background())
		qv.host.QueueUpdateDraw(func() {
			if err != nil {
				slog.Error("queues: failed to list queues", "error", err)
				qv.showError(err)
				return
			}
			qv.repaint(summaries)
		})
	}()
}

func (qv *QueuesView) applyFilter(s string) {
	qv.filter = s
	qv.updateTitle()
	qv.repaint(qv.allSummaries)
}

// updateTitle sets the table's border title. The filtered case uses
// "(text)", not "[text]" — tview.Box titles run through the same
// tag-parsing Print() that Table cells do (see messages.go's markerCell
// for the same issue with "[x]"), so a filter string wrapped in square
// brackets is silently swallowed as an invalid color tag instead of
// displayed. Found live: GetTitle() still returns the literal "[text]"
// string (it's just the stored value), so this was invisible to any test
// that doesn't actually Draw() to a screen and read back the rendered
// output — see TestQueuesViewFilteredTitleActuallyRenders.
func (qv *QueuesView) updateTitle() {
	if qv.filter == "" {
		qv.table.SetTitle(" Queues ")
	} else {
		qv.table.SetTitle(fmt.Sprintf(" Queues (%s) ", qv.filter))
	}
}

func (qv *QueuesView) repaint(summaries []queue.Summary) {
	qv.allSummaries = summaries

	// Apply filter.
	filtered := summaries
	if qv.filter != "" {
		lower := strings.ToLower(qv.filter)
		filtered = make([]queue.Summary, 0, len(summaries))
		for _, s := range summaries {
			if strings.Contains(strings.ToLower(s.Name), lower) {
				filtered = append(filtered, s)
			}
		}
	}

	// Sort by active column and direction, with name as tiebreaker.
	sort.SliceStable(filtered, func(i, j int) bool {
		a, b := filtered[i], filtered[j]
		var less, equal bool
		switch qv.sortCol {
		case 1:
			less, equal = a.PendingCount < b.PendingCount, a.PendingCount == b.PendingCount
		case 2:
			less, equal = a.ConsumerCount < b.ConsumerCount, a.ConsumerCount == b.ConsumerCount
		case 3:
			less, equal = a.EnqueueCount < b.EnqueueCount, a.EnqueueCount == b.EnqueueCount
		case 4:
			less, equal = a.DequeueCount < b.DequeueCount, a.DequeueCount == b.DequeueCount
		default:
			less, equal = a.Name < b.Name, a.Name == b.Name
		}
		if equal {
			return a.Name < b.Name // stable tiebreaker
		}
		if qv.sortAsc {
			return less
		}
		return !less
	})

	qv.setHeader()

	// Clear data rows (keep header at row 0).
	for qv.table.GetRowCount() > 1 {
		qv.table.RemoveRow(qv.table.GetRowCount() - 1)
	}

	p := qv.host.Config().Colors
	nameColor := tcell.GetColor(p.Value)
	textColor := tcell.GetColor(p.Text)
	accentColor := tcell.GetColor(p.Accent)

	for i, s := range filtered {
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

	if qv.table.GetRowCount() > 1 {
		qv.table.Select(1, 0)
		// Select alone isn't enough: tview.Table's "track end" auto-scroll
		// (meant for tailing logs) latches on during the first draw of the
		// still-empty table (before this load completes) and stays latched
		// through this repaint, scrolling a long list to the bottom instead
		// of the top. SetOffset resets that internal flag.
		qv.table.SetOffset(0, 0)
	}
}

func (qv *QueuesView) showError(err error) {
	for qv.table.GetRowCount() > 1 {
		qv.table.RemoveRow(qv.table.GetRowCount() - 1)
	}
	qv.table.SetCell(1, 0,
		tview.NewTableCell(fmt.Sprintf("Error: %v", err)).
			SetTextColor(tcell.ColorRed).
			SetExpansion(5),
	)
}
