# Tasks — FE 41

1. [x] `internal/app/datadoglogdetail.go`: `extractCorrelationID`,
   `Shortcuts()` update, `g` key handler. Unit tests for
   `extractCorrelationID` (table-driven) and the key handler (found /
   not-found cases).
2. [x] `internal/app/logsearch.go`: `open(logGroupName, initialPattern
   string)` signature change. Update the one existing call site and the
   existing test call site; add a test for the pre-filled-pattern case.
3. [x] `internal/app/app.go`: `pendingCloudWatchPattern` field,
   `switchTo`'s clear-on-navigate-away guard, `openLogSearch`'s
   consume-and-clear. Unit tests for both.
4. [ ] Manual verification (per `tui/CLAUDE.md`): with a real Datadog
   log containing a CorrelationID and a CloudWatch log group known to
   contain the matching line, press `g` from the Datadog detail view,
   pick the log group, confirm the CloudWatch search runs with the
   CorrelationID pre-filled and finds the match. Also verify the
   abandon case: press `g`, then press `h` (Home) instead of picking a
   group, then open some other unrelated log group later and confirm
   its search does *not* get the stale CorrelationID.
