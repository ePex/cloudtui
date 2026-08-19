package view

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
	"github.com/ePex/cloudtui/tui/internal/dialog"
	"github.com/ePex/cloudtui/tui/internal/ui"
)

// filterAnyOption is the sentinel option in the Service/Env filter
// dropdowns that means "no filter on this facet" — see
// spec/42-fe-datadog-logs-service-host-filters.
const filterAnyOption = "(any)"

// DatadogLogsView is the Datadog Logs search screen: a registered
// top-level ui.View (unlike logSearchView, which is opened per
// CloudWatch log group) — Datadog Logs is one flat, taggable/queryable
// stream, not organized into named groups the way CloudWatch is, so
// there's no separate group-list step first (see
// spec/39-fe-datadog-logs decision 1).
type DatadogLogsView struct {
	table           *tview.Table
	serviceFilterDD *tview.DropDown
	envFilterDD     *tview.DropDown
	queryInput      *tview.InputField
	flex            *tview.Flex
	host            ui.ViewHost
	timeRangeModal  *dialog.TimeRangeModal
	query           string
	serviceFilter   string
	envFilter       string
	// knownServices/knownEnvs accumulate every distinct value ever seen
	// across searches (not just the latest result set) — see
	// rebuildFilterOptions's doc comment for why.
	knownServices map[string]bool
	knownEnvs     map[string]bool
	tr            ui.TimeRange
	results       []datadoglogs.LogEvent
	hasMore       bool
	wrap          bool  // message column word-wrap toggle
	rowToIdx      []int // row -> index into results; index 0 unused (header placeholder)
}

var _ ui.View = (*DatadogLogsView)(nil)
var _ ui.Shortcuttable = (*DatadogLogsView)(nil)
var _ ui.Themeable = (*DatadogLogsView)(nil)

// ApplyPalette recolors the Datadog Logs view for a live theme switch.
func (dv *DatadogLogsView) ApplyPalette(p config.Palette) {
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

func (dv *DatadogLogsView) Name() string               { return "datadog-logs" }
func (dv *DatadogLogsView) Title() string              { return "Datadog Logs" }
func (dv *DatadogLogsView) Primitive() tview.Primitive { return dv.flex }
func (dv *DatadogLogsView) Table() *tview.Table        { return dv.table }
func (dv *DatadogLogsView) FilterInputs() []tview.Primitive {
	return []tview.Primitive{dv.queryInput, dv.serviceFilterDD, dv.envFilterDD}
}

func (dv *DatadogLogsView) Shortcuts() []ui.Shortcut {
	wrap := "off"
	if dv.wrap {
		wrap = "on"
	}
	return []ui.Shortcut{
		{Key: "r", Description: "refresh"},
		{Key: "t", Description: "time range"},
		{Key: "/", Description: "query"},
		{Key: "S", Description: "filter service"},
		{Key: "E", Description: "filter env"},
		{Key: "w", Description: "wrap: " + wrap},
	}
}

// NewDatadogLogsView constructs the Datadog Logs search view.
func NewDatadogLogsView(a ui.ViewHost, timeRangeModal *dialog.TimeRangeModal, onSelect func(event datadoglogs.LogEvent)) *DatadogLogsView {
	table := tview.NewTable()
	table.SetBorder(true).SetTitle(" Datadog Logs ")
	table.SetSelectable(true, false)
	table.SetFixed(1, 0)

	p := a.Config().Colors
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

	dv := &DatadogLogsView{
		table:           table,
		serviceFilterDD: serviceFilterDD,
		envFilterDD:     envFilterDD,
		queryInput:      queryInput,
		flex:            flex,
		host:            a,
		timeRangeModal:  timeRangeModal,
		tr:              ui.TimeRange{Mode: ui.TimeRangeRelative, PresetIdx: ui.DefaultPresetIdx},
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
		dv.host.SetFocus(dv.table)
	})
	queryInput.SetInputCapture(func(event *tcell.EventKey) *tcell.EventKey {
		if event.Key() == tcell.KeyUp || event.Key() == tcell.KeyDown {
			dv.host.SetFocus(dv.table)
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
			dv.host.SetFocus(dv.table)
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
			timeRangeModal.Show(dv.tr, func(tr ui.TimeRange) {
				dv.tr = tr
				dv.search()
			})
			return nil
		case '/':
			dv.queryInput.SetText(dv.query)
			dv.host.SetFocus(dv.queryInput)
			return nil
		case 'S':
			dv.host.SetFocus(dv.serviceFilterDD)
			return nil
		case 'E':
			dv.host.SetFocus(dv.envFilterDD)
			return nil
		case 'w':
			dv.wrap = !dv.wrap
			dv.renderRows()
			lines := make([]string, 0, len(dv.Shortcuts()))
			for _, sc := range dv.Shortcuts() {
				lines = append(lines, fmt.Sprintf("[%s]<%s>[-] %s", a.Config().Colors.Accent, sc.Key, sc.Description))
			}
			a.SetContextHint(strings.Join(lines, "\n"))
			return nil
		case 'j':
			return tcell.NewEventKey(tcell.KeyDown, 0, tcell.ModNone)
		case 'k':
			return tcell.NewEventKey(tcell.KeyUp, 0, tcell.ModNone)
		}
		return event
	})

	table.SetSelectedFunc(func(row, _ int) {
		if row <= 0 || row >= len(dv.rowToIdx) {
			return
		}
		idx := dv.rowToIdx[row]
		if idx < 0 || idx >= len(dv.results) {
			return
		}
		onSelect(dv.results[idx])
	})

	return dv
}

// Activate runs a search with the current query/time range. Called by
// SwitchTo each time the view becomes active, same as every other
// registered view's Activate — and matching logSearchView.open's
// behavior of always running a search immediately (an empty query is
// valid: "search everything in range"). Also kicks off facet discovery
// (spec/52-fe-datadog-logs-facet-listing) — independent of the search
// above, so it never delays what the user actually sees first.
func (dv *DatadogLogsView) Activate() {
	dv.search()
	dv.discoverFacetValues()
}

// datadogLogsColumns are the results table's columns — see
// messageColumns' doc comment in messages.go for why header and data
// cells share this instead of each setting their own Expansion.
// SERVICE's MaxWidth caps a service name from eating space MESSAGE —
// the actually important column, found live to be visibly too small
// with 3 narrow columns competing for it — needs far more of.
var datadogLogsColumns = []struct {
	label     string
	expansion int
	maxWidth  int // 0 = uncapped
}{
	{"TIMESTAMP", 1, 0},
	{"SERVICE", 1, 20},
	{"STATUS", 1, 0},
	{"MESSAGE", 12, 0},
}

const datadogLogsMessageColumn = 3 // index into datadogLogsColumns

func (dv *DatadogLogsView) setHeader() {
	p := dv.host.Config().Colors
	bg := tcell.GetColor(p.Label)
	fg := tcell.GetColor(p.Background)
	for i, col := range datadogLogsColumns {
		dv.table.SetCell(0, i,
			tview.NewTableCell(col.label).
				SetTextColor(fg).
				SetBackgroundColor(bg).
				SetSelectable(false).
				SetExpansion(col.expansion).
				SetAlign(tview.AlignCenter))
	}
}

// effectiveQuery combines the Service/Env filters with the free-text
// query into the actual Datadog query string. Values are quoted for the
// same reason FE 41's CorrelationID fix quotes its value — Datadog's
// query tokenizer can split an unquoted term on internal punctuation.
func (dv *DatadogLogsView) effectiveQuery() string {
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
func (dv *DatadogLogsView) applyFilterOptions(dd *tview.DropDown, values []string, current *string, onSelect func(string)) {
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
func (dv *DatadogLogsView) rebuildFilterOptions() {
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
func (dv *DatadogLogsView) refreshFilterDropdowns() {
	dv.applyFilterOptions(dv.serviceFilterDD, sortedKeys(dv.knownServices), &dv.serviceFilter, func(v string) {
		dv.serviceFilter = v
		dv.search()
		dv.host.SetFocus(dv.table)
	})
	dv.applyFilterOptions(dv.envFilterDD, sortedKeys(dv.knownEnvs), &dv.envFilter, func(v string) {
		dv.envFilter = v
		dv.search()
		dv.host.SetFocus(dv.table)
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
func (dv *DatadogLogsView) discoverFacetValues() {
	dv.discoverFacetValuesFor("service", dv.knownServices)
	dv.discoverFacetValuesFor("env", dv.knownEnvs)
}

// discoverFacetValuesFor runs a single facet-listing call in a goroutine
// and hands the outcome to handleFacetDiscoveryResult on the tview event
// loop — same shape as search/handleSearchResult.
func (dv *DatadogLogsView) discoverFacetValuesFor(facet string, known map[string]bool) {
	cfg := dv.host.Config().Datadog
	end := time.Now()
	start := end.Add(-facetDiscoveryWindow)
	go func() {
		values, err := dv.host.ListDatadogFacetValues(context.Background(), cfg, facet, start, end)
		dv.host.QueueUpdateDraw(func() {
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
func (dv *DatadogLogsView) handleFacetDiscoveryResult(known map[string]bool, values []string, err error) {
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
func (dv *DatadogLogsView) search() {
	query := dv.effectiveQuery()
	start, end := dv.tr.Bounds(time.Now())
	cfg := dv.host.Config().Datadog
	go func() {
		events, hasMore, err := dv.host.SearchDatadogLogs(context.Background(), cfg, query, start, end)
		dv.host.QueueUpdateDraw(func() {
			dv.handleSearchResult(events, hasMore, err)
		})
	}()
}

// handleSearchResult processes the outcome of a Search call: on error,
// logs and shows it; on success, caches the results and repaints. Split
// out from search so this — the part with actual logic — is directly
// testable without spawning a goroutine or needing a running tview
// event loop (QueueUpdateDraw blocks forever without one).
func (dv *DatadogLogsView) handleSearchResult(events []datadoglogs.LogEvent, hasMore bool, err error) {
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

func (dv *DatadogLogsView) repaint() {
	dv.renderRows()

	if dv.table.GetRowCount() > 1 {
		dv.table.Select(1, 0)
		// Select alone isn't enough — see queues.go's repaint for why
		// SetOffset is also needed (tview.Table's "track end" auto-scroll
		// latches on during the table's first, still-empty draw).
		dv.table.SetOffset(0, 0)
	}

	dv.updateTitle()
}

// renderRows rebuilds the table body from dv.results, wrap-aware,
// without resetting scroll position — unlike repaint (which always
// jumps to row 1, matching a genuine reload), this is also called
// directly by the wrap toggle, which has no reason to reset the user's
// place in the results. Preserves the currently-selected event across
// the rebuild (by index, not identity — dv.results's order/set doesn't
// change from a call to this function alone), since row numbers shift
// once wrapping changes how many rows an item spans.
func (dv *DatadogLogsView) renderRows() {
	selectedIdx := -1
	if row, _ := dv.table.GetSelection(); row > 0 && row < len(dv.rowToIdx) {
		selectedIdx = dv.rowToIdx[row]
	}

	for dv.table.GetRowCount() > 1 {
		dv.table.RemoveRow(dv.table.GetRowCount() - 1)
	}

	p := dv.host.Config().Colors
	tsColor := tcell.GetColor(p.Label)
	textColor := tcell.GetColor(p.Text)

	dv.rowToIdx = make([]int, 1, len(dv.results)+1) // index 0 unused (header)
	idxToRow := make([]int, len(dv.results))

	row := 1
	for i, e := range dv.results {
		idxToRow[i] = row
		dv.rowToIdx = append(dv.rowToIdx, i)

		preview := logEventPreview(e.Message)
		lines := []string{preview}
		if dv.wrap {
			lines = wrapText(preview, previewWrapWidth)
		}

		dv.table.SetCell(row, 0, tview.NewTableCell(e.Timestamp.Local().Format("2006-01-02 15:04:05")).SetTextColor(tsColor).
			SetExpansion(datadogLogsColumns[0].expansion))
		dv.table.SetCell(row, 1, tview.NewTableCell(e.Service).SetTextColor(textColor).
			SetExpansion(datadogLogsColumns[1].expansion).SetMaxWidth(datadogLogsColumns[1].maxWidth))
		dv.table.SetCell(row, 2, tview.NewTableCell(e.Status).SetTextColor(textColor).
			SetExpansion(datadogLogsColumns[2].expansion))
		dv.table.SetCell(row, datadogLogsMessageColumn, tview.NewTableCell(lines[0]).SetTextColor(textColor).
			SetExpansion(datadogLogsColumns[datadogLogsMessageColumn].expansion))
		row++

		for _, extra := range lines[1:] {
			dv.rowToIdx = append(dv.rowToIdx, i)
			setContinuationRow(dv.table, row, len(datadogLogsColumns), datadogLogsMessageColumn, extra, textColor, datadogLogsColumns[datadogLogsMessageColumn].expansion)
			row++
		}
	}

	if selectedIdx >= 0 && selectedIdx < len(idxToRow) {
		dv.table.Select(idxToRow[selectedIdx], 0)
	}
}

// updateTitle never uses "[text]" — see queues.go's updateTitle for why
// (tview.Box titles run through the same tag-parsing Print() that Table
// cells do, silently swallowing square brackets).
func (dv *DatadogLogsView) updateTitle() {
	label := dv.tr.Label()
	title := fmt.Sprintf(" Datadog Logs — %s — %d events", label, len(dv.results))
	if dv.hasMore {
		title += " (more available — narrow your search)"
	}
	dv.table.SetTitle(title + " ")
}

func (dv *DatadogLogsView) showError(err error) {
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
