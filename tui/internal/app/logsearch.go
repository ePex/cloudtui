package app

import (
	"context"
	"fmt"
	"log/slog"
	"strings"
	"time"

	"github.com/gdamore/tcell/v2"
	"github.com/rivo/tview"

	"github.com/ePex/cloudtui/tui/internal/awslogs"
	"github.com/ePex/cloudtui/tui/internal/ui"
)

// timeRangePreset is one relative time window offered by logSearchView's
// 't' cycling key (spec/34-fe-cloudwatch-logs decision 4 — relative
// presets, not free-form timestamps).
type timeRangePreset struct {
	label    string
	duration time.Duration
}

var timeRangePresets = []timeRangePreset{
	{"15m", 15 * time.Minute},
	{"1h", time.Hour},
	{"3h", 3 * time.Hour},
	{"24h", 24 * time.Hour},
}

// defaultPresetIdx is "1h" — a reasonable default investigation window.
const defaultPresetIdx = 1

// logSearchView is the CloudWatch Logs search screen: opened per log
// group via App.openLogSearch, not a registered ui.View. Unlike every
// other list view in this app, its filter (the pattern input) is a real
// AWS API call, not a local client-side filter — so it only searches on
// Enter, never on keystroke, and results are paginated by AWS but this
// view never auto-paginates (spec/34-fe-cloudwatch-logs decision 5).
type logSearchView struct {
	table        *tview.Table
	patternInput *tview.InputField
	flex         *tview.Flex
	app          *App
	logGroupName string
	pattern      string
	presetIdx    int
	results      []awslogs.LogEvent
	hasMore      bool
}

func (sv *logSearchView) Shortcuts() []ui.Shortcut {
	return []ui.Shortcut{
		{Key: "Esc", Description: "back"},
		{Key: "r", Description: "refresh"},
		{Key: "t", Description: "time range"},
		{Key: "/", Description: "filter pattern"},
	}
}

// newLogSearchView constructs the CloudWatch Logs search view.
func newLogSearchView(a *App) *logSearchView {
	table := tview.NewTable()
	table.SetBorder(true)
	table.SetSelectable(true, false)
	table.SetFixed(1, 0)

	p := a.cfg.Colors
	patternInput := tview.NewInputField()
	patternInput.SetLabel(" / pattern: ")
	patternInput.SetLabelColor(tcell.GetColor(p.Label))
	patternInput.SetFieldBackgroundColor(tcell.GetColor(p.SelectionBg))
	patternInput.SetFieldTextColor(tcell.GetColor(p.SelectionText))

	flex := tview.NewFlex().SetDirection(tview.FlexRow).
		AddItem(table, 0, 1, true).
		AddItem(patternInput, 1, 0, false)

	sv := &logSearchView{table: table, patternInput: patternInput, flex: flex, app: a, presetIdx: defaultPresetIdx}
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
		sv.app.tv.SetFocus(sv.table)
	})
	patternInput.SetInputCapture(func(event *tcell.EventKey) *tcell.EventKey {
		if event.Key() == tcell.KeyUp || event.Key() == tcell.KeyDown {
			sv.app.tv.SetFocus(sv.table)
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
			sv.cycleTimeRange()
			return nil
		case '/':
			sv.patternInput.SetText(sv.pattern)
			sv.app.tv.SetFocus(sv.patternInput)
			return nil
		case 'j':
			return tcell.NewEventKey(tcell.KeyDown, 0, tcell.ModNone)
		case 'k':
			return tcell.NewEventKey(tcell.KeyUp, 0, tcell.ModNone)
		}
		switch event.Key() {
		case tcell.KeyEscape, tcell.KeyBackspace, tcell.KeyBackspace2:
			a.pages.SwitchToPage("cloudwatch-logs")
			a.tv.SetFocus(a.logsV.table)
			a.updateContextPanel(a.logsV)
			return nil
		}
		return event
	})

	return sv
}

func (sv *logSearchView) setHeader() {
	p := sv.app.cfg.Colors
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

// open resets the view for a freshly-selected log group and runs the
// first search with the default time range. initialPattern pre-fills
// the filter pattern (used when arriving via FE 41's CorrelationID
// jump from a Datadog log); pass "" for the normal empty-pattern
// default.
func (sv *logSearchView) open(logGroupName, initialPattern string) {
	sv.logGroupName = logGroupName
	sv.pattern = initialPattern
	sv.patternInput.SetText(initialPattern)
	sv.presetIdx = defaultPresetIdx
	sv.results = nil
	sv.hasMore = false
	for sv.table.GetRowCount() > 1 {
		sv.table.RemoveRow(sv.table.GetRowCount() - 1)
	}
	sv.setHeader()
	sv.table.SetTitle(fmt.Sprintf(" %s ", logGroupName))
	sv.search()
}

func (sv *logSearchView) cycleTimeRange() {
	sv.presetIdx = (sv.presetIdx + 1) % len(timeRangePresets)
	sv.search()
}

// search runs FilterEvents in a goroutine (a real AWS API call) and
// hands the outcome to handleSearchResult on the tview event loop.
// Requires an active AWS profile; errors clearly rather than calling
// into awslogs with an empty one.
func (sv *logSearchView) search() {
	profile := sv.app.cfg.ActiveAWSProfile
	if profile == "" {
		sv.showError(fmt.Errorf("no AWS profile selected — use :ap to select one"))
		return
	}
	logGroupName := sv.logGroupName
	pattern := sv.pattern
	end := time.Now()
	start := end.Add(-timeRangePresets[sv.presetIdx].duration)
	go func() {
		events, hasMore, err := sv.app.filterLogEvents(context.Background(), profile, logGroupName, start, end, pattern)
		sv.app.tv.QueueUpdateDraw(func() {
			sv.handleSearchResult(events, hasMore, err)
		})
	}()
}

// handleSearchResult processes the outcome of a FilterEvents call: on
// error, logs and shows it; on success, caches the results and repaints.
// Split out from search so this — the part with actual logic — is
// directly testable without spawning a goroutine or needing a running
// tview event loop (QueueUpdateDraw blocks forever without one).
func (sv *logSearchView) handleSearchResult(events []awslogs.LogEvent, hasMore bool, err error) {
	if err != nil {
		slog.Error("cloudwatch logs: search failed", "logGroup", sv.logGroupName, "error", err)
		sv.showError(err)
		return
	}
	sv.results = events
	sv.hasMore = hasMore
	sv.repaint()
}

func (sv *logSearchView) repaint() {
	for sv.table.GetRowCount() > 1 {
		sv.table.RemoveRow(sv.table.GetRowCount() - 1)
	}

	p := sv.app.cfg.Colors
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
func (sv *logSearchView) updateTitle() {
	preset := timeRangePresets[sv.presetIdx].label
	title := fmt.Sprintf(" %s — %s — %d events", sv.logGroupName, preset, len(sv.results))
	if sv.hasMore {
		title += " (more available — narrow your search)"
	}
	sv.table.SetTitle(title + " ")
}

func (sv *logSearchView) showError(err error) {
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
