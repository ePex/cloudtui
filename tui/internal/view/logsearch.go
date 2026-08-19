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
	nextToken      string // "" if no further pages are available
	wrap           bool   // message column word-wrap toggle
	rowToIdx       []int  // row -> index into results; index 0 unused (header placeholder)
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

// TimeRange returns the currently active time range — exported for the
// same reason as Pattern: internal/app's tests need to verify a
// computed range (e.g. spec-origin/91's CorrelationID-jump window)
// crossed the package boundary correctly.
func (sv *LogSearchView) TimeRange() ui.TimeRange { return sv.tr }

func (sv *LogSearchView) Shortcuts() []ui.Shortcut {
	wrap := "off"
	if sv.wrap {
		wrap = "on"
	}
	return []ui.Shortcut{
		{Key: "Esc", Description: "back"},
		{Key: "r", Description: "refresh"},
		{Key: "n", Description: "load more"},
		{Key: "t", Description: "time range"},
		{Key: "/", Description: "filter pattern"},
		{Key: "w", Description: "wrap: " + wrap},
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
		case 'n':
			sv.loadMore()
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
		case 'w':
			sv.wrap = !sv.wrap
			sv.renderRows()
			lines := make([]string, 0, len(sv.Shortcuts()))
			for _, sc := range sv.Shortcuts() {
				lines = append(lines, fmt.Sprintf("[%s]<%s>[-] %s", a.Config().Colors.Accent, sc.Key, sc.Description))
			}
			a.SetContextHint(strings.Join(lines, "\n"))
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
		if row <= 0 || row >= len(sv.rowToIdx) {
			return
		}
		idx := sv.rowToIdx[row]
		if idx < 0 || idx >= len(sv.results) {
			return
		}
		onSelect(sv.results[idx])
	})

	return sv
}

// logSearchColumns are the results table's columns — see messageColumns'
// doc comment in messages.go for why header and data cells share this
// instead of each setting their own Expansion. STREAM's MaxWidth caps a
// log stream name (frequently a long ARN) from eating space MESSAGE —
// the actually important column, found live to be visibly too
// small — needs far more of.
var logSearchColumns = []struct {
	label     string
	expansion int
	maxWidth  int // 0 = uncapped
}{
	{"TIMESTAMP", 1, 0},
	{"STREAM", 1, 30},
	{"MESSAGE", 10, 0},
}

const logSearchMessageColumn = 2 // index into logSearchColumns

func (sv *LogSearchView) setHeader() {
	p := sv.host.Config().Colors
	bg := tcell.GetColor(p.Label)
	fg := tcell.GetColor(p.Background)
	for i, col := range logSearchColumns {
		sv.table.SetCell(0, i,
			tview.NewTableCell(col.label).
				SetTextColor(fg).
				SetBackgroundColor(bg).
				SetSelectable(false).
				SetExpansion(col.expansion).
				SetAlign(tview.AlignCenter))
	}
}

// Open resets the view for a freshly-selected log group and runs the
// first search. initialPattern pre-fills the filter pattern (used when
// arriving via FE 41's CorrelationID jump from a Datadog log); pass ""
// for the normal empty-pattern default. initialTimeRange overrides the
// usual reset-to-relative-default behavior — pass nil for that default,
// non-nil (e.g. spec-origin/91's ±15m absolute window centered on the
// originating Datadog event) to open on that exact range instead.
func (sv *LogSearchView) Open(logGroupName, initialPattern string, initialTimeRange *ui.TimeRange) {
	sv.logGroupName = logGroupName
	sv.pattern = initialPattern
	sv.patternInput.SetText(initialPattern)
	if initialTimeRange != nil {
		sv.tr = *initialTimeRange
	} else {
		sv.tr = ui.TimeRange{Mode: ui.TimeRangeRelative, PresetIdx: ui.DefaultPresetIdx}
	}
	sv.results = nil
	sv.nextToken = ""
	for sv.table.GetRowCount() > 1 {
		sv.table.RemoveRow(sv.table.GetRowCount() - 1)
	}
	sv.setHeader()
	sv.table.SetTitle(fmt.Sprintf(" %s ", logGroupName))
	sv.search()
}

// maxAutoContinuePages bounds how many FilterLogEvents pages search()
// fetches automatically in one call when a filter pattern is set (see
// fetchPages) — without a cap, a broad pattern against a very
// high-volume log group could keep fetching indefinitely.
const maxAutoContinuePages = 10

// fetchPages calls fetch once per page (chained by nextToken, starting
// from ""), accumulating events, until either a page's returned token
// is "" (no more results) or maxPages calls have been made — whichever
// comes first. Stops immediately on error, discarding any events
// already accumulated: a partial result set with no indication it's
// partial would be misleading, so this matches handleSearchResult's
// existing all-or-nothing error handling.
//
// Takes fetch as a plain closure (not a *LogSearchView method) rather
// than calling host.FilterLogEvents directly, so it has no
// goroutine/UI dependency and is directly unit-testable with a
// call-counting stub — same reasoning as buildLogEvents/
// handleSearchResult being split out from their network-calling
// callers.
func fetchPages(fetch func(nextToken string) ([]awslogs.LogEvent, string, error), maxPages int) ([]awslogs.LogEvent, string, error) {
	var events []awslogs.LogEvent
	token := ""
	for i := 0; i < maxPages; i++ {
		page, next, err := fetch(token)
		if err != nil {
			return nil, "", err
		}
		events = append(events, page...)
		token = next
		if token == "" {
			break
		}
	}
	return events, token, nil
}

// search runs FilterEvents (via fetchPages) in a goroutine (real AWS API
// calls) and hands the outcome to handleSearchResult on the tview event
// loop. Requires an active AWS profile; errors clearly rather than
// calling into awslogs with an empty one. If a filter pattern is set,
// fetches up to maxAutoContinuePages pages automatically rather than
// stopping at the first — see fetchPages and spec-wip/
// 90-cr-log-search-pagination for why a pattern shouldn't let the
// single-page cap hide a matching event.
func (sv *LogSearchView) search() {
	profile := sv.host.Config().ActiveAWSProfile
	if profile == "" {
		sv.showError(fmt.Errorf("no AWS profile selected — use :ap to select one"))
		return
	}
	logGroupName := sv.logGroupName
	pattern := sv.pattern
	start, end := sv.tr.Bounds(time.Now())
	maxPages := 1
	if pattern != "" {
		maxPages = maxAutoContinuePages
	}
	go func() {
		events, next, err := fetchPages(func(nextToken string) ([]awslogs.LogEvent, string, error) {
			return sv.host.FilterLogEvents(context.Background(), profile, logGroupName, start, end, pattern, nextToken)
		}, maxPages)
		sv.host.QueueUpdateDraw(func() {
			sv.handleSearchResult(events, next, err)
		})
	}()
}

// handleSearchResult processes the outcome of a search() call: on
// error, logs and shows it; on success, caches the results and repaints.
// Split out from search so this — the part with actual logic — is
// directly testable without spawning a goroutine or needing a running
// tview event loop (QueueUpdateDraw blocks forever without one).
func (sv *LogSearchView) handleSearchResult(events []awslogs.LogEvent, next string, err error) {
	if err != nil {
		slog.Error("cloudwatch logs: search failed", "logGroup", sv.logGroupName, "error", err)
		sv.showError(err)
		return
	}
	sv.results = events
	sv.nextToken = next
	sv.repaint()
}

// loadMore fetches exactly one more page beyond the current results,
// continuing from sv.nextToken, and appends it — unlike search(), which
// replaces sv.results outright. A no-op if there's no further page to
// fetch (sv.nextToken == ""), which is also how it's reached whether or
// not a filter pattern is set: search() only auto-continues past the
// first page when a pattern is set (see maxAutoContinuePages), so this
// is what lets a plain browse (or a pattern search that hit the
// auto-continue cap) go further.
func (sv *LogSearchView) loadMore() {
	if sv.nextToken == "" {
		return
	}
	profile := sv.host.Config().ActiveAWSProfile
	if profile == "" {
		sv.showError(fmt.Errorf("no AWS profile selected — use :ap to select one"))
		return
	}
	logGroupName := sv.logGroupName
	pattern := sv.pattern
	start, end := sv.tr.Bounds(time.Now())
	token := sv.nextToken
	go func() {
		events, next, err := sv.host.FilterLogEvents(context.Background(), profile, logGroupName, start, end, pattern, token)
		sv.host.QueueUpdateDraw(func() {
			sv.handleLoadMoreResult(events, next, err)
		})
	}()
}

// handleLoadMoreResult processes the outcome of a loadMore() call: on
// error, logs it but leaves the existing results/table untouched —
// unlike handleSearchResult's error path, loadMore augments an
// already-successful search, so its failure shouldn't discard what's
// already there. On success, appends the new page (rather than
// replacing, see handleSearchResult) and repaints.
func (sv *LogSearchView) handleLoadMoreResult(events []awslogs.LogEvent, next string, err error) {
	if err != nil {
		slog.Error("cloudwatch logs: load more failed", "logGroup", sv.logGroupName, "error", err)
		return
	}
	sv.results = append(sv.results, events...)
	sv.nextToken = next
	sv.repaint()
}

func (sv *LogSearchView) repaint() {
	sv.renderRows()

	if sv.table.GetRowCount() > 1 {
		sv.table.Select(1, 0)
		// Select alone isn't enough — see queues.go's repaint for why
		// SetOffset is also needed (tview.Table's "track end" auto-scroll
		// latches on during the table's first, still-empty draw).
		sv.table.SetOffset(0, 0)
	}

	sv.updateTitle()
}

// renderRows rebuilds the table body from sv.results, wrap-aware, without
// resetting scroll position — unlike repaint (which always jumps to row
// 1, matching a genuine reload/load-more), this is also called directly
// by the wrap toggle, which has no reason to reset the user's place in
// the results. Preserves the currently-selected event across the
// rebuild (by index, not identity — sv.results's order/set doesn't
// change from a call to this function alone), since row numbers shift
// once wrapping changes how many rows an item spans.
func (sv *LogSearchView) renderRows() {
	selectedIdx := -1
	if row, _ := sv.table.GetSelection(); row > 0 && row < len(sv.rowToIdx) {
		selectedIdx = sv.rowToIdx[row]
	}

	for sv.table.GetRowCount() > 1 {
		sv.table.RemoveRow(sv.table.GetRowCount() - 1)
	}

	p := sv.host.Config().Colors
	tsColor := tcell.GetColor(p.Label)
	textColor := tcell.GetColor(p.Text)

	sv.rowToIdx = make([]int, 1, len(sv.results)+1) // index 0 unused (header)
	idxToRow := make([]int, len(sv.results))

	row := 1
	for i, e := range sv.results {
		idxToRow[i] = row
		sv.rowToIdx = append(sv.rowToIdx, i)

		// Off: logEventPreview's short, single-line, first-line-only
		// summary. On: wrap the raw, un-truncated event message —
		// logEventPreview's 200-char/first-line-only cap exists for the
		// compact off case; wrap's whole purpose is to reveal more than
		// that, including a multi-line message's later lines (see
		// wrapMultilineText).
		var lines []string
		if sv.wrap {
			lines = wrapMultilineText(e.Message, previewWrapWidth, maxWrapLines)
		} else {
			lines = []string{logEventPreview(e.Message)}
		}

		sv.table.SetCell(row, 0, tview.NewTableCell(e.Timestamp.Local().Format("2006-01-02 15:04:05")).SetTextColor(tsColor).
			SetExpansion(logSearchColumns[0].expansion))
		sv.table.SetCell(row, 1, tview.NewTableCell(e.LogStream).SetTextColor(textColor).
			SetExpansion(logSearchColumns[1].expansion).SetMaxWidth(logSearchColumns[1].maxWidth))
		sv.table.SetCell(row, logSearchMessageColumn, tview.NewTableCell(lines[0]).SetTextColor(textColor).
			SetExpansion(logSearchColumns[logSearchMessageColumn].expansion))
		row++

		for _, extra := range lines[1:] {
			sv.rowToIdx = append(sv.rowToIdx, i)
			setContinuationRow(sv.table, row, len(logSearchColumns), logSearchMessageColumn, extra, textColor, logSearchColumns[logSearchMessageColumn].expansion)
			row++
		}
	}

	if selectedIdx >= 0 && selectedIdx < len(idxToRow) {
		sv.table.Select(idxToRow[selectedIdx], 0)
	}
}

// updateTitle never uses "[text]" — see queues.go's updateTitle for why
// (tview.Box titles run through the same tag-parsing Print() that Table
// cells do, silently swallowing square brackets).
func (sv *LogSearchView) updateTitle() {
	label := sv.tr.Label()
	title := fmt.Sprintf(" %s — %s — %d events", sv.logGroupName, label, len(sv.results))
	if sv.nextToken != "" {
		title += " (more available — press n to load more, or narrow your search)"
	}
	sv.table.SetTitle(title + " ")
}

func (sv *LogSearchView) showError(err error) {
	sv.results = nil
	sv.nextToken = ""
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
