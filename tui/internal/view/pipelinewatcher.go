package view

import (
	"context"
	"log/slog"
	"time"

	"github.com/ePex/cloudtui/tui/internal/awsauth"
	"github.com/ePex/cloudtui/tui/internal/awscodepipeline"
	"github.com/ePex/cloudtui/tui/internal/ui"
)

// pipelinePollInterval is fixed, not user-configurable in this slice —
// frequent enough to catch a stage transition promptly for a
// minutes-scale build, infrequent enough not to hammer the API. See
// spec/43-fe-codepipeline-monitor decision 9.
const pipelinePollInterval = 20 * time.Second

// pollTicker abstracts *time.Ticker's C/Stop surface so pollPipeline's
// loop can be driven deterministically in tests instead of waiting on
// real wall-clock time.
type pollTicker interface {
	C() <-chan time.Time
	Stop()
}

// realPollTicker adapts *time.Ticker to pollTicker — time.Ticker's C is
// a field, not a method, so it can't satisfy the interface directly.
type realPollTicker struct {
	*time.Ticker
}

func (t *realPollTicker) C() <-chan time.Time { return t.Ticker.C }

// PipelineWatcher owns the background poll loop that keeps a
// CodePipeline's stage status current once the user starts watching
// it: notifying on stage transitions, stopping automatically once the
// pipeline execution reaches a terminal state, and repainting
// listV/detailV directly since both need a live refresh whenever a
// poll completes, regardless of which one the user is currently on.
type PipelineWatcher struct {
	watched    map[string]chan struct{}
	lastStages map[string]map[string]string
	host       ui.CodePipelineHost
	notify     func(title, message string)
	listV      *CodePipelineListView
	detailV    *CodePipelineDetailView
	newTicker  func(d time.Duration) pollTicker
}

// NewPipelineWatcher constructs a PipelineWatcher driving listV/detailV.
func NewPipelineWatcher(host ui.CodePipelineHost, notify func(title, message string), listV *CodePipelineListView, detailV *CodePipelineDetailView) *PipelineWatcher {
	return &PipelineWatcher{
		watched:    map[string]chan struct{}{},
		lastStages: map[string]map[string]string{},
		host:       host,
		notify:     notify,
		listV:      listV,
		detailV:    detailV,
		newTicker: func(d time.Duration) pollTicker {
			return &realPollTicker{time.NewTicker(d)}
		},
	}
}

// IsWatchingPipeline reports whether name currently has an active
// background watch.
func (w *PipelineWatcher) IsWatchingPipeline(name string) bool {
	_, ok := w.watched[name]
	return ok
}

// StartWatchingPipeline begins polling name's stage state every
// pipelinePollInterval until it either reaches a terminal state or is
// explicitly stopped. A no-op if already watching name. Only ever
// called from the main goroutine (a key handler, or code already
// running inside a QueueUpdateDraw callback) — watched and lastStages
// are otherwise untouched by any other goroutine, so no mutex is
// needed (see spec/43-fe-codepipeline-monitor's watcher section for
// the full reasoning).
func (w *PipelineWatcher) StartWatchingPipeline(name string) {
	if w.IsWatchingPipeline(name) {
		return
	}
	stop := make(chan struct{})
	w.watched[name] = stop
	// Captured once here, on the main goroutine, and passed into
	// pollPipeline by value — never re-read from host.Config() inside
	// the goroutine, which would otherwise race the main goroutine's
	// own reads/writes of config (e.g. switching AWS profiles). Same
	// discipline as datadogLogsView.search() capturing cfg.Datadog once
	// before spawning its own goroutine.
	profile := w.host.Config().ActiveAWSProfile
	go w.pollPipeline(name, profile, stop)
}

// StopWatchingPipeline stops name's background watch, if any. A no-op
// if not currently watching name.
func (w *PipelineWatcher) StopWatchingPipeline(name string) {
	if stop, ok := w.watched[name]; ok {
		close(stop)
		delete(w.watched, name)
	}
}

// pollPipeline runs on its own goroutine for the lifetime of one watch.
// All AWS calls happen here; all state mutation and UI work happens
// inside handlePipelinePoll, dispatched via QueueUpdateDraw — the same
// single-writer pattern every other background-goroutine feature in
// this app already uses for its own search()-style calls.
func (w *PipelineWatcher) pollPipeline(name, profile string, stop chan struct{}) {
	ticker := w.newTicker(pipelinePollInterval)
	defer ticker.Stop()
	for {
		select {
		case <-stop:
			return
		case <-ticker.C():
		}

		ctx := context.Background()
		authType, _ := w.host.AWSAuthTypeFor(ctx, profile)
		stages, err := awsauth.WithReauth(ctx, profile, authType, w.host.AWSSSOLogin,
			nil, // no in-progress status message — this isn't a visible search view
			nil, // ...and so no device code/URL display either, for the same reason
			func(ctx context.Context) ([]awscodepipeline.StageStatus, error) {
				return w.host.GetPipelineState(ctx, profile, name)
			},
		)
		w.host.QueueUpdateDraw(func() {
			w.handlePipelinePoll(name, stages, err)
		})
	}
}

// handlePipelinePoll processes one poll's outcome: on error, notifies
// once and stops the watch (rather than silently retrying forever — if
// e.g. the pipeline was deleted, or auth is broken in a way WithReauth
// couldn't fix, the user should find out, not just stop getting
// updates without knowing why). On success, notifies for every stage
// whose status changed since the last poll, stops the watch once the
// pipeline execution reaches a terminal state, and live-refreshes
// detailV if it's currently open for this pipeline. Split out from
// pollPipeline so this — the part with actual logic to get wrong — is
// directly testable without a goroutine, a ticker, or a real AWS call
// (same "handleSearchResult" pattern every other view in this app
// already uses).
func (w *PipelineWatcher) handlePipelinePoll(name string, stages []awscodepipeline.StageStatus, err error) {
	if err != nil {
		slog.Error("codepipeline: poll failed", "pipeline", name, "error", err)
		w.notify("Stopped watching "+name, err.Error())
		w.StopWatchingPipeline(name)
		return
	}

	prev := w.lastStages[name]
	for _, msg := range stageTransitions(prev, stages) {
		w.notify(name, msg)
	}
	w.lastStages[name] = snapshotStages(stages)

	if pipelineFinished(stages) {
		w.notify(name, "Pipeline finished")
		w.StopWatchingPipeline(name)
	}

	if w.detailV.PipelineName() == name {
		w.detailV.Render(stages)
	}
	w.listV.Repaint()
}

// stageTransitions returns one human-readable message per stage whose
// status differs from prev (keyed by stage name). prev == nil means "no
// baseline yet" — the first poll after starting a watch — and returns
// no messages, so establishing the initial snapshot doesn't spuriously
// "transition" from nothing.
func stageTransitions(prev map[string]string, stages []awscodepipeline.StageStatus) []string {
	if prev == nil {
		return nil
	}
	var messages []string
	for _, s := range stages {
		if prev[s.Name] != s.Status {
			messages = append(messages, s.Name+": "+StatusLabel(s.Status))
		}
	}
	return messages
}

// snapshotStages converts the current poll into the map format stored
// in lastStages.
func snapshotStages(stages []awscodepipeline.StageStatus) map[string]string {
	snapshot := make(map[string]string, len(stages))
	for _, s := range stages {
		snapshot[s.Name] = s.Status
	}
	return snapshot
}

// pipelineFinished reports whether the pipeline execution has reached a
// terminal state: any stage Failed or Stopped (the pipeline won't
// progress further on its own), or the last stage in the list
// Succeeded (the pipeline completed).
//
// GetPipelineState reports each stage's *last* execution independently
// (see StageStatus's doc comment) — a stage the current run hasn't
// reached yet still carries whatever status (possibly Succeeded or
// Failed) it had from a previous execution. Naively reading Status
// across all stages would then report an actively-running pipeline as
// finished the moment any downstream stage happens to have a stale
// terminal status left over from its last run. A new execution always
// starts at the first stage, so that stage's PipelineExecutionID is
// always the current run's ID — stages from any other execution are
// ignored here as "not yet reached by this run".
func pipelineFinished(stages []awscodepipeline.StageStatus) bool {
	if len(stages) == 0 {
		return false
	}
	currentExecution := stages[0].PipelineExecutionID
	if currentExecution == "" {
		return false
	}
	for _, s := range stages {
		if s.PipelineExecutionID != currentExecution {
			continue
		}
		if s.Status == "Failed" || s.Status == "Stopped" {
			return true
		}
	}
	last := stages[len(stages)-1]
	return last.PipelineExecutionID == currentExecution && last.Status == "Succeeded"
}
