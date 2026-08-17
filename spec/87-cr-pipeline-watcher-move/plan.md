# Plan — CR 87: extract the CodePipeline background watcher out of `internal/app`

## Approach

Unlike CR 82-86's "adopt then move" mechanics, there's no existing
type to export here — `PipelineWatcher` is a new type from the start.
Build it directly in `internal/view` (no staged "de-risk in place"
phase makes sense, since there's no in-place version to de-risk: the
logic has to move to get a home at all), verified incrementally
method-by-method, then reduce `internal/app/codepipelinewatch.go` to
its 3 trampolines last.

### 1. `PipelineWatcher`'s shape

```go
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

// PipelineWatcher owns the background poll loop that keeps a
// CodePipeline's stage status current once the user starts watching
// it: notifying on stage transitions, stopping automatically once the
// pipeline execution reaches a terminal state, and repainting
// listV/detailV directly since both need a live refresh whenever a
// poll completes, regardless of which one the user is currently on.
type PipelineWatcher struct {
	watched    map[string]chan struct{}
	lastStages map[string]map[string]string
	host       ui.ViewHost
	notify     func(title, message string)
	listV      *CodePipelineListView
	detailV    *CodePipelineDetailView
}

func NewPipelineWatcher(host ui.ViewHost, notify func(title, message string), listV *CodePipelineListView, detailV *CodePipelineDetailView) *PipelineWatcher {
	return &PipelineWatcher{
		watched:    map[string]chan struct{}{},
		lastStages: map[string]map[string]string{},
		host:       host,
		notify:     notify,
		listV:      listV,
		detailV:    detailV,
	}
}
```

`listV`/`detailV` are concrete same-package pointers, not interfaces —
`PipelineWatcher` always drives exactly these 2 views for the app's
lifetime, so there's no substitutability to design for (unlike
`ui.ViewHost`, which exists because many different concrete types
need to reach the same App capabilities).

### 2. The 3 methods `ui.ViewHost` requires — logic unchanged, receiver and host calls updated

```go
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
// running inside a QueueUpdateDraw callback) — watched/lastStages are
// otherwise untouched by any other goroutine, so no mutex is needed
// (see spec/43-fe-codepipeline-monitor's watcher section for the full
// reasoning).
func (w *PipelineWatcher) StartWatchingPipeline(name string) {
	if w.IsWatchingPipeline(name) {
		return
	}
	stop := make(chan struct{})
	w.watched[name] = stop
	// Captured once here, on the main goroutine, and passed into
	// pollPipeline by value — never re-read from host.Config() inside
	// the goroutine, which would otherwise race the main goroutine's
	// own reads/writes (e.g. switching AWS profiles). Same discipline
	// as datadogLogsView.search() capturing cfg.Datadog once before
	// spawning its own goroutine.
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
```

### 3. `pollPipeline`/`handlePipelinePoll` — `a.X` → `w.host.X`, sibling reach-ins become direct fields

```go
// pollPipeline runs on its own goroutine for the lifetime of one watch.
// All AWS calls happen here; all state mutation and UI work happens
// inside handlePipelinePoll, dispatched via QueueUpdateDraw — the same
// single-writer pattern every other background-goroutine feature in
// this app already uses for its own search()-style calls.
func (w *PipelineWatcher) pollPipeline(name, profile string, stop chan struct{}) {
	ticker := time.NewTicker(pipelinePollInterval)
	defer ticker.Stop()
	for {
		select {
		case <-stop:
			return
		case <-ticker.C:
		}

		ctx := context.Background()
		authType, _ := w.host.AWSAuthTypeFor(ctx, profile)
		stages, err := awsauth.WithReauth(ctx, profile, authType, w.host.AWSSSOLogin,
			nil, // no in-progress status message — this isn't a visible search view
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
// once and stops the watch. On success, notifies for every stage whose
// status changed since the last poll, stops the watch once the
// pipeline execution reaches a terminal state, and live-refreshes
// detailV if it's currently open for this pipeline. Split out from
// pollPipeline so this — the part with actual logic to get wrong — is
// directly testable without a goroutine, a ticker, or a real AWS call.
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
```

**No nil guards on `w.detailV`/`w.listV`**, unlike today's
`a.codePipelineDetailV != nil`/`a.codePipelineListV != nil` checks:
those existed because `*App`'s fields could theoretically be read
before `New()` finished constructing them; `PipelineWatcher` can't be
constructed at all without both already built (they're required
constructor params), so the guard has nothing left to guard against —
same reasoning CR 85 used to drop `ApplyPalette`'s nil check.

`stageTransitions`/`snapshotStages`/`pipelineFinished` move unchanged
— already package-level functions, no receiver either way, no logic
touched.

### 4. `internal/app/codepipelinewatch.go` — reduced to 3 trampolines

```go
package app

// IsWatchingPipeline reports whether name currently has an active
// background watch. See view.PipelineWatcher for the implementation.
func (a *App) IsWatchingPipeline(name string) bool {
	return a.pipelineWatcher.IsWatchingPipeline(name)
}

// StartWatchingPipeline begins polling name's stage state in the
// background. See view.PipelineWatcher for the implementation.
func (a *App) StartWatchingPipeline(name string) {
	a.pipelineWatcher.StartWatchingPipeline(name)
}

// StopWatchingPipeline stops name's background watch, if any. See
// view.PipelineWatcher for the implementation.
func (a *App) StopWatchingPipeline(name string) {
	a.pipelineWatcher.StopWatchingPipeline(name)
}
```

Everything else in the file (the old struct-free logic, `pollPipeline`,
`handlePipelinePoll`, `stageTransitions`, `snapshotStages`,
`pipelineFinished`, the `pipelinePollInterval` const) is deleted from
here — moved to `internal/view/pipelinewatcher.go`, not duplicated.

### 5. `app.go` changes

**Struct fields**: `watchedPipelines map[string]chan struct{}` and
`lastPipelineStages map[string]map[string]string` (app.go:98-99)
removed; `pipelineWatcher *view.PipelineWatcher` added.

**Construction**: the 2 init lines
(`a.watchedPipelines = map[string]chan struct{}{}` /
`a.lastPipelineStages = map[string]map[string]string{}`, app.go:270-271)
removed; construction of `pipelineWatcher` added right after
`codePipelineListV`/`codePipelineDetailV` (app.go:272-276, both must
already exist — a real ordering dependency, same shape as CR 85's
Settings needing its 4 dialogs built first):

```go
a.codePipelineListV = view.NewCodePipelineListView(a, a.OpenCodePipelineDetail)
a.codePipelineDetailV = view.NewCodePipelineDetailView(a, func() {
	...
})
a.pipelineWatcher = view.NewPipelineWatcher(a, a.notify, a.codePipelineListV, a.codePipelineDetailV)
```

`a.notify` (assigned at app.go:191, `ui.DesktopNotify`) is already set
well before this point — no reordering needed for it.

### 6. `internal/ui/viewhost.go` doc comment

```go
// CodePipeline background watcher — App forwards to
// view.PipelineWatcher (internal/app/codepipelinewatch.go's 3
// trampoline methods), which owns the actual poll loop and state.
IsWatchingPipeline(name string) bool
StartWatchingPipeline(name string)
StopWatchingPipeline(name string)
```

### 7. Tests

**Pure logic — ported unchanged** to
`internal/view/pipelinewatcher_test.go`: `TestStageTransitionsNilPrevReturnsNoMessages`,
`TestStageTransitionsReportsChangedStages`,
`TestStageTransitionsNeverRunStageLabel`, `TestSnapshotStages`,
`TestPipelineFinished` — call the package-level helpers directly, no
`*App`/`PipelineWatcher` involved either before or after.

**`fakeNotifier`** moves unchanged into the same new test file:

```go
type fakeNotifier struct {
	calls []struct{ title, message string }
}

func (f *fakeNotifier) notify(title, message string) {
	f.calls = append(f.calls, struct{ title, message string }{title, message})
}
```

**New construction helper**, matching every other CR 82-86 test file's
`fakeViewHost` pattern, plus real sibling-view instances since
`PipelineWatcher` needs them too:

```go
func newTestPipelineWatcher(t *testing.T) (*fakeViewHost, *fakeNotifier, *PipelineWatcher) {
	t.Helper()
	host := newFakeViewHost()
	fn := &fakeNotifier{}
	listV := NewCodePipelineListView(host, func(string) {})
	detailV := NewCodePipelineDetailView(host, func() {})
	return host, fn, NewPipelineWatcher(host, fn.notify, listV, detailV)
}
```

**Rewritten against `PipelineWatcher` directly** (not a port — the
originals only worked because they poked a real `*App`'s unexported
fields; same behavior, new construction):
`TestHandlePipelinePollFirstPollEstablishesSilentBaseline`,
`TestHandlePipelinePollNotifiesOnChangedStage`,
`TestHandlePipelinePollErrorStopsWatchAndNotifiesOnce`,
`TestHandlePipelinePollStopsWatchWhenFinished`,
`TestStartStopWatchingPipeline` — each swaps `a.notify = fn.notify` /
`a.lastPipelineStages[...]` / `a.watchedPipelines[...]` /
`a.handlePipelinePoll(...)` for `w.lastStages[...]` /
`w.watched[...]` / `w.handlePipelinePoll(...)` on the `w` returned by
`newTestPipelineWatcher`; assertions unchanged in substance.

**Genuinely new coverage**: grepping the original 9 confirms none of
them ever asserted that `handlePipelinePoll` actually refreshes
`detailV`/repaints `listV` — every existing test's `stages` happened
to be checked only through `notify`/`watched`/`lastStages`, never
through what the 2 views end up displaying. One new test closes the
`detailV` half of that gap (same-package field access, no need to
route through `Open()`'s own goroutine-spawning `load()`):

```go
func TestHandlePipelinePollRendersOpenDetailView(t *testing.T) {
	_, _, w := newTestPipelineWatcher(t)
	w.detailV.pipelineName = "my-pipeline" // as if Open("my-pipeline") had already run

	w.handlePipelinePoll("my-pipeline", []awscodepipeline.StageStatus{
		{Name: "Deploy", Status: "InProgress"},
	}, nil)

	if got := w.detailV.table.GetCell(1, 0).Text; got != "Deploy" {
		t.Errorf("detailV table after poll = %q, want %q", got, "Deploy")
	}
}
```

The `listV.Repaint()` half is deliberately left unasserted: `listV`'s
WATCHING column reads through `lv.host.IsWatchingPipeline(...)` —
`fakeViewHost`'s own independent `watching` map in tests, disconnected
from `PipelineWatcher.watched` (in production both route through the
same `*App`, since `App.IsWatchingPipeline` trampolines to
`a.pipelineWatcher`, but the test fakes don't wire that same
indirection). `Repaint()` is called unconditionally, with no branch to
differentiate — every one of the 6 tests above already exercises the
call executing without panicking; a dedicated assertion would only be
checking that a `tview.Table` redraws given empty data, which isn't
meaningful signal. Live verification (task 6) covers the real,
wired-together repaint instead.

### 8. Verification order

Steps 1-3 (new type, standalone, `internal/view`) → step 7's pure-logic
port + rewritten `PipelineWatcher` tests (verify the new type fully in
isolation) → step 4 (`internal/app/codepipelinewatch.go` reduced to
trampolines) → step 5 (`app.go` wiring) → step 6 (doc comment).
`gofmt -l`/`go build ./...`/`go vet ./...`/`go test ./...` after each
step. Final repo-wide pass, then live verification.

## Files touched

- New `internal/view/pipelinewatcher.go`.
- New `internal/view/pipelinewatcher_test.go`.
- `internal/app/codepipelinewatch.go` (reduced to 3 trampolines).
- `internal/app/codepipelinewatch_test.go` deleted (all 9 tests
  relocated/rewritten into the new file).
- `internal/app/app.go` (fields, construction).
- `internal/ui/viewhost.go` (doc comment only — no signature change).

## Key decisions

- **New type, not a "move"** — every prior phase-4 CR relocated an
  existing type; this one has no existing type to relocate, since the
  logic has always lived directly on `*App`. Named to match its role
  (`PipelineWatcher`, not `codePipelineWatchView` — it isn't a view,
  has no `Name()`/`Title()`/`Primitive()`, and shouldn't pretend to be
  one just for naming consistency with its neighbors).
- **`internal/view`, not a new package** — it's tightly coupled to
  exactly 2 types that already live there (`CodePipelineListView`/
  `CodePipelineDetailView`), and holds them as direct same-package
  fields specifically to avoid needing new cross-package exports for
  what's effectively private collaboration between 3 closely related
  types. A dedicated package would only add import-boundary ceremony
  for no isolation benefit — nothing else needs to construct or
  reference `PipelineWatcher` except `internal/app`.
- **`notify` stays a constructor-injected func, not a `ui.ViewHost`
  method** — no other `ui.ViewHost` caller needs desktop notifications;
  widening the interface for this file's sole use would be a bigger
  change than the problem calls for, and constructor injection already
  matches how `internal/dialog` types take their peer dependencies.
- **`ui.ViewHost`'s 3 method signatures don't change** — only *App*'s
  implementation becomes a forward. Both `codepipelinelist.go` and
  `codepipelinedetail.go` already call these 3 through `host.X(...)`
  and need zero changes.
- **One new test, not a matching pair** — see step 7's closing note:
  `detailV`'s refresh-on-poll behavior was genuinely uncovered and is
  cheaply testable; `listV`'s repaint-on-poll behavior is exercised
  (called, doesn't panic) by every rewritten test already, and a
  dedicated assertion would need either wiring the fakes together
  (out of scope — a bigger change to `fakeViewHost` itself) or
  asserting something not meaningfully different from "doesn't
  panic," so it's left to live verification instead.

## Definition of done

Unchanged from spec.md — `PipelineWatcher` in `internal/view` owns the
watch state and poll loop; `internal/app/codepipelinewatch.go` holds
only 3 trampolines; no `watchedPipelines`/`lastPipelineStages` fields
left on `*App`; `ui.ViewHost`'s doc comment reflects the new owner;
`go build`/`go test`/`go vet` clean, `gofmt -l` clean, zero import
cycle; pure-logic tests ported, behavior tests rewritten, 1 new test
for the previously-uncovered detail-view-refresh path; live
verification confirms no behavior change (with an explicit note on
what an idle AWS account couldn't exercise).
