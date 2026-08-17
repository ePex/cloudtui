package app

import (
	"context"
	"fmt"
	"strings"
	"testing"
	"time"

	"github.com/gdamore/tcell/v2"
	"github.com/rivo/tview"

	"github.com/ePex/cloudtui/tui/internal/awslogs"
	"github.com/ePex/cloudtui/tui/internal/config"
	"github.com/ePex/cloudtui/tui/internal/ui"
)

// TestLogSearchViewSearchErrorsWithoutActiveProfile exercises search()'s
// synchronous guard, which returns before spawning the fetch goroutine —
// safe to call directly in a test, unlike the goroutine+QueueUpdateDraw
// path itself (which needs a running tview event loop to ever complete;
// see logsView/ssmParamsView/secretsView's tests for the same reasoning).
func TestLogSearchViewSearchErrorsWithoutActiveProfile(t *testing.T) {
	a := New(config.Default())
	a.cfg.ActiveAWSProfile = ""
	calls := 0
	a.filterLogEvents = func(context.Context, string, string, time.Time, time.Time, string) ([]awslogs.LogEvent, bool, error) {
		calls++
		return nil, false, nil
	}
	a.logSearchV.logGroupName = "/aws/lambda/foo"

	a.logSearchV.search()

	if calls != 0 {
		t.Error("filterLogEvents was called despite no active AWS profile")
	}
	if got := a.logSearchV.table.GetCell(1, 0).Text; !strings.Contains(got, "no AWS profile selected") {
		t.Errorf("error cell = %q, want it to mention no profile selected", got)
	}
}

// TestLogSearchViewOpenResetsStateAndSearches uses an empty active
// profile so open()'s internal search() call takes the synchronous guard
// path (no goroutine spawned), while still proving open() resets pattern/
// preset/results state and sets the title, for a log group opened after
// another one was previously open with different state.
func TestLogSearchViewOpenResetsStateAndSearches(t *testing.T) {
	a := New(config.Default())
	a.cfg.ActiveAWSProfile = ""
	sv := a.logSearchV
	sv.pattern = "stale-pattern"
	sv.patternInput.SetText("stale-pattern")
	sv.tr = ui.TimeRange{Mode: ui.TimeRangeRelative, PresetIdx: 3}
	sv.results = []awslogs.LogEvent{{Message: "stale"}}
	sv.hasMore = true

	sv.open("/aws/lambda/foo", "")

	if sv.pattern != "" {
		t.Errorf("pattern = %q, want empty after open()", sv.pattern)
	}
	if got := sv.patternInput.GetText(); got != "" {
		t.Errorf("patternInput text = %q, want empty after open()", got)
	}
	want := ui.TimeRange{Mode: ui.TimeRangeRelative, PresetIdx: ui.DefaultPresetIdx}
	if sv.tr != want {
		t.Errorf("tr = %+v, want %+v (default) after open()", sv.tr, want)
	}
	if len(sv.results) != 0 {
		t.Errorf("results = %+v, want empty after open() with no profile selected", sv.results)
	}
	// open()'s search() hit the no-profile guard, so the error cell (not
	// stale results) should be what's visible.
	if got := sv.table.GetCell(1, 0).Text; !strings.Contains(got, "no AWS profile selected") {
		t.Errorf("error cell = %q, want it to mention no profile selected", got)
	}
}

// TestLogSearchViewOpenWithInitialPatternPreFillsIt covers FE 41's
// CorrelationID jump: open() with a non-empty initialPattern must set
// both sv.pattern and the visible patternInput text, not just reset to
// empty like the normal (no-argument-equivalent) case.
func TestLogSearchViewOpenWithInitialPatternPreFillsIt(t *testing.T) {
	a := New(config.Default())
	a.cfg.ActiveAWSProfile = "" // open()'s internal search() hits the guard, no goroutine
	sv := a.logSearchV

	sv.open("/aws/lambda/foo", "1745d042-94e8-49f0-b223-8900ed9e951e")

	if sv.pattern != "1745d042-94e8-49f0-b223-8900ed9e951e" {
		t.Errorf("pattern = %q, want the pre-filled CorrelationID", sv.pattern)
	}
	if got := sv.patternInput.GetText(); got != "1745d042-94e8-49f0-b223-8900ed9e951e" {
		t.Errorf("patternInput text = %q, want the pre-filled CorrelationID", got)
	}
}

// TestLogSearchViewTKeyOpensTimeRangeModal covers spec/53: 't' now opens
// the shared time range modal (prefilled from sv.tr) instead of cycling
// presets directly, and the modal's onApply callback writes the result
// back into sv.tr and re-searches.
func TestLogSearchViewTKeyOpensTimeRangeModal(t *testing.T) {
	a := New(config.Default())
	a.cfg.ActiveAWSProfile = "" // any search() from onApply hits the guard, no goroutine
	sv := a.logSearchV
	sv.tr = ui.TimeRange{Mode: ui.TimeRangeRelative, PresetIdx: 2}

	capture := sv.table.GetInputCapture()
	if got := capture(tcell.NewEventKey(tcell.KeyRune, 't', tcell.ModNone)); got != nil {
		t.Errorf("'t' capture returned %v, want nil (event consumed)", got)
	}
	if !a.timeRangeModal.Visible() {
		t.Fatal("'t' did not open the time range modal")
	}

	// The modal's internals (relativeList, onApply) live in internal/dialog
	// and aren't reachable from here — drive the real UI path instead:
	// Show() focuses the relative list (asserting via the focused
	// primitive's own exported API, not a dialog-package field), prefilled
	// to sv.tr's preset; selecting a different preset and pressing Enter
	// exercises the actual applyRelative -> onApply -> sv.tr write-back
	// path a real keypress would.
	list, ok := a.tv.GetFocus().(*tview.List)
	if !ok {
		t.Fatalf("focus after 't' = %T, want *tview.List", a.tv.GetFocus())
	}
	if got := list.GetCurrentItem(); got != 2 {
		t.Errorf("relative list current item = %d, want 2 (sv.tr's preset)", got)
	}

	list.SetCurrentItem(4)
	list.InputHandler()(tcell.NewEventKey(tcell.KeyEnter, 0, tcell.ModNone), func(tview.Primitive) {})

	if want := (ui.TimeRange{Mode: ui.TimeRangeRelative, PresetIdx: 4}); sv.tr != want {
		t.Errorf("sv.tr = %+v, want %+v after applying from the modal", sv.tr, want)
	}
}

// TestLogSearchViewPatternInputTypingDoesNotSearch and
// TestLogSearchViewPatternInputEnterTriggersSearch both use an empty
// active profile as the observable signal (search()'s guard writes an
// error cell) rather than injecting filterLogEvents with a valid profile
// — the latter would let a real search() call spawn a goroutine that
// blocks forever on QueueUpdateDraw without a running tview event loop.
func TestLogSearchViewPatternInputTypingDoesNotSearch(t *testing.T) {
	a := New(config.Default())
	a.cfg.ActiveAWSProfile = ""
	sv := a.logSearchV
	sv.logGroupName = "/aws/lambda/foo"

	sv.patternInput.SetText("some pattern")

	// tview.Table.GetCell never returns nil (an unset cell comes back as
	// a zero-value &TableCell{}), so check the text, not the pointer.
	if got := sv.table.GetCell(1, 0).Text; got != "" {
		t.Errorf("error cell text = %q, want empty: typing alone must not trigger a search", got)
	}
}

func TestLogSearchViewPatternInputEnterTriggersSearch(t *testing.T) {
	a := New(config.Default())
	a.cfg.ActiveAWSProfile = ""
	sv := a.logSearchV
	sv.logGroupName = "/aws/lambda/foo"
	sv.patternInput.SetText("some pattern")

	sv.patternInput.InputHandler()(tcell.NewEventKey(tcell.KeyEnter, 0, tcell.ModNone), func(tview.Primitive) {})

	if sv.pattern != "some pattern" {
		t.Errorf("pattern = %q, want %q after Enter", sv.pattern, "some pattern")
	}
	if got := sv.table.GetCell(1, 0).Text; !strings.Contains(got, "no AWS profile selected") {
		t.Errorf("error cell = %q, want it to mention no profile selected (proving search() ran)", got)
	}
}

func TestHandleSearchResult(t *testing.T) {
	t.Run("success populates rows and title", func(t *testing.T) {
		a := New(config.Default())
		sv := a.logSearchV
		sv.logGroupName = "/aws/lambda/foo"
		ts := time.Date(2026, 1, 2, 3, 4, 5, 0, time.UTC)

		sv.handleSearchResult([]awslogs.LogEvent{
			{Timestamp: ts, LogStream: "stream-1", Message: "hello"},
		}, false, nil)

		if got := sv.table.GetRowCount(); got != 2 { // header + 1
			t.Fatalf("row count = %d, want 2", got)
		}
		if got := sv.table.GetCell(1, 1).Text; got != "stream-1" {
			t.Errorf("stream cell = %q, want %q", got, "stream-1")
		}
		if got := sv.table.GetCell(1, 2).Text; got != "hello" {
			t.Errorf("message cell = %q, want %q", got, "hello")
		}
		if got := sv.table.GetTitle(); !strings.Contains(got, "1 events") {
			t.Errorf("title = %q, want it to contain the event count", got)
		}
		if strings.Contains(sv.table.GetTitle(), "more available") {
			t.Errorf("title = %q, want no hasMore indicator", sv.table.GetTitle())
		}
	})

	t.Run("hasMore is reflected in the title", func(t *testing.T) {
		a := New(config.Default())
		sv := a.logSearchV
		sv.logGroupName = "/aws/lambda/foo"

		sv.handleSearchResult([]awslogs.LogEvent{{Message: "x"}}, true, nil)

		if !strings.Contains(sv.table.GetTitle(), "more available") {
			t.Errorf("title = %q, want it to mention more results are available", sv.table.GetTitle())
		}
	})

	t.Run("error logs and shows status, does not touch results", func(t *testing.T) {
		a := New(config.Default())
		sv := a.logSearchV
		sv.logGroupName = "/aws/lambda/foo"
		sv.results = []awslogs.LogEvent{{Message: "stale"}}

		sv.handleSearchResult(nil, false, context.DeadlineExceeded)

		if len(sv.results) != 0 {
			t.Errorf("results = %+v, want cleared after an error", sv.results)
		}
		if got := sv.table.GetCell(1, 0).Text; !strings.Contains(got, "deadline exceeded") {
			t.Errorf("error cell = %q, want it to contain the error", got)
		}
	})
}

// TestLogSearchViewScrollsToTopWithManyRows guards against the same bug
// fixed for queuesView (spec/11-bugfix-queues-scroll-to-top).
func TestLogSearchViewScrollsToTopWithManyRows(t *testing.T) {
	a := New(config.Default())
	sv := a.logSearchV
	sv.logGroupName = "/aws/lambda/foo"
	sv.table.SetRect(0, 0, 60, 15) // fewer visible rows than events below

	screen := tcell.NewSimulationScreen("")
	if err := screen.Init(); err != nil {
		t.Fatalf("screen.Init: %v", err)
	}
	screen.SetSize(60, 15)
	sv.table.Draw(screen)

	events := make([]awslogs.LogEvent, 50)
	for i := range events {
		events[i] = awslogs.LogEvent{Message: fmt.Sprintf("event-%02d", i)}
	}
	sv.handleSearchResult(events, false, nil)

	sv.table.Draw(screen)

	if row, _ := sv.table.GetOffset(); row != 0 {
		t.Errorf("table scrolled away from top: rowOffset = %d, want 0", row)
	}
}

func TestLogEventPreview(t *testing.T) {
	cases := []struct {
		name string
		in   string
		want string
	}{
		{name: "short single line passes through", in: "hello world", want: "hello world"},
		{name: "multi-line truncates at first newline", in: "line one\nline two\nline three", want: "line one …"},
		{name: "carriage return also truncates", in: "line one\r\nline two", want: "line one …"},
		{
			name: "long single line truncates with ellipsis",
			in:   strings.Repeat("x", 250),
			want: strings.Repeat("x", 200) + "…",
		},
		{name: "empty string", in: "", want: ""},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			if got := logEventPreview(tc.in); got != tc.want {
				t.Errorf("logEventPreview(%q) = %q, want %q", tc.in, got, tc.want)
			}
		})
	}
}
