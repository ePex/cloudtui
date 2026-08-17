package app

import (
	"context"
	"log/slog"
	"time"

	"github.com/ePex/cloudtui/tui/internal/awsauth"
	"github.com/ePex/cloudtui/tui/internal/awscodepipeline"
)

// pipelinePollInterval is fixed, not user-configurable in this slice —
// frequent enough to catch a stage transition promptly for a
// minutes-scale build, infrequent enough not to hammer the API. See
// spec/43-fe-codepipeline-monitor decision 9.
const pipelinePollInterval = 20 * time.Second

// IsWatchingPipeline reports whether name currently has an active
// background watch.
func (a *App) IsWatchingPipeline(name string) bool {
	_, ok := a.watchedPipelines[name]
	return ok
}

// StartWatchingPipeline begins polling name's stage state every
// pipelinePollInterval until it either reaches a terminal state or is
// explicitly stopped. A no-op if already watching name. Only ever
// called from the main goroutine (a key handler, or code already
// running inside a tv.QueueUpdateDraw callback) — watchedPipelines and
// lastPipelineStages are otherwise untouched by any other goroutine, so
// no mutex is needed (see spec/43-fe-codepipeline-monitor's watcher
// section for the full reasoning).
func (a *App) StartWatchingPipeline(name string) {
	if a.IsWatchingPipeline(name) {
		return
	}
	stop := make(chan struct{})
	a.watchedPipelines[name] = stop
	// Captured once here, on the main goroutine, and passed into
	// pollPipeline by value — never re-read from a.cfg inside the
	// goroutine, which would otherwise race the main goroutine's own
	// reads/writes of a.cfg (e.g. switching AWS profiles). Same
	// discipline as datadogLogsView.search() capturing cfg.Datadog once
	// before spawning its own goroutine.
	profile := a.cfg.ActiveAWSProfile
	go a.pollPipeline(name, profile, stop)
}

// StopWatchingPipeline stops name's background watch, if any. A no-op
// if not currently watching name.
func (a *App) StopWatchingPipeline(name string) {
	if stop, ok := a.watchedPipelines[name]; ok {
		close(stop)
		delete(a.watchedPipelines, name)
	}
}

// pollPipeline runs on its own goroutine for the lifetime of one watch.
// All AWS calls happen here; all state mutation and UI work happens
// inside handlePipelinePoll, dispatched via tv.QueueUpdateDraw — the
// same single-writer pattern every other background-goroutine feature
// in this app already uses for its own search()-style calls.
func (a *App) pollPipeline(name, profile string, stop chan struct{}) {
	ticker := time.NewTicker(pipelinePollInterval)
	defer ticker.Stop()
	for {
		select {
		case <-stop:
			return
		case <-ticker.C:
		}

		ctx := context.Background()
		authType, _ := a.awsAuthTypeFor(ctx, profile)
		stages, err := awsauth.WithReauth(ctx, profile, authType, a.awsSSOLogin,
			nil, // no in-progress status message — this isn't a visible search view
			func(ctx context.Context) ([]awscodepipeline.StageStatus, error) {
				return a.getPipelineState(ctx, profile, name)
			},
		)
		a.tv.QueueUpdateDraw(func() {
			a.handlePipelinePoll(name, stages, err)
		})
	}
}

// handlePipelinePoll processes one poll's outcome: on error, notifies
// once and stops the watch (rather than silently retrying forever — if
// e.g. the pipeline was deleted, or auth is broken in a way WithReauth
// couldn't fix, the user should find out, not just stop getting
// updates without knowing why). On success, notifies for every stage
// whose status changed since the last poll, stops the watch once the
// pipeline execution reaches a terminal state, and live-refreshes the
// detail view if it's currently open for this pipeline. Split out from
// pollPipeline so this — the part with actual logic to get wrong — is
// directly testable without a goroutine, a ticker, or a real AWS call
// (same "handleSearchResult" pattern every other view in this app
// already uses).
func (a *App) handlePipelinePoll(name string, stages []awscodepipeline.StageStatus, err error) {
	if err != nil {
		slog.Error("codepipeline: poll failed", "pipeline", name, "error", err)
		a.notify("Stopped watching "+name, err.Error())
		a.StopWatchingPipeline(name)
		return
	}

	prev := a.lastPipelineStages[name]
	for _, msg := range stageTransitions(prev, stages) {
		a.notify(name, msg)
	}
	a.lastPipelineStages[name] = snapshotStages(stages)

	if pipelineFinished(stages) {
		a.notify(name, "Pipeline finished")
		a.StopWatchingPipeline(name)
	}

	if a.codePipelineDetailV != nil && a.codePipelineDetailV.pipelineName == name {
		a.codePipelineDetailV.render(stages)
	}
	if a.codePipelineListV != nil {
		a.codePipelineListV.repaint(a.codePipelineListV.all)
	}
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
			messages = append(messages, s.Name+": "+statusLabel(s.Status))
		}
	}
	return messages
}

// snapshotStages converts the current poll into the map format stored
// in lastPipelineStages.
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
