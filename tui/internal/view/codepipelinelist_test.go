package view

import (
	"context"
	"strings"
	"sync"
	"testing"

	"github.com/gdamore/tcell/v2"
	"github.com/rivo/tview"

	"github.com/ePex/cloudtui/tui/internal/awscodepipeline"
)

func newTestCodePipelineListView(t *testing.T) (*fakeViewHost, *CodePipelineListView) {
	t.Helper()
	host := newFakeViewHost()
	return host, NewCodePipelineListView(host, func(string) {})
}

// TestCodePipelineListViewSelectedFuncMapsThroughFilter covers the
// SetSelectedFunc wiring done in New() — Enter on a row calls onSelect
// for that pipeline, mapped through the current filter, not the raw
// table index.
func TestCodePipelineListViewSelectedFuncMapsThroughFilter(t *testing.T) {
	host := newFakeViewHost()
	var selected string
	lv := NewCodePipelineListView(host, func(name string) { selected = name })
	lv.repaint([]awscodepipeline.Pipeline{
		{Name: "pipeline-one"},
		{Name: "pipeline-two"},
	})
	lv.applyFilter("two") // only "pipeline-two" remains, at row 1
	lv.table.Select(1, 0)

	handler := lv.table.InputHandler()
	handler(tcell.NewEventKey(tcell.KeyEnter, 0, tcell.ModNone), func(tview.Primitive) {})

	if selected != "pipeline-two" {
		t.Errorf("onSelect called with %q, want %q", selected, "pipeline-two")
	}
}

func TestCodePipelineListViewNameAndTitle(t *testing.T) {
	_, lv := newTestCodePipelineListView(t)
	if got := lv.Name(); got != "codepipeline" {
		t.Errorf("Name() = %q, want %q", got, "codepipeline")
	}
	if got := lv.Title(); got != "AWS CodePipeline" {
		t.Errorf("Title() = %q, want %q", got, "AWS CodePipeline")
	}
}

func TestCodePipelineListViewHeaderLabels(t *testing.T) {
	_, lv := newTestCodePipelineListView(t)
	want := []string{"NAME", "WATCHING", "UPDATED"}
	for col, label := range want {
		cell := lv.table.GetCell(0, col)
		if cell == nil {
			t.Fatalf("header cell at column %d is nil", col)
		}
		if got := cell.Text; got != label {
			t.Errorf("header col %d = %q, want %q", col, got, label)
		}
	}
}

// TestCodePipelineListViewLoadErrorsWithoutActiveProfile exercises load()'s
// synchronous guard, which returns before spawning the fetch goroutine —
// safe to call directly in a test, unlike the goroutine+QueueUpdateDraw
// path itself (which needs a running tview event loop to ever complete;
// see logs_test.go's equivalent test for the same reasoning).
func TestCodePipelineListViewLoadErrorsWithoutActiveProfile(t *testing.T) {
	host, lv := newTestCodePipelineListView(t)
	host.cfg.ActiveAWSProfile = ""
	calls := 0
	host.listPipelinesFn = func(context.Context, string) ([]awscodepipeline.Pipeline, error) {
		calls++
		return nil, nil
	}

	lv.load()

	if calls != 0 {
		t.Error("listPipelines was called despite no active AWS profile")
	}
	if got := lv.table.GetCell(1, 0).Text; !strings.Contains(got, "no AWS profile selected") {
		t.Errorf("error cell = %q, want it to mention no profile selected", got)
	}
}

func TestCodePipelineListViewRepaintPopulatesRows(t *testing.T) {
	_, lv := newTestCodePipelineListView(t)

	lv.repaint([]awscodepipeline.Pipeline{
		{Name: "pipeline-one"},
		{Name: "pipeline-two"},
	})

	if got := lv.table.GetRowCount(); got != 3 { // header + 2
		t.Fatalf("row count = %d, want 3", got)
	}
	if got := lv.table.GetCell(1, 0).Text; got != "pipeline-one" {
		t.Errorf("row 1 name = %q, want %q", got, "pipeline-one")
	}
	if got := lv.table.GetTitle(); got != " AWS CodePipeline (2) " {
		t.Errorf("title = %q, want %q", got, " AWS CodePipeline (2) ")
	}
}

func TestCodePipelineListViewRepaintShowsDashForNoUpdatedAt(t *testing.T) {
	_, lv := newTestCodePipelineListView(t)

	lv.repaint([]awscodepipeline.Pipeline{{Name: "x"}})

	if got := lv.table.GetCell(1, 2).Text; got != "-" {
		t.Errorf("updated cell = %q, want %q", got, "-")
	}
}

// TestCodePipelineListViewRepaintShowsWatchingIndicator covers the
// WATCHING column — populated from host.IsWatchingPipeline, not from any
// field on awscodepipeline.Pipeline itself.
func TestCodePipelineListViewRepaintShowsWatchingIndicator(t *testing.T) {
	host, lv := newTestCodePipelineListView(t)
	host.watching["pipeline-one"] = true

	lv.repaint([]awscodepipeline.Pipeline{
		{Name: "pipeline-one"},
		{Name: "pipeline-two"},
	})

	if got := lv.table.GetCell(1, 1).Text; got != "▶ watching" {
		t.Errorf("watched row's WATCHING cell = %q, want %q", got, "▶ watching")
	}
	if got := lv.table.GetCell(2, 1).Text; got != "" {
		t.Errorf("unwatched row's WATCHING cell = %q, want empty", got)
	}
}

func TestCodePipelineListViewFilterNarrowsRowsByName(t *testing.T) {
	_, lv := newTestCodePipelineListView(t)
	lv.repaint([]awscodepipeline.Pipeline{
		{Name: "db-writer"},
		{Name: "db-reader"},
		{Name: "ecs-other"},
	})

	lv.applyFilter("db")

	if got := lv.table.GetRowCount(); got != 3 { // header + 2 matches
		t.Fatalf("row count after filter = %d, want 3", got)
	}
	if got := lv.table.GetTitle(); got != " AWS CodePipeline (db) " {
		t.Errorf("title after filter = %q, want %q", got, " AWS CodePipeline (db) ")
	}
}

func TestCodePipelineListViewFilterClearRestoresAll(t *testing.T) {
	_, lv := newTestCodePipelineListView(t)
	lv.repaint([]awscodepipeline.Pipeline{{Name: "a"}, {Name: "b"}})
	lv.applyFilter("a")

	lv.applyFilter("")

	if got := lv.table.GetRowCount(); got != 3 {
		t.Errorf("row count after clearing filter = %d, want 3", got)
	}
}

// TestCodePipelineListViewToggleWatchSelectedStartsAndStops exercises
// toggleWatchSelected directly against the table's current selection,
// mirroring how the 'w' key handler invokes it.
func TestCodePipelineListViewToggleWatchSelectedStartsAndStops(t *testing.T) {
	host, lv := newTestCodePipelineListView(t)
	lv.repaint([]awscodepipeline.Pipeline{{Name: "pipeline-one"}})
	lv.table.Select(1, 0)

	lv.toggleWatchSelected()
	if !host.IsWatchingPipeline("pipeline-one") {
		t.Fatal("toggleWatchSelected() did not start watching")
	}
	if got := lv.table.GetCell(1, 1).Text; got != "▶ watching" {
		t.Errorf("WATCHING cell after starting = %q, want %q", got, "▶ watching")
	}

	lv.toggleWatchSelected()
	if host.IsWatchingPipeline("pipeline-one") {
		t.Error("toggleWatchSelected() did not stop watching on second call")
	}
	if got := lv.table.GetCell(1, 1).Text; got != "" {
		t.Errorf("WATCHING cell after stopping = %q, want empty", got)
	}
}

func TestCodePipelineListViewShowErrorRendersMessage(t *testing.T) {
	_, lv := newTestCodePipelineListView(t)

	lv.showError(context.DeadlineExceeded)

	if got := lv.table.GetCell(1, 0).Text; !strings.Contains(got, "deadline exceeded") {
		t.Errorf("error cell = %q, want it to contain the error", got)
	}
}

// TestCodePipelineListViewShowStatusRendersMessage covers the in-progress
// status message load() shows while awsauth.WithReauth is running an SSO
// re-auth (spec/36-fe-aws-sso-reauth) — see logs_test.go's equivalent test.
func TestCodePipelineListViewShowStatusRendersMessage(t *testing.T) {
	host, lv := newTestCodePipelineListView(t)

	lv.showStatus("AWS SSO session expired — opening browser to log in...")

	if got := lv.table.GetCell(1, 0).Text; !strings.Contains(got, "opening browser") {
		t.Errorf("status cell = %q, want it to contain the status message", got)
	}
	fg, _, _ := lv.table.GetCell(1, 0).Style.Decompose()
	if want := tcell.GetColor(host.cfg.Colors.Accent); fg != want {
		t.Errorf("status cell color = %v, want accent color %v", fg, want)
	}
}

// TestCodePipelineListViewShowStatusRendersDeviceCodeMessage — see
// ssmparams_test.go's equivalent test for why load() itself isn't
// driven here.
func TestCodePipelineListViewShowStatusRendersDeviceCodeMessage(t *testing.T) {
	_, lv := newTestCodePipelineListView(t)

	lv.showStatus("AWS SSO session expired — opening browser to log in... Verify code WDJB-MJHT at https://device.sso.us-east-1.amazonaws.com/")

	if got := lv.table.GetCell(1, 0).Text; !strings.Contains(got, "Verify code WDJB-MJHT at") {
		t.Errorf("status cell = %q, want it to contain the device verification code/URL", got)
	}
}

func TestCodePipelineListViewShowReauthWaitingThenDone(t *testing.T) {
	_, lv := newTestCodePipelineListView(t)
	lv.repaint([]awscodepipeline.Pipeline{{Name: "foo"}}) // some prior state to overwrite

	const msg = "AWS SSO session expired — opening browser to log in…"
	lv.ShowReauthWaiting(msg)
	if got := lv.table.GetCell(1, 0).Text; got != msg {
		t.Errorf("row(1,0) after ShowReauthWaiting(%q) = %q, want it unchanged", msg, got)
	}

	lv.ShowReauthDone()
	if got := lv.table.GetCell(1, 0).Text; got != loadingPipelinesStatus {
		t.Errorf("row(1,0) after ShowReauthDone() = %q, want %q", got, loadingPipelinesStatus)
	}
}

func TestCodePipelineListViewLoadShowsLoadingStatusImmediately(t *testing.T) {
	host, lv := newTestCodePipelineListView(t)
	host.cfg.ActiveAWSProfile = "work"
	unblock := make(chan struct{})
	host.listPipelinesFn = func(context.Context, string) ([]awscodepipeline.Pipeline, error) {
		<-unblock
		return nil, nil
	}

	lv.load()

	cell := lv.table.GetCell(1, 0)
	if cell == nil || cell.Text != loadingPipelinesStatus {
		t.Errorf("row(1,0) after load() = %+v, want text %q", cell, loadingPipelinesStatus)
	}
	close(unblock) // let the goroutine finish so it doesn't leak past the test
}

// newTestCodePipelineListViewWithDrawSignal is newTestCodePipelineListView's
// draw-signaling counterpart — see queues_test.go's drawSignalingHost/
// newTestQueuesViewWithDrawSignal for why this exists.
func newTestCodePipelineListViewWithDrawSignal(t *testing.T, bufSize int) (*drawSignalingHost, *CodePipelineListView) {
	t.Helper()
	base := newFakeViewHost()
	host := &drawSignalingHost{fakeViewHost: base, drawn: make(chan struct{}, bufSize)}
	return host, NewCodePipelineListView(host, func(string) {})
}

// TestCodePipelineListViewLoadDiscardsStaleResponse is the key regression
// test for loadSeq — see queues_test.go's
// TestQueuesViewLoadDiscardsStaleResponse, the pattern this mirrors.
func TestCodePipelineListViewLoadDiscardsStaleResponse(t *testing.T) {
	var mu sync.Mutex
	calls := 0
	firstCalled := make(chan struct{})
	releaseFirst := make(chan struct{})

	host, lv := newTestCodePipelineListViewWithDrawSignal(t, 2)
	host.cfg.ActiveAWSProfile = "work"
	host.listPipelinesFn = func(context.Context, string) ([]awscodepipeline.Pipeline, error) {
		mu.Lock()
		calls++
		n := calls
		mu.Unlock()
		if n == 1 {
			close(firstCalled)
			<-releaseFirst
			return []awscodepipeline.Pipeline{{Name: "stale"}}, nil
		}
		return []awscodepipeline.Pipeline{{Name: "fresh"}}, nil
	}

	lv.load()     // call 1 — will become "stale"; blocks inside listPipelinesFn
	<-firstCalled // call 1's fetch has started (and is now blocked on releaseFirst)

	lv.load()    // call 2 — "fresh"; proceeds and draws immediately
	<-host.drawn // call 2's draw has landed (guaranteed first: call 1 can't proceed yet)

	if got := lv.table.GetCell(1, 0).Text; got != "fresh" {
		t.Fatalf("row(1,0) after call 2's draw = %q, want %q", got, "fresh")
	}

	close(releaseFirst) // let call 1 (stale) proceed to its now-discarded draw attempt
	<-host.drawn        // call 1's draw attempt has landed (and should have no-opped)

	if got := lv.table.GetCell(1, 0).Text; got != "fresh" {
		t.Errorf("row(1,0) after stale call 1's draw = %q, want unchanged %q", got, "fresh")
	}
}
