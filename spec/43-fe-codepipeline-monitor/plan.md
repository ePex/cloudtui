# Plan — FE 43

## `internal/awscodepipeline`

```go
type Pipeline struct {
    Name    string
    Created time.Time
    Updated time.Time
}

type StageStatus struct {
    Name                string
    Status              string // "InProgress", "Succeeded", "Failed", "Stopped", "Stopping", or "" if the stage has never run
    PipelineExecutionID string // which execution Status belongs to — see bugfix note below
}

func ListPipelines(ctx context.Context, profile string) ([]Pipeline, error)
func GetPipelineState(ctx context.Context, profile, pipelineName string) ([]StageStatus, error)
```

`ListPipelines` uses `codepipeline.NewListPipelinesPaginator` (standard
AWS SDK v2 pagination, same idiom as every other paginated call already
in this repo). `GetPipelineState` maps `output.StageStates[].StageName`
+ `.LatestExecution.Status` — `LatestExecution` can be nil for a stage
that has never run; treat that as `Status: ""`. Split the raw-response
→ `[]StageStatus` mapping into its own function (`buildStageStatuses`),
same pattern as `awslogs.buildLogEvents`/`datadoglogs.buildLogEvents`,
so it's unit-testable without a real API call.

`newClient(ctx, profile)` guard mirrors every other `awsXxx` package
(`profile == ""` → clear error before ever calling AWS).

## `internal/app` — views

`codepipelinelist.go`: `codePipelineListView` — registered `ui.View`
("codepipeline" in Home's "Apps" section), table (Name / Created /
Updated / a "▶" watching-indicator column), filterable by name
(standard local substring filter, same as every other list view — this
data isn't server-searched). `w` on a selected row toggles watching
that pipeline directly from the list, no need to open detail first.
`Enter` opens the detail view.

`codepipelinedetail.go`: `codePipelineDetailView` — not a registered
view (opened via `App.openCodePipelineDetail`, mirrors
`logSearchView`'s non-registered shape), table of stages (Stage /
Status), `w` toggles watching this pipeline, `r` manual refresh
(one-shot `GetPipelineState`, independent of any active watch), `Esc`
back to the list.

## `internal/app` — watcher mechanism (`codepipelinewatch.go`)

```go
// App fields:
watchedPipelines   map[string]chan struct{}    // pipeline name -> stop signal; main-goroutine-owned
lastPipelineStages map[string]map[string]string // pipeline name -> stage name -> last known status; main-goroutine-owned
notify             func(title, message string)  // real impl wired in New(), fakeable in tests
listPipelines      func(ctx context.Context, profile string) ([]awscodepipeline.Pipeline, error)
getPipelineState   func(ctx context.Context, profile, name string) ([]awscodepipeline.StageStatus, error)
```

Both maps and the map of stop channels are **only ever touched from the
main goroutine** — either directly (key handlers, which tview already
runs on the main goroutine) or from inside a `tv.QueueUpdateDraw`
callback. No mutex needed; same single-writer discipline as every other
background-goroutine feature in this app.

```go
const pipelinePollInterval = 20 * time.Second

func (a *App) startWatchingPipeline(name string) {
    if _, already := a.watchedPipelines[name]; already {
        return
    }
    stop := make(chan struct{})
    a.watchedPipelines[name] = stop
    profile := a.cfg.ActiveAWSProfile // captured once, never re-read from a.cfg inside the goroutine
    go a.pollPipeline(name, profile, stop)
}

func (a *App) stopWatchingPipeline(name string) {
    if stop, ok := a.watchedPipelines[name]; ok {
        close(stop)
        delete(a.watchedPipelines, name)
    }
}

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
            nil, // no in-progress status message needed here — this isn't a visible search view
            func(ctx context.Context) ([]awscodepipeline.StageStatus, error) {
                return a.getPipelineState(ctx, profile, name)
            },
        )
        a.tv.QueueUpdateDraw(func() {
            a.handlePipelinePoll(name, stages, err)
        })
    }
}
```

`handlePipelinePoll` (always runs inside `QueueUpdateDraw`, so free to
touch `a.watchedPipelines`/`a.lastPipelineStages`/call `a.notify`
directly):

```go
func (a *App) handlePipelinePoll(name string, stages []awscodepipeline.StageStatus, err error) {
    if err != nil {
        slog.Error("codepipeline: poll failed", "pipeline", name, "error", err)
        a.notify("Stopped watching "+name, err.Error())
        a.stopWatchingPipeline(name)
        return
    }

    prev := a.lastPipelineStages[name]
    for _, msg := range stageTransitions(prev, stages) {
        a.notify(name, msg)
    }
    a.lastPipelineStages[name] = snapshotStages(stages)

    if pipelineFinished(stages) {
        a.notify(name, "Pipeline finished")
        a.stopWatchingPipeline(name)
    }

    if a.codePipelineDetailV != nil && a.codePipelineDetailV.pipelineName == name {
        a.codePipelineDetailV.render(stages)
    }
    a.refreshCodePipelineListWatchIndicators()
}
```

Pure, independently-testable helpers (no goroutines, no AWS, no tview):

```go
// stageTransitions returns one message per stage whose status differs
// from prev (keyed by stage name). prev == nil means "no baseline yet"
// (the first poll after starting a watch) — returns no messages, so
// the initial snapshot doesn't spuriously "transition" from nothing.
func stageTransitions(prev map[string]string, stages []awscodepipeline.StageStatus) []string

// snapshotStages converts the current poll into the map format stored
// in lastPipelineStages.
func snapshotStages(stages []awscodepipeline.StageStatus) map[string]string

// pipelineFinished reports whether the pipeline execution has reached a
// terminal state: any stage (belonging to the current execution) is
// Failed/Stopped, or the last stage in the list (also belonging to the
// current execution) is Succeeded.
func pipelineFinished(stages []awscodepipeline.StageStatus) bool
```

**Bugfix (found live, post-ship)**: `GetPipelineState` reports each
stage's *last* execution independently — a stage the current run hasn't
reached yet still carries whatever status it had from a *previous*
execution (e.g. Deploy still shows `Succeeded` from last time while
Source is `InProgress` this time). The original `pipelineFinished` read
`Status` across all stages at face value, so it reported "Pipeline
finished" the moment any downstream stage happened to have a stale
terminal status — a false positive while the pipeline was still actively
running. Fixed by adding `PipelineExecutionID` to `StageStatus`
(`LatestExecution.PipelineExecutionId`) and having `pipelineFinished`
only consider stages whose `PipelineExecutionID` matches the first
stage's (a new execution always starts at the first stage, so that
stage's ID is always the current run's).

## Notification helper (`internal/app/notify.go`)

```go
func desktopNotify(title, message string) {
    if err := beeep.Notify(title, message, ""); err != nil {
        slog.Warn("desktop notification failed", "error", err)
    }
}
```
Wired as `a.notify = desktopNotify` in `New()`, same DI pattern as every
AWS call — tests set `a.notify` to a recording fake so `go test` never
pops a real OS notification.

## Testing

- `internal/awscodepipeline`: `buildStageStatuses`/pagination-mapping
  unit tests (no real API call — same shape as `datadoglogs`'s
  `buildLogEvents` tests); `newClient` empty-profile guard test.
- `internal/app`: `stageTransitions`/`snapshotStages`/`pipelineFinished`
  table-driven unit tests (pure functions, no goroutines). `handlePipelinePoll`
  tested directly (it's the same "split the goroutine's actual logic out
  so it's callable without a goroutine or a running tview event loop"
  pattern every other view's `handleSearchResult`-equivalent already
  uses) with a fake `a.notify` recording calls, covering: first poll
  establishes a silent baseline; a later poll with a changed stage
  notifies; a poll error stops the watch and notifies once; the pipeline
  reaching a terminal state stops the watch and notifies. `w` toggle
  behavior (start/stop watching, `watchedPipelines` map state) tested
  directly on `startWatchingPipeline`/`stopWatchingPipeline` — starting
  spawns a real goroutine (acceptable background-goroutine-leak-in-tests
  precedent already established elsewhere in this app's tests, e.g.
  `TestDatadogLogsViewCycleTimeRange`), but the test only asserts on the
  synchronous state mutation (the map gaining/losing an entry, the
  channel closing), never on anything the goroutine itself does
  asynchronously — same discipline as this codebase's other
  goroutine-adjacent tests.
- Manual verification (per `tui/CLAUDE.md`): watch a real pipeline
  through at least one real stage transition, confirm a desktop
  notification appears while a *different* cloudtui view is focused;
  confirm the detail view live-updates if left open; confirm watching
  auto-stops with a final notification once the pipeline finishes.
