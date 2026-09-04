package view

import (
	"context"
	"errors"
	"sync"
	"testing"
	"time"

	"github.com/ePex/cloudtui/tui/internal/awscodepipeline"
)

func TestStageTransitionsNilPrevReturnsNoMessages(t *testing.T) {
	got := stageTransitions(nil, []awscodepipeline.StageStatus{
		{Name: "Build", Status: "InProgress"},
	})
	if len(got) != 0 {
		t.Errorf("stageTransitions(nil, ...) = %v, want no messages for the first poll (no baseline yet)", got)
	}
}

func TestStageTransitionsReportsChangedStages(t *testing.T) {
	prev := map[string]string{"Source": "Succeeded", "Build": "InProgress"}
	got := stageTransitions(prev, []awscodepipeline.StageStatus{
		{Name: "Source", Status: "Succeeded"},  // unchanged
		{Name: "Build", Status: "Succeeded"},   // changed
		{Name: "Deploy", Status: "InProgress"}, // new stage, prev had no entry ("" != "InProgress")
	})

	want := []string{"Build: Succeeded", "Deploy: InProgress"}
	if len(got) != len(want) {
		t.Fatalf("stageTransitions() = %v, want %v", got, want)
	}
	for i, w := range want {
		if got[i] != w {
			t.Errorf("got[%d] = %q, want %q", i, got[i], w)
		}
	}
}

func TestStageTransitionsNeverRunStageLabel(t *testing.T) {
	prev := map[string]string{"Deploy": "InProgress"}
	got := stageTransitions(prev, []awscodepipeline.StageStatus{
		{Name: "Deploy", Status: ""}, // regressed to "never run"? unlikely in practice, but exercises statusLabel's empty-string path
	})
	if len(got) != 1 || got[0] != "Deploy: (never run)" {
		t.Errorf("stageTransitions() = %v, want [\"Deploy: (never run)\"]", got)
	}
}

func TestSnapshotStages(t *testing.T) {
	got := snapshotStages([]awscodepipeline.StageStatus{
		{Name: "Source", Status: "Succeeded"},
		{Name: "Build", Status: "InProgress"},
	})
	want := map[string]string{"Source": "Succeeded", "Build": "InProgress"}
	if len(got) != len(want) {
		t.Fatalf("snapshotStages() = %v, want %v", got, want)
	}
	for k, v := range want {
		if got[k] != v {
			t.Errorf("snapshotStages()[%q] = %q, want %q", k, got[k], v)
		}
	}
}

func TestPipelineFinished(t *testing.T) {
	cases := []struct {
		name   string
		stages []awscodepipeline.StageStatus
		want   bool
	}{
		{"empty", nil, false},
		{"never executed — no execution id at all", []awscodepipeline.StageStatus{{Name: "Source", Status: ""}}, false},
		{
			"all in progress",
			[]awscodepipeline.StageStatus{
				{Name: "Source", Status: "Succeeded", PipelineExecutionID: "exec-1"},
				{Name: "Build", Status: "InProgress", PipelineExecutionID: "exec-1"},
			},
			false,
		},
		{
			"last stage succeeded",
			[]awscodepipeline.StageStatus{
				{Name: "Source", Status: "Succeeded", PipelineExecutionID: "exec-1"},
				{Name: "Deploy", Status: "Succeeded", PipelineExecutionID: "exec-1"},
			},
			true,
		},
		{
			"a middle stage failed",
			[]awscodepipeline.StageStatus{
				{Name: "Source", Status: "Succeeded", PipelineExecutionID: "exec-1"},
				{Name: "Build", Status: "Failed", PipelineExecutionID: "exec-1"},
				{Name: "Deploy", Status: "", PipelineExecutionID: "exec-1"},
			},
			true,
		},
		{
			"a stage stopped",
			[]awscodepipeline.StageStatus{
				{Name: "Source", Status: "Succeeded", PipelineExecutionID: "exec-1"},
				{Name: "Build", Status: "Stopped", PipelineExecutionID: "exec-1"},
			},
			true,
		},
		{
			"last stage still in progress",
			[]awscodepipeline.StageStatus{
				{Name: "Source", Status: "Succeeded", PipelineExecutionID: "exec-1"},
				{Name: "Deploy", Status: "InProgress", PipelineExecutionID: "exec-1"},
			},
			false,
		},
		// Regression coverage for the reported bug: GetPipelineState
		// reports each stage's *last* execution independently, so a
		// stage the current run hasn't reached yet still carries a
		// terminal status left over from a previous, different
		// execution. That must not read as "pipeline finished".
		{
			"downstream stage has a stale Succeeded from a previous execution",
			[]awscodepipeline.StageStatus{
				{Name: "Source", Status: "InProgress", PipelineExecutionID: "exec-2"},
				{Name: "Deploy", Status: "Succeeded", PipelineExecutionID: "exec-1"},
			},
			false,
		},
		{
			"downstream stage has a stale Failed from a previous execution",
			[]awscodepipeline.StageStatus{
				{Name: "Source", Status: "InProgress", PipelineExecutionID: "exec-2"},
				{Name: "Build", Status: "Failed", PipelineExecutionID: "exec-1"},
				{Name: "Deploy", Status: "Succeeded", PipelineExecutionID: "exec-1"},
			},
			false,
		},
		{
			"current execution's own last stage succeeded despite an older stale Failed",
			[]awscodepipeline.StageStatus{
				{Name: "Source", Status: "Succeeded", PipelineExecutionID: "exec-2"},
				{Name: "Build", Status: "Succeeded", PipelineExecutionID: "exec-2"},
				{Name: "Deploy", Status: "Succeeded", PipelineExecutionID: "exec-2"},
			},
			true,
		},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			if got := pipelineFinished(c.stages); got != c.want {
				t.Errorf("pipelineFinished(%v) = %v, want %v", c.stages, got, c.want)
			}
		})
	}
}

// fakeNotifier records desktopNotify-shaped calls for assertions,
// without ever touching a real OS notification.
type fakeNotifier struct {
	calls []struct{ title, message string }
}

func (f *fakeNotifier) notify(title, message string) {
	f.calls = append(f.calls, struct{ title, message string }{title, message})
}

func newTestPipelineWatcher(t *testing.T) (*fakeViewHost, *fakeNotifier, *PipelineWatcher) {
	t.Helper()
	host := newFakeViewHost()
	fn := &fakeNotifier{}
	listV := NewCodePipelineListView(host, func(string) {})
	detailV := NewCodePipelineDetailView(host, func() {})
	return host, fn, NewPipelineWatcher(host, fn.notify, listV, detailV)
}

func TestHandlePipelinePollFirstPollEstablishesSilentBaseline(t *testing.T) {
	_, fn, w := newTestPipelineWatcher(t)

	// Source InProgress, not Succeeded/Failed/Stopped as the last stage,
	// so pipelineFinished doesn't also fire its own notification —
	// isolating just the "first poll is a silent baseline" behavior.
	w.handlePipelinePoll("my-pipeline", []awscodepipeline.StageStatus{
		{Name: "Source", Status: "InProgress"},
	}, nil)

	if len(fn.calls) != 0 {
		t.Errorf("notify called %d times on the first poll, want 0 (no baseline yet)", len(fn.calls))
	}
	if got := w.lastStages["my-pipeline"]["Source"]; got != "InProgress" {
		t.Errorf("lastStages[my-pipeline][Source] = %q, want %q", got, "InProgress")
	}
}

func TestHandlePipelinePollNotifiesOnChangedStage(t *testing.T) {
	_, fn, w := newTestPipelineWatcher(t)
	w.lastStages["my-pipeline"] = map[string]string{"Source": "Succeeded", "Build": "InProgress", "Deploy": "InProgress"}
	w.watched["my-pipeline"] = make(chan struct{})

	// Deploy stays InProgress (as the last stage) so pipelineFinished
	// doesn't also fire its own notification — isolating just the
	// Build transition.
	w.handlePipelinePoll("my-pipeline", []awscodepipeline.StageStatus{
		{Name: "Source", Status: "Succeeded"},
		{Name: "Build", Status: "Succeeded"},
		{Name: "Deploy", Status: "InProgress"},
	}, nil)

	if len(fn.calls) != 1 {
		t.Fatalf("notify called %d times, want 1", len(fn.calls))
	}
	if fn.calls[0].title != "my-pipeline" || fn.calls[0].message != "Build: Succeeded" {
		t.Errorf("notify call = %+v, want title=%q message=%q", fn.calls[0], "my-pipeline", "Build: Succeeded")
	}
}

func TestHandlePipelinePollErrorStopsWatchAndNotifiesOnce(t *testing.T) {
	_, fn, w := newTestPipelineWatcher(t)
	w.watched["my-pipeline"] = make(chan struct{})

	w.handlePipelinePoll("my-pipeline", nil, errors.New("boom"))

	if len(fn.calls) != 1 {
		t.Fatalf("notify called %d times, want 1", len(fn.calls))
	}
	if w.IsWatchingPipeline("my-pipeline") {
		t.Error("still watching after a poll error, want stopped")
	}
}

func TestHandlePipelinePollStopsWatchWhenFinished(t *testing.T) {
	_, fn, w := newTestPipelineWatcher(t)
	w.lastStages["my-pipeline"] = map[string]string{"Deploy": "InProgress"}
	w.watched["my-pipeline"] = make(chan struct{})

	w.handlePipelinePoll("my-pipeline", []awscodepipeline.StageStatus{
		{Name: "Deploy", Status: "Succeeded", PipelineExecutionID: "exec-1"},
	}, nil)

	if w.IsWatchingPipeline("my-pipeline") {
		t.Error("still watching after the pipeline finished, want stopped")
	}
	foundFinishedNotice := false
	for _, c := range fn.calls {
		if c.message == "Pipeline finished" {
			foundFinishedNotice = true
		}
	}
	if !foundFinishedNotice {
		t.Errorf("notify calls = %+v, want one with message %q", fn.calls, "Pipeline finished")
	}
}

// TestHandlePipelinePollRendersOpenDetailView closes a gap none of the
// pre-move tests covered: a poll landing while detailV is open for the
// polled pipeline must refresh it. detailV.pipelineName is set
// directly (same-package access) rather than via Open(), which would
// spawn its own goroutine and real host call — irrelevant to what
// this test asserts.
func TestHandlePipelinePollRendersOpenDetailView(t *testing.T) {
	_, _, w := newTestPipelineWatcher(t)
	w.detailV.pipelineName = "my-pipeline"

	w.handlePipelinePoll("my-pipeline", []awscodepipeline.StageStatus{
		{Name: "Deploy", Status: "InProgress"},
	}, nil)

	if got := w.detailV.table.GetCell(1, 0).Text; got != "Deploy" {
		t.Errorf("detailV table after poll = %q, want %q", got, "Deploy")
	}
}

// TestStartStopWatchingPipeline calls StartWatchingPipeline directly,
// which spawns a real goroutine (pollPipeline) that blocks on its
// ticker/stop channel — same accepted background-goroutine-in-tests
// precedent as this app's other tests (e.g.
// TestDatadogLogsViewCycleTimeRange), but this test only asserts on
// the synchronous map/channel state StartWatchingPipeline itself
// mutates before the goroutine's first tick could ever fire (the
// ticker's first tick is pipelinePollInterval away), never on anything
// the goroutine does asynchronously.
func TestStartStopWatchingPipeline(t *testing.T) {
	_, _, w := newTestPipelineWatcher(t)

	if w.IsWatchingPipeline("my-pipeline") {
		t.Fatal("IsWatchingPipeline() = true before starting, want false")
	}

	w.StartWatchingPipeline("my-pipeline")
	if !w.IsWatchingPipeline("my-pipeline") {
		t.Error("IsWatchingPipeline() = false after starting, want true")
	}

	// Starting again while already watching must not replace the
	// existing entry (would leak the original goroutine's stop channel).
	existing := w.watched["my-pipeline"]
	w.StartWatchingPipeline("my-pipeline")
	if w.watched["my-pipeline"] != existing {
		t.Error("StartWatchingPipeline() while already watching replaced the stop channel, want no-op")
	}

	w.StopWatchingPipeline("my-pipeline")
	if w.IsWatchingPipeline("my-pipeline") {
		t.Error("IsWatchingPipeline() = true after stopping, want false")
	}
}

// fakeTicker is pollTicker's test double: c is a channel a test sends
// synthetic ticks on directly, so pollPipeline's loop advances without
// waiting on real wall-clock time. stopped closes once Stop() is
// called, letting a test wait on it deterministically instead of
// polling.
type fakeTicker struct {
	c       chan time.Time
	stopped chan struct{}
}

func newFakeTicker() *fakeTicker {
	return &fakeTicker{c: make(chan time.Time, 1), stopped: make(chan struct{})}
}

func (f *fakeTicker) C() <-chan time.Time { return f.c }
func (f *fakeTicker) Stop()               { close(f.stopped) }

// newTestPipelineWatcherWithDrawSignal is newTestPipelineWatcher's
// draw-signaling counterpart — see queues_test.go's drawSignalingHost/
// newTestQueuesViewWithDrawSignal for why this exists: it lets a test
// block until pollPipeline's QueueUpdateDraw dispatch has actually
// landed instead of guessing with a sleep.
func newTestPipelineWatcherWithDrawSignal(t *testing.T, bufSize int) (*drawSignalingHost, *fakeNotifier, *PipelineWatcher) {
	t.Helper()
	base := newFakeViewHost()
	host := &drawSignalingHost{fakeViewHost: base, drawn: make(chan struct{}, bufSize)}
	fn := &fakeNotifier{}
	listV := NewCodePipelineListView(host, func(string) {})
	detailV := NewCodePipelineDetailView(host, func() {})
	return host, fn, NewPipelineWatcher(host, fn.notify, listV, detailV)
}

// TestPollPipelineTicksDispatchPolls is the actual regression coverage
// for pollPipeline's loop itself — see TestStartStopWatchingPipeline's
// own comment above: it only ever asserts on state set before the
// first real tick, which is pipelinePollInterval (20s) away, so the
// loop's tick-driven behavior has never actually been exercised.
// Substituting a fakeTicker lets ticks be sent synchronously, proving
// pollPipeline dispatches a poll via QueueUpdateDraw on each one — more
// than once, so the loop is shown to continue rather than fire a
// single poll and stop — and that Stop() reaches the injected ticker
// once the watch ends.
func TestPollPipelineTicksDispatchPolls(t *testing.T) {
	host, _, w := newTestPipelineWatcherWithDrawSignal(t, 4)

	ft := newFakeTicker()
	w.newTicker = func(time.Duration) pollTicker { return ft }

	var mu sync.Mutex
	pollCalls := 0
	host.getPipelineStateFn = func(context.Context, string, string) ([]awscodepipeline.StageStatus, error) {
		mu.Lock()
		pollCalls++
		mu.Unlock()
		return []awscodepipeline.StageStatus{{Name: "Source", Status: "InProgress"}}, nil
	}

	w.StartWatchingPipeline("my-pipeline")

	ft.c <- time.Now()
	<-host.drawn
	mu.Lock()
	got := pollCalls
	mu.Unlock()
	if got != 1 {
		t.Fatalf("pollCalls after first tick = %d, want 1", got)
	}

	ft.c <- time.Now()
	<-host.drawn
	mu.Lock()
	got = pollCalls
	mu.Unlock()
	if got != 2 {
		t.Fatalf("pollCalls after second tick = %d, want 2 (the loop must continue, not fire once and stop)", got)
	}

	w.StopWatchingPipeline("my-pipeline")
	select {
	case <-ft.stopped:
	case <-time.After(2 * time.Second):
		t.Error("fakeTicker.Stop() was not called after StopWatchingPipeline")
	}
}
