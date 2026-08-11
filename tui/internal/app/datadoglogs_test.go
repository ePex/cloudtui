package app

import (
	"context"
	"fmt"
	"strings"
	"testing"
	"time"

	"github.com/gdamore/tcell/v2"
	"github.com/rivo/tview"

	"github.com/ePex/cloudtui/tui/internal/config"
	"github.com/ePex/cloudtui/tui/internal/datadoglogs"
)

func TestEffectiveQuery(t *testing.T) {
	cases := []struct {
		name          string
		serviceFilter string
		hostFilter    string
		query         string
		want          string
	}{
		{"no filters, just free text", "", "", "error", "error"},
		{"service only", "bar-proxy", "", "", `service:"bar-proxy"`},
		{"host only", "", "ip-10-0-1-23", "", `host:"ip-10-0-1-23"`},
		{"service and host and free text", "bar-proxy", "ip-10-0-1-23", "error", `service:"bar-proxy" host:"ip-10-0-1-23" error`},
		{"nothing set", "", "", "", ""},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			a := New(config.Default())
			dv := a.datadogLogsV
			dv.serviceFilter = c.serviceFilter
			dv.hostFilter = c.hostFilter
			dv.query = c.query

			if got := dv.effectiveQuery(); got != c.want {
				t.Errorf("effectiveQuery() = %q, want %q", got, c.want)
			}
		})
	}
}

func TestApplyFilterOptionsPreservesSelectionWhenStillPresent(t *testing.T) {
	a := New(config.Default())
	dv := a.datadogLogsV
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
	a := New(config.Default())
	dv := a.datadogLogsV
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
	a := New(config.Default())
	dv := a.datadogLogsV
	current := ""

	dv.applyFilterOptions(dv.serviceFilterDD, []string{"svc-a", "svc-b"}, &current, func(string) {})

	if got := dv.serviceFilterDD.GetOptionCount(); got != 3 { // "(any)" + 2 values
		t.Errorf("option count = %d, want 3", got)
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
	a := New(config.Default())
	dv := a.datadogLogsV
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
	a := New(config.Default())
	dv := a.datadogLogsV
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

func TestDatadogLogsViewShortcutsIncludeServiceAndHostFilters(t *testing.T) {
	a := New(config.Default())
	var haveS, haveH bool
	for _, sc := range a.datadogLogsV.Shortcuts() {
		if sc.Key == "S" {
			haveS = true
		}
		if sc.Key == "H" {
			haveH = true
		}
	}
	if !haveS || !haveH {
		t.Errorf("Shortcuts() missing S/H filter shortcuts (haveS=%v, haveH=%v)", haveS, haveH)
	}
}

func TestDatadogLogsViewNameAndTitle(t *testing.T) {
	a := New(config.Default())
	if got := a.datadogLogsV.Name(); got != "datadog-logs" {
		t.Errorf("Name() = %q, want %q", got, "datadog-logs")
	}
	if got := a.datadogLogsV.Title(); got != "Datadog Logs" {
		t.Errorf("Title() = %q, want %q", got, "Datadog Logs")
	}
}

func TestDatadogLogsViewHeaderLabels(t *testing.T) {
	a := New(config.Default())
	want := []string{"TIMESTAMP", "SERVICE", "STATUS", "MESSAGE"}
	for col, label := range want {
		cell := a.datadogLogsV.table.GetCell(0, col)
		if cell == nil {
			t.Fatalf("header cell at column %d is nil", col)
		}
		if got := cell.Text; got != label {
			t.Errorf("header col %d = %q, want %q", col, got, label)
		}
	}
}

// TestDatadogLogsViewCycleTimeRange calls cycleTimeRange() directly,
// which also calls search() (no synchronous "not configured" guard at
// the view layer — that check lives inside datadoglogs.Search itself,
// called asynchronously). The resulting goroutine blocks forever on
// QueueUpdateDraw without a running tview event loop (same reasoning as
// logSearchView/ssmParamsView's tests), but presetIdx is mutated
// synchronously before search() is even called, so it's still safe to
// assert on here.
func TestDatadogLogsViewCycleTimeRange(t *testing.T) {
	a := New(config.Default())
	dv := a.datadogLogsV
	dv.presetIdx = defaultPresetIdx

	want := []int{2, 3, 0, 1} // wraps around back to the default ("1h")
	for _, w := range want {
		dv.cycleTimeRange()
		if dv.presetIdx != w {
			t.Errorf("presetIdx = %d, want %d", dv.presetIdx, w)
		}
	}
}

// TestDatadogLogsViewQueryInputTypingDoesNotSearch: queryInput has no
// SetChangedFunc wired (only SetDoneFunc for Enter), so typing alone
// has no side effects — checked here via dv.query staying empty.
func TestDatadogLogsViewQueryInputTypingDoesNotSearch(t *testing.T) {
	a := New(config.Default())
	dv := a.datadogLogsV

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
	a := New(config.Default())
	dv := a.datadogLogsV
	dv.queryInput.SetText("env:testt service:bar-proxy")

	dv.queryInput.InputHandler()(tcell.NewEventKey(tcell.KeyEnter, 0, tcell.ModNone), func(tview.Primitive) {})

	if dv.query != "env:testt service:bar-proxy" {
		t.Errorf("query = %q, want %q after Enter", dv.query, "env:testt service:bar-proxy")
	}
}

func TestDatadogLogsViewHandleSearchResult(t *testing.T) {
	t.Run("success populates rows and title", func(t *testing.T) {
		a := New(config.Default())
		dv := a.datadogLogsV
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
		a := New(config.Default())
		dv := a.datadogLogsV

		dv.handleSearchResult([]datadoglogs.LogEvent{{Message: "x"}}, true, nil)

		if !strings.Contains(dv.table.GetTitle(), "more available") {
			t.Errorf("title = %q, want it to mention more results are available", dv.table.GetTitle())
		}
	})

	t.Run("error logs and shows status, does not touch results", func(t *testing.T) {
		a := New(config.Default())
		dv := a.datadogLogsV
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
	a := New(config.Default())
	dv := a.datadogLogsV
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
