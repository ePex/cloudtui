# Tasks — FE 34: CloudWatch Logs investigation

Plan: [plan.md](plan.md)

Each task needs separate approval before it's implemented — see
`CLAUDE.md`.

1. [x] Add `github.com/aws/aws-sdk-go-v2/service/cloudwatchlogs` to
   `tui/go.mod` (`go get` + `go mod tidy`); confirm `go build ./...`
   still passes with no code using it yet. `go mod tidy` prunes the
   entry again since nothing imports it yet (same as FE 28/32/33's
   precedent) — it lands for real once task 2's code imports the
   package.
2. [x] `tui/internal/awslogs/`: `awslogs.go` (`LogGroup`/`LogEvent`
   types, `newClient`), `list.go` (`ListLogGroups`, paginated
   `DescribeLogGroups`), `filter.go` (`FilterEvents`, single
   `FilterLogEvents` call with `Limit: 1000`, `StartFromHead: false`).
3. [x] `awslogs` tests: `newClient`'s empty-profile guard,
   `buildLogGroups` (nil-date/nil-retention handling, sorting),
   `buildLogEvents` (nil-safe field unwrapping) — pure functions per
   plan.md's testing note; `ListLogGroups`/`FilterEvents` themselves
   aren't unit tested (same precedent as `awsssm`/`awssecrets`).
4. [x] `App.listLogGroups`/`App.filterLogEvents` fields (defaulting to
   `awslogs.ListLogGroups`/`awslogs.FilterEvents`) — same
   dependency-injection shape as the other AWS features.
5. [x] New view `tui/internal/app/logs.go`: table (NAME/RETENTION/
   CREATED) + filter input (mirrors `secretsView`), registered as
   `ui.View` + `ui.Shortcuttable`, added to `a.views` and Home's "Apps"
   section. Errors clearly if `cfg.ActiveAWSProfile` is empty. Filtered
   title uses `"(text)"` and the repaint includes the
   `Select(1,0)`/`SetOffset(0,0)` scroll-to-top pair from the start
   (not retrofitted after a live bug report, per FE 32/33's precedent).
   `logs_test.go` written alongside it, including the render-based
   filtered-title test and the scroll-to-top test from the start.
6. [x] `onGlobalKey` exemption for `a.logsV.filterInput`, matching the
   existing per-view filter-input entries. Covered by
   `TestOnGlobalKeyPassesThroughWhenLogsFilterFocused`.
7. [x] New view `tui/internal/app/logsearch.go`: pattern `InputField`
   (`patternInput` — triggers `search()` only on `SetDoneFunc`/Enter,
   never on keystroke, and not on Escape/Tab either even though those
   also reach `SetDoneFunc`), time-range preset cycling on `t`
   (`15m → 1h → 3h → 24h`, default `1h`, re-searches immediately), `r`
   to re-run unchanged, results table (TIMESTAMP/STREAM/MESSAGE
   preview via `logEventPreview`) with scroll-to-top on repaint, title
   showing log group/preset/count/`hasMore` (parentheses, never
   brackets, from the start). Opens with an immediate default-range
   search via `open(logGroupName)`. Async `search()` +
   `handleSearchResult(events, hasMore, err)` split (mirrors FE 33's
   `handleFetchResult`, done from the start). Wired into `App`:
   `logSearchV` field, `"log-search"` page, `logsV.table.
   SetSelectedFunc` → `a.openLogSearch(...)`. Dedicated tests land in
   task 11, per this task's scope (implementation only) —
   `go build`/`go vet`/`go test ./...` all pass in the meantime.
8. [x] `onGlobalKey` exemption for `a.logSearchV.patternInput` (named
   `patternInput`, not `filterInput`, since it's a server-search field
   rather than a client-side filter — see task 7). Covered by
   `TestOnGlobalKeyPassesThroughWhenLogSearchPatternFocused`.
9. [x] New view `tui/internal/app/logdetail.go`: Timestamp/Log Stream
   metadata + full unwrapped message, `c` to copy (always available,
   no reveal-gating). Wired into `App`: `logDetailV` field,
   `"log-event-detail"` page, `logSearchV.table.SetSelectedFunc` →
   `a.openLogEventDetail(...)`.
10. [x] `theme.go`: table/filter blocks for `logs.go` and
    `logsearch.go`, textview block for `logdetail.go`, mirroring FE
    32/33's blocks (`p.ViewColor("cloudwatch-logs")`).
11. [x] Tests for the view layer: `logs.go`'s tests written in task 5
    (construction, header, filter, no-active-profile error path,
    render-based filtered-title test, scroll-to-top). `logsearch_test.go`:
    `search()`'s no-profile guard, `open()`'s state reset, time-range
    cycling, the Enter-triggers/typing-doesn't distinction (proved via
    the no-profile guard as an observable signal, since `InputField`
    has no `GetChangedFunc()`/`GetDoneFunc()` getters to assert wiring
    directly), `handleSearchResult`'s success/hasMore/error branches,
    scroll-to-top, and `logEventPreview`'s table-driven cases.
    `logdetail_test.go`: render, `Shortcuts()` always including `c`,
    copy, and Esc-back. All via injected `listLogGroups`/
    `filterLogEvents`, no real AWS calls.
12. [x] `go build ./...`, `go vet ./...`, `go test ./...` — all pass.
13. [x] Manual verification per `verify-live`, against this machine's
    real active AWS profile, via tmux:
    - Home shows the new `cloudwatch-logs` entry; the log group list
      loaded 111 real log groups with NAME/RETENTION/CREATED
      populated correctly; filtering (`/amazonmq/broker`) narrowed
      rows with the parenthesized title actually rendering.
    - Opening a log group ran the default 1h search immediately
      (0 events for an idle group — confirmed the mechanism runs
      rather than erroring). `t` cycled 1h → 3h → 24h, re-searching
      each time; on an active broker's audit log, 24h found 303 real
      events with the "(more available — narrow your search)"
      indicator correctly shown, newest-first ordering confirmed, and
      multi-line messages correctly truncated to a single preview line
      in the table.
    - Confirmed typing a pattern does **not** change the results
      (proving no live/keystroke search), then Enter narrowed 303 → 211
      events for a pattern known to match a subset of visible messages
      — confirms `FilterPattern` is genuinely applied server-side, not
      just cosmetic.
    - Opening a result showed the full, untruncated (multi-line)
      message; `c` copied it (status bar confirmed, message text never
      shown in the status bar); Esc returned to the search view with
      its state (pattern, time range, results) intact, then Esc again
      returned to the log group list.
    - `~/.cloudtui/cloudtui.log` had no unexpected errors across the
      whole session.
    - One self-inflicted incident during driving, not a product bug:
      after Esc from the search view, focus lands on the log group
      **table**, not the filter input — typing letters directly (e.g.
      "amazonmq/broker" without first pressing `/`) hit global hotkeys
      letter-by-letter, and the "q" in "amazonmq" quit the app. Redid
      the check pressing `/` first each time; no code changes needed
      (this is the same, already-tested-for behavior as every other
      list view's filter input).
    - No log message content — some lines may contain request/operational
      data — was pasted into any commit message, spec file, or this
      summary beyond confirming the mechanism works.
