package view

import (
	"context"
	"fmt"
	"strings"
	"testing"
	"time"

	"github.com/gdamore/tcell/v2"
	"github.com/rivo/tview"

	"github.com/ePex/cloudtui/tui/internal/awslogs"
	"github.com/ePex/cloudtui/tui/internal/dialog"
	"github.com/ePex/cloudtui/tui/internal/ui"
)

func newTestLogSearchView(t *testing.T) (*fakeViewHost, *dialog.TimeRangeModal, *LogSearchView) {
	t.Helper()
	host := newFakeViewHost()
	timeRangeModal := dialog.NewTimeRangeModal(host)
	return host, timeRangeModal, NewLogSearchView(host, timeRangeModal, func(awslogs.LogEvent) {}, func() {})
}

// TestLogSearchViewSearchErrorsWithoutActiveProfile exercises search()'s
// synchronous guard, which returns before spawning the fetch goroutine —
// safe to call directly in a test, unlike the goroutine+QueueUpdateDraw
// path itself (which needs a running tview event loop to ever complete;
// see logsView/ssmParamsView/secretsView's tests for the same reasoning).
func TestLogSearchViewSearchErrorsWithoutActiveProfile(t *testing.T) {
	host, _, sv := newTestLogSearchView(t)
	host.cfg.ActiveAWSProfile = ""
	calls := 0
	host.filterLogEventsFn = func(context.Context, string, string, time.Time, time.Time, string, string) ([]awslogs.LogEvent, string, error) {
		calls++
		return nil, "", nil
	}
	sv.logGroupName = "/aws/lambda/foo"

	sv.search()

	if calls != 0 {
		t.Error("filterLogEvents was called despite no active AWS profile")
	}
	if got := sv.table.GetCell(1, 0).Text; !strings.Contains(got, "no AWS profile selected") {
		t.Errorf("error cell = %q, want it to mention no profile selected", got)
	}
}

// TestLogSearchViewOpenResetsStateAndSearches uses an empty active
// profile so Open()'s internal search() call takes the synchronous guard
// path (no goroutine spawned), while still proving Open() resets pattern/
// preset/results state and sets the title, for a log group opened after
// another one was previously open with different state.
func TestLogSearchViewOpenResetsStateAndSearches(t *testing.T) {
	host, _, sv := newTestLogSearchView(t)
	host.cfg.ActiveAWSProfile = ""
	sv.pattern = "stale-pattern"
	sv.patternInput.SetText("stale-pattern")
	sv.tr = ui.TimeRange{Mode: ui.TimeRangeRelative, PresetIdx: 3}
	sv.results = []awslogs.LogEvent{{Message: "stale"}}
	sv.nextToken = "stale-token"

	sv.Open("/aws/lambda/foo", "", nil)

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
	host, _, sv := newTestLogSearchView(t)
	host.cfg.ActiveAWSProfile = "" // open()'s internal search() hits the guard, no goroutine

	sv.Open("/aws/lambda/foo", "1745d042-94e8-49f0-b223-8900ed9e951e", nil)

	if sv.pattern != "1745d042-94e8-49f0-b223-8900ed9e951e" {
		t.Errorf("pattern = %q, want the pre-filled CorrelationID", sv.pattern)
	}
	if got := sv.patternInput.GetText(); got != "1745d042-94e8-49f0-b223-8900ed9e951e" {
		t.Errorf("patternInput text = %q, want the pre-filled CorrelationID", got)
	}
}

// TestLogSearchViewOpenWithInitialTimeRangeOverridesDefault covers
// spec-origin/91: a non-nil initialTimeRange (e.g. the CorrelationID
// jump's computed absolute window) must be used exactly as given,
// instead of Open()'s usual reset to the relative default.
func TestLogSearchViewOpenWithInitialTimeRangeOverridesDefault(t *testing.T) {
	host, _, sv := newTestLogSearchView(t)
	host.cfg.ActiveAWSProfile = "" // open()'s internal search() hits the guard, no goroutine
	from := time.Date(2026, 1, 2, 3, 0, 0, 0, time.UTC)
	to := time.Date(2026, 1, 2, 3, 30, 0, 0, time.UTC)
	override := ui.TimeRange{Mode: ui.TimeRangeAbsolute, From: from, To: to}

	sv.Open("/aws/lambda/foo", "some-pattern", &override)

	if sv.tr != override {
		t.Errorf("tr = %+v, want the override %+v (not the relative default)", sv.tr, override)
	}
	if got := sv.TimeRange(); got != override {
		t.Errorf("TimeRange() = %+v, want %+v", got, override)
	}
}

// TestLogSearchViewTKeyOpensTimeRangeModal covers spec/53: 't' now opens
// the shared time range modal (prefilled from sv.tr) instead of cycling
// presets directly, and the modal's onApply callback writes the result
// back into sv.tr and re-searches.
func TestLogSearchViewTKeyOpensTimeRangeModal(t *testing.T) {
	host, timeRangeModal, sv := newTestLogSearchView(t)
	host.cfg.ActiveAWSProfile = "" // any search() from onApply hits the guard, no goroutine
	sv.tr = ui.TimeRange{Mode: ui.TimeRangeRelative, PresetIdx: 2}

	capture := sv.table.GetInputCapture()
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
	// to sv.tr's preset; selecting a different preset and pressing Enter
	// exercises the actual applyRelative -> onApply -> sv.tr write-back
	// path a real keypress would.
	list, ok := host.focused.(*tview.List)
	if !ok {
		t.Fatalf("focus after 't' = %T, want *tview.List", host.focused)
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
	host, _, sv := newTestLogSearchView(t)
	host.cfg.ActiveAWSProfile = ""
	sv.logGroupName = "/aws/lambda/foo"

	sv.patternInput.SetText("some pattern")

	// tview.Table.GetCell never returns nil (an unset cell comes back as
	// a zero-value &TableCell{}), so check the text, not the pointer.
	if got := sv.table.GetCell(1, 0).Text; got != "" {
		t.Errorf("error cell text = %q, want empty: typing alone must not trigger a search", got)
	}
}

func TestLogSearchViewPatternInputEnterTriggersSearch(t *testing.T) {
	host, _, sv := newTestLogSearchView(t)
	host.cfg.ActiveAWSProfile = ""
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
		_, _, sv := newTestLogSearchView(t)
		sv.logGroupName = "/aws/lambda/foo"
		ts := time.Date(2026, 1, 2, 3, 4, 5, 0, time.UTC)

		sv.handleSearchResult([]awslogs.LogEvent{
			{Timestamp: ts, LogStream: "stream-1", Message: "hello"},
		}, "", nil)

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

	t.Run("a non-empty next token is reflected in the title, mentioning n", func(t *testing.T) {
		_, _, sv := newTestLogSearchView(t)
		sv.logGroupName = "/aws/lambda/foo"

		sv.handleSearchResult([]awslogs.LogEvent{{Message: "x"}}, "some-token", nil)

		if !strings.Contains(sv.table.GetTitle(), "more available — press n to load more, or narrow your search") {
			t.Errorf("title = %q, want it to mention more results and the n keybinding", sv.table.GetTitle())
		}
	})

	t.Run("error logs and shows status, does not touch results", func(t *testing.T) {
		_, _, sv := newTestLogSearchView(t)
		sv.logGroupName = "/aws/lambda/foo"
		sv.results = []awslogs.LogEvent{{Message: "stale"}}

		sv.handleSearchResult(nil, "", context.DeadlineExceeded)

		if len(sv.results) != 0 {
			t.Errorf("results = %+v, want cleared after an error", sv.results)
		}
		if got := sv.table.GetCell(1, 0).Text; !strings.Contains(got, "deadline exceeded") {
			t.Errorf("error cell = %q, want it to contain the error", got)
		}
	})
}

// TestLoadMoreNoopWhenNoNextToken proves loadMore() checks
// sv.nextToken before doing anything else — no fetch, no goroutine —
// which is also what makes it safe to drive through the real 'n'
// keybinding in a test without an event loop (see
// TestLogSearchViewNKeyLoadMore below).
func TestLoadMoreNoopWhenNoNextToken(t *testing.T) {
	host, _, sv := newTestLogSearchView(t)
	sv.logGroupName = "/aws/lambda/foo"
	sv.nextToken = ""
	calls := 0
	host.filterLogEventsFn = func(context.Context, string, string, time.Time, time.Time, string, string) ([]awslogs.LogEvent, string, error) {
		calls++
		return nil, "", nil
	}

	sv.loadMore()

	if calls != 0 {
		t.Error("loadMore() called FilterLogEvents despite nextToken being empty")
	}
}

// TestLogSearchViewNKeyLoadMore drives the real 'n' keybinding, with
// nextToken left empty so loadMore()'s guard is hit before any
// goroutine would be spawned (see the no-profile tests elsewhere in
// this file for why that matters without a running tview event loop).
func TestLogSearchViewNKeyLoadMore(t *testing.T) {
	host, _, sv := newTestLogSearchView(t)
	sv.logGroupName = "/aws/lambda/foo"
	sv.nextToken = ""
	calls := 0
	host.filterLogEventsFn = func(context.Context, string, string, time.Time, time.Time, string, string) ([]awslogs.LogEvent, string, error) {
		calls++
		return nil, "", nil
	}

	capture := sv.table.GetInputCapture()
	if got := capture(tcell.NewEventKey(tcell.KeyRune, 'n', tcell.ModNone)); got != nil {
		t.Errorf("'n' capture returned %v, want nil (event consumed)", got)
	}
	if calls != 0 {
		t.Error("'n' triggered a fetch despite nextToken being empty")
	}
}

func TestHandleLoadMoreResult(t *testing.T) {
	t.Run("success appends rather than replaces, and updates nextToken", func(t *testing.T) {
		_, _, sv := newTestLogSearchView(t)
		sv.logGroupName = "/aws/lambda/foo"
		sv.results = []awslogs.LogEvent{{Message: "first"}}
		sv.nextToken = "tok-1"

		sv.handleLoadMoreResult([]awslogs.LogEvent{{Message: "second"}}, "tok-2", nil)

		if len(sv.results) != 2 {
			t.Fatalf("results = %+v, want 2 (appended, not replaced)", sv.results)
		}
		if sv.results[0].Message != "first" || sv.results[1].Message != "second" {
			t.Errorf("results = %+v, want [first second]", sv.results)
		}
		if sv.nextToken != "tok-2" {
			t.Errorf("nextToken = %q, want %q", sv.nextToken, "tok-2")
		}
		if got := sv.table.GetRowCount(); got != 3 { // header + 2
			t.Errorf("row count = %d, want 3", got)
		}
	})

	t.Run("error preserves existing results and the table", func(t *testing.T) {
		_, _, sv := newTestLogSearchView(t)
		sv.logGroupName = "/aws/lambda/foo"
		sv.results = []awslogs.LogEvent{{Message: "first"}}
		sv.nextToken = "tok-1"
		sv.repaint() // populate the table to match sv.results, as a real prior search would have

		sv.handleLoadMoreResult(nil, "", context.DeadlineExceeded)

		if len(sv.results) != 1 || sv.results[0].Message != "first" {
			t.Errorf("results = %+v, want unchanged after a load-more error", sv.results)
		}
		if sv.nextToken != "tok-1" {
			t.Errorf("nextToken = %q, want unchanged (%q) after a load-more error", sv.nextToken, "tok-1")
		}
		if got := sv.table.GetRowCount(); got != 2 { // header + 1, untouched
			t.Errorf("row count = %d, want 2 (table untouched by the error)", got)
		}
	})
}

// TestLogSearchViewScrollsToTopWithManyRows guards against the same bug
// fixed for queuesView (spec/11-bugfix-queues-scroll-to-top).
func TestLogSearchViewScrollsToTopWithManyRows(t *testing.T) {
	_, _, sv := newTestLogSearchView(t)
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
	sv.handleSearchResult(events, "", nil)

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

// pageStub returns a fetchPages-compatible closure serving pages from
// tokens, plus a pointer to the running call count so tests can assert
// how many pages were actually fetched. If errAt is >= 0, the call at
// that index (0-based) returns err and no page.
func pageStub(t *testing.T, pages [][]awslogs.LogEvent, tokens []string, errAt int, err error) (fetch func(nextToken string) ([]awslogs.LogEvent, string, error), calls *int) {
	t.Helper()
	if len(pages) != len(tokens) {
		t.Fatalf("pageStub: len(pages)=%d != len(tokens)=%d", len(pages), len(tokens))
	}
	n := 0
	return func(nextToken string) ([]awslogs.LogEvent, string, error) {
		i := n
		n++
		if i == errAt {
			return nil, "", err
		}
		if i >= len(pages) {
			t.Fatalf("pageStub: unexpected call %d (only %d pages configured)", i, len(pages))
		}
		return pages[i], tokens[i], nil
	}, &n
}

func TestFetchPages(t *testing.T) {
	t.Run("exhausts before the cap", func(t *testing.T) {
		fetch, calls := pageStub(t,
			[][]awslogs.LogEvent{
				{{Message: "a"}},
				{{Message: "b"}},
				{{Message: "c"}},
			},
			[]string{"tok-1", "tok-2", ""},
			-1, nil,
		)

		events, next, err := fetchPages(fetch, maxAutoContinuePages)

		if err != nil {
			t.Fatalf("fetchPages() err = %v, want nil", err)
		}
		if next != "" {
			t.Errorf("next = %q, want empty (exhausted)", next)
		}
		if *calls != 3 {
			t.Errorf("calls = %d, want 3 (stopped once a page returned no token)", *calls)
		}
		if len(events) != 3 {
			t.Errorf("events = %+v, want 3 accumulated across all pages", events)
		}
	})

	t.Run("hits the cap with more remaining", func(t *testing.T) {
		fetch, calls := pageStub(t,
			[][]awslogs.LogEvent{
				{{Message: "a"}},
				{{Message: "b"}},
			},
			[]string{"tok-1", "tok-2"}, // both pages still report more
			-1, nil,
		)

		events, next, err := fetchPages(fetch, 2)

		if err != nil {
			t.Fatalf("fetchPages() err = %v, want nil", err)
		}
		if next != "tok-2" {
			t.Errorf("next = %q, want %q (cap hit with more remaining)", next, "tok-2")
		}
		if *calls != 2 {
			t.Errorf("calls = %d, want 2 (stopped at maxPages)", *calls)
		}
		if len(events) != 2 {
			t.Errorf("events = %+v, want 2 accumulated before the cap", events)
		}
	})

	t.Run("errors on a later page discard everything already fetched", func(t *testing.T) {
		wantErr := context.DeadlineExceeded
		fetch, calls := pageStub(t,
			[][]awslogs.LogEvent{
				{{Message: "a"}},
			},
			[]string{"tok-1"},
			1, wantErr, // second call (index 1) errors
		)

		events, next, err := fetchPages(fetch, maxAutoContinuePages)

		if err != wantErr {
			t.Errorf("err = %v, want %v", err, wantErr)
		}
		if events != nil {
			t.Errorf("events = %+v, want nil (partial results discarded on error)", events)
		}
		if next != "" {
			t.Errorf("next = %q, want empty on error", next)
		}
		if *calls != 2 {
			t.Errorf("calls = %d, want 2 (stopped at the error)", *calls)
		}
	})

	t.Run("single page (maxPages=1) fetches exactly one page even with more available", func(t *testing.T) {
		fetch, calls := pageStub(t,
			[][]awslogs.LogEvent{
				{{Message: "a"}, {Message: "b"}},
			},
			[]string{"tok-1"}, // more available, but maxPages=1 must not chase it
			-1, nil,
		)

		events, next, err := fetchPages(fetch, 1)

		if err != nil {
			t.Fatalf("fetchPages() err = %v, want nil", err)
		}
		if next != "tok-1" {
			t.Errorf("next = %q, want %q (not fetched further)", next, "tok-1")
		}
		if *calls != 1 {
			t.Errorf("calls = %d, want 1", *calls)
		}
		if len(events) != 2 {
			t.Errorf("events = %+v, want the single page's 2 events", events)
		}
	})
}
