# Spec — CR 81: move row-selection wiring into each view via constructor callbacks

Date: 2026-08-17

## Background

CR 79 split the 8 `openX`/`wireXOpensY` trampoline pairs out of their
view files into `internal/app/viewwiring.go`. Per that CR's own
framing, this was staging, not the end state: `wireXOpensY` still
reaches directly into the *source* view's unexported `.table` (to
register the row-selection callback) and, inside that callback,
resolves the selected row against the source view's own unexported
slice (`.filtered`, `.msgs`, `.results`) to find the right domain
object before calling `OpenX`. None of that compiles once the view
lives in a different package.

Audited all 8 pairs in full (constructors, exact resolution logic,
target `render` signatures, and whether each target's `render`
already builds its own context panel) before designing the fix —
found one design-changing fact and one small, unrelated bug along the
way:

- **The construction-order constraint disappears.** `app.go`'s `New()`
  today constructs every source view, then every target view, then
  calls each `wireXOpensY()` — each one annotated with a comment like
  `// messagesV must exist first`. That's because the old trampolines
  *read* `a.messagesV` at wiring time. Once the callback passed to a
  source view's constructor is a bound method value (`a.OpenMessages`),
  the target field isn't dereferenced until the callback actually
  *fires* — a real keypress, well after `New()` finishes. The ordering
  constraint, and the 8 separate `wireXOpensY()` calls entirely, go
  away.
- **`datadogLogDetailView` already has a stale-field bug CR 80 should
  have caught.** Its `'g'` (jump to CloudWatch) handler still writes
  `dv.app.pendingCloudWatchPattern = ...` directly instead of calling
  the `SetPendingCloudWatchPattern` setter CR 80 added for exactly
  this. Still compiles today (same package), but breaks the moment
  this file moves packages — fixing it here since it's the same
  "reaches into `*App}` directly" theme this CR is already touching.

## Problem

Once the 12 view types move to `internal/view`, `viewwiring.go`
(staying in `internal/app`) can no longer reach into a source view's
`.table`/`.filtered`/`.msgs`/`.results` to wire selection — those
become unexported fields of a foreign package. The fix has to live in
the view's own constructor, which still has that state in scope.

## Solution

Each of the 8 source views' constructors gains an `onSelect`-shaped
parameter (mirroring the pattern `internal/dialog`'s overlays already
use — e.g. `movePicker.Show(sourceQueue, onSelect func(string),
onClose func())`, not a new convention). The view wires its own
`table.SetSelectedFunc` internally, does the exact same row-index
resolution `wireXOpensY` used to do (unchanged logic, just relocated),
and calls the callback with the resolved domain object.
`app.go`'s `New()` passes the matching `ViewHost` method
(`a.OpenMessages`, etc.) directly at construction time — no separate
wiring step.

Two representative shapes, illustrating the range across the 8 pairs:

**Simple / string-keyed** (`queuesView` → `messagesView`):

```go
// before (viewwiring.go)
func (a *App) wireQueuesOpensMessages() {
	a.queuesV.table.SetSelectedFunc(func(row, _ int) {
		cell := a.queuesV.table.GetCell(row, 0)
		if cell == nil || cell.Text == "" {
			return
		}
		a.OpenMessages(cell.Text)
	})
}

// after (queues.go's constructor) — receiver type unchanged (still *App;
// switching to ui.ViewHost is a separate, later CR), only the new
// onSelect parameter is added
func newQueuesView(a *App, b queue.Backend, onSelect func(queueName string)) *queuesView {
	// ...
	qv.table.SetSelectedFunc(func(row, _ int) {
		cell := qv.table.GetCell(row, 0)
		if cell == nil || cell.Text == "" {
			return
		}
		onSelect(cell.Text)
	})
	// ...
}

// app.go's New()
a.queuesV = newQueuesView(a, a.backend, a.OpenMessages)
```

**Struct-keyed, target already self-sufficient**
(`ssmParamsView` → `paramDetailView`): same shape, `onSelect
func(param awsssm.Parameter)`, resolved via `ssmParamsV.filtered[idx]`
inside the view instead of `viewwiring.go`. `OpenParamDetail` doesn't
change at all here — `paramDetailV.render(param)` already builds its
own context panel internally (confirmed: 4 of the 6 target views
already do this — `paramDetailView`, `secretDetailView`,
`logDetailView`, `datadogLogDetailView` — only `messageDetailView`'s
and `logSearchView`'s/`codePipelineDetailView`'s `OpenX` still build
the context panel manually from `.Shortcuts()`, because
`messagesView`/`logSearchView` aren't registered `ui.View`s so the
generic path doesn't apply to them either way — unrelated to this
CR, not changing).

See plan.md for the exact signature, resolved-value type, and
constructor-call change for all 8 pairs — they follow one of these
two shapes with no further variation.

## Scope

### In scope

- 8 source-view constructors gain an `onSelect`-shaped parameter;
  each wires its own `table.SetSelectedFunc` internally, using the
  exact resolution logic currently in `viewwiring.go`'s matching
  `wireXOpensY`.
- `viewwiring.go`: all 8 `wireXOpensY` methods deleted; the 8 `OpenX`
  methods otherwise unchanged (still App-level: page switch, focus,
  context panel where the target isn't self-sufficient).
- `app.go`'s `New()`: each source view's construction call gains the
  matching `a.OpenX` argument; the 8 separate `a.wireXOpensY()` calls
  removed; the "must exist first" ordering comments removed (no
  longer true).
- `datadoglogdetail.go`: the one stray `dv.app.pendingCloudWatchPattern
  = ...` write fixed to `dv.app.SetPendingCloudWatchPattern(...)`.
- `gofmt`/`go vet`/`go build`/`go test` clean across `tui/`.
- Live verification via `verify-live`: exercise all 8 selection paths
  (Enter on a queues/SSM-params/secrets/logs/log-search/Datadog-logs/
  CodePipeline row, and Enter on a messages row) against a real
  broker/AWS profile/Datadog config, confirming each still opens the
  right detail view with the right content — this CR relocates live
  selection-handling logic, not just static wiring, so it's worth
  confirming live rather than trusting `go test` alone.

### Out of scope

- Switching any view from `app *App` to `ui.ViewHost` — separate,
  later CR (this one only reshapes the callback wiring; the receiver
  type doesn't change here).
- Any view's direct `internal/dialog` calls (`a.confirm.Show(...)`,
  `a.movePicker.Show(...)`, etc.) — per CR 80's decision, these become
  direct `internal/dialog` imports once views actually move, not
  something this CR touches.
- `codepipelinewatch.go`'s separate, already-flagged coupling into
  `codePipelineDetailV`/`codePipelineListV` (a background poller
  reaching into `.pipelineName`/`.render()`/`.repaint()`/`.all`
  directly) — different problem, different CR, per CR 79's spec.
- The `ui.Shortcuttable` compile-time-assertion inconsistency (5 of 6
  detail views have it, `codePipelineDetailView` doesn't) — cosmetic,
  not touched.
- The physical move of any view file into `internal/view` — later,
  once every view depends on `ui.ViewHost` instead of `*App`.
- Any behavior change beyond the `datadoglogdetail.go` bug fix (a true
  bug fix — the field write and the setter do the exact same thing
  today, only the setter still works once the type moves).

## Definition of done

1. `go build ./...` and `go test ./...` pass in `tui/`.
2. All 8 source views wire their own row-selection internally via an
   injected callback; `viewwiring.go` has no `wireXOpensY` methods
   left; `app.go` passes each callback at construction time.
3. `gofmt -l` reports nothing; `go vet ./...` clean.
4. All 8 selection paths live-verified working correctly.
5. `datadoglogdetail.go`'s stale field write fixed.
