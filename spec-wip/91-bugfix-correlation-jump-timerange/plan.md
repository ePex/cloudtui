# Implementation plan

## Approach

Extend the existing one-shot pending-jump queue
(`App.pendingCloudWatchPattern`) with a paired timestamp, and give
`LogSearchView.Open()` a way to receive an initial absolute time range
override instead of always resetting to the relative default. The
queue's existing one-shot/self-clearing rules (spec/19) apply
identically to both fields — they're always set, read, and cleared
together.

## Signature changes

- `ui.ViewHost.SetPendingCloudWatchPattern(pattern string, timestamp
  time.Time)` — adds the timestamp param. Name kept as-is (not
  renamed) to keep the diff minimal; `pattern` stays the primary value,
  `timestamp` is new context for it.
- `app.App.SetPendingCloudWatchPattern` — same shape change, plus a new
  `pendingCloudWatchTimestamp time.Time` field alongside
  `pendingCloudWatchPattern string`.
- `view.LogSearchView.Open(logGroupName, initialPattern string,
  initialTimeRange *ui.TimeRange)` — adds a third param. `nil` means
  "no override" — reset to the relative default exactly as today, so
  every non-jump open (the overwhelming majority of opens) is
  byte-for-byte unchanged. Non-nil replaces the reset with `sv.tr =
  *initialTimeRange`.
- `view.fakeViewHost.SetPendingCloudWatchPattern` (test double) — same
  shape change.
- New: `view.LogSearchView.TimeRange() ui.TimeRange` — exported getter,
  mirrors the existing `Pattern()` getter added for the same reason
  (FE 41): `internal/app`'s tests need to verify the computed time
  range crossed the package boundary correctly, and there's currently
  no way to observe `sv.tr` from outside `internal/view`.

## Where the window gets built

`app.OpenLogSearch` (`internal/app/viewwiring.go`) is the consumption
point — it already reads+clears the pending pattern here, so the
timestamp is read+cleared alongside it, and this is where the
`*ui.TimeRange` gets constructed:

```go
const correlationJumpWindowBuffer = 15 * time.Minute

func (a *App) OpenLogSearch(logGroupName string) {
	pattern := a.pendingCloudWatchPattern
	ts := a.pendingCloudWatchTimestamp
	a.pendingCloudWatchPattern = ""
	a.pendingCloudWatchTimestamp = time.Time{}

	var tr *ui.TimeRange
	if !ts.IsZero() {
		tr = &ui.TimeRange{
			Mode: ui.TimeRangeAbsolute,
			From: ts.Add(-correlationJumpWindowBuffer),
			To:   ts.Add(correlationJumpWindowBuffer),
		}
	}
	a.logSearchV.Open(logGroupName, pattern, tr)
	// ...unchanged from here
}
```

Gating on `ts.IsZero()` alone (not `pattern != ""`) keeps a single
source of truth — a normal open never has a queued timestamp, so this
naturally covers "no jump happened" without a redundant check.

## Other call sites touched

- `datadoglogdetail.go`'s `g` handler: `dv.host.SetPendingCloudWatchPattern(fmt.Sprintf("%q", id), dv.event.Timestamp)`.
- `app.SwitchTo`: the existing "navigated away from cloudwatch-logs"
  clear also zeroes `a.pendingCloudWatchTimestamp`, same condition as
  today's pattern clear.

## Tests

- `internal/view/logsearch_test.go`:
  - Update the two existing `sv.Open(...)` calls to pass a third `nil`
    arg (no behavior change intended there).
  - New case: `Open(..., &ui.TimeRange{...})` sets `sv.tr` to exactly
    that value instead of resetting to the relative default.
- `internal/app/app_test.go` / `viewwiring_test.go`:
  - `TestSwitchToClearsAbandonedPendingCloudWatchPattern` /
    `TestSwitchToCloudWatchLogsPreservesPendingPattern`: also seed/
    assert `pendingCloudWatchTimestamp` alongside the pattern.
  - `TestOpenLogSearchConsumesPendingCloudWatchPattern`: seed a
    timestamp too; assert `a.logSearchV.TimeRange()` equals the
    expected `±15m` absolute window, and
    `a.pendingCloudWatchTimestamp` is cleared after consumption.
  - `TestOpenLogSearchWithoutPendingPatternIsUnaffected`: add an
    assertion that `a.logSearchV.TimeRange()` is still the relative
    default — explicitly covers the `ts.IsZero()` branch.
  - `TestDatadogLogDetailViewGoToCloudWatchWithCorrelationID`: give the
    test's `datadoglogs.LogEvent` a `Timestamp`, assert
    `a.pendingCloudWatchTimestamp` matches it after pressing `g`.
  - `TestDatadogLogDetailViewGoToCloudWatchWithoutCorrelationID`: assert
    `a.pendingCloudWatchTimestamp` stays zero (the handler returns
    before queuing anything when there's no CorrelationID match).

## Manual verification

Same AWS-integration caveat as CR 90 — needs a real profile and can't
run in CI:

1. In Datadog Logs, open a log event with a `CorrelationID:` whose
   timestamp is well outside the current relative default (e.g. from
   several hours ago). Press `g`.
2. Confirm the CloudWatch search opens with an **absolute** time range
   (title shows `From → To`, not a relative preset label) centered on
   that event's timestamp, ±15 minutes.
3. Confirm the matching CloudWatch event is now found without any
   manual time-range adjustment.
4. Confirm a normal (non-jump) log group open still resets to the
   relative "1h" default as before.

Record what was actually checked in `tasks.md` once done.
