package view

import (
	"context"
	"fmt"
	"log/slog"
	"sort"
	"strings"
	"sync"

	"github.com/gdamore/tcell/v2"
	"github.com/rivo/tview"

	"github.com/ePex/cloudtui/tui/internal/config"
	"github.com/ePex/cloudtui/tui/internal/dialog"
	"github.com/ePex/cloudtui/tui/internal/queue"
	"github.com/ePex/cloudtui/tui/internal/ui"
)

// MessagesView shows the messages currently in a specific queue. It is not a
// registered ui.View (no SwitchTo / home dashboard entry); it is opened
// exclusively via App.OpenMessages and returns to "queues" on Esc/Backspace.
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
type MessagesView struct {
	table         *tview.Table
	searchInput   *tview.InputField
	flex          *tview.Flex
	host          ui.MessagesHost
	messageFilter *dialog.MessageFilter
	sendMessage   *dialog.SendMessageOverlay
	confirm       *dialog.ConfirmDialog
	movePicker    *dialog.MovePicker
	purge         purgeFlow
	queueName     string
	allMsgs       []queue.Message     // full set from the last load, pre-quick-search
	quickSearch   string              // active quick-search text (client-side)
	filter        queue.MessageFilter // active server-side filter
	browseNote    string              // last load's "showing n of m"-style title note (browseNote)
	loadHasMore   bool                // last load: the backend reported more (for the export's note)
	loadPending   int64               // last load: the queue's pending count, -1 if unknown
	export        exportFlow
	msgs          []queue.Message // sorted, quick-search-filtered snapshot, index 0 = row 1
	marked        map[string]bool // message IDs currently marked
	wrap          bool            // preview column word-wrap toggle
	rowToIdx      []int           // row -> index into msgs; index 0 unused (header placeholder)
	idxToRow      []int           // msgs index -> that item's primary row
}

var _ ui.Themeable = (*MessagesView)(nil)

// ApplyPalette recolors the messages view for a live theme switch.
func (mv *MessagesView) ApplyPalette(p config.Palette) {
	bg := tcell.GetColor(p.Background)
	mv.table.SetBackgroundColor(bg)
	mv.table.SetBorderColor(tcell.GetColor(p.ViewColor("queues")))
	mv.table.SetTitleColor(tcell.GetColor(p.ViewColor("queues")))
	ui.StyleFilterInput(mv.searchInput, p)
	mv.table.SetBordersColor(tcell.GetColor(p.Border)) // the column separators
	// The header row's cells are colored when they're created, not on
	// every palette change, so redraw them with the new palette.
	mv.setHeader()
}

func (mv *MessagesView) Primitive() tview.Primitive { return mv.flex }
func (mv *MessagesView) Table() *tview.Table        { return mv.table }
func (mv *MessagesView) FilterInputs() []tview.Primitive {
	return []tview.Primitive{mv.searchInput}
}

// QueueName returns the queue this view is currently showing messages for.
func (mv *MessagesView) QueueName() string { return mv.queueName }

// Filter returns the active server-side filter.
func (mv *MessagesView) Filter() queue.MessageFilter { return mv.filter }

// AllMessages returns the full set from the last load, pre-quick-search
// (see allMsgs's own doc comment) — used by the message filter overlay's
// JMS Type autocomplete to suggest types already visible without a fresh
// fetch.
func (mv *MessagesView) AllMessages() []queue.Message { return mv.allMsgs }

// ApplyFilter sets f as the active server-side filter, updates the title,
// and reloads.
func (mv *MessagesView) ApplyFilter(f queue.MessageFilter) {
	mv.filter = f
	mv.updateTitle()
	mv.Load()
}

// Open switches this view to show queueName's messages. Quick search and
// the server-side filter persist when reopened on the same queue, but
// reset when switching to a different one — carrying a leftover filter
// across queues would silently narrow what the user sees without them
// asking.
func (mv *MessagesView) Open(queueName string) {
	if mv.queueName != queueName {
		mv.filter = queue.MessageFilter{}
		mv.quickSearch = ""
		mv.searchInput.SetText("")
	}
	mv.queueName = queueName
	mv.updateTitle()
	mv.setHeader()
	mv.Load()
}

// restoreShortcuts returns focus to the message table and rewrites the
// context panel's shortcut hints after an overlay opened from here closes
// (the purge flow's JMS Type prompt) — QueuesView.restoreShortcuts' twin.
func (mv *MessagesView) restoreShortcuts() {
	mv.host.SetFocus(mv.table)
	lines := make([]string, 0, len(mv.Shortcuts()))
	for _, sc := range mv.Shortcuts() {
		lines = append(lines, fmt.Sprintf("[%s]<%s>[-] %s", mv.host.Config().Colors.Accent, sc.Key, sc.Description))
	}
	mv.host.SetContextHint(strings.Join(lines, "\n"))
}

func (mv *MessagesView) Shortcuts() []ui.Shortcut {
	wrap := "off"
	if mv.wrap {
		wrap = "on"
	}
	return []ui.Shortcut{
		{Key: "space", Description: "mark"},
		{Key: "a/n", Description: "mark all/none"},
		{Key: "d", Description: "delete marked/current"},
		{Key: "m", Description: "move marked/current"},
		{Key: "r", Description: "refresh"},
		{Key: "p", Description: "purge"},
		{Key: "c", Description: "create message"},
		{Key: "e", Description: "export marked/shown"},
		{Key: "/", Description: "quick search"},
		{Key: "f", Description: "filter"},
		{Key: "w", Description: "wrap: " + wrap},
		{Key: "Esc", Description: "back"},
	}
}

// NewMessagesView constructs the messages view. The queue name is set later
// via app.OpenMessages before the view is shown.
func NewMessagesView(a ui.MessagesHost, messageFilter *dialog.MessageFilter, sendMessage *dialog.SendMessageOverlay, confirm *dialog.ConfirmDialog, movePicker *dialog.MovePicker, jmsTypePrompt *dialog.JMSTypePrompt, textPrompt *dialog.TextPrompt, onSelect func(queueName string, msg queue.Message)) *MessagesView {
	table := tview.NewTable()
	table.SetBorder(true).SetTitle(" Messages ")
	table.SetSelectable(true, false)
	table.SetFixed(1, 0)

	p := a.Config().Colors
	searchInput := tview.NewInputField()
	searchInput.SetLabel(" / search: ")
	ui.StyleFilterInput(searchInput, p)

	flex := tview.NewFlex().SetDirection(tview.FlexRow).
		AddItem(table, 0, 1, true).
		AddItem(searchInput, 1, 0, false)

	mv := &MessagesView{table: table, searchInput: searchInput, flex: flex, host: a, messageFilter: messageFilter, sendMessage: sendMessage, confirm: confirm, movePicker: movePicker}
	mv.purge = purgeFlow{host: a, backend: a.Backend, prompt: jmsTypePrompt, confirm: confirm}
	mv.export = newExportFlow(a, textPrompt, confirm)
	mv.setHeader()

	searchInput.SetChangedFunc(func(text string) {
		mv.applyQuickSearch(text)
	})
	searchInput.SetDoneFunc(func(_ tcell.Key) {
		mv.applyQuickSearch(mv.searchInput.GetText())
		mv.host.SetFocus(mv.table)
	})
	searchInput.SetInputCapture(func(event *tcell.EventKey) *tcell.EventKey {
		if event.Key() == tcell.KeyUp || event.Key() == tcell.KeyDown {
			mv.applyQuickSearch(mv.searchInput.GetText())
			mv.host.SetFocus(mv.table)
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
			mv.Load()
			return nil
		case event.Rune() == '/':
			ui.SetInputFieldText(mv.searchInput, mv.quickSearch)
			mv.host.SetFocus(mv.searchInput)
			return nil
		case event.Rune() == 'f':
			mv.messageFilter.Show()
			return nil
		case event.Rune() == 'w':
			mv.wrap = !mv.wrap
			mv.renderRows()
			lines := make([]string, 0, len(mv.Shortcuts()))
			for _, sc := range mv.Shortcuts() {
				lines = append(lines, fmt.Sprintf("[%s]<%s>[-] %s", a.Config().Colors.Accent, sc.Key, sc.Description))
			}
			a.SetContextHint(strings.Join(lines, "\n"))
			return nil
		case event.Rune() == 'e':
			set := mv.exportSet()
			if len(set.msgs) == 0 {
				mv.host.SetStatus("[yellow]No messages to export[-]")
				return nil
			}
			mv.export.start(mv.queueName, func(context.Context) (exportSet, error) { return set, nil }, mv.restoreShortcuts)
			return nil
		case event.Rune() == 'c':
			name := mv.queueName
			mv.sendMessage.Show(name, func() {
				a.SetFocus(mv.table)
				lines := make([]string, 0, len(mv.Shortcuts()))
				for _, sc := range mv.Shortcuts() {
					lines = append(lines, fmt.Sprintf("[%s]<%s>[-] %s", a.Config().Colors.Accent, sc.Key, sc.Description))
				}
				a.SetContextHint(strings.Join(lines, "\n"))
			})
			return nil
		case event.Rune() == 'p':
			mv.purge.start(mv.queueName, mv.restoreShortcuts, mv.Load, mv.showError)
			return nil
		case event.Rune() == 'j':
			return tcell.NewEventKey(tcell.KeyDown, 0, tcell.ModNone)
		case event.Rune() == 'k':
			return tcell.NewEventKey(tcell.KeyUp, 0, tcell.ModNone)
		case event.Key() == tcell.KeyEscape, event.Key() == tcell.KeyBackspace, event.Key() == tcell.KeyBackspace2:
			a.SwitchTo("queues")
			return nil
		}
		return event
	})

	table.SetSelectedFunc(func(row, _ int) {
		if row <= 0 || row >= len(mv.rowToIdx) {
			return
		}
		msgIdx := mv.rowToIdx[row]
		if msgIdx < 0 || msgIdx >= len(mv.msgs) {
			return
		}
		onSelect(mv.queueName, mv.msgs[msgIdx])
	})

	return mv
}

// messageColumns are the messages table's columns, in order — one entry
// per column index. Header and data cells share this so a column's
// weight is never set in two places that can drift apart (which is
// exactly what happened before: the header blanket-set every column to
// Expansion(1), and since tview.Table computes a column's effective
// expansion as the max across every row including the header, that
// silently overrode data cells' own (lower, or unset/0) values — most
// visibly the marker column, which grew on resize despite never
// wanting any extra width at all).
//
// idColumn/corrIDColumn additionally cap their cells' MaxWidth: a full
// message ID or correlation UUID is long but rarely what someone's
// actually trying to read, so — found live — they were consistently
// eating space PREVIEW needed far more. Expansion alone can't shrink a
// column below its content's natural width; MaxWidth does (tview clips
// with the same "…" indicator already used for narrow columns). TYPE's
// cap (40) is a safety bound rather than a real-world limit — JMS type
// strings are typically well under it — added only so
// messagesOtherColumnsWidth (below) has a known worst case to subtract;
// an uncapped column has no such bound, which is exactly what let
// PREVIEW's dynamic wrap width still exceed the real available space
// on messages with a long Type value (found live, CR 92 follow-up).
var messageColumns = []struct {
	label     string
	expansion int
	maxWidth  int // 0 = uncapped
}{
	{"", 0, 0},
	{"ID", 0, 20},
	{"TYPE", 1, 40},
	{"CORR.ID", 0, 20},
	{"TIMESTAMP", 1, 0},
	{"PREVIEW", 10, 0},
}

const previewColumn = 5 // index into messageColumns

// messagesOtherColumnsWidth estimates every non-PREVIEW column's
// rendered width — used by dynamicWrapWidth (wraptext.go) to derive how
// much space is actually left for PREVIEW at render time: each
// capped column's MaxWidth, plus TIMESTAMP's fixed
// "2006-01-02 15:04:05" content width (19 — never capped, since it's
// always exactly that long), plus a margin for inter-column spacing.
const messagesOtherColumnsWidth = 3 /* marker */ +
	20 /* ID */ +
	40 /* Type */ +
	20 /* Corr.ID */ +
	19 /* Timestamp */ +
	8 /* inter-column spacing */

func (mv *MessagesView) setHeader() {
	p := mv.host.Config().Colors
	bg := tcell.GetColor(p.Label)
	fg := tcell.GetColor(p.Background)

	for i, col := range messageColumns {
		mv.table.SetCell(0, i,
			tview.NewTableCell(col.label).
				SetTextColor(fg).
				SetBackgroundColor(bg).
				SetSelectable(false).
				SetExpansion(col.expansion).
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

// Load browses the queue in the background. An unfiltered browse also
// fetches the queue's pending count, concurrently, so the title can say
// when the browser shows only part of the queue (browseNote).
func (mv *MessagesView) Load() {
	queueName := mv.queueName
	filter := withDefaultMaxCount(mv.filter)
	mv.browseNote = "" // the previous load's; a different queue or filter makes it wrong
	mv.loadHasMore, mv.loadPending = false, -1
	mv.updateTitle()
	go func() {
		ctx := context.Background()
		backend := mv.host.Backend()
		pending := int64(-1)
		var wg sync.WaitGroup
		if !narrowsMessages(filter) {
			wg.Go(func() {
				if n, ok := pendingCount(ctx, backend, queueName); ok {
					pending = n
				}
			})
		}
		msgs, hasMore, err := backend.BrowseMessages(ctx, queueName, filter)
		wg.Wait()
		mv.host.QueueUpdateDraw(func() {
			mv.applyLoad(queueName, filter, msgs, hasMore, pending, err)
		})
	}()
}

// applyLoad applies a completed Load — split out from its goroutine so
// tests can call it directly. pending is the queue's pending count, or -1
// when it wasn't fetched (a filtered browse) or couldn't be read.
func (mv *MessagesView) applyLoad(queueName string, filter queue.MessageFilter, msgs []queue.Message, hasMore bool, pending int64, err error) {
	if err != nil {
		slog.Error("messages: failed to browse messages", "queue", queueName, "error", err)
		mv.showError(err)
		return
	}
	mv.browseNote = browseNote(narrowsMessages(filter), len(msgs), hasMore, pending)
	mv.loadHasMore, mv.loadPending = hasMore, pending
	mv.updateTitle()
	mv.repaint(msgs)
	if len(msgs) > 0 && msgs[0].ID == "" {
		mv.host.SetStatus("[yellow]Note: limited message info — individual move/delete unavailable[-]")
	}
}

// narrowsMessages reports whether f selects a subset of the queue (by JMS
// type, message ID or date) rather than just capping how many are shown —
// only an unfiltered browse is comparable with the queue's pending count.
func narrowsMessages(f queue.MessageFilter) bool {
	return f.JMSType != "" || f.MessageID != "" || !f.FromDate.IsZero() || !f.ToDate.IsZero()
}

// browseNote says, for the title, when a browse of n messages shows only
// part of what's on the queue — the broker stops browsing at its browse
// limit (400 by default), and mq-proxy reports that as hasMore (spec-wip
// bugfix-purge-completeness). Unfiltered with a known pending count, it
// compares the two, which works even when hasMore doesn't; otherwise it
// relies on hasMore. Empty when the browse looks complete.
func browseNote(filtered bool, n int, hasMore bool, pending int64) string {
	if !filtered && pending >= 0 {
		if int64(n) < pending {
			return fmt.Sprintf("showing %d of %d", n, pending)
		}
		return ""
	}
	if hasMore {
		return fmt.Sprintf("first %d browsed, more on queue", n)
	}
	return ""
}

// applyQuickSearch sets the active quick-search text and re-filters the
// currently-loaded messages against it, without re-fetching from the
// backend — unlike the server-side filter form, this is purely client-side
// and safe to call on every keystroke.
func (mv *MessagesView) applyQuickSearch(s string) {
	mv.quickSearch = s
	mv.updateTitle()
	mv.repaint(mv.allMsgs)
}

// updateTitle sets the table's border title, reflecting the queue name plus
// whichever of the two filter mechanisms are active, and the last load's
// browseNote. Parens/brackets wrap active segments only — see queues.go's
// updateTitle for why "(text)" is used instead of "[text]" (square
// brackets are swallowed as color tags).
func (mv *MessagesView) updateTitle() {
	title := fmt.Sprintf(" Messages — %s ", mv.queueName)
	if desc := describeMessageFilter(withDefaultMaxCount(mv.filter)); desc != "" {
		title = fmt.Sprintf(" Messages — %s (filter: %s) ", mv.queueName, desc)
	}
	if mv.browseNote != "" {
		title = strings.TrimRight(title, " ") + fmt.Sprintf(" (%s) ", mv.browseNote)
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

func (mv *MessagesView) repaint(msgs []queue.Message) {
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

	mv.renderRows()

	if mv.table.GetRowCount() > 1 {
		mv.table.Select(1, 0)
		// Select alone isn't enough — see queues.go's repaint for why
		// SetOffset is also needed (tview.Table's "track end" auto-scroll
		// latches on during the table's first, still-empty draw).
		mv.table.SetOffset(0, 0)
	}
}

// renderRows rebuilds the table body from mv.msgs, wrap-aware, without
// touching mv.allMsgs/mv.quickSearch/mv.marked or the active filter —
// unlike repaint (which also re-fetches/re-filters/re-sorts and resets
// marks on every call), this only redraws. Used both by repaint, after
// it's done its own data processing, and directly by the wrap toggle,
// which has no reason to touch any of that — toggling wrap is purely
// cosmetic and shouldn't silently drop the user's marks.
//
// Preserves the currently-selected message across the rebuild (by
// index, not identity — mv.msgs's order/set doesn't change from a call
// to this function alone), since row numbers shift once wrapping
// changes how many rows an item spans.
func (mv *MessagesView) renderRows() {
	selectedIdx := -1
	if row, _ := mv.table.GetSelection(); row > 0 && row < len(mv.rowToIdx) {
		selectedIdx = mv.rowToIdx[row]
	}

	for mv.table.GetRowCount() > 1 {
		mv.table.RemoveRow(mv.table.GetRowCount() - 1)
	}

	p := mv.host.Config().Colors
	idColor := tcell.GetColor(p.Value)
	tsColor := tcell.GetColor(p.Label)
	textColor := tcell.GetColor(p.Text)

	mv.rowToIdx = make([]int, 1, len(mv.msgs)+1) // index 0 unused (header)
	mv.idxToRow = make([]int, len(mv.msgs))

	var wrapWidth int
	if mv.wrap {
		wrapWidth = dynamicWrapWidth(mv.table, messagesOtherColumnsWidth)
	}

	row := 1
	for i, m := range mv.msgs {
		mv.idxToRow[i] = row
		mv.rowToIdx = append(mv.rowToIdx, i)

		lines := []string{m.Preview}
		if mv.wrap {
			lines = wrapMultilineText(m.Preview, wrapWidth, maxWrapLines)
		}

		mv.table.SetCell(row, 0, mv.markerCell(mv.marked[m.ID]))
		mv.table.SetCell(row, 1, tview.NewTableCell(m.ID).SetTextColor(idColor).
			SetExpansion(messageColumns[1].expansion).SetMaxWidth(messageColumns[1].maxWidth))
		mv.table.SetCell(row, 2, tview.NewTableCell(m.JMSType).SetTextColor(tsColor).
			SetExpansion(messageColumns[2].expansion).SetMaxWidth(messageColumns[2].maxWidth))
		mv.table.SetCell(row, 3, tview.NewTableCell(m.CorrelationID).SetTextColor(idColor).
			SetExpansion(messageColumns[3].expansion).SetMaxWidth(messageColumns[3].maxWidth))
		mv.table.SetCell(row, 4, tview.NewTableCell(m.Timestamp.Local().Format("2006-01-02 15:04:05")).SetTextColor(tsColor).
			SetExpansion(messageColumns[4].expansion))
		mv.table.SetCell(row, previewColumn, tview.NewTableCell(lines[0]).SetTextColor(textColor).
			SetExpansion(messageColumns[previewColumn].expansion))
		row++

		for _, extra := range lines[1:] {
			mv.rowToIdx = append(mv.rowToIdx, i)
			setContinuationRow(mv.table, row, len(messageColumns), previewColumn, extra, textColor, messageColumns[previewColumn].expansion)
			row++
		}
	}

	if selectedIdx >= 0 && selectedIdx < len(mv.idxToRow) {
		mv.table.Select(mv.idxToRow[selectedIdx], 0)
	}
}

// markerCell builds the checkbox cell shown in the marker column. Plain "[x]"
// text doesn't work here: tview.Table always interprets "[...]" in cell text
// as a color/region tag, so it gets silently swallowed instead of displayed.
func (mv *MessagesView) markerCell(marked bool) *tview.TableCell {
	p := mv.host.Config().Colors
	text, color := " ", tcell.GetColor(p.Text)
	if marked {
		text, color = "✓", tcell.GetColor(p.Accent)
	}
	return tview.NewTableCell(text).SetTextColor(color).SetAlign(tview.AlignCenter)
}

// refreshMarkerColumn redraws column 0 to reflect the current marked set,
// without re-fetching, resorting, or rebuilding wrapped rows — mv.idxToRow
// (not i+1) since an earlier message may have wrapped into more than one
// row, shifting every row after it.
func (mv *MessagesView) refreshMarkerColumn() {
	for i, m := range mv.msgs {
		mv.table.SetCell(mv.idxToRow[i], 0, mv.markerCell(mv.marked[m.ID]))
	}
}

// markedIDs returns the IDs of currently marked messages, in the table's
// current display order.
// exportSet is what 'e' exports: the marked messages if any are marked —
// exactly those, never "incomplete" — otherwise every message shown (the
// server-side filter and quick search applied). The latter says when the
// last load itself was incomplete, so "of <pending>" reaches the result
// line (spec-wip fe-message-export).
func (mv *MessagesView) exportSet() exportSet {
	if len(mv.marked) > 0 {
		var msgs []queue.Message
		for _, m := range mv.msgs {
			if mv.marked[m.ID] {
				msgs = append(msgs, m)
			}
		}
		return exportSet{msgs: msgs}
	}
	set := exportSet{msgs: append([]queue.Message(nil), mv.msgs...), hasMore: mv.loadHasMore, why: "only what the list shows"}
	if mv.loadPending > int64(len(mv.allMsgs)) {
		set.total = mv.loadPending
	}
	return set
}

func (mv *MessagesView) markedIDs() []string {
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
func (mv *MessagesView) targetIDs() []string {
	if ids := mv.markedIDs(); len(ids) > 0 {
		return ids
	}
	row, _ := mv.table.GetSelection()
	if row <= 0 || row >= len(mv.rowToIdx) {
		return nil
	}
	idx := mv.rowToIdx[row]
	if idx < 0 || idx >= len(mv.msgs) || mv.msgs[idx].ID == "" {
		return nil
	}
	return []string{mv.msgs[idx].ID}
}

// toggleMark flips the mark on the row under the cursor and advances the
// cursor, so repeated space presses mark a run of messages quickly. Messages
// without an ID (limited-info mode) can't be marked, matching the existing
// restriction on individual move/delete.
func (mv *MessagesView) toggleMark() {
	row, _ := mv.table.GetSelection()
	if row <= 0 || row >= len(mv.rowToIdx) {
		return
	}
	idx := mv.rowToIdx[row]
	if idx < 0 || idx >= len(mv.msgs) {
		return
	}
	m := mv.msgs[idx]
	if m.ID == "" {
		mv.host.SetStatus("[yellow]Cannot mark: message ID unavailable[-]")
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
	// Forward a synthetic KeyDown through the table's own InputHandler
	// rather than a raw Select(row+1, 0) — tview.Table's built-in down()
	// already skips non-selectable rows (see wraptext.go), so this
	// correctly lands on the next message's primary row even when the
	// current one wrapped into more than one row, with no extra code.
	mv.table.InputHandler()(tcell.NewEventKey(tcell.KeyDown, 0, tcell.ModNone), func(tview.Primitive) {})
}

// markAll marks every message that has an ID.
func (mv *MessagesView) markAll() {
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
		mv.host.SetStatus(fmt.Sprintf("Marked %d message(s); %d skipped (no ID)", len(mv.marked), skipped))
	} else {
		mv.host.SetStatus(fmt.Sprintf("Marked %d message(s)", len(mv.marked)))
	}
}

// clearMarks deselects every marked message.
func (mv *MessagesView) clearMarks() {
	if len(mv.marked) == 0 {
		return
	}
	mv.marked = map[string]bool{}
	mv.refreshMarkerColumn()
	mv.host.SetStatus("Cleared marks")
}

// deleteMarked confirms and deletes every marked message, or — if nothing is
// marked — the single message under the cursor. Each deletion is
// independent, so one failure doesn't stop the rest; the status bar reports
// how many of the batch actually succeeded.
func (mv *MessagesView) deleteMarked() {
	ids := mv.targetIDs()
	if len(ids) == 0 {
		mv.host.SetStatus("[yellow]No message marked or selected[-]")
		return
	}
	queueName := mv.queueName
	question := fmt.Sprintf("Delete message from %q?", queueName)
	if len(mv.markedIDs()) > 0 {
		question = fmt.Sprintf("Delete %d marked message(s) from %q?", len(ids), queueName)
	}
	mv.confirm.Show(question, func() {
		mv.deleteMessages(ids, queueName)
	})
}

func (mv *MessagesView) deleteMessages(ids []string, queueName string) {
	a := mv.host
	total := len(ids)
	progressID := a.StartOperationProgress(fmt.Sprintf("Deleting %d message(s) from %s", total, queueName), total)
	go func() {
		failed := 0
		for completed, id := range ids {
			if err := a.Backend().RemoveMessage(context.Background(), queueName, id); err != nil {
				slog.Error("messages: bulk delete failed", "queue", queueName, "id", id, "error", err)
				failed++
			}
			count := completed + 1
			a.QueueUpdateDraw(func() { a.UpdateOperationProgress(progressID, count, total) })
		}
		a.QueueUpdateDraw(func() {
			a.FinishOperationProgress(progressID)
			switch {
			case failed > 0:
				a.SetStatus(fmt.Sprintf("[red]Deleted %d/%d message(s); %d failed[-]", len(ids)-failed, len(ids), failed))
			case len(ids) == 1:
				a.SetStatus("Deleted message")
			default:
				a.SetStatus(fmt.Sprintf("Deleted %d message(s)", len(ids)))
			}
			mv.Load()
		})
	}()
}

// moveMarked opens the move picker once and, on target selection, moves
// every marked message there, or — if nothing is marked — the single
// message under the cursor. As with deleteMarked, one failure doesn't stop
// the rest.
func (mv *MessagesView) moveMarked() {
	ids := mv.targetIDs()
	if len(ids) == 0 {
		mv.host.SetStatus("[yellow]No message marked or selected[-]")
		return
	}
	a := mv.host
	srcQueue := mv.queueName
	restore := func() {
		a.SetFocus(mv.table)
		lines := make([]string, 0, len(mv.Shortcuts()))
		for _, sc := range mv.Shortcuts() {
			lines = append(lines, fmt.Sprintf("[%s]<%s>[-] %s", a.Config().Colors.Accent, sc.Key, sc.Description))
		}
		a.SetContextHint(strings.Join(lines, "\n"))
	}
	mv.movePicker.Show(srcQueue, func(target string) {
		a.QueueUpdateDraw(func() { mv.moveMessages(ids, srcQueue, target, restore) })
	}, restore)
}

func (mv *MessagesView) moveMessages(ids []string, srcQueue, target string, restore func()) {
	a := mv.host
	total := len(ids)
	progressID := a.StartOperationProgress(fmt.Sprintf("Moving %d message(s) from %s", total, srcQueue), total)
	go func() {
		failed := 0
		for completed, id := range ids {
			if err := a.Backend().MoveMessage(context.Background(), srcQueue, id, target); err != nil {
				slog.Error("messages: bulk move failed", "src", srcQueue, "dst", target, "id", id, "error", err)
				failed++
			}
			count := completed + 1
			a.QueueUpdateDraw(func() { a.UpdateOperationProgress(progressID, count, total) })
		}
		a.QueueUpdateDraw(func() {
			a.FinishOperationProgress(progressID)
			switch {
			case failed > 0:
				a.SetStatus(fmt.Sprintf("[red]Moved %d/%d message(s) to %q; %d failed[-]", len(ids)-failed, len(ids), target, failed))
			case len(ids) == 1:
				a.SetStatus(fmt.Sprintf("Moved message to %q", target))
			default:
				a.SetStatus(fmt.Sprintf("Moved %d message(s) to %q", len(ids), target))
			}
			restore()
			mv.Load()
		})
	}()
}

func (mv *MessagesView) showError(err error) {
	for mv.table.GetRowCount() > 1 {
		mv.table.RemoveRow(mv.table.GetRowCount() - 1)
	}
	mv.table.SetCell(1, 0,
		tview.NewTableCell(fmt.Sprintf("Error: %v", err)).
			SetTextColor(tcell.ColorRed).
			SetExpansion(3),
	)
}
