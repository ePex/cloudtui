# Tasks — CR 81: constructor-injected selection callbacks

1. [x] `queues.go`: added `onSelect func(queueName string)` to
   `newQueuesView`; added the `table.SetSelectedFunc` wiring with the
   resolution logic from `wireQueuesOpensMessages`. Removed
   `wireQueuesOpensMessages` from `viewwiring.go`. Updated `app.go`'s
   `New()`: `newQueuesView(a, a.backend, a.OpenMessages)`, removed the
   `a.wireQueuesOpensMessages()` call and its ordering comment.

   **Correction during implementation**: `go build` alone doesn't
   catch same-package test files calling the old constructor
   signature (lesson already learned in CR 80, re-confirmed here) —
   `queues_test.go` had 12 direct `newQueuesView(a, &fakeQueueBackend{})`
   calls, all fixed to pass a no-op `func(string) {}` (none of these
   tests exercise selection). `go vet`/`go test` clean after.

2. [x] `messages.go`: added `onSelect func(queueName string, msg
   queue.Message)` to `newMessagesView`; added the wiring from
   `wireMessagesOpensDetail`. Removed it from `viewwiring.go`. Updated
   `app.go`: `newMessagesView(a, a.OpenMessageDetail)`, removed the
   wiring call. `messages_test.go` had 6 direct constructor calls,
   fixed with a no-op `func(string, queue.Message) {}`. `gofmt -l`,
   `go build ./...`, `go vet ./...`, `go test ./...` clean.

3. [x] `ssmparams.go`: added `onSelect func(param awsssm.Parameter)`
   to `newSSMParamsView`; added the wiring from
   `wireSSMParamsOpensDetail`. Removed it from `viewwiring.go`.
   Updated `app.go`: `newSSMParamsView(a, a.OpenParamDetail)`, removed
   the wiring call. No test file calls the constructor directly (all
   go through `New()`) — confirmed via grep before assuming so.
   `gofmt -l`, `go build ./...`, `go vet ./...`, `go test ./...`
   clean; `TestSSMParamsViewSelectedFuncMapsThroughFilter` (existing
   coverage) still passes unmodified.

4. [x] `secrets.go`: added `onSelect func(secret awssecrets.Secret)`
   to `newSecretsView`; added the wiring from
   `wireSecretsOpensDetail`. Removed it from `viewwiring.go`. Updated
   `app.go`: `newSecretsView(a, a.OpenSecretDetail)`, removed the
   wiring call. No direct test constructor calls. `gofmt -l`,
   `go build ./...`, `go vet ./...`, `go test ./...` clean.

5. [x] `logs.go`: added `onSelect func(logGroupName string)` to
   `newLogsView`; added the wiring from `wireLogsOpensSearch`.
   Removed it from `viewwiring.go`. Updated `app.go`: `newLogsView(a,
   a.OpenLogSearch)`, removed the wiring call. No direct test
   constructor calls. `gofmt -l`, `go build ./...`, `go vet ./...`,
   `go test ./...` clean.

6. [x] `logsearch.go`: added `onSelect func(event awslogs.LogEvent)`
   to `newLogSearchView`; added the wiring from
   `wireLogSearchOpensEventDetail`. Removed it from `viewwiring.go`.
   Updated `app.go`: `newLogSearchView(a, a.OpenLogEventDetail)`,
   removed the wiring call. No direct test constructor calls.
   `gofmt -l`, `go build ./...`, `go vet ./...`, `go test ./...`
   clean.

7. [x] `datadoglogs.go`: added `onSelect func(event
   datadoglogs.LogEvent)` to `newDatadogLogsView`; added the wiring
   from `wireDatadogLogsOpensDetail`. Removed it from `viewwiring.go`.
   Updated `app.go`: `newDatadogLogsView(a, a.OpenDatadogLogDetail)`,
   removed the wiring call. No direct test constructor calls.
   `gofmt -l`, `go build ./...`, `go vet ./...`, `go test ./...`
   clean.

8. [x] `codepipelinelist.go`: added `onSelect func(pipelineName
   string)` to `newCodePipelineListView`; added the wiring from
   `wireCodePipelineListOpensDetail`. Removed it from `viewwiring.go`.
   Updated `app.go`: `newCodePipelineListView(a,
   a.OpenCodePipelineDetail)`, removed the wiring call. No direct test
   constructor calls. Confirmed via grep: `viewwiring.go` has zero
   `wireXOpensY` methods left — only the 8 `OpenX` methods remain.
   `gofmt -l` flagged a trailing-whitespace/blank-line issue in
   `viewwiring.go` from the deletions; fixed with `gofmt -w`.
   `go build ./...`, `go vet ./...`, `go test ./...` clean.

9. [x] `datadoglogdetail.go`: fixed the stale
   `dv.app.pendingCloudWatchPattern = ...` write to
   `dv.app.SetPendingCloudWatchPattern(...)`. `gofmt -l`, `go build
   ./...` clean; `TestDatadogLogDetailViewGoToCloudWatchWithCorrelationID`
   (existing coverage for this exact path) still passes.

10. [x] Repo-wide verification: `gofmt -l tui/` clean; `go vet ./...`
    clean; `go build ./...` and `go test ./...` pass (all packages
    `ok`).

11. [x] Live verification via `verify-live`, against the real
    configured broker/AWS profile/Datadog config. Built
    `/tmp/cloudtui_verify81`, drove it via tmux. Verified 7 of the 8
    selection paths with real data end-to-end:
    - Queues → Messages: filtered to `orders`, Enter opened Messages
      for that queue.
    - Messages → Message Detail: seeded 1 message (`task seed:queue
      -- orders 1`), Enter opened the detail view with the right
      queue/ID/timestamp; deleted it afterward to restore the
      pre-seed state (0 pending, confirmed).
    - SSM Parameters → Param Detail: real parameter list (async
      load), Enter opened detail with correct name/type/value.
    - Secrets Manager → Secret Detail: 119 real secrets, Enter opened
      detail with correct name/ARN/value.
    - CloudWatch Logs → Log Search: 111 real log groups, Enter opened
      search pre-populated with 121 real events for the selected
      group.
    - Log Search → Log Event Detail: Enter on a result row opened the
      correct event (timestamp/stream/message matched the row
      selected).
    - Datadog Logs → Datadog Log Event Detail: real Datadog data (1000
      events in range), Enter opened detail with correct
      service/env/status/host/tags.
    8th path (CodePipeline → Pipeline Detail) **could not be exercised
    live** — the configured AWS account/profile has zero CodePipeline
    pipelines (confirmed empty list, not an error). Covered instead by
    the existing automated test `TestCodePipelineListViewSelectedFuncOpensDetail`
    (passing) plus structural symmetry with the other 7 confirmed-working
    pairs (identical resolution-logic shape, same code pattern).
    Quit cleanly with `q` (app closed the tmux session itself). No
    real data touched beyond the one seeded-and-deleted scratch
    message on `orders`; net state unchanged.
