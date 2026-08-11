package app

import (
	"context"
	"fmt"
	"log/slog"
	"sort"
	"strings"
	"time"

	"github.com/gdamore/tcell/v2"
	"github.com/rivo/tview"

	"github.com/ePex/cloudtui/tui/internal/datadoglogs"
	"github.com/ePex/cloudtui/tui/internal/ui"
)

// filterAnyOption is the sentinel option in the Service/Host filter
// dropdowns that means "no filter on this facet" — see
// spec/42-fe-datadog-logs-service-host-filters.
const filterAnyOption = "(any)"

// datadogLogsView is the Datadog Logs search screen: a registered
// top-level ui.View (unlike logSearchView, which is opened per
// CloudWatch log group) — Datadog Logs is one flat, taggable/queryable
// stream, not organized into named groups the way CloudWatch is, so
// there's no separate group-list step first (see
// spec/39-fe-datadog-logs decision 1).
type datadogLogsView struct {
	table           *tview.Table
	serviceFilterDD *tview.DropDown
	hostFilterDD    *tview.DropDown
	queryInput      *tview.InputField
	flex            *tview.Flex
	app             *App
	query           string
	serviceFilter   string
	hostFilter      string
	presetIdx       int
	results         []datadoglogs.LogEvent
	hasMore         bool
}

var _ ui.View = (*datadogLogsView)(nil)
var _ ui.Shortcuttable = (*datadogLogsView)(nil)

func (dv *datadogLogsView) Name() string               { return "datadog-logs" }
func (dv *datadogLogsView) Title() string              { return "Datadog Logs" }
func (dv *datadogLogsView) Primitive() tview.Primitive { return dv.flex }

func (dv *datadogLogsView) Shortcuts() []ui.Shortcut {
	return []ui.Shortcut{
		{Key: "r", Description: "refresh"},
		{Key: "t", Description: "time range"},
		{Key: "/", Description: "query"},
		{Key: "S", Description: "filter service"},
		{Key: "H", Description: "filter host"},
	}
}

// newDatadogLogsView constructs the Datadog Logs search view.
func newDatadogLogsView(a *App) *datadogLogsView {
	table := tview.NewTable()
	table.SetBorder(true).SetTitle(" Datadog Logs ")
	table.SetSelectable(true, false)
	table.SetFixed(1, 0)

	p := a.cfg.Colors
	queryInput := tview.NewInputField()
	queryInput.SetLabel(" / query: ")
	queryInput.SetLabelColor(tcell.GetColor(p.Label))
	queryInput.SetFieldBackgroundColor(tcell.GetColor(p.SelectionBg))
	queryInput.SetFieldTextColor(tcell.GetColor(p.SelectionText))

	serviceFilterDD := tview.NewDropDown()
	serviceFilterDD.SetLabel(" Service: ")
	serviceFilterDD.SetLabelColor(tcell.GetColor(p.Label))
	serviceFilterDD.SetFieldBackgroundColor(tcell.GetColor(p.SelectionBg))
	serviceFilterDD.SetFieldTextColor(tcell.GetColor(p.SelectionText))

	hostFilterDD := tview.NewDropDown()
	hostFilterDD.SetLabel(" Host: ")
	hostFilterDD.SetLabelColor(tcell.GetColor(p.Label))
	hostFilterDD.SetFieldBackgroundColor(tcell.GetColor(p.SelectionBg))
	hostFilterDD.SetFieldTextColor(tcell.GetColor(p.SelectionText))

	filterRow := tview.NewFlex().SetDirection(tview.FlexColumn).
		AddItem(serviceFilterDD, 0, 1, false).
		AddItem(hostFilterDD, 0, 1, false)

	flex := tview.NewFlex().SetDirection(tview.FlexRow).
		AddItem(table, 0, 1, true).
		AddItem(filterRow, 1, 0, false).
		AddItem(queryInput, 1, 0, false)

	dv := &datadogLogsView{
		table:           table,
		serviceFilterDD: serviceFilterDD,
		hostFilterDD:    hostFilterDD,
		queryInput:      queryInput,
		flex:            flex,
		app:             a,
		presetIdx:       defaultPresetIdx,
	}
	dv.setHeader()
	// Seeded with just "(any)" until the first search discovers real
	// values — applyFilterOptions/rebuildFilterOptions take over from
	// there (see handleSearchResult).
	serviceFilterDD.SetOptions([]string{filterAnyOption}, nil)
	serviceFilterDD.SetCurrentOption(0)
	hostFilterDD.SetOptions([]string{filterAnyOption}, nil)
	hostFilterDD.SetCurrentOption(0)

	// Unlike every filter input elsewhere in the app, typing here must
	// not trigger anything — each keystroke would otherwise be a real
	// Datadog API call. Only Enter (not Escape/Tab, which also reach
	// SetDoneFunc) runs the search — same convention as logSearchView.
	queryInput.SetDoneFunc(func(key tcell.Key) {
		if key == tcell.KeyEnter {
			dv.query = dv.queryInput.GetText()
			dv.search()
		}
		dv.app.tv.SetFocus(dv.table)
	})
	queryInput.SetInputCapture(func(event *tcell.EventKey) *tcell.EventKey {
		if event.Key() == tcell.KeyUp || event.Key() == tcell.KeyDown {
			dv.app.tv.SetFocus(dv.table)
			dv.table.InputHandler()(event, func(tview.Primitive) {})
			return nil
		}
		return event
	})

	// Unlike queryInput, arrow keys here are the dropdown's own list
	// navigation — only Esc is special-cased, to return focus to the
	// table.
	backToTable := func(event *tcell.EventKey) *tcell.EventKey {
		if event.Key() == tcell.KeyEscape {
			dv.app.tv.SetFocus(dv.table)
			return nil
		}
		return event
	}
	serviceFilterDD.SetInputCapture(backToTable)
	hostFilterDD.SetInputCapture(backToTable)

	table.SetInputCapture(func(event *tcell.EventKey) *tcell.EventKey {
		switch event.Rune() {
		case 'r':
			dv.search()
			return nil
		case 't':
			dv.cycleTimeRange()
			return nil
		case '/':
			dv.queryInput.SetText(dv.query)
			dv.app.tv.SetFocus(dv.queryInput)
			return nil
		case 'S':
			dv.app.tv.SetFocus(dv.serviceFilterDD)
			return nil
		case 'H':
			dv.app.tv.SetFocus(dv.hostFilterDD)
			return nil
		case 'j':
			return tcell.NewEventKey(tcell.KeyDown, 0, tcell.ModNone)
		case 'k':
			return tcell.NewEventKey(tcell.KeyUp, 0, tcell.ModNone)
		}
		return event
	})

	return dv
}

// Activate runs a search with the current query/time range. Called by
// switchTo each time the view becomes active, same as every other
// registered view's Activate — and matching logSearchView.open's
// behavior of always running a search immediately (an empty query is
// valid: "search everything in range").
func (dv *datadogLogsView) Activate() {
	dv.search()
}

func (dv *datadogLogsView) setHeader() {
	p := dv.app.cfg.Colors
	bg := tcell.GetColor(p.Label)
	fg := tcell.GetColor(p.Background)
	for i, label := range []string{"TIMESTAMP", "SERVICE", "STATUS", "MESSAGE"} {
		dv.table.SetCell(0, i,
			tview.NewTableCell(label).
				SetTextColor(fg).
				SetBackgroundColor(bg).
				SetSelectable(false).
				SetExpansion(1).
				SetAlign(tview.AlignCenter))
	}
}

func (dv *datadogLogsView) cycleTimeRange() {
	dv.presetIdx = (dv.presetIdx + 1) % len(timeRangePresets)
	dv.search()
}

// effectiveQuery combines the Service/Host filters with the free-text
// query into the actual Datadog query string. Values are quoted for the
// same reason FE 41's CorrelationID fix quotes its value — Datadog's
// query tokenizer can split an unquoted term on internal punctuation
// (e.g. a hostname like "ip-10-0-1-23").
func (dv *datadogLogsView) effectiveQuery() string {
	var parts []string
	if dv.serviceFilter != "" {
		parts = append(parts, fmt.Sprintf("service:%q", dv.serviceFilter))
	}
	if dv.hostFilter != "" {
		parts = append(parts, fmt.Sprintf("host:%q", dv.hostFilter))
	}
	if dv.query != "" {
		parts = append(parts, dv.query)
	}
	return strings.Join(parts, " ")
}

// applyFilterOptions rebuilds dd's option list from values (plus the
// leading "(any)" sentinel), preserving *current's selection if it's
// still among values, resetting to "(any)" (clearing *current) if not.
//
// The callback is cleared via SetOptions(..., nil) before
// SetCurrentOption: tview.DropDown.SetCurrentOption invokes the
// selected callback if one is set, and this function runs after every
// search to reconcile state — attaching onSelect first would make
// restoring an unchanged selection recursively call search() again.
// onSelect is only wired up (via SetSelectedFunc) once reconciliation
// is done, so it only fires for genuine user-driven selections.
func (dv *datadogLogsView) applyFilterOptions(dd *tview.DropDown, values []string, current *string, onSelect func(string)) {
	options := append([]string{filterAnyOption}, values...)
	idx := 0
	for i, v := range options {
		if v == *current {
			idx = i
			break
		}
	}
	if idx == 0 {
		*current = ""
	}
	dd.SetOptions(options, nil)
	dd.SetCurrentOption(idx)
	dd.SetSelectedFunc(func(text string, _ int) {
		if text == filterAnyOption {
			onSelect("")
			return
		}
		onSelect(text)
	})
}

// rebuildFilterOptions refreshes both filter dropdowns from the
// distinct Service/Host values actually present in dv.results, so the
// options offered always match what's on screen.
func (dv *datadogLogsView) rebuildFilterOptions() {
	serviceSet, hostSet := map[string]bool{}, map[string]bool{}
	for _, e := range dv.results {
		if e.Service != "" {
			serviceSet[e.Service] = true
		}
		if e.Host != "" {
			hostSet[e.Host] = true
		}
	}
	dv.applyFilterOptions(dv.serviceFilterDD, sortedKeys(serviceSet), &dv.serviceFilter, func(v string) {
		dv.serviceFilter = v
		dv.search()
	})
	dv.applyFilterOptions(dv.hostFilterDD, sortedKeys(hostSet), &dv.hostFilter, func(v string) {
		dv.hostFilter = v
		dv.search()
	})
}

// sortedKeys returns set's keys sorted ascending.
func sortedKeys(set map[string]bool) []string {
	keys := make([]string, 0, len(set))
	for k := range set {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	return keys
}

// search runs datadoglogs.Search in a goroutine (a real HTTP call) and
// hands the outcome to handleSearchResult on the tview event loop.
func (dv *datadogLogsView) search() {
	query := dv.effectiveQuery()
	end := time.Now()
	start := end.Add(-timeRangePresets[dv.presetIdx].duration)
	cfg := dv.app.cfg.Datadog
	go func() {
		events, hasMore, err := dv.app.searchDatadogLogs(context.Background(), cfg, query, start, end)
		dv.app.tv.QueueUpdateDraw(func() {
			dv.handleSearchResult(events, hasMore, err)
		})
	}()
}

// handleSearchResult processes the outcome of a Search call: on error,
// logs and shows it; on success, caches the results and repaints. Split
// out from search so this — the part with actual logic — is directly
// testable without spawning a goroutine or needing a running tview
// event loop (QueueUpdateDraw blocks forever without one).
func (dv *datadogLogsView) handleSearchResult(events []datadoglogs.LogEvent, hasMore bool, err error) {
	if err != nil {
		slog.Error("datadog logs: search failed", "error", err)
		dv.showError(err)
		return
	}
	dv.results = events
	dv.hasMore = hasMore
	dv.rebuildFilterOptions()
	dv.repaint()
}

func (dv *datadogLogsView) repaint() {
	for dv.table.GetRowCount() > 1 {
		dv.table.RemoveRow(dv.table.GetRowCount() - 1)
	}

	p := dv.app.cfg.Colors
	tsColor := tcell.GetColor(p.Label)
	textColor := tcell.GetColor(p.Text)
	for i, e := range dv.results {
		row := i + 1
		dv.table.SetCell(row, 0, tview.NewTableCell(e.Timestamp.Local().Format("2006-01-02 15:04:05")).SetTextColor(tsColor).SetExpansion(1))
		dv.table.SetCell(row, 1, tview.NewTableCell(e.Service).SetTextColor(textColor).SetExpansion(1))
		dv.table.SetCell(row, 2, tview.NewTableCell(e.Status).SetTextColor(textColor).SetExpansion(1))
		dv.table.SetCell(row, 3, tview.NewTableCell(logEventPreview(e.Message)).SetTextColor(textColor).SetExpansion(4))
	}

	if dv.table.GetRowCount() > 1 {
		dv.table.Select(1, 0)
		// Select alone isn't enough — see queues.go's repaint for why
		// SetOffset is also needed (tview.Table's "track end" auto-scroll
		// latches on during the table's first, still-empty draw).
		dv.table.SetOffset(0, 0)
	}

	dv.updateTitle()
}

// updateTitle never uses "[text]" — see queues.go's updateTitle for why
// (tview.Box titles run through the same tag-parsing Print() that Table
// cells do, silently swallowing square brackets).
func (dv *datadogLogsView) updateTitle() {
	preset := timeRangePresets[dv.presetIdx].label
	title := fmt.Sprintf(" Datadog Logs — %s — %d events", preset, len(dv.results))
	if dv.hasMore {
		title += " (more available — narrow your search)"
	}
	dv.table.SetTitle(title + " ")
}

func (dv *datadogLogsView) showError(err error) {
	dv.results = nil
	dv.hasMore = false
	for dv.table.GetRowCount() > 1 {
		dv.table.RemoveRow(dv.table.GetRowCount() - 1)
	}
	dv.table.SetCell(1, 0,
		tview.NewTableCell(fmt.Sprintf("Error: %v", err)).
			SetTextColor(tcell.ColorRed).
			SetExpansion(3),
	)
	dv.table.SetTitle(" Datadog Logs ")
}
