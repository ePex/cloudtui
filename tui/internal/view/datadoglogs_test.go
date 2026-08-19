package view

import (
	"context"
	"fmt"
	"strings"
	"testing"
	"time"

	"github.com/gdamore/tcell/v2"
	"github.com/rivo/tview"

	"github.com/ePex/cloudtui/tui/internal/datadoglogs"
	"github.com/ePex/cloudtui/tui/internal/dialog"
	"github.com/ePex/cloudtui/tui/internal/ui"
)

func newTestDatadogLogsView(t *testing.T) (*fakeViewHost, *dialog.TimeRangeModal, *DatadogLogsView) {
	t.Helper()
	host := newFakeViewHost()
	timeRangeModal := dialog.NewTimeRangeModal(host)
	return host, timeRangeModal, NewDatadogLogsView(host, timeRangeModal, func(datadoglogs.LogEvent) {})
}

func TestEffectiveQuery(t *testing.T) {
	cases := []struct {
		name          string
		serviceFilter string
		envFilter     string
		query         string
		want          string
	}{
		{"no filters, just free text", "", "", "error", "error"},
		{"service only", "bar-proxy", "", "", `service:"bar-proxy"`},
		{"env only", "", "prod", "", `env:"prod"`},
		{"service and env and free text", "bar-proxy", "prod", "error", `service:"bar-proxy" env:"prod" error`},
		{"nothing set", "", "", "", ""},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			_, _, dv := newTestDatadogLogsView(t)
			dv.serviceFilter = c.serviceFilter
			dv.envFilter = c.envFilter
			dv.query = c.query

			if got := dv.effectiveQuery(); got != c.want {
				t.Errorf("effectiveQuery() = %q, want %q", got, c.want)
			}
		})
	}
}

func TestApplyFilterOptionsPreservesSelectionWhenStillPresent(t *testing.T) {
	_, _, dv := newTestDatadogLogsView(t)
	current := "svc-a"

	dv.applyFilterOptions(dv.serviceFilterDD, []string{"svc-a", "svc-b"}, &current, func(string) {})

	if current != "svc-a" {
		t.Errorf("current = %q, want preserved %q", current, "svc-a")
	}
	idx, text := dv.serviceFilterDD.GetCurrentOption()
	if idx != 1 || text != "svc-a" {
		t.Errorf("selected option = (%d, %q), want (1, %q)", idx, text, "svc-a")
	}
}

func TestApplyFilterOptionsResetsSelectionWhenNoLongerPresent(t *testing.T) {
	_, _, dv := newTestDatadogLogsView(t)
	current := "svc-gone"

	dv.applyFilterOptions(dv.serviceFilterDD, []string{"svc-a", "svc-b"}, &current, func(string) {})

	if current != "" {
		t.Errorf("current = %q, want cleared", current)
	}
	idx, text := dv.serviceFilterDD.GetCurrentOption()
	if idx != 0 || text != filterAnyOption {
		t.Errorf("selected option = (%d, %q), want (0, %q)", idx, text, filterAnyOption)
	}
}

func TestApplyFilterOptionsOptionCountIncludesAnySentinel(t *testing.T) {
	_, _, dv := newTestDatadogLogsView(t)
	current := ""

	dv.applyFilterOptions(dv.serviceFilterDD, []string{"svc-a", "svc-b"}, &current, func(string) {})

	if got := dv.serviceFilterDD.GetOptionCount(); got != 3 { // "(any)" + 2 values
		t.Errorf("option count = %d, want 3", got)
	}
}

// TestRebuildFilterOptionsAccumulatesAcrossNarrowedSearches guards
// against a real bug found live: once a Service/Env filter is active,
// every subsequent search response only contains events matching that
// filter, so rebuilding the dropdown purely from the latest results
// shrank the option list down to just the current selection + "(any)"
// on every search — every other previously-seen value became
// unselectable without resetting to "(any)" first. Values must
// accumulate across searches instead of being replaced each time.
func TestRebuildFilterOptionsAccumulatesAcrossNarrowedSearches(t *testing.T) {
	_, _, dv := newTestDatadogLogsView(t)

	// First (unfiltered) search discovers two services.
	dv.results = []datadoglogs.LogEvent{
		{Service: "activemq", Env: "prod"},
		{Service: "bar-proxy", Env: "testt"},
	}
	dv.rebuildFilterOptions()
	if got := dv.serviceFilterDD.GetOptionCount(); got != 3 { // "(any)" + 2
		t.Fatalf("after first search: option count = %d, want 3", got)
	}

	// Second search, now filtered to just "activemq" — Datadog would
	// only return matching events, so dv.results narrows accordingly.
	dv.serviceFilter = "activemq"
	dv.results = []datadoglogs.LogEvent{
		{Service: "activemq", Env: "prod"},
	}
	dv.rebuildFilterOptions()

	if got := dv.serviceFilterDD.GetOptionCount(); got != 3 { // "(any)" + 2, unchanged
		t.Errorf("after filtered search: option count = %d, want 3 (bar-proxy must still be offered)", got)
	}
	if got := dv.envFilterDD.GetOptionCount(); got != 3 { // "(any)" + prod + testt
		t.Errorf("after filtered search: env option count = %d, want 3 (testt must still be offered)", got)
	}
}

// TestRebuildFilterOptionsSelectingAnOptionRefocusesTable confirms
// picking a value from either filter dropdown returns focus to the
// results table, not left sitting on the dropdown — the onSelect
// closures wired in rebuildFilterOptions call search() then
// SetFocus(dv.table) synchronously, so this is safe to assert on
// immediately (same reasoning as this file's other tests that only
// check state set before search()'s goroutine is spawned).
func TestRebuildFilterOptionsSelectingAnOptionRefocusesTable(t *testing.T) {
	host, _, dv := newTestDatadogLogsView(t)
	dv.results = []datadoglogs.LogEvent{{Service: "activemq"}}
	dv.rebuildFilterOptions()
	host.SetFocus(dv.serviceFilterDD)

	// Simulate picking "activemq" (options: 0="(any)", 1="activemq").
	dv.serviceFilterDD.SetCurrentOption(1)

	if got := host.focused; got != dv.table {
		t.Errorf("focus after selecting a Service option = %v, want the results table", got)
	}
}

// TestApplyFilterOptionsDoesNotFireCallbackDuringReconciliation guards
// against the recursion risk documented in spec/42's plan.md:
// tview.DropDown.SetCurrentOption invokes the selected callback if one
// is set, so restoring the current selection after a rebuild must not
// itself fire onSelect (which would recursively call search()).
// Deliberately doesn't go through the real search()/goroutine path —
// this checks a synchronous side effect (a plain bool set by a test
// callback passed directly to applyFilterOptions), avoiding the race
// an async fake-and-counter approach would have against a background
// goroutine (same discipline as this file's other tests that only
// assert on state mutated before search() is ever called).
func TestApplyFilterOptionsDoesNotFireCallbackDuringReconciliation(t *testing.T) {
	_, _, dv := newTestDatadogLogsView(t)
	current := "svc-a"
	fired := false

	dv.applyFilterOptions(dv.serviceFilterDD, []string{"svc-a", "svc-b"}, &current, func(string) {
		fired = true
	})

	if fired {
		t.Error("onSelect fired during applyFilterOptions itself — SetCurrentOption must not invoke the callback while reconciling")
	}
}

// TestApplyFilterOptionsCallbackFiresOnSubsequentSelection confirms the
// other half of that same guard: a genuine selection made *after*
// applyFilterOptions has returned (simulating the user picking an
// option, the same way tview's internal list selection does) still
// reaches onSelect — the recursion guard above must not have also
// disabled real selection handling.
func TestApplyFilterOptionsCallbackFiresOnSubsequentSelection(t *testing.T) {
	_, _, dv := newTestDatadogLogsView(t)
	current := ""
	var selected string
	fireCount := 0

	dv.applyFilterOptions(dv.serviceFilterDD, []string{"svc-a", "svc-b"}, &current, func(v string) {
		selected = v
		fireCount++
	})

	// Simulate picking "svc-b" (options: 0="(any)", 1="svc-a", 2="svc-b").
	dv.serviceFilterDD.SetCurrentOption(2)

	if fireCount != 1 {
		t.Fatalf("onSelect fired %d times after a selection, want 1", fireCount)
	}
	if selected != "svc-b" {
		t.Errorf("selected = %q, want %q", selected, "svc-b")
	}
}

func TestDatadogLogsViewShortcutsIncludeServiceAndEnvFilters(t *testing.T) {
	_, _, dv := newTestDatadogLogsView(t)
	var haveS, haveE bool
	for _, sc := range dv.Shortcuts() {
		if sc.Key == "S" {
			haveS = true
		}
		if sc.Key == "E" {
			haveE = true
		}
	}
	if !haveS || !haveE {
		t.Errorf("Shortcuts() missing S/E filter shortcuts (haveS=%v, haveE=%v)", haveS, haveE)
	}
}

func TestDatadogLogsViewWrapShortcutPresent(t *testing.T) {
	_, _, dv := newTestDatadogLogsView(t)
	for _, s := range dv.Shortcuts() {
		if s.Key == "W" {
			return
		}
	}
	t.Error("Shortcuts() missing key \"W\"")
}

func TestDatadogLogsViewWrapTogglesAtBottomEdge(t *testing.T) {
	_, _, dv := newTestDatadogLogsView(t)
	dv.handleSearchResult([]datadoglogs.LogEvent{{Message: "a"}, {Message: "b"}, {Message: "c"}}, false, nil)
	dv.table.Select(3, 0) // last result row

	capture := dv.table.GetInputCapture()
	capture(tcell.NewEventKey(tcell.KeyRune, 'W', tcell.ModNone))
	capture(tcell.NewEventKey(tcell.KeyRune, 'j', tcell.ModNone))

	if row, _ := dv.table.GetSelection(); row != 1 {
		t.Errorf("selection after wrap = %d, want 1 (wrapped to first result row)", row)
	}
}

func TestDatadogLogsViewNameAndTitle(t *testing.T) {
	_, _, dv := newTestDatadogLogsView(t)
	if got := dv.Name(); got != "datadog-logs" {
		t.Errorf("Name() = %q, want %q", got, "datadog-logs")
	}
	if got := dv.Title(); got != "Datadog Logs" {
		t.Errorf("Title() = %q, want %q", got, "Datadog Logs")
	}
}

func TestDatadogLogsViewHeaderLabels(t *testing.T) {
	_, _, dv := newTestDatadogLogsView(t)
	want := []string{"TIMESTAMP", "SERVICE", "STATUS", "MESSAGE"}
	for col, label := range want {
		cell := dv.table.GetCell(0, col)
		if cell == nil {
			t.Fatalf("header cell at column %d is nil", col)
		}
		if got := cell.Text; got != label {
			t.Errorf("header col %d = %q, want %q", col, got, label)
		}
	}
}

// TestDatadogLogsViewTKeyOpensTimeRangeModal covers spec/53: 't' now opens
// the shared time range modal (prefilled from dv.tr) instead of cycling
// presets directly, and the modal's onApply callback writes the result
// back into dv.tr. Applying spawns search()'s goroutine (no synchronous
// "not configured" guard at the view layer — that check lives inside
// datadoglogs.Search itself), which blocks forever on QueueUpdateDraw
// without a running tview event loop (same reasoning as
// logSearchView/ssmParamsView's tests), but dv.tr is mutated synchronously
// before search() is even called, so it's still safe to assert on here.
func TestDatadogLogsViewTKeyOpensTimeRangeModal(t *testing.T) {
	host, timeRangeModal, dv := newTestDatadogLogsView(t)
	dv.tr = ui.TimeRange{Mode: ui.TimeRangeRelative, PresetIdx: 2}

	capture := dv.table.GetInputCapture()
	if got := capture(tcell.NewEventKey(tcell.KeyRune, 't', tcell.ModNone)); got != nil {
		t.Errorf("'t' capture returned %v, want nil (event consumed)", got)
	}
	if !timeRangeModal.Visible() {
		t.Fatal("'t' did not open the time range modal")
	}

	// The modal's internals (relativeList, onApply) live in internal/dialog
	// and aren't reachable from here — drive the real UI path instead:
	// Show() focuses the relative list (asserting via the focused
	// primitive's own exported API, not a dialog-package field), prefilled
	// to dv.tr's preset; selecting a different preset and pressing Enter
	// exercises the actual applyRelative -> onApply -> dv.tr write-back
	// path a real keypress would.
	list, ok := host.focused.(*tview.List)
	if !ok {
		t.Fatalf("focus after 't' = %T, want *tview.List", host.focused)
	}
	if got := list.GetCurrentItem(); got != 2 {
		t.Errorf("relative list current item = %d, want 2 (dv.tr's preset)", got)
	}

	list.SetCurrentItem(4)
	list.InputHandler()(tcell.NewEventKey(tcell.KeyEnter, 0, tcell.ModNone), func(tview.Primitive) {})

	if want := (ui.TimeRange{Mode: ui.TimeRangeRelative, PresetIdx: 4}); dv.tr != want {
		t.Errorf("dv.tr = %+v, want %+v after applying from the modal", dv.tr, want)
	}
}

// TestDatadogLogsViewQueryInputTypingDoesNotSearch: queryInput has no
// SetChangedFunc wired (only SetDoneFunc for Enter), so typing alone
// has no side effects — checked here via dv.query staying empty.
func TestDatadogLogsViewQueryInputTypingDoesNotSearch(t *testing.T) {
	_, _, dv := newTestDatadogLogsView(t)

	dv.queryInput.SetText("env:testt service:bar-proxy")

	if dv.query != "" {
		t.Errorf("query = %q, want empty: typing alone must not set query", dv.query)
	}
}

// TestDatadogLogsViewQueryInputEnterSetsQuery only asserts on dv.query,
// which SetDoneFunc sets synchronously before calling search() — so this
// stays deterministic despite search()'s goroutine never completing
// under the test's non-running tview event loop (same reasoning as
// TestDatadogLogsViewCycleTimeRange above).
func TestDatadogLogsViewQueryInputEnterSetsQuery(t *testing.T) {
	_, _, dv := newTestDatadogLogsView(t)
	dv.queryInput.SetText("env:testt service:bar-proxy")

	dv.queryInput.InputHandler()(tcell.NewEventKey(tcell.KeyEnter, 0, tcell.ModNone), func(tview.Primitive) {})

	if dv.query != "env:testt service:bar-proxy" {
		t.Errorf("query = %q, want %q after Enter", dv.query, "env:testt service:bar-proxy")
	}
}

func TestDatadogLogsViewHandleSearchResult(t *testing.T) {
	t.Run("success populates rows and title", func(t *testing.T) {
		_, _, dv := newTestDatadogLogsView(t)
		ts := time.Date(2026, 1, 2, 3, 4, 5, 0, time.UTC)

		dv.handleSearchResult([]datadoglogs.LogEvent{
			{Timestamp: ts, Service: "bar-proxy", Status: "error", Message: "hello"},
		}, false, nil)

		if got := dv.table.GetRowCount(); got != 2 { // header + 1
			t.Fatalf("row count = %d, want 2", got)
		}
		if got := dv.table.GetCell(1, 1).Text; got != "bar-proxy" {
			t.Errorf("service cell = %q, want %q", got, "bar-proxy")
		}
		if got := dv.table.GetCell(1, 2).Text; got != "error" {
			t.Errorf("status cell = %q, want %q", got, "error")
		}
		if got := dv.table.GetCell(1, 3).Text; got != "hello" {
			t.Errorf("message cell = %q, want %q", got, "hello")
		}
		if got := dv.table.GetTitle(); !strings.Contains(got, "1 events") {
			t.Errorf("title = %q, want it to contain the event count", got)
		}
		if strings.Contains(dv.table.GetTitle(), "more available") {
			t.Errorf("title = %q, want no hasMore indicator", dv.table.GetTitle())
		}
	})

	t.Run("hasMore is reflected in the title", func(t *testing.T) {
		_, _, dv := newTestDatadogLogsView(t)

		dv.handleSearchResult([]datadoglogs.LogEvent{{Message: "x"}}, true, nil)

		if !strings.Contains(dv.table.GetTitle(), "more available") {
			t.Errorf("title = %q, want it to mention more results are available", dv.table.GetTitle())
		}
	})

	t.Run("error logs and shows status, does not touch results", func(t *testing.T) {
		_, _, dv := newTestDatadogLogsView(t)
		dv.results = []datadoglogs.LogEvent{{Message: "stale"}}

		dv.handleSearchResult(nil, false, context.DeadlineExceeded)

		if len(dv.results) != 0 {
			t.Errorf("results = %+v, want cleared after an error", dv.results)
		}
		if got := dv.table.GetCell(1, 0).Text; !strings.Contains(got, "deadline exceeded") {
			t.Errorf("error cell = %q, want it to contain the error", got)
		}
	})
}

// TestDatadogLogsViewScrollsToTopWithManyRows guards against the same
// bug fixed for queuesView (spec/11-bugfix-queues-scroll-to-top).
func TestDatadogLogsViewScrollsToTopWithManyRows(t *testing.T) {
	_, _, dv := newTestDatadogLogsView(t)
	dv.table.SetRect(0, 0, 60, 15) // fewer visible rows than events below

	screen := tcell.NewSimulationScreen("")
	if err := screen.Init(); err != nil {
		t.Fatalf("screen.Init: %v", err)
	}
	screen.SetSize(60, 15)
	dv.table.Draw(screen)

	events := make([]datadoglogs.LogEvent, 50)
	for i := range events {
		events[i] = datadoglogs.LogEvent{Message: fmt.Sprintf("event-%02d", i)}
	}
	dv.handleSearchResult(events, false, nil)

	dv.table.Draw(screen)

	if row, _ := dv.table.GetOffset(); row != 0 {
		t.Errorf("table scrolled away from top: rowOffset = %d, want 0", row)
	}
}

// TestHandleFacetDiscoveryResultMergesNewValues confirms newly-discovered
// values are merged into known and reflected in the dropdown, alongside
// whatever rebuildFilterOptions had already accumulated from search
// results — the two merge paths (spec/52) must compose.
func TestHandleFacetDiscoveryResultMergesNewValues(t *testing.T) {
	_, _, dv := newTestDatadogLogsView(t)
	dv.results = []datadoglogs.LogEvent{{Service: "activemq"}}
	dv.rebuildFilterOptions()
	if got := dv.serviceFilterDD.GetOptionCount(); got != 2 { // "(any)" + activemq
		t.Fatalf("before discovery: option count = %d, want 2", got)
	}

	dv.handleFacetDiscoveryResult(dv.knownServices, []string{"activemq", "bar-proxy"}, nil)

	if !dv.knownServices["bar-proxy"] {
		t.Error("knownServices does not contain the newly-discovered bar-proxy")
	}
	if got := dv.serviceFilterDD.GetOptionCount(); got != 3 { // "(any)" + activemq + bar-proxy
		t.Errorf("after discovery: option count = %d, want 3", got)
	}
}

// TestHandleFacetDiscoveryResultNoopOnError confirms a failed discovery
// call leaves known and the dropdown untouched — fails soft, per spec.
func TestHandleFacetDiscoveryResultNoopOnError(t *testing.T) {
	_, _, dv := newTestDatadogLogsView(t)
	dv.results = []datadoglogs.LogEvent{{Service: "activemq"}}
	dv.rebuildFilterOptions()
	before := dv.serviceFilterDD.GetOptionCount()

	dv.handleFacetDiscoveryResult(dv.knownServices, nil, context.DeadlineExceeded)

	if len(dv.knownServices) != 1 {
		t.Errorf("knownServices = %v, want unchanged (len 1)", dv.knownServices)
	}
	if got := dv.serviceFilterDD.GetOptionCount(); got != before {
		t.Errorf("option count = %d, want unchanged at %d", got, before)
	}
}

// TestHandleFacetDiscoveryResultSkipsEmptyValues confirms an empty
// string in the discovered values (shouldn't happen given
// ListFacetValues already filters these, but defensive here too) never
// ends up in known.
func TestHandleFacetDiscoveryResultSkipsEmptyValues(t *testing.T) {
	_, _, dv := newTestDatadogLogsView(t)

	dv.handleFacetDiscoveryResult(dv.knownServices, []string{"", "activemq", ""}, nil)

	if len(dv.knownServices) != 1 || !dv.knownServices["activemq"] {
		t.Errorf("knownServices = %v, want just {activemq: true}", dv.knownServices)
	}
}

// TestHandleFacetDiscoveryResultPreservesCurrentSelectionWhenValuesArrive
// confirms a Service filter picked before discovery finishes isn't
// clobbered when discovery's result lands and rebuilds the dropdown —
// exercises applyFilterOptions's existing selection-preservation logic
// through the new discovery call path.
func TestHandleFacetDiscoveryResultPreservesCurrentSelectionWhenValuesArrive(t *testing.T) {
	_, _, dv := newTestDatadogLogsView(t)
	dv.results = []datadoglogs.LogEvent{{Service: "activemq"}}
	dv.rebuildFilterOptions()
	dv.serviceFilter = "activemq"
	dv.serviceFilterDD.SetCurrentOption(1)

	dv.handleFacetDiscoveryResult(dv.knownServices, []string{"activemq", "bar-proxy"}, nil)

	if dv.serviceFilter != "activemq" {
		t.Errorf("serviceFilter = %q, want %q (unchanged)", dv.serviceFilter, "activemq")
	}
	_, selected := dv.serviceFilterDD.GetCurrentOption()
	if selected != "activemq" {
		t.Errorf("dropdown's selected option = %q, want %q", selected, "activemq")
	}
}
