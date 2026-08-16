# Tasks — CR 62: move detail-navigation trampolines out of app.go

Each task moves one pair (trampoline + wiring) and must build/test clean
before the next.

1. [x] `messages.go`: move `openMessages` + new `wireQueuesOpensMessages()`
       (wires `a.queuesV.table`); `New()`'s inline block → one call.
2. [x] `message_detail.go`: move `openMessageDetail` + new
       `wireMessagesOpensDetail()` (wires `a.messagesV.table`); `New()`'s
       inline block → one call.
3. [x] `paramdetail.go`: move `openParamDetail` + new
       `wireSSMParamsOpensDetail()` (wires `a.ssmParamsV.table`); `New()`'s
       inline block → one call.
4. [x] `secretdetail.go`: move `openSecretDetail` + new
       `wireSecretsOpensDetail()` (wires `a.secretsV.table`); `New()`'s
       inline block → one call.
5. [x] `logsearch.go`: move `openLogSearch` + new `wireLogsOpensSearch()`
       (wires `a.logsV.table`); `New()`'s inline block → one call.
6. [x] `logdetail.go`: move `openLogEventDetail` + new
       `wireLogSearchOpensEventDetail()` (wires `a.logSearchV.table`);
       `New()`'s inline block → one call.
7. [x] `datadoglogdetail.go`: move `openDatadogLogDetail` + new
       `wireDatadogLogsOpensDetail()` (wires `a.datadogLogsV.table`);
       `New()`'s inline block → one call.
8. [x] `codepipelinedetail.go`: move `openCodePipelineDetail` + new
       `wireCodePipelineListOpensDetail()` (wires
       `a.codePipelineListV.table`); `New()`'s inline block → one call.
9. [x] Verify: `go build ./...` and `go test ./...` pass in `tui/`;
       confirm `app.go`'s line count dropped by roughly 230 lines; manual
       live verification (`verify-live` skill) for 2–3 of the 8 pairs
       (e.g. queues→messages, messages→message-detail, SSM
       params→detail) — Enter opens the right view with the right
       content and context-panel shortcuts; the rest rely on each view's
       existing unit coverage of the same `open*` functions.

       **Results:** `gofmt -l .`, `go vet ./...`, `go build ./...`, and
       `go test ./...` all clean across the whole module.  `app.go` went
       from 762 to 602 lines (-160). Live-verified via `verify-live`
       against the real local broker (Jolokia) and real AWS SSM
       (`example-dev` profile):
       - queues→messages (pair 1): selecting the `orders` queue opened
         "Messages — orders", correct context-panel shortcuts, messages
         loaded correctly (seeded 3 via `task seed:queue`).
       - messages→message-detail (pair 2): Enter on a message opened
         "Message Details — orders" with correct headers/body and
         context panel.
       - SSM params→detail (pair 3): Enter on a parameter opened
         "Parameter — <name>" with correct value/type/last-modified and
         context panel, against the real 398-parameter SSM inventory.
       Cleanup: deleted the 3 seeded test messages from `orders`
       (mark-all + delete, confirmed via dialog), killed the tmux
       sessions, removed the scratch binaries.
