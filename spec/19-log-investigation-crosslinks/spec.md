# Log investigation cross-links: correlation jump + shared time-range modal

_Condensed from spec/41, spec/53 — see those folders for the incremental history. CorrelationID jump's time-range behavior updated by spec-origin/91-bugfix-correlation-jump-timerange._

## Purpose

Two pieces of shared infrastructure across the CloudWatch Logs
(spec-origin/17) and Datadog Logs (spec-origin/18) search views: jumping
from a Datadog log to its matching CloudWatch line, and a shared,
richer time-range picker that replaced both views' original preset-cycling.

## Behavior / user flow

### CorrelationID jump (Datadog → CloudWatch)

- On the Datadog log detail view (`datadogLogDetailView`), pressing `g`
  extracts a CorrelationID from the log's message text and jumps to
  CloudWatch Logs with it queued as the search pattern, **alongside the
  Datadog event's own timestamp**.
- If the message has no CorrelationID, `g` shows a status-bar message
  ("No CorrelationID found in this log message") and does nothing else
  — no pattern or timestamp is queued.
- The jump lands on the CloudWatch Logs *group list* (there's no
  cross-log-group search — CloudWatch Logs Insights is out of scope, see
  spec-origin/17) with the CorrelationID and timestamp queued. Picking a
  group (the normal Enter-on-a-row flow) opens that group's search
  **pre-filled** with the CorrelationID as the filter pattern, instead
  of the normal empty default — **and** on an **absolute time range**
  centered on the Datadog event's timestamp (±15 minutes), instead of
  resetting to the relative default. This exists because the relative
  default is relative to *now*: without it, jumping to an event older
  than the default window landed on a CloudWatch search that
  structurally could not contain the event, silently — narrowing the
  filter pattern or spec-origin/90's pagination couldn't fix it, since
  the event was outside the queried window entirely, not hidden by a
  page cap.
- A normal (non-jump) group open is unaffected — it still resets to the
  relative default exactly as before; the absolute-window behavior only
  triggers when a jump actually queued a timestamp.
- The queued pattern and timestamp are **one-shot and self-clearing,
  together**: if the user navigates away (Home, Settings, another
  top-level view) without picking a log group, both are dropped rather
  than silently pre-filling/re-windowing a later, unrelated,
  manually-opened group's search.
- Only this one direction exists (Datadog → CloudWatch); the reverse was
  not requested.

### Shared time-range modal

- Both `logSearchView` (CloudWatch) and `datadogLogsView` (Datadog) share
  one `App`-owned modal overlay, opened by pressing `t` in either view.
  There is no more preset-cycling — `t` always opens the modal.
- **Relative tab**: a fixed list — 15 minutes, 1 hour, 4 hours, 1 day, 2
  days, 3 days, 7 days, 15 days, 1 month (`1mo` approximated as 30 days —
  `time.Duration` has no calendar-aware month). Selecting one applies
  immediately and closes the modal. Default preset is "1h".
- **Absolute tab**: `From`/`Until` fields, applied via an explicit Apply
  action. Accepted formats: RFC3339, bare date (`2006-01-02`), or date +
  time of day (`2006-01-02 15:04`, minute precision).
  - The bare date-only format is interpreted as **UTC midnight**
    (unchanged, pre-existing convention shared with the message-filter
    date parser).
  - The date+time format is interpreted as **local time**, converted to
    UTC for storage — deliberately different from the bare-date format.
    Both the modal's display and the view's title render times back via
    `.Local()`, so what the user types round-trips as what they see.
- `R`/`A` (uppercase) switch tabs — not arrow keys, since `tview.InputField`
  already consumes Left/Right for in-field cursor movement on the
  Absolute tab's date fields. Up/Down stay reserved for list/field
  navigation within a tab; Tab stays reserved for field-to-field
  navigation on the Absolute tab. `Esc` cancels/closes without applying,
  from either tab.
- Opening the modal prefills it from the calling view's current time
  range (which tab, which preset/dates) — each view keeps its own state,
  not a shared "last used" global.

## Data & config

```go
type timeRangeMode int
const (
    timeRangeRelative timeRangeMode = iota
    timeRangeAbsolute
)
type timeRange struct {
    mode      timeRangeMode
    presetIdx int       // meaningful when mode == timeRangeRelative
    from, to  time.Time // meaningful when mode == timeRangeAbsolute
}
```
`bounds(now time.Time) (start, end time.Time)` and `label() string`
methods. Both views hold a `tr timeRange` field (replacing a bare
`presetIdx int`) and call `tr.bounds(time.Now())` for the actual query
window, `tr.label()` for title-bar display.

- CorrelationID extraction: `extractCorrelationID(message string) (string,
  bool)` — regex against the message text, case-insensitive on the
  `CorrelationID:` label, strict on standard 36-char UUID shape
  (`[0-9a-f]{8}-[0-9a-f]{4}-[0-9a-f]{4}-[0-9a-f]{4}-[0-9a-f]{12}`) so it
  doesn't over-match trailing punctuation/words. Matches only this exact
  `CorrelationID: <uuid>` text shape — no fuzzy matching, no other
  correlation-field labels.
- `ui.ViewHost.SetPendingCloudWatchPattern(pattern string, timestamp
  time.Time)` queues both values together; `App.pendingCloudWatchTimestamp
  time.Time` holds the timestamp alongside `pendingCloudWatchPattern`,
  cleared/consumed in lockstep with it.
- `App.OpenLogSearch` builds the absolute window when a timestamp was
  queued: `correlationJumpWindowBuffer = 15 * time.Minute`, `From:
  ts.Add(-correlationJumpWindowBuffer)`, `To:
  ts.Add(correlationJumpWindowBuffer)`. Gated on the timestamp being
  non-zero (`!ts.IsZero()`), not on the pattern being non-empty — a
  single source of truth for "this is a jump, not a normal open."
- `logSearchView.Open(logGroupName, initialPattern string,
  initialTimeRange *ui.TimeRange)` — `initialTimeRange` is `nil` for a
  normal open (reset to the relative default, unchanged) or the
  computed absolute window for a jump. `logSearchView.TimeRange() ui.
  TimeRange` (mirrors the existing `Pattern()` getter) lets `internal/
  app` verify the computed range crossed the package boundary.

## Implementation notes

- Shared modal type/construction lives in `internal/ui`/`internal/dialog`
  post the package split (spec-origin/03) — `timeRange`/`timeRangeMode`/
  presets were promoted out of the CloudWatch-specific file to
  `internal/ui` so the dialog package doesn't depend on a resource-view
  file (originally `logsearch.go`).
- `App.pendingCloudWatchPattern string` and `App.pendingCloudWatchTimestamp
  time.Time` (or their current-package equivalents) hold the queued
  CorrelationID and its event's timestamp between the jump and the
  group pick; both cleared on navigation to anything other than
  `cloudwatch-logs`.

## Notable gotchas worth preserving

- The queued CorrelationID must be **double-quoted** before being handed
  to CloudWatch — its filter-pattern syntax otherwise tokenizes an
  unquoted term on the UUID's internal hyphens and never matches as the
  literal phrase it is. This is scoped to this one programmatically
  injected value; a user's own typed CloudWatch search pattern is still
  passed through completely unmodified.
- The date+time absolute-format's local-vs-UTC handling is a real trap:
  the results table always displays timestamps via `.Local()`, but a
  naive `time.Parse` on a bare layout defaults to UTC — so a
  local-looking typed time (`2026-08-11 15:00`) would silently be read as
  UTC, producing a search window offset by the local UTC delta from what
  was intended (caught live: results came back ~2 hours off in a UTC+2
  zone). Fix: parse the date+time layout with
  `time.ParseInLocation(layout, s, time.Local)`, then convert to UTC for
  storage. The bare date-only layout intentionally keeps its original
  UTC-midnight interpretation — this asymmetry between the two absolute
  formats is deliberate, not an oversight, because at day granularity a
  local/UTC mismatch is far less visible than at minute granularity.
- Tab-indicator styling can't use literal `[Relative]`/`[Absolute]` —
  square brackets are swallowed as `tview` color/region tags, a
  recurring gotcha throughout this codebase (see also the multi-select
  `"[x]"` mark gotcha in spec-origin/08).
