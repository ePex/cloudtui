package app

import (
	"errors"
	"testing"

	"github.com/ePex/cloudtui/tui/internal/awscodepipeline"
	"github.com/ePex/cloudtui/tui/internal/config"
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

func TestHandlePipelinePollFirstPollEstablishesSilentBaseline(t *testing.T) {
	a := New(config.Default())
	fn := &fakeNotifier{}
	a.notify = fn.notify

	// Source InProgress, not Succeeded/Failed/Stopped as the last stage,
	// so pipelineFinished doesn't also fire its own notification —
	// isolating just the "first poll is a silent baseline" behavior.
	a.handlePipelinePoll("my-pipeline", []awscodepipeline.StageStatus{
		{Name: "Source", Status: "InProgress"},
	}, nil)

	if len(fn.calls) != 0 {
		t.Errorf("notify called %d times on the first poll, want 0 (no baseline yet)", len(fn.calls))
	}
	if got := a.lastPipelineStages["my-pipeline"]["Source"]; got != "InProgress" {
		t.Errorf("lastPipelineStages[my-pipeline][Source] = %q, want %q", got, "InProgress")
	}
}

func TestHandlePipelinePollNotifiesOnChangedStage(t *testing.T) {
	a := New(config.Default())
	fn := &fakeNotifier{}
	a.notify = fn.notify
	a.lastPipelineStages["my-pipeline"] = map[string]string{"Source": "Succeeded", "Build": "InProgress", "Deploy": "InProgress"}
	a.watchedPipelines["my-pipeline"] = make(chan struct{})

	// Deploy stays InProgress (as the last stage) so pipelineFinished
	// doesn't also fire its own notification — isolating just the
	// Build transition.
	a.handlePipelinePoll("my-pipeline", []awscodepipeline.StageStatus{
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
	a := New(config.Default())
	fn := &fakeNotifier{}
	a.notify = fn.notify
	a.watchedPipelines["my-pipeline"] = make(chan struct{})

	a.handlePipelinePoll("my-pipeline", nil, errors.New("boom"))

	if len(fn.calls) != 1 {
		t.Fatalf("notify called %d times, want 1", len(fn.calls))
	}
	if a.IsWatchingPipeline("my-pipeline") {
		t.Error("still watching after a poll error, want stopped")
	}
}

func TestHandlePipelinePollStopsWatchWhenFinished(t *testing.T) {
	a := New(config.Default())
	fn := &fakeNotifier{}
	a.notify = fn.notify
	a.lastPipelineStages["my-pipeline"] = map[string]string{"Deploy": "InProgress"}
	a.watchedPipelines["my-pipeline"] = make(chan struct{})

	a.handlePipelinePoll("my-pipeline", []awscodepipeline.StageStatus{
		{Name: "Deploy", Status: "Succeeded", PipelineExecutionID: "exec-1"},
	}, nil)

	if a.IsWatchingPipeline("my-pipeline") {
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
	a := New(config.Default())

	if a.IsWatchingPipeline("my-pipeline") {
		t.Fatal("IsWatchingPipeline() = true before starting, want false")
	}

	a.StartWatchingPipeline("my-pipeline")
	if !a.IsWatchingPipeline("my-pipeline") {
		t.Error("IsWatchingPipeline() = false after starting, want true")
	}

	// Starting again while already watching must not replace the
	// existing entry (would leak the original goroutine's stop channel).
	existing := a.watchedPipelines["my-pipeline"]
	a.StartWatchingPipeline("my-pipeline")
	if a.watchedPipelines["my-pipeline"] != existing {
		t.Error("StartWatchingPipeline() while already watching replaced the stop channel, want no-op")
	}

	a.StopWatchingPipeline("my-pipeline")
	if a.IsWatchingPipeline("my-pipeline") {
		t.Error("IsWatchingPipeline() = true after stopping, want false")
	}
}
