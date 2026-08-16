package app

import (
	"context"
	"fmt"
	"log/slog"
	"sort"
	"strconv"
	"strings"
	"time"

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
//
// Two independent, composable filter mechanisms narrow what's shown: the
// server-side filter ('f', a queue.MessageFilter applied by the backend —
// see load) determines what's fetched; quick search ('/', a live,
// client-side substring match over the already-fetched allMsgs — see
// applyQuickSearch/repaint) determines what's displayed from that. Mark/
// target logic only ever sees msgs, the currently-displayed (search-
// filtered) set.
type messagesView struct {
	table       *tview.Table
	searchInput *tview.InputField
	flex        *tview.Flex
	app         *App
	queueName   string
	allMsgs     []queue.Message     // full set from the last load, pre-quick-search
	quickSearch string              // active quick-search text (client-side)
	filter      queue.MessageFilter // active server-side filter
	msgs        []queue.Message     // sorted, quick-search-filtered snapshot, index 0 = row 1
	marked      map[string]bool     // message IDs currently marked
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
		{Key: "/", Description: "quick search"},
		{Key: "f", Description: "filter"},
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

	p := a.cfg.Colors
	searchInput := tview.NewInputField()
	searchInput.SetLabel(" / search: ")
	searchInput.SetLabelColor(tcell.GetColor(p.Label))
	searchInput.SetFieldBackgroundColor(tcell.GetColor(p.SelectionBg))
	searchInput.SetFieldTextColor(tcell.GetColor(p.SelectionText))

	flex := tview.NewFlex().SetDirection(tview.FlexRow).
		AddItem(table, 0, 1, true).
		AddItem(searchInput, 1, 0, false)

	mv := &messagesView{table: table, searchInput: searchInput, flex: flex, app: a}
	mv.setHeader()

	searchInput.SetChangedFunc(func(text string) {
		mv.applyQuickSearch(text)
	})
	searchInput.SetDoneFunc(func(_ tcell.Key) {
		mv.applyQuickSearch(mv.searchInput.GetText())
		mv.app.tv.SetFocus(mv.table)
	})
	searchInput.SetInputCapture(func(event *tcell.EventKey) *tcell.EventKey {
		if event.Key() == tcell.KeyUp || event.Key() == tcell.KeyDown {
			mv.applyQuickSearch(mv.searchInput.GetText())
			mv.app.tv.SetFocus(mv.table)
			mv.table.InputHandler()(event, func(tview.Primitive) {})
			return nil
		}
		return event
	})

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
		case event.Rune() == '/':
			mv.searchInput.SetText(mv.quickSearch)
			mv.app.tv.SetFocus(mv.searchInput)
			return nil
		case event.Rune() == 'f':
			a.messageFilter.show()
			return nil
		case event.Rune() == 'c':
			name := mv.queueName
			a.sendMessage.show(name, func() {
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
			a.confirm.show(fmt.Sprintf("Purge %q? All messages will be deleted.", name), func() {
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

// defaultBrowseMaxCount caps how many messages a browse fetches when the
// user hasn't set an explicit Max Count via the filter form — mq-proxy
// requires list-messages' filter.maxCount to be set (spec
// 51-cr-mq-proxy-require-list-messages-maxcount), and even for the Jolokia
// backend (which has no request-time cap) this bounds what gets rendered.
const defaultBrowseMaxCount = 500

// withDefaultMaxCount returns f unchanged if it already has a positive
// MaxCount, otherwise a copy with MaxCount set to defaultBrowseMaxCount.
// mv.filter itself is never mutated by this — see load/updateTitle.
func withDefaultMaxCount(f queue.MessageFilter) queue.MessageFilter {
	if f.MaxCount <= 0 {
		f.MaxCount = defaultBrowseMaxCount
	}
	return f
}

func (mv *messagesView) load() {
	queueName := mv.queueName
	filter := withDefaultMaxCount(mv.filter)
	go func() {
		msgs, err := mv.app.backend.BrowseMessages(context.Background(), queueName, filter)
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

// applyQuickSearch sets the active quick-search text and re-filters the
// currently-loaded messages against it, without re-fetching from the
// backend — unlike the server-side filter form, this is purely client-side
// and safe to call on every keystroke.
func (mv *messagesView) applyQuickSearch(s string) {
	mv.quickSearch = s
	mv.updateTitle()
	mv.repaint(mv.allMsgs)
}

// updateTitle sets the table's border title, reflecting the queue name plus
// whichever of the two filter mechanisms are active. Parens/brackets wrap
// active segments only — see queues.go's updateTitle for why "(text)" is
// used instead of "[text]" (square brackets are swallowed as color tags).
func (mv *messagesView) updateTitle() {
	title := fmt.Sprintf(" Messages — %s ", mv.queueName)
	if desc := describeMessageFilter(withDefaultMaxCount(mv.filter)); desc != "" {
		title = fmt.Sprintf(" Messages — %s (filter: %s) ", mv.queueName, desc)
	}
	if mv.quickSearch != "" {
		title = strings.TrimRight(title, " ") + fmt.Sprintf(" [search: %s] ", mv.quickSearch)
	}
	mv.table.SetTitle(title)
}

// describeMessageFilter renders f as a short human-readable summary for the
// table title, e.g. "type=order-created max=100" — empty for a zero-value
// filter (nothing active).
func describeMessageFilter(f queue.MessageFilter) string {
	var parts []string
	if f.JMSType != "" {
		parts = append(parts, "type="+f.JMSType)
	}
	if !f.FromDate.IsZero() {
		parts = append(parts, "from="+f.FromDate.Format("2006-01-02"))
	}
	if !f.ToDate.IsZero() {
		parts = append(parts, "to="+f.ToDate.Format("2006-01-02"))
	}
	if f.MaxCount > 0 {
		parts = append(parts, fmt.Sprintf("max=%d", f.MaxCount))
	}
	return strings.Join(parts, " ")
}

// filterDateLayout is the bare "YYYY-MM-DD" (date-only, no time-of-day)
// format parseFilterDate accepts in addition to RFC3339 — kept as its own
// named constant (rather than folded into the local-time branch below)
// because it's interpreted as UTC midnight, not local time, matching the
// message filter's original, already-shipped convention (proxy backend's
// toFilterDTO normalizes filter dates to UTC the same way).
const filterDateLayout = "2006-01-02"

// filterDateTimeLayout is the bare "YYYY-MM-DD HH:MM" (minute precision)
// format parseFilterDate accepts in addition to RFC3339 and
// filterDateLayout — interpreted in the *local* timezone, not UTC (see
// parseFilterDate's doc comment for why). Added for the time range
// modal's Absolute tab (spec/53-fe-log-time-range-modal decision 4).
const filterDateTimeLayout = "2006-01-02 15:04"

// parseFilterDate parses s as RFC3339 (explicit zone honored as written),
// filterDateTimeLayout (local timezone), or filterDateLayout (UTC
// midnight) — in that order. The local-vs-UTC split between the two bare
// layouts is deliberate, not an oversight: the results table already
// displays event timestamps via .Local() (see logsearch.go/datadoglogs.go's
// repaint), so parsing a typed time as UTC would silently filter a
// different window than what the table then displays, offset by the local
// UTC offset — reported live as "I filtered 15:00-15:30 and got a message
// timestamped 17:29". filterDateLayout (date-only, no time-of-day) is left
// UTC-midnight, unchanged from before this fix — it's the already-shipped
// message filter's convention, and changing it wasn't asked for.
//
// An empty (post-trim) s returns the zero time with no error — "unset".
// label is used only to name the field in the returned error. Shared by
// parseMessageFilterForm and the time range modal's Absolute tab.
func parseFilterDate(label, s string) (time.Time, error) {
	s = strings.TrimSpace(s)
	if s == "" {
		return time.Time{}, nil
	}
	if t, err := time.Parse(time.RFC3339, s); err == nil {
		return t.UTC(), nil
	}
	if t, err := time.ParseInLocation(filterDateTimeLayout, s, time.Local); err == nil {
		return t.UTC(), nil
	}
	if t, err := time.Parse(filterDateLayout, s); err == nil {
		return t.UTC(), nil
	}
	return time.Time{}, fmt.Errorf("invalid %s %q: want RFC3339, YYYY-MM-DD HH:MM, or YYYY-MM-DD", label, s)
}

// parseMessageFilterForm parses the message filter form's four field values
// into a queue.MessageFilter. from/to accept any format parseFilterDate
// does. maxCount must be a non-negative integer; empty fields
// are left unset (zero value).
func parseMessageFilterForm(jmsType, from, to, maxCount string) (queue.MessageFilter, error) {
	f := queue.MessageFilter{JMSType: strings.TrimSpace(jmsType)}

	var err error
	if f.FromDate, err = parseFilterDate("from", from); err != nil {
		return queue.MessageFilter{}, err
	}
	if f.ToDate, err = parseFilterDate("to", to); err != nil {
		return queue.MessageFilter{}, err
	}

	maxCount = strings.TrimSpace(maxCount)
	if maxCount != "" {
		n, err := strconv.Atoi(maxCount)
		if err != nil || n < 0 {
			return queue.MessageFilter{}, fmt.Errorf("invalid max count %q: want a non-negative integer", maxCount)
		}
		f.MaxCount = n
	}

	return f, nil
}

func (mv *messagesView) repaint(msgs []queue.Message) {
	mv.allMsgs = msgs

	// Apply quick search.
	filtered := msgs
	if mv.quickSearch != "" {
		lower := strings.ToLower(mv.quickSearch)
		filtered = make([]queue.Message, 0, len(msgs))
		for _, m := range msgs {
			if strings.Contains(strings.ToLower(m.JMSType), lower) || strings.Contains(strings.ToLower(m.Preview), lower) {
				filtered = append(filtered, m)
			}
		}
	}

	// Sort descending by timestamp (newest first).
	sort.Slice(filtered, func(i, j int) bool {
		return filtered[i].Timestamp.After(filtered[j].Timestamp)
	})
	mv.msgs = filtered
	mv.marked = map[string]bool{} // a reload may reorder/drop messages; marks don't survive it

	for mv.table.GetRowCount() > 1 {
		mv.table.RemoveRow(mv.table.GetRowCount() - 1)
	}

	p := mv.app.cfg.Colors
	idColor := tcell.GetColor(p.Value)
	tsColor := tcell.GetColor(p.Label)
	textColor := tcell.GetColor(p.Text)

	for i, m := range filtered {
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
	a.confirm.show(question, func() {
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
	a.movePicker.show(srcQueue, func(target string) {
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
