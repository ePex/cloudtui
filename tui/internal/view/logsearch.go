package view

import (
	"context"
	"fmt"
	"log/slog"
	"strings"
	"time"

	"github.com/gdamore/tcell/v2"
	"github.com/rivo/tview"

	"github.com/ePex/cloudtui/tui/internal/awslogs"
	"github.com/ePex/cloudtui/tui/internal/config"
	"github.com/ePex/cloudtui/tui/internal/dialog"
	"github.com/ePex/cloudtui/tui/internal/ui"
)

// LogSearchView is the CloudWatch Logs search screen: opened per log
// group via App.OpenLogSearch, not a registered ui.View. Unlike every
// other list view in this app, its filter (the pattern input) is a real
// AWS API call, not a local client-side filter — so it only searches on
// Enter, never on keystroke, and results are paginated by AWS but this
// view never auto-paginates (spec/34-fe-cloudwatch-logs decision 5).
type LogSearchView struct {
	table          *tview.Table
	patternInput   *tview.InputField
	flex           *tview.Flex
	host           ui.ViewHost
	timeRangeModal *dialog.TimeRangeModal
	onBack         func()
	logGroupName   string
	pattern        string
	tr             ui.TimeRange
	results        []awslogs.LogEvent
	hasMore        bool
}

var _ ui.Themeable = (*LogSearchView)(nil)

// ApplyPalette recolors the CloudWatch Logs search view for a live theme switch.
func (sv *LogSearchView) ApplyPalette(p config.Palette) {
	bg := tcell.GetColor(p.Background)
	sv.table.SetBackgroundColor(bg)
	sv.table.SetBorderColor(tcell.GetColor(p.ViewColor("cloudwatch-logs")))
	sv.table.SetTitleColor(tcell.GetColor(p.ViewColor("cloudwatch-logs")))
	sv.patternInput.SetLabelColor(tcell.GetColor(p.Label))
	sv.patternInput.SetFieldBackgroundColor(tcell.GetColor(p.SelectionBg))
	sv.patternInput.SetFieldTextColor(tcell.GetColor(p.SelectionText))
}

func (sv *LogSearchView) Primitive() tview.Primitive { return sv.flex }
func (sv *LogSearchView) Table() *tview.Table        { return sv.table }
func (sv *LogSearchView) FilterInputs() []tview.Primitive {
	return []tview.Primitive{sv.patternInput}
}

// Pattern returns the currently active filter pattern (e.g. after
// App.OpenLogSearch consumes a queued CorrelationID jump).
func (sv *LogSearchView) Pattern() string { return sv.pattern }

func (sv *LogSearchView) Shortcuts() []ui.Shortcut {
	return []ui.Shortcut{
		{Key: "Esc", Description: "back"},
		{Key: "r", Description: "refresh"},
		{Key: "t", Description: "time range"},
		{Key: "/", Description: "filter pattern"},
	}
}

// NewLogSearchView constructs the CloudWatch Logs search view.
func NewLogSearchView(a ui.ViewHost, timeRangeModal *dialog.TimeRangeModal, onSelect func(event awslogs.LogEvent), onBack func()) *LogSearchView {
	table := tview.NewTable()
	table.SetBorder(true)
	table.SetSelectable(true, false)
	table.SetFixed(1, 0)

	p := a.Config().Colors
	patternInput := tview.NewInputField()
	patternInput.SetLabel(" / pattern: ")
	patternInput.SetLabelColor(tcell.GetColor(p.Label))
	patternInput.SetFieldBackgroundColor(tcell.GetColor(p.SelectionBg))
	patternInput.SetFieldTextColor(tcell.GetColor(p.SelectionText))

	flex := tview.NewFlex().SetDirection(tview.FlexRow).
		AddItem(table, 0, 1, true).
		AddItem(patternInput, 1, 0, false)

	sv := &LogSearchView{table: table, patternInput: patternInput, flex: flex, host: a, timeRangeModal: timeRangeModal, onBack: onBack, tr: ui.TimeRange{Mode: ui.TimeRangeRelative, PresetIdx: ui.DefaultPresetIdx}}
	sv.setHeader()

	// Unlike every filter input elsewhere in the app, typing here must
	// not trigger anything — each keystroke would otherwise be a real
	// AWS API call. Only Enter (not Escape/Tab, which also reach
	// SetDoneFunc) runs the search.
	patternInput.SetDoneFunc(func(key tcell.Key) {
		if key == tcell.KeyEnter {
			sv.pattern = sv.patternInput.GetText()
			sv.search()
		}
		sv.host.SetFocus(sv.table)
	})
	patternInput.SetInputCapture(func(event *tcell.EventKey) *tcell.EventKey {
		if event.Key() == tcell.KeyUp || event.Key() == tcell.KeyDown {
			sv.host.SetFocus(sv.table)
			sv.table.InputHandler()(event, func(tview.Primitive) {})
			return nil
		}
		return event
	})

	table.SetInputCapture(func(event *tcell.EventKey) *tcell.EventKey {
		switch event.Rune() {
		case 'r':
			sv.search()
			return nil
		case 't':
			timeRangeModal.Show(sv.tr, func(tr ui.TimeRange) {
				sv.tr = tr
				sv.search()
			})
			return nil
		case '/':
			sv.patternInput.SetText(sv.pattern)
			sv.host.SetFocus(sv.patternInput)
			return nil
		case 'j':
			return tcell.NewEventKey(tcell.KeyDown, 0, tcell.ModNone)
		case 'k':
			return tcell.NewEventKey(tcell.KeyUp, 0, tcell.ModNone)
		}
		switch event.Key() {
		case tcell.KeyEscape, tcell.KeyBackspace, tcell.KeyBackspace2:
			sv.onBack()
			return nil
		}
		return event
	})

	table.SetSelectedFunc(func(row, _ int) {
		idx := row - 1 // row 0 is the header
		if idx < 0 || idx >= len(sv.results) {
			return
		}
		onSelect(sv.results[idx])
	})

	return sv
}

func (sv *LogSearchView) setHeader() {
	p := sv.host.Config().Colors
	bg := tcell.GetColor(p.Label)
	fg := tcell.GetColor(p.Background)
	for i, label := range []string{"TIMESTAMP", "STREAM", "MESSAGE"} {
		sv.table.SetCell(0, i,
			tview.NewTableCell(label).
				SetTextColor(fg).
				SetBackgroundColor(bg).
				SetSelectable(false).
				SetExpansion(1).
				SetAlign(tview.AlignCenter))
	}
}

// Open resets the view for a freshly-selected log group and runs the
// first search with the default time range. initialPattern pre-fills
// the filter pattern (used when arriving via FE 41's CorrelationID
// jump from a Datadog log); pass "" for the normal empty-pattern
// default.
func (sv *LogSearchView) Open(logGroupName, initialPattern string) {
	sv.logGroupName = logGroupName
	sv.pattern = initialPattern
	sv.patternInput.SetText(initialPattern)
	sv.tr = ui.TimeRange{Mode: ui.TimeRangeRelative, PresetIdx: ui.DefaultPresetIdx}
	sv.results = nil
	sv.hasMore = false
	for sv.table.GetRowCount() > 1 {
		sv.table.RemoveRow(sv.table.GetRowCount() - 1)
	}
	sv.setHeader()
	sv.table.SetTitle(fmt.Sprintf(" %s ", logGroupName))
	sv.search()
}

// search runs FilterEvents in a goroutine (a real AWS API call) and
// hands the outcome to handleSearchResult on the tview event loop.
// Requires an active AWS profile; errors clearly rather than calling
// into awslogs with an empty one.
func (sv *LogSearchView) search() {
	profile := sv.host.Config().ActiveAWSProfile
	if profile == "" {
		sv.showError(fmt.Errorf("no AWS profile selected — use :ap to select one"))
		return
	}
	logGroupName := sv.logGroupName
	pattern := sv.pattern
	start, end := sv.tr.Bounds(time.Now())
	go func() {
		events, hasMore, err := sv.host.FilterLogEvents(context.Background(), profile, logGroupName, start, end, pattern)
		sv.host.QueueUpdateDraw(func() {
			sv.handleSearchResult(events, hasMore, err)
		})
	}()
}

// handleSearchResult processes the outcome of a FilterEvents call: on
// error, logs and shows it; on success, caches the results and repaints.
// Split out from search so this — the part with actual logic — is
// directly testable without spawning a goroutine or needing a running
// tview event loop (QueueUpdateDraw blocks forever without one).
func (sv *LogSearchView) handleSearchResult(events []awslogs.LogEvent, hasMore bool, err error) {
	if err != nil {
		slog.Error("cloudwatch logs: search failed", "logGroup", sv.logGroupName, "error", err)
		sv.showError(err)
		return
	}
	sv.results = events
	sv.hasMore = hasMore
	sv.repaint()
}

func (sv *LogSearchView) repaint() {
	for sv.table.GetRowCount() > 1 {
		sv.table.RemoveRow(sv.table.GetRowCount() - 1)
	}

	p := sv.host.Config().Colors
	tsColor := tcell.GetColor(p.Label)
	textColor := tcell.GetColor(p.Text)
	for i, e := range sv.results {
		row := i + 1
		sv.table.SetCell(row, 0, tview.NewTableCell(e.Timestamp.Local().Format("2006-01-02 15:04:05")).SetTextColor(tsColor).SetExpansion(1))
		sv.table.SetCell(row, 1, tview.NewTableCell(e.LogStream).SetTextColor(textColor).SetExpansion(2))
		sv.table.SetCell(row, 2, tview.NewTableCell(logEventPreview(e.Message)).SetTextColor(textColor).SetExpansion(4))
	}

	if sv.table.GetRowCount() > 1 {
		sv.table.Select(1, 0)
		// Select alone isn't enough — see queues.go's repaint for why
		// SetOffset is also needed (tview.Table's "track end" auto-scroll
		// latches on during the table's first, still-empty draw).
		sv.table.SetOffset(0, 0)
	}

	sv.updateTitle()
}

// updateTitle never uses "[text]" — see queues.go's updateTitle for why
// (tview.Box titles run through the same tag-parsing Print() that Table
// cells do, silently swallowing square brackets).
func (sv *LogSearchView) updateTitle() {
	label := sv.tr.Label()
	title := fmt.Sprintf(" %s — %s — %d events", sv.logGroupName, label, len(sv.results))
	if sv.hasMore {
		title += " (more available — narrow your search)"
	}
	sv.table.SetTitle(title + " ")
}

func (sv *LogSearchView) showError(err error) {
	sv.results = nil
	sv.hasMore = false
	for sv.table.GetRowCount() > 1 {
		sv.table.RemoveRow(sv.table.GetRowCount() - 1)
	}
	sv.table.SetCell(1, 0,
		tview.NewTableCell(fmt.Sprintf("Error: %v", err)).
			SetTextColor(tcell.ColorRed).
			SetExpansion(3),
	)
	sv.table.SetTitle(fmt.Sprintf(" %s ", sv.logGroupName))
}

// logEventPreview collapses a (possibly multi-line, possibly very long)
// log message into a single display line for the results table — the
// full text is only ever shown in the detail view.
func logEventPreview(message string) string {
	if idx := strings.IndexAny(message, "\r\n"); idx >= 0 {
		message = message[:idx] + " …"
	}
	const maxLen = 200
	if len(message) > maxLen {
		message = message[:maxLen] + "…"
	}
	return message
}
