# Tasks

1. [ ] Thread a timestamp through the pending-jump queue: update
   `ui.ViewHost.SetPendingCloudWatchPattern` to
   `(pattern string, timestamp time.Time)`; add `App.pendingCloudWatchTimestamp
   time.Time`; update `App.SetPendingCloudWatchPattern` to set it;
   `App.SwitchTo` clears it alongside the pattern when navigating away
   from `cloudwatch-logs`; update `datadoglogdetail.go`'s `g` handler
   to pass `dv.event.Timestamp`; update `fakeViewHost`'s stub. No
   behavior change yet — `OpenLogSearch` doesn't consume the new field
   in this task. Update the existing tests that call
   `SetPendingCloudWatchPattern`/seed `pendingCloudWatchPattern`
   directly to also thread a timestamp (even if not yet asserted on).

2. [ ] Give `LogSearchView.Open` a third param, `initialTimeRange
   *ui.TimeRange` (`nil` = today's relative-default reset, unchanged);
   add the `TimeRange() ui.TimeRange` exported getter. Update the two
   existing `sv.Open(...)` call sites (`logsearch_test.go`) and the one
   real call site (`viewwiring.go`, passing `nil` for now — still no
   behavior change). Unit test: non-nil `initialTimeRange` sets `sv.tr`
   to that exact value instead of resetting.

3. [ ] Wire it together in `app.OpenLogSearch`: read+clear
   `pendingCloudWatchTimestamp` alongside the pattern, build the
   `±15m` absolute `*ui.TimeRange` when it's non-zero (see plan.md),
   pass it to `logSearchV.Open`. Update/add the app-level tests: seed a
   timestamp in `TestOpenLogSearchConsumesPendingCloudWatchPattern` and
   assert `a.logSearchV.TimeRange()`; assert the no-timestamp branch in
   `TestOpenLogSearchWithoutPendingPatternIsUnaffected`; update the
   `SwitchTo` preserve/clear tests; update the `g`-key
   `TestDatadogLogDetailViewGoToCloudWatch*` tests to seed/assert a
   `Timestamp` on the test event.

4. [ ] Manual verification against a real AWS profile + Datadog log
   with an old CorrelationID (the 4 scenarios in plan.md's "Manual
   verification" section). Record what was actually checked here once
   done.

5. [ ] Merge-back: update `spec/19-log-investigation-crosslinks/spec.md`
   to describe the new end-state behavior (timestamp queued alongside
   the CorrelationID, absolute ±15m window on jump); delete
   `spec-wip/91-bugfix-correlation-jump-timerange/`.
