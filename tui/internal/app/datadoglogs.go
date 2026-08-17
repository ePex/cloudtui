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

	"github.com/ePex/cloudtui/tui/internal/config"
	"github.com/ePex/cloudtui/tui/internal/datadoglogs"
	"github.com/ePex/cloudtui/tui/internal/ui"
)

// filterAnyOption is the sentinel option in the Service/Env filter
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
	envFilterDD     *tview.DropDown
	queryInput      *tview.InputField
	flex            *tview.Flex
	app             *App
	query           string
	serviceFilter   string
	envFilter       string
	// knownServices/knownEnvs accumulate every distinct value ever seen
	// across searches (not just the latest result set) — see
	// rebuildFilterOptions's doc comment for why.
	knownServices map[string]bool
	knownEnvs     map[string]bool
	tr            timeRange
	results       []datadoglogs.LogEvent
	hasMore       bool
}

var _ ui.View = (*datadogLogsView)(nil)
var _ ui.Shortcuttable = (*datadogLogsView)(nil)
var _ ui.Themeable = (*datadogLogsView)(nil)

// ApplyPalette recolors the Datadog Logs view for a live theme switch.
func (dv *datadogLogsView) ApplyPalette(p config.Palette) {
	bg := tcell.GetColor(p.Background)
	dv.table.SetBackgroundColor(bg)
	dv.table.SetBorderColor(tcell.GetColor(p.ViewColor("datadog-logs")))
	dv.table.SetTitleColor(tcell.GetColor(p.ViewColor("datadog-logs")))
	dv.queryInput.SetLabelColor(tcell.GetColor(p.Label))
	dv.queryInput.SetFieldBackgroundColor(tcell.GetColor(p.SelectionBg))
	dv.queryInput.SetFieldTextColor(tcell.GetColor(p.SelectionText))
	ui.StyleDropDown(dv.serviceFilterDD, p)
	ui.StyleDropDown(dv.envFilterDD, p)
}

func (dv *datadogLogsView) Name() string               { return "datadog-logs" }
func (dv *datadogLogsView) Title() string              { return "Datadog Logs" }
func (dv *datadogLogsView) Primitive() tview.Primitive { return dv.flex }

func (dv *datadogLogsView) Shortcuts() []ui.Shortcut {
	return []ui.Shortcut{
		{Key: "r", Description: "refresh"},
		{Key: "t", Description: "time range"},
		{Key: "/", Description: "query"},
		{Key: "S", Description: "filter service"},
		{Key: "E", Description: "filter env"},
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
	// Without this, unselected popup-list items are unreadable — same
	// gotcha already hit (and fixed via styleDropDown) for the theme and
	// connection-editor Backend dropdowns.
	ui.StyleDropDown(serviceFilterDD, p)

	envFilterDD := tview.NewDropDown()
	envFilterDD.SetLabel(" Env: ")
	envFilterDD.SetLabelColor(tcell.GetColor(p.Label))
	envFilterDD.SetFieldBackgroundColor(tcell.GetColor(p.SelectionBg))
	envFilterDD.SetFieldTextColor(tcell.GetColor(p.SelectionText))
	ui.StyleDropDown(envFilterDD, p)

	filterRow := tview.NewFlex().SetDirection(tview.FlexColumn).
		AddItem(serviceFilterDD, 0, 1, false).
		AddItem(envFilterDD, 0, 1, false)

	flex := tview.NewFlex().SetDirection(tview.FlexRow).
		AddItem(table, 0, 1, true).
		AddItem(filterRow, 1, 0, false).
		AddItem(queryInput, 1, 0, false)

	dv := &datadogLogsView{
		table:           table,
		serviceFilterDD: serviceFilterDD,
		envFilterDD:     envFilterDD,
		queryInput:      queryInput,
		flex:            flex,
		app:             a,
		tr:              timeRange{mode: timeRangeRelative, presetIdx: defaultPresetIdx},
		knownServices:   map[string]bool{},
		knownEnvs:       map[string]bool{},
	}
	dv.setHeader()
	// Seeded with just "(any)" until the first search discovers real
	// values — applyFilterOptions/rebuildFilterOptions take over from
	// there (see handleSearchResult).
	serviceFilterDD.SetOptions([]string{filterAnyOption}, nil)
	serviceFilterDD.SetCurrentOption(0)
	envFilterDD.SetOptions([]string{filterAnyOption}, nil)
	envFilterDD.SetCurrentOption(0)

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
	envFilterDD.SetInputCapture(backToTable)

	table.SetInputCapture(func(event *tcell.EventKey) *tcell.EventKey {
		switch event.Rune() {
		case 'r':
			dv.search()
			return nil
		case 't':
			a.timeRangeModal.show(dv.tr, func(tr timeRange) {
				dv.tr = tr
				dv.search()
			})
			return nil
		case '/':
			dv.queryInput.SetText(dv.query)
			dv.app.tv.SetFocus(dv.queryInput)
			return nil
		case 'S':
			dv.app.tv.SetFocus(dv.serviceFilterDD)
			return nil
		case 'E':
			dv.app.tv.SetFocus(dv.envFilterDD)
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
// valid: "search everything in range"). Also kicks off facet discovery
// (spec/52-fe-datadog-logs-facet-listing) — independent of the search
// above, so it never delays what the user actually sees first.
func (dv *datadogLogsView) Activate() {
	dv.search()
	dv.discoverFacetValues()
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

// effectiveQuery combines the Service/Env filters with the free-text
// query into the actual Datadog query string. Values are quoted for the
// same reason FE 41's CorrelationID fix quotes its value — Datadog's
// query tokenizer can split an unquoted term on internal punctuation.
func (dv *datadogLogsView) effectiveQuery() string {
	var parts []string
	if dv.serviceFilter != "" {
		parts = append(parts, fmt.Sprintf("service:%q", dv.serviceFilter))
	}
	if dv.envFilter != "" {
		parts = append(parts, fmt.Sprintf("env:%q", dv.envFilter))
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

// rebuildFilterOptions merges the distinct Service/Env values from the
// latest dv.results into the running knownServices/knownEnvs sets, then
// refreshes both dropdowns from those accumulated sets — not from
// dv.results alone. Found live: once a Service/Env filter is active,
// every subsequent search response only ever contains events matching
// that filter, so rebuilding purely from the latest results shrank the
// option list down to just the current selection + "(any)" on every
// search, making every other previously-seen value effectively
// unselectable without resetting to "(any)" first. Accumulating instead
// means a value discovered once stays offered regardless of how later
// searches narrow the actual results.
func (dv *datadogLogsView) rebuildFilterOptions() {
	for _, e := range dv.results {
		if e.Service != "" {
			dv.knownServices[e.Service] = true
		}
		if e.Env != "" {
			dv.knownEnvs[e.Env] = true
		}
	}
	dv.refreshFilterDropdowns()
}

// refreshFilterDropdowns rebuilds both dropdowns from the current
// knownServices/knownEnvs sets — split out from rebuildFilterOptions so
// facet discovery (see handleFacetDiscoveryResult) can refresh the
// dropdowns after merging in newly-discovered values without also
// re-scanning dv.results.
func (dv *datadogLogsView) refreshFilterDropdowns() {
	dv.applyFilterOptions(dv.serviceFilterDD, sortedKeys(dv.knownServices), &dv.serviceFilter, func(v string) {
		dv.serviceFilter = v
		dv.search()
		dv.app.tv.SetFocus(dv.table)
	})
	dv.applyFilterOptions(dv.envFilterDD, sortedKeys(dv.knownEnvs), &dv.envFilter, func(v string) {
		dv.envFilter = v
		dv.search()
		dv.app.tv.SetFocus(dv.table)
	})
}

// facetDiscoveryWindow is the fixed, wide time window facet discovery
// uses to ask Datadog what Service/Env values exist — independent of
// whatever time range the user has selected for search() (see
// spec/52-fe-datadog-logs-facet-listing).
const facetDiscoveryWindow = 30 * 24 * time.Hour

// discoverFacetValues asks Datadog for every distinct Service/Env value
// seen in the last facetDiscoveryWindow, so the dropdowns can offer
// values that haven't shown up in anything actually searched yet this
// session (rebuildFilterOptions alone only ever discovers what's been
// fetched). Two independent calls, one per facet.
func (dv *datadogLogsView) discoverFacetValues() {
	dv.discoverFacetValuesFor("service", dv.knownServices)
	dv.discoverFacetValuesFor("env", dv.knownEnvs)
}

// discoverFacetValuesFor runs a single facet-listing call in a goroutine
// and hands the outcome to handleFacetDiscoveryResult on the tview event
// loop — same shape as search/handleSearchResult.
func (dv *datadogLogsView) discoverFacetValuesFor(facet string, known map[string]bool) {
	cfg := dv.app.cfg.Datadog
	end := time.Now()
	start := end.Add(-facetDiscoveryWindow)
	go func() {
		values, err := dv.app.listDatadogFacetValues(context.Background(), cfg, facet, start, end)
		dv.app.tv.QueueUpdateDraw(func() {
			dv.handleFacetDiscoveryResult(known, values, err)
		})
	}()
}

// handleFacetDiscoveryResult merges values into known (mutating it in
// place, same as rebuildFilterOptions merges dv.results into
// knownServices/knownEnvs) and refreshes the dropdowns only if something
// new was actually found — avoids resetting the dropdowns' state on
// every activation once nothing new turns up, which is most of them in
// practice. Fails soft on error (logged, not surfaced to the user): this
// is a completeness enhancement, not the primary search functionality,
// and the accumulate-from-results mechanism still works regardless.
// Split out from discoverFacetValuesFor so it's directly testable
// without a goroutine or a running tview event loop, same rationale as
// handleSearchResult.
func (dv *datadogLogsView) handleFacetDiscoveryResult(known map[string]bool, values []string, err error) {
	if err != nil {
		slog.Warn("datadog logs: facet discovery failed", "error", err)
		return
	}
	changed := false
	for _, v := range values {
		if v != "" && !known[v] {
			known[v] = true
			changed = true
		}
	}
	if changed {
		dv.refreshFilterDropdowns()
	}
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
	start, end := dv.tr.bounds(time.Now())
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
	label := dv.tr.label()
	title := fmt.Sprintf(" Datadog Logs — %s — %d events", label, len(dv.results))
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
