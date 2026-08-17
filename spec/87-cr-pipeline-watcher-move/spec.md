# Spec — CR 87: extract the CodePipeline background watcher out of `internal/app`

Date: 2026-08-17

## Background

Phase 4's original scope (`spec/64`) lists `codepipelinewatch.go`
alongside `log.go` as the last 2 files left after CR 84/85's moves.
CR 86 handled `log.go`. CR 79 already flagged this file as different
from the rest: "a problem for the future accessor-methods CR" — no
view type of its own to move, unlike every other file in phase 4.

Auditing it fresh confirms that and adds detail:

**1. It isn't a view.** `codepipelinewatch.go` is 6 free functions/
methods on `(a *App)` — `IsWatchingPipeline`, `StartWatchingPipeline`,
`StopWatchingPipeline`, `pollPipeline`, `handlePipelinePoll`, plus 3
package-level helpers (`stageTransitions`, `snapshotStages`,
`pipelineFinished`). No `Name()`/`Title()`/`Primitive()` — nothing
shaped like the 16 types CR 82-86 already moved. It's a headless
background poller: on `StartWatchingPipeline`, a goroutine ticks every
`pipelinePollInterval`, calls AWS, and dispatches the result back onto
the UI goroutine via `QueueUpdateDraw`.

**2. Most of what it calls is already on `ui.ViewHost`.**
`QueueUpdateDraw`, `Config()`, `GetPipelineState`, `AWSAuthTypeFor`,
`AWSSSOLogin` are all already exported there (CR 82's adoption already
covers these methods generically) — this file just hasn't been
switched over to use them, since it never moved out of `internal/app`
in the first place.

**3. Three things are genuinely private, and are the real blocker:**

- `a.watchedPipelines map[string]chan struct{}` and
  `a.lastPipelineStages map[string]map[string]string` (`app.go:98-99`)
  — plain map fields living directly on `*App`, read/written only by
  this file. This is the watcher's own state; it just doesn't have
  anywhere of its own to live today.
- `a.notify func(title, message string)` (`app.go:110`, assigned
  `ui.DesktopNotify` at construction, `app.go:191`) — a private func
  field, called only from this file, and **not exposed on
  `ui.ViewHost`/`ui.Host` at all** — unlike everything else this file
  touches, there's no existing accessor to switch to.
- Direct reach-ins into 2 sibling views:
  `a.codePipelineDetailV.PipelineName()`/`.Render(stages)` and
  `a.codePipelineListV.Repaint()`. Both methods are already exported
  (confirmed: `CodePipelineDetailView.PipelineName()`/`.Render()` and
  `CodePipelineListView.Repaint()` all exist in
  `internal/view/codepipelinedetail.go`/`codepipelinelist.go` today)
  — this is a question of who holds the pointers, not a missing
  export.

**4. `IsWatchingPipeline`/`StartWatchingPipeline`/`StopWatchingPipeline`
are load-bearing on `ui.ViewHost` today.**
`codepipelinelist.go`'s `toggleWatchSelected()` and
`codepipelinedetail.go`'s `toggleWatch()` both already call
`host.IsWatchingPipeline(...)` etc. through the interface. Whatever
this CR does, `*App` must keep satisfying these 3 methods — either by
keeping the logic itself (status quo) or by thin trampolines
delegating to wherever the logic actually ends up.

**5. The existing 9 tests reach far deeper into `*App` than any file
moved in CR 82-86** — `a.notify = fn.notify` (overwriting the private
func field directly), `a.lastPipelineStages["x"] = ...`,
`a.watchedPipelines["x"] = ...`, and a direct call to the unexported
`a.handlePipelinePoll(...)`. None of them use `fakeViewHost` — they
all construct a real `a := New(config.Default())` and poke its
internals.

## Problem

None of `a.watchedPipelines`/`a.lastPipelineStages`/`a.notify`/
`a.codePipelineDetailV`/`a.codePipelineListV` are reachable from a
type living outside `internal/app` — 2 are unexported state with
nowhere else to live, 1 is a private func field with no interface
exposure, and 2 are direct concrete-sibling-view reach-ins. This is a
materially different shape than every prior CR in phase 4: those all
had a pre-existing (if thin) view type to export and relocate; this
file has no such type at all — the state and behavior have always
lived directly on `*App`.

## Solution

Introduce a new type, `view.PipelineWatcher`, in `internal/view`
(alongside the 2 CodePipeline views it drives) that owns the watcher's
state and behavior:

1. **New struct**: `watched map[string]chan struct{}`,
   `lastStages map[string]map[string]string]`, `host ui.ViewHost`,
   `notify func(title, message string)`, `listV *CodePipelineListView`,
   `detailV *CodePipelineDetailView`. `listV`/`detailV` are held as
   concrete same-package pointers (not interfaces) — same pattern CR
   84 already uses for e.g. `logDetailView`'s `onBack` closure reaching
   a sibling view's fields, just via direct struct fields instead of a
   closure, since here the relationship is permanent (one watcher
   always drives the same 2 views) rather than a single callback.
2. **Constructor**: `NewPipelineWatcher(host ui.ViewHost, notify
   func(string, string), listV *CodePipelineListView, detailV
   *CodePipelineDetailView) *PipelineWatcher`. `notify` is injected
   directly (mirroring how `internal/dialog` types take their
   `confirm`/etc. dependencies) rather than added to `ui.ViewHost` —
   nothing else in the app needs a `Notify` capability, so extending
   the interface for one caller would be a wider change than this CR
   needs.
3. **Methods**: `IsWatchingPipeline`/`StartWatchingPipeline`/
   `StopWatchingPipeline`/`pollPipeline`/`handlePipelinePoll` move onto
   `*PipelineWatcher` unchanged in logic, using `w.host.X(...)` in
   place of `a.X(...)` throughout. `stageTransitions`/`snapshotStages`/
   `pipelineFinished` move unchanged (package-level helpers, no
   receiver either way).
4. **`*App` keeps satisfying `ui.ViewHost`** via 3 one-line
   trampolines: `func (a *App) IsWatchingPipeline(name string) bool {
   return a.pipelineWatcher.IsWatchingPipeline(name) }` (and the same
   shape for Start/Stop) — same "fold logic into the owning type, keep
   a thin forwarding method where interface conformance requires it"
   pattern CR 85 already used for `Refresh()`.
5. **`app.go` construction**: `a.pipelineWatcher =
   view.NewPipelineWatcher(a, a.notify, a.codePipelineListV,
   a.codePipelineDetailV)`, added after both CodePipeline views are
   constructed (a real ordering dependency, same shape as CR 85's
   Settings needing its 4 dialogs built first).

## Scope

### In scope

- New `internal/view/pipelinewatcher.go` holding `PipelineWatcher`
  per Solution.
- `internal/app/codepipelinewatch.go`: reduced to 3 trampoline methods
  (`IsWatchingPipeline`/`StartWatchingPipeline`/`StopWatchingPipeline`)
  delegating to `a.pipelineWatcher`; everything else deleted (moved,
  not duplicated).
- `app.go`: `watchedPipelines`/`lastPipelineStages` fields removed;
  new `pipelineWatcher *view.PipelineWatcher` field added and
  constructed after `codePipelineListV`/`codePipelineDetailV`.
- `internal/ui/viewhost.go`: update the "CodePipeline background
  watcher (implemented by App's codepipelinewatch.go)" doc comment —
  it's now implemented by `view.PipelineWatcher`, with `*App` only
  forwarding.
- Existing 9 tests: the pure-logic ones
  (`TestStageTransitions*`/`TestSnapshotStages`/`TestPipelineFinished`)
  move to `internal/view/pipelinewatcher_test.go` unchanged (no `*App`
  involved today either). The `*App`-reaching ones
  (`TestHandlePipelinePoll*`/`TestStartStopWatchingPipeline`) are
  rewritten against `PipelineWatcher` directly, constructed via
  `fakeViewHost` + fake `CodePipelineListView`/`CodePipelineDetailView`
  instances, matching every other CR 82-86 test's construction
  pattern — this is genuinely new test-shape work, not a port, since
  today's tests only work because they poke a real `*App`.
- `gofmt`/`go vet`/`go build`/`go test` clean; live verification.

### Out of scope

- Any change to `ui.ViewHost`'s public surface — `IsWatchingPipeline`/
  `StartWatchingPipeline`/`StopWatchingPipeline`'s signatures are
  unchanged; only their implementation moves.
- Adding `Notify` to `ui.Host`/`ui.ViewHost` — `notify` stays a
  constructor-injected func, per Solution point 2.
- Phase 5 (`connectionsecrets.go`'s `secretBackend`) — unrelated,
  still backlogged.
- Any behavior change — polling interval, notification text,
  stage-transition logic, and the finished-pipeline detection are all
  relocated verbatim.

### Live verification

This is the one file in phase 4 whose live behavior genuinely can't be
observed in a single quick pass — it depends on watching a real
pipeline transition stages over real time (`pipelinePollInterval` is
20s). `verify-live`: start a watch on a real pipeline from both the
list (`w`) and the detail view (`w`), confirm the WATCHING
column/title update immediately in both places, confirm `IsWatching`
state stays consistent between the two views (since they're reading
through the same `ui.ViewHost` methods), and stop the watch from
each. If no pipeline in the connected AWS account is actively running
during verification, a full stage-transition/notification/auto-stop
cycle may not be observable live — same data-availability caveat CR 83/
84 already recorded for CodePipeline (empty account, watcher exercised
via code inspection instead for the parts an idle pipeline can't
trigger).

## Definition of done

1. `internal/view/pipelinewatcher.go` holds `PipelineWatcher`, owning
   its own watch state; `internal/app/codepipelinewatch.go` holds only
   3 trampoline methods.
2. `internal/app` has no `watchedPipelines`/`lastPipelineStages`
   fields; `ui.ViewHost`'s doc comment reflects the new owner.
3. `go build`/`go test`/`go vet` clean, `gofmt -l` clean, zero import
   cycle.
4. `internal/view/pipelinewatcher_test.go` covers the pure-logic
   helpers (ported) and the `*App`-reaching behavior (rewritten
   against `PipelineWatcher` + fakes).
5. Live verification per above, including an explicit note on what an
   idle AWS account couldn't exercise.
6. No behavior change.
