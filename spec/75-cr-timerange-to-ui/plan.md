# Plan — CR 75: promote `timeRange` group to `internal/ui`

## Approach

### 1. New file `internal/ui/timerange.go`

Move these declarations verbatim from `logsearch.go` (lines 18–81
today), exporting each identifier and updating their doc comments'
wording accordingly:

```go
package ui

import "time"

// TimeRangePreset is one relative time window offered by the time range
// modal's Relative tab (spec/34-fe-cloudwatch-logs decision 4 — relative
// presets, not free-form timestamps).
type TimeRangePreset struct {
	Label    string
	Duration time.Duration
}

var TimeRangePresets = []TimeRangePreset{
	{"15m", 15 * time.Minute},
	// ... same 9 entries, unchanged
}

// DefaultPresetIdx is "1h" — a reasonable default investigation window.
const DefaultPresetIdx = 1

// TimeRangeMode selects which of TimeRange's two representations is
// active — a relative preset (index into TimeRangePresets) or an
// absolute [from, to) window (spec/53-fe-log-time-range-modal).
type TimeRangeMode int

const (
	TimeRangeRelative TimeRangeMode = iota
	TimeRangeAbsolute
)

// TimeRange is the shared time-window selection for logSearchView and
// datadogLogsView, driven by the shared Relative/Absolute modal
// (spec/53-fe-log-time-range-modal). Only one of PresetIdx or From/To is
// meaningful, selected by Mode.
type TimeRange struct {
	Mode      TimeRangeMode
	PresetIdx int
	From, To  time.Time
}

// Bounds returns the [start, end) window for tr, resolving a relative
// preset against now.
func (tr TimeRange) Bounds(now time.Time) (start, end time.Time) { ... }

// Label renders tr for display in a view's title. ...
func (tr TimeRange) Label() string { ... }
```

Note: `TimeRangePreset.Label`/`.Duration` and `TimeRange.Mode`/
`.PresetIdx`/`.From`/`.To` are struct *fields*, exported alongside
their types (unlike the unexported `presetIdx`/`from`/`to`/`mode`
today) — a field on an exported type used from another package must
itself be exported to be reachable, unlike the overlay `.flex`/`.form`
fields in CR 73's `Primitive()` pattern, which stay reachable only
through a method precisely because they *don't* need direct access
from outside. Here every caller (`logsearch.go`, `datadoglogs.go`,
`timerangemodal.go`) constructs and reads these fields directly today
(e.g. `timeRange{mode: timeRangeRelative, presetIdx: 4}`), so keeping
them unexported would just force an equivalent constructor/accessor
layer for no benefit — a plain exported struct matches how it's
actually used.

### 2. `internal/ui/timerange_test.go` (new file)

Move `TestTimeRangeBounds`/`TestTimeRangeLabel` from
`logsearch_test.go` verbatim, updating `timeRange{...}` literals to
`ui.TimeRange{...}` and `tr.bounds`/`tr.label` to `tr.Bounds`/
`tr.Label`. Package `ui` (not `ui_test`) — matches `filter_test.go`'s
existing convention in this directory.

### 3. `logsearch.go` / `logsearch_test.go`

Remove the moved declarations. Update every remaining call site:

```go
// before
sv.tr = timeRange{mode: timeRangeRelative, presetIdx: defaultPresetIdx}
// ...
start, end := sv.tr.bounds(time.Now())
// ...
label := sv.tr.label()

// after
sv.tr = ui.TimeRange{Mode: ui.TimeRangeRelative, PresetIdx: ui.DefaultPresetIdx}
// ...
start, end := sv.tr.Bounds(time.Now())
// ...
label := sv.tr.Label()
```

`logSearchView.tr`'s field type changes from `timeRange` to
`ui.TimeRange`. The `a.timeRangeModal.Show(sv.tr, func(tr timeRange) {
sv.tr = tr })` callback's parameter type becomes `func(tr ui.TimeRange)`.

### 4. `datadoglogs.go` / `datadoglogs_test.go`

Same treatment: `datadogLogsView.tr` field type, the `Show(...)`
callback parameter, and the `dv.tr.bounds`/`dv.tr.label` call sites.

### 5. `timerangemodal.go` / `timerangemodal_test.go`

`TimeRangeModal` doesn't declare any of the 7 promoted identifiers —
it only consumes them (`Show(current timeRange, onApply func(timeRange))`,
internal use of `timeRangeMode`/`timeRangeRelative`/`timeRangeAbsolute`/
`timeRangePresets` in `switchTab`/`renderTabs`/`applyRelative`).
Straight rename to the `ui.`-qualified names at every occurrence; no
declarations to remove.

### 6. Verification order

Do `internal/ui/timerange.go` + its test file first (new code, builds
independently), then `logsearch.go` (declares the old symbols — must
lose them before `datadoglogs.go`/`timerangemodal.go` stop seeing
undefined-symbol errors), then `datadoglogs.go`, then `timerangemodal.go`,
running `gofmt -l`/`go build ./...` after each to confirm the compiler
errors are shrinking as expected. Final `go vet ./...` and
`go test ./...` repo-wide.

## Files touched

- `internal/ui/timerange.go` (new), `internal/ui/timerange_test.go` (new)
- `internal/app/logsearch.go`, `internal/app/logsearch_test.go`
- `internal/app/datadoglogs.go`, `internal/app/datadoglogs_test.go`
- `internal/app/timerangemodal.go`, `internal/app/timerangemodal_test.go`

## Key decisions

- **Destination is `internal/ui`, not a new package** — matches CR
  71's precedent exactly (same reasoning: the shared, dependency-free
  base both `internal/app` and the future `internal/dialog` sit
  above), and `internal/ui` already holds the sibling
  `ParseFilterDate`/`ParseMessageFilterForm` this type's callers sit
  next to conceptually (both are shared inputs to overlay forms).
- **Struct fields exported, not method-gated** — see the note under
  step 1. This is a genuine judgment call distinct from CR 73's
  `Primitive()`/`Visible()` pattern: those exist specifically to avoid
  exposing mutable internal widget state across a package boundary;
  `TimeRange` is a plain value type every caller already constructs
  and reads directly by field, so hiding the fields behind accessors
  would add a layer with no corresponding encapsulation benefit.
- **No new tests beyond the moved ones** — pure promotion + rename,
  identical behavior; the existing `TestTimeRangeBounds`/
  `TestTimeRangeLabel` coverage is sufficient and moves as-is.
- **No new dependencies** — `time` is already a `ui` package
  dependency (via `filter.go`'s `ParseFilterDate`).

## Definition of done

Unchanged from spec.md — `go build`/`go test`/`go vet` pass, `gofmt -l`
clean, the 7 identifiers exist only in `internal/ui` (exported), every
former call site updated, zero behavior change.
