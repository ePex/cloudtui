package view

import (
	"context"
	"strings"
	"sync"
	"testing"

	"github.com/gdamore/tcell/v2"

	"github.com/ePex/cloudtui/tui/internal/awscodepipeline"
)

func newTestCodePipelineDetailView(t *testing.T) (*fakeViewHost, *CodePipelineDetailView) {
	t.Helper()
	host := newFakeViewHost()
	return host, NewCodePipelineDetailView(host, func() {})
}

func TestCodePipelineDetailViewShortcuts(t *testing.T) {
	_, dv := newTestCodePipelineDetailView(t)
	want := []string{"r", "w", "Esc"}
	got := dv.Shortcuts()
	if len(got) != len(want) {
		t.Fatalf("Shortcuts() = %+v, want %d entries", got, len(want))
	}
	for i, k := range want {
		if got[i].Key != k {
			t.Errorf("Shortcuts()[%d].Key = %q, want %q", i, got[i].Key, k)
		}
	}
}

func TestCodePipelineDetailViewRenderShowsStages(t *testing.T) {
	_, dv := newTestCodePipelineDetailView(t)

	dv.Render([]awscodepipeline.StageStatus{
		{Name: "Source", Status: "Succeeded"},
		{Name: "Build", Status: "InProgress"},
	})

	if got := dv.table.GetRowCount(); got != 3 { // header + 2
		t.Fatalf("row count = %d, want 3", got)
	}
	if got := dv.table.GetCell(1, 0).Text; got != "Source" {
		t.Errorf("row 1 name = %q, want %q", got, "Source")
	}
	if got := dv.table.GetCell(1, 1).Text; got != "Succeeded" {
		t.Errorf("row 1 status = %q, want %q", got, "Succeeded")
	}
	if got := dv.table.GetCell(2, 1).Text; got != "InProgress" {
		t.Errorf("row 2 status = %q, want %q", got, "InProgress")
	}
}

// TestCodePipelineDetailViewRenderShowsNeverRunForEmptyStatus covers
// StatusLabel's empty-string path — a stage that hasn't executed yet in
// this pipeline's current run.
func TestCodePipelineDetailViewRenderShowsNeverRunForEmptyStatus(t *testing.T) {
	_, dv := newTestCodePipelineDetailView(t)

	dv.Render([]awscodepipeline.StageStatus{{Name: "Deploy", Status: ""}})

	if got := dv.table.GetCell(1, 1).Text; got != "(never run)" {
		t.Errorf("status cell = %q, want %q", got, "(never run)")
	}
}

// TestCodePipelineDetailViewToggleWatchUpdatesTitle covers the 'w'
// handler's effect: starts/stops the background watch and updates the
// table title's "▶ watching" suffix (see updateTitle).
func TestCodePipelineDetailViewToggleWatchUpdatesTitle(t *testing.T) {
	host, dv := newTestCodePipelineDetailView(t)
	dv.pipelineName = "my-pipeline"

	dv.toggleWatch()
	if !host.IsWatchingPipeline("my-pipeline") {
		t.Fatal("toggleWatch() did not start watching")
	}
	if got := dv.table.GetTitle(); !strings.Contains(got, "▶ watching") {
		t.Errorf("title after starting watch = %q, want it to contain %q", got, "▶ watching")
	}

	dv.toggleWatch()
	if host.IsWatchingPipeline("my-pipeline") {
		t.Error("toggleWatch() did not stop watching on second call")
	}
	if got := dv.table.GetTitle(); strings.Contains(got, "▶ watching") {
		t.Errorf("title after stopping watch = %q, want no watching indicator", got)
	}
}

func TestCodePipelineDetailViewShowErrorRendersMessage(t *testing.T) {
	_, dv := newTestCodePipelineDetailView(t)

	dv.showError(context.DeadlineExceeded)

	if got := dv.table.GetCell(1, 0).Text; !strings.Contains(got, "deadline exceeded") {
		t.Errorf("error cell = %q, want it to contain the error", got)
	}
}

// TestCodePipelineDetailViewShowStatusRendersMessage covers the
// in-progress status message load() shows while awsauth.WithReauth is
// running an SSO re-auth (spec/36-fe-aws-sso-reauth).
func TestCodePipelineDetailViewShowStatusRendersMessage(t *testing.T) {
	host, dv := newTestCodePipelineDetailView(t)

	dv.showStatus("AWS SSO session expired — opening browser to log in...")

	if got := dv.table.GetCell(1, 0).Text; !strings.Contains(got, "opening browser") {
		t.Errorf("status cell = %q, want it to contain the status message", got)
	}
	fg, _, _ := dv.table.GetCell(1, 0).Style.Decompose()
	if want := tcell.GetColor(host.cfg.Colors.Accent); fg != want {
		t.Errorf("status cell color = %v, want accent color %v", fg, want)
	}
}

// TestCodePipelineDetailViewShowStatusRendersDeviceCodeMessage — see
// ssmparams_test.go's equivalent test for why load() itself isn't
// driven here.
func TestCodePipelineDetailViewShowStatusRendersDeviceCodeMessage(t *testing.T) {
	_, dv := newTestCodePipelineDetailView(t)

	dv.showStatus("AWS SSO session expired — opening browser to log in... Verify code WDJB-MJHT at https://device.sso.us-east-1.amazonaws.com/")

	if got := dv.table.GetCell(1, 0).Text; !strings.Contains(got, "Verify code WDJB-MJHT at") {
		t.Errorf("status cell = %q, want it to contain the device verification code/URL", got)
	}
}

func TestCodePipelineDetailViewShowReauthWaitingThenDone(t *testing.T) {
	_, dv := newTestCodePipelineDetailView(t)
	dv.pipelineName = "my-pipeline"
	dv.Render([]awscodepipeline.StageStatus{{Name: "Source", Status: "Succeeded"}}) // some prior state to overwrite

	const msg = "AWS SSO session expired — opening browser to log in…"
	dv.ShowReauthWaiting(msg)
	if got := dv.table.GetCell(1, 0).Text; got != msg {
		t.Errorf("row(1,0) after ShowReauthWaiting(%q) = %q, want it unchanged", msg, got)
	}

	dv.ShowReauthDone()
	want := "Loading my-pipeline…"
	if got := dv.table.GetCell(1, 0).Text; got != want {
		t.Errorf("row(1,0) after ShowReauthDone() = %q, want %q", got, want)
	}
}

func TestCodePipelineDetailViewLoadShowsLoadingStatusImmediately(t *testing.T) {
	host, dv := newTestCodePipelineDetailView(t)
	dv.pipelineName = "my-pipeline"
	host.cfg.ActiveAWSProfile = "work"
	unblock := make(chan struct{})
	host.getPipelineStateFn = func(context.Context, string, string) ([]awscodepipeline.StageStatus, error) {
		<-unblock
		return nil, nil
	}

	dv.load()

	want := "Loading my-pipeline…"
	if got := dv.table.GetCell(1, 0).Text; got != want {
		t.Errorf("row(1,0) after load() = %q, want %q", got, want)
	}
	close(unblock) // let the goroutine finish so it doesn't leak past the test
}

// newTestCodePipelineDetailViewWithDrawSignal is
// newTestCodePipelineDetailView's draw-signaling counterpart — see
// queues_test.go's drawSignalingHost/newTestQueuesViewWithDrawSignal for
// why this exists.
func newTestCodePipelineDetailViewWithDrawSignal(t *testing.T, bufSize int) (*drawSignalingHost, *CodePipelineDetailView) {
	t.Helper()
	base := newFakeViewHost()
	host := &drawSignalingHost{fakeViewHost: base, drawn: make(chan struct{}, bufSize)}
	return host, NewCodePipelineDetailView(host, func() {})
}

// TestCodePipelineDetailViewLoadDiscardsStaleResponse is the key
// regression test for loadSeq — see queues_test.go's
// TestQueuesViewLoadDiscardsStaleResponse, the pattern this mirrors.
func TestCodePipelineDetailViewLoadDiscardsStaleResponse(t *testing.T) {
	var mu sync.Mutex
	calls := 0
	firstCalled := make(chan struct{})
	releaseFirst := make(chan struct{})

	host, dv := newTestCodePipelineDetailViewWithDrawSignal(t, 2)
	dv.pipelineName = "my-pipeline"
	host.cfg.ActiveAWSProfile = "work"
	host.getPipelineStateFn = func(context.Context, string, string) ([]awscodepipeline.StageStatus, error) {
		mu.Lock()
		calls++
		n := calls
		mu.Unlock()
		if n == 1 {
			close(firstCalled)
			<-releaseFirst
			return []awscodepipeline.StageStatus{{Name: "stale"}}, nil
		}
		return []awscodepipeline.StageStatus{{Name: "fresh"}}, nil
	}

	dv.load()     // call 1 — will become "stale"; blocks inside getPipelineStateFn
	<-firstCalled // call 1's fetch has started (and is now blocked on releaseFirst)

	dv.load()    // call 2 — "fresh"; proceeds and draws immediately
	<-host.drawn // call 2's draw has landed (guaranteed first: call 1 can't proceed yet)

	if got := dv.table.GetCell(1, 0).Text; got != "fresh" {
		t.Fatalf("row(1,0) after call 2's draw = %q, want %q", got, "fresh")
	}

	close(releaseFirst) // let call 1 (stale) proceed to its now-discarded draw attempt
	<-host.drawn        // call 1's draw attempt has landed (and should have no-opped)

	if got := dv.table.GetCell(1, 0).Text; got != "fresh" {
		t.Errorf("row(1,0) after stale call 1's draw = %q, want unchanged %q", got, "fresh")
	}
}
