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

// queuesView is the Queues screen: a bordered tview.Table showing Name,
// Pending, Consumers, Enqueued, and Dequeued for each queue on the broker.
var queueColumns = []string{"NAME", "PENDING", "CONSUMERS", "ENQUEUED", "DEQUEUED"}

type queuesView struct {
	table        *tview.Table
	filterInput  *tview.InputField
	flex         *tview.Flex
	app          *App
	backend      queue.Backend
	filter       string          // active filter (empty = no filter)
	allSummaries []queue.Summary // full unfiltered list from last load
	sortCol      int             // 0=NAME,1=PENDING,2=CONSUMERS,3=ENQUEUED,4=DEQUEUED
	sortAsc      bool            // true = ascending
}

var _ ui.View = (*queuesView)(nil)
var _ ui.Shortcuttable = (*queuesView)(nil)

func (qv *queuesView) Name() string               { return "queues" }
func (qv *queuesView) Title() string              { return "Queues" }
func (qv *queuesView) Primitive() tview.Primitive { return qv.flex }

func (qv *queuesView) Shortcuts() []ui.Shortcut {
	return []ui.Shortcut{
		{Key: "r", Description: "refresh"},
		{Key: "p", Description: "purge"},
		{Key: "M", Description: "move queue"},
		{Key: "/", Description: "filter"},
		{Key: "o/O", Description: "sort col/dir"},
	}
}

// newQueuesView constructs the queues view backed by b.
func newQueuesView(a *App, b queue.Backend) *queuesView {
	table := tview.NewTable()
	table.SetBorder(true).SetTitle(" Queues ")
	table.SetSelectable(true, false)
	table.SetFixed(1, 0) // keep header row visible when scrolling

	p := a.cfg.Colors
	filterInput := tview.NewInputField()
	filterInput.SetLabel(" / filter: ")
	filterInput.SetLabelColor(tcell.GetColor(p.Label))
	filterInput.SetFieldBackgroundColor(tcell.GetColor(p.SelectionBg))
	filterInput.SetFieldTextColor(tcell.GetColor(p.SelectionText))

	flex := tview.NewFlex().SetDirection(tview.FlexRow).
		AddItem(table, 0, 1, true).
		AddItem(filterInput, 1, 0, false)

	qv := &queuesView{table: table, filterInput: filterInput, flex: flex, app: a, backend: b, sortAsc: true}
	qv.setHeader()

	filterInput.SetChangedFunc(func(text string) {
		qv.applyFilter(text)
	})
	filterInput.SetDoneFunc(func(_ tcell.Key) {
		qv.applyFilter(qv.filterInput.GetText())
		qv.app.tv.SetFocus(qv.table)
	})
	filterInput.SetInputCapture(func(event *tcell.EventKey) *tcell.EventKey {
		if event.Key() == tcell.KeyUp || event.Key() == tcell.KeyDown {
			qv.applyFilter(qv.filterInput.GetText())
			qv.app.tv.SetFocus(qv.table)
			qv.table.InputHandler()(event, func(tview.Primitive) {})
			return nil
		}
		return event
	})

	table.SetInputCapture(func(event *tcell.EventKey) *tcell.EventKey {
		switch event.Rune() {
		case 'r':
			qv.load()
			return nil
		case 'j':
			return tcell.NewEventKey(tcell.KeyDown, 0, tcell.ModNone)
		case 'k':
			return tcell.NewEventKey(tcell.KeyUp, 0, tcell.ModNone)
		case '/':
			qv.filterInput.SetText(qv.filter)
			qv.app.tv.SetFocus(qv.filterInput)
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
			qv.app.showConfirm(fmt.Sprintf("Purge %q? All messages will be deleted.", name), func() {
				go func() {
					err := qv.backend.PurgeQueue(context.Background(), name)
					qv.app.tv.QueueUpdateDraw(func() {
						if err != nil {
							slog.Error("queues: purge failed", "queue", name, "error", err)
							qv.showError(err)
							return
						}
						qv.load()
					})
				}()
			})
			return nil
		case 'M':
			row, _ := qv.table.GetSelection()
			cell := qv.table.GetCell(row, 0)
			if cell == nil || cell.Text == "" || row == 0 {
				return nil
			}
			srcQueue := cell.Text
			restoreQueues := func() {
				qv.app.tv.SetFocus(qv.table)
				lines := make([]string, 0, len(qv.Shortcuts()))
				for _, sc := range qv.Shortcuts() {
					lines = append(lines, fmt.Sprintf("[%s]<%s>[-] %s", qv.app.cfg.Colors.Accent, sc.Key, sc.Description))
				}
				qv.app.contextPanel.SetText(strings.Join(lines, "\n"))
			}
			qv.app.showMovePicker(srcQueue, func(target string) {
				count, err := qv.backend.MoveAllMessages(context.Background(), srcQueue, target)
				qv.app.tv.QueueUpdateDraw(func() {
					if err != nil {
						slog.Error("queues: move all failed", "src", srcQueue, "dst", target, "error", err)
						qv.showError(err)
						return
					}
					qv.app.statusBar.SetText(fmt.Sprintf("Moved %d message(s) from %q to %q", count, srcQueue, target))
					qv.load()
				})
			}, restoreQueues)
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
	p := qv.app.cfg.Colors
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

func (qv *queuesView) applyFilter(s string) {
	qv.filter = s
	qv.updateTitle()
	qv.repaint(qv.allSummaries)
}

func (qv *queuesView) updateTitle() {
	if qv.filter == "" {
		qv.table.SetTitle(" Queues ")
	} else {
		qv.table.SetTitle(fmt.Sprintf(" Queues [%s] ", qv.filter))
	}
}

func (qv *queuesView) repaint(summaries []queue.Summary) {
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

	p := qv.app.cfg.Colors
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
