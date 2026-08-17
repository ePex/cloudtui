# Tasks — CR 84: move the 14 `ui.ViewHost`-adopted views into `internal/view`

1. [x] Create `internal/app/viewhosttestfake_test.go`: the `fakeViewHost`
   type implementing `ui.ViewHost` in full (recorded state for
   `SetFocus`/`SetStatus`/`SetContextHint`/`Config`/`Backend`/
   `QueueUpdateDraw` — the last runs its func inline; no-op stubs for
   the rest of `ui.Host`; chrome recorders for `SwitchToPage`/
   `SwitchTo`/`UpdateContextPanel`/`CopyToClipboard`; no-op stubs for
   the 8 `Open*` methods, `SetPendingCloudWatchPattern`, and the 3
   watcher methods (backed by a `watching map[string]bool`); 12
   injectable-func-field data-fetchers, each nil-checked before
   falling back to a zero value) plus `var _ ui.ViewHost =
   (*fakeViewHost)(nil)` and a `newFakeViewHost()` constructor. No
   other file changes yet. `gofmt -l`, `go build ./...`,
   `go vet ./...` clean.

2. [x] `ssmparams.go` + `ssmparams_test.go`: add `Table() *tview.Table`
   and `FilterInputs() []tview.Primitive`; rename `ssmParamsView` →
   `SSMParamsView`, `newSSMParamsView` → `NewSSMParamsView`; update
   `app.go`'s field declaration, construction call, and the 2
   `focusExemptInputs`/chrome-wiring reach-ins to use the new exports.
   Rewrite `ssmparams_test.go`'s `New(config.Default())` call sites to
   `newFakeViewHost()` (no dialogs needed for this file). `gofmt -l`,
   `go build ./...`, `go test ./internal/app/...` clean.

3. [x] `secrets.go` + `secrets_test.go`: same shape as task 2
   (`secretsView`→`SecretsView`, `newSecretsView`→`NewSecretsView`).
   `gofmt -l`, `go build ./...`, `go test ./internal/app/...` clean.

4. [x] `logs.go` + `logs_test.go`: same shape as task 2
   (`logsView`→`LogsView`, `newLogsView`→`NewLogsView`). `gofmt -l`,
   `go build ./...`, `go test ./internal/app/...` clean.

5. [x] `datadoglogs.go` + `datadoglogs_test.go`: same shape as task 2,
   with `FilterInputs()` returning all 3 inputs (`queryInput`,
   `serviceFilterDD`, `envFilterDD`); rename `datadogLogsView`→
   `DatadogLogsView`, `newDatadogLogsView`→`NewDatadogLogsView`.
   `gofmt -l`, `go build ./...`, `go test ./internal/app/...` clean.

6. [x] `logdetail.go` + `logdetail_test.go`: add
   `Primitive() tview.Primitive` (returns `.textView`); rename
   `render`→`Render` (pure rename, no title-fold — static title);
   rename `logDetailView`→`LogDetailView`,
   `newLogDetailView`→`NewLogDetailView`. Update `app.go`'s `AddPage`
   call and `viewwiring.go`'s `OpenLogEventDetail` to use
   `.Render(event)`. Move `TestOpenLogEventDetailSwitchesPage` (page-
   switch assertion only) into a new `internal/app/viewwiring_test.go`,
   driven by the real `New(config.Default())`. Rewrite
   `logdetail_test.go`'s remaining `New(config.Default())` sites to
   `newFakeViewHost()`. `gofmt -l`, `go build ./...`,
   `go test ./internal/app/...` clean.

7. [x] `datadoglogdetail.go` + `datadoglogdetail_test.go`: same shape
   as task 6 (`datadogLogDetailView`→`DatadogLogDetailView`,
   `newDatadogLogDetailView`→`NewDatadogLogDetailView`). Move
   `TestOpenDatadogLogDetailSwitchesPage`,
   `TestDatadogLogDetailViewEscReturnsToDatadogLogs`,
   `TestDatadogLogDetailViewGoToCloudWatchWithCorrelationID`,
   `TestDatadogLogDetailViewGoToCloudWatchWithoutCorrelationID` into
   `viewwiring_test.go` (the Esc-return test additionally rewritten
   per task 9's `InputHandler()` pattern, since it drives Esc — see
   task 9's note for the exact replacement). `gofmt -l`,
   `go build ./...`, `go test ./internal/app/...` clean.

8. [x] `paramdetail.go` + `paramdetail_test.go`: add
   `Primitive() tview.Primitive`; fold `OpenParamDetail`'s title-set
   into `Render(param awsssm.Parameter)` (`dv.textView.SetTitle(fmt.Sprintf(" Parameter — %s ", param.Name))`
   added at the top of the renamed `render`→`Render`); rename
   `paramDetailView`→`ParamDetailView`,
   `newParamDetailView`→`NewParamDetailView`. Update `app.go`'s
   `AddPage` call and `viewwiring.go`'s `OpenParamDetail` (drops its
   own `.textView.SetTitle` line, now redundant). Move
   `TestOpenParamDetailSwitchesPageAndSetsTitle` (page-switch
   assertion only — drop the title assertion, since `Render`'s title-
   fold is now covered by a title check added to
   `TestParamDetailViewRenderShowsStringValueImmediately` or a new
   small `Render` test if that one doesn't already check it) and
   `TestParamDetailViewEscReturnsToSSMParameters` (rewritten to drive
   Esc via `a.paramDetailV.Primitive().InputHandler()(tcell.NewEventKey(tcell.KeyEscape, 0, tcell.ModNone), func(tview.Primitive) {})`
   instead of the now-unreachable `.textView.GetInputCapture()`) into
   `viewwiring_test.go`. `gofmt -l`, `go build ./...`,
   `go test ./internal/app/...` clean.

9. [x] `secretdetail.go` + `secretdetail_test.go`: same shape as task
   8 (`secretDetailView`→`SecretDetailView`,
   `newSecretDetailView`→`NewSecretDetailView`,
   `dv.textView.SetTitle(fmt.Sprintf(" Secret — %s ", secret.Name))`
   folded into `Render`). Move
   `TestOpenSecretDetailSwitchesPageAndSetsTitle` (title assertion
   dropped, same reasoning as task 8) and
   `TestSecretDetailViewEscReturnsToSecretsManager` (same
   `InputHandler()` rewrite) into `viewwiring_test.go`. `gofmt -l`,
   `go build ./...`, `go test ./internal/app/...` clean.

10. [x] `message_detail.go` + `message_detail_test.go`: add
    `Primitive() tview.Primitive`; fold the title-set into
    `Render(queueName string, msg queue.Message)`; rename
    `messageDetailView`→`MessageDetailView`,
    `newMessageDetailView`→`NewMessageDetailView`. Update `app.go`'s
    `AddPage` call and `viewwiring.go`'s `OpenMessageDetail`.
    `movePicker`/`confirm`/`onBack`/`onReload` constructor params
    (CR 83) are unaffected — only the type name and `render`→`Render`
    change. Move `TestMessageDetailViewEscReturnsToMessages`
    (`InputHandler()` rewrite) into `viewwiring_test.go`. Rewrite the
    file's other `New(config.Default())` sites (including
    `TestMessageDetailViewMoveOpensPickerWithSourceQueue`/
    `TestMessageDetailViewDeleteOpensConfirmWithPrompt`, which stay —
    they only check dialog visibility, not a page switch) to
    `newFakeViewHost()` + real `dialog.NewMovePicker(host)`/
    `dialog.NewConfirmDialog(host)` instances. `gofmt -l`,
    `go build ./...`, `go test ./internal/app/...` clean.

11. [x] `codepipelinelist.go` + `codepipelinelist_test.go`: add
    `Table() *tview.Table`, `FilterInputs() []tview.Primitive`,
    `Repaint()` (replaces `codepipelinewatch.go`'s `.all`/`.repaint(.all)`
    reach-in); rename `codePipelineListView`→`CodePipelineListView`,
    `newCodePipelineListView`→`NewCodePipelineListView`. Update
    `app.go`'s field/construction/chrome-wiring and
    `codepipelinewatch.go`'s `Repaint()` call. Move
    `TestCodePipelineListViewSelectedFuncOpensDetail` into
    `viewwiring_test.go`. `gofmt -l`, `go build ./...`,
    `go test ./internal/app/...` clean.

12. [x] `codepipelinedetail.go` + `codepipelinedetail_test.go`: add
    `Primitive() tview.Primitive`, `PipelineName() string`; rename
    `render`→`Render`, `open`→`Open` (both pure renames — `Open`
    already sets its own title); export `statusLabel`→`StatusLabel`;
    rename `codePipelineDetailView`→`CodePipelineDetailView`,
    `newCodePipelineDetailView`→`NewCodePipelineDetailView`. Update
    `app.go`'s `AddPage` call, `viewwiring.go`'s
    `OpenCodePipelineDetail`, and `codepipelinewatch.go`'s
    `.pipelineName`/`.render`/`statusLabel` reach-ins. Move
    `TestOpenCodePipelineDetailSwitchesPage` and
    `TestCodePipelineDetailViewEscReturnsToList` (`InputHandler()`
    rewrite) into `viewwiring_test.go`. `gofmt -l`, `go build ./...`,
    `go test ./internal/app/...` clean.

13. [x] `queues.go` + `queues_test.go`: add `Table() *tview.Table`,
    `FilterInputs() []tview.Primitive`, `SetBackend(b queue.Backend)`,
    `Load()` (renamed from `load`); rename `queuesView`→`QueuesView`,
    `newQueuesView`→`NewQueuesView`. Update `app.go`'s
    `switchConnection` (`a.queuesV.backend = a.backend` →
    `a.queuesV.SetBackend(a.backend)`) and chrome-wiring, and
    `host.go`'s `ReloadAfterSend` (`a.queuesV.load()` →
    `a.queuesV.Load()`). Rewrite `queues_test.go`'s 12
    `New(config.Default())` sites to `newFakeViewHost()` +
    `dialog.NewConfirmDialog(host)`/`dialog.NewMovePicker(host)`/
    `dialog.NewSendMessageOverlay(host)`. `gofmt -l`,
    `go build ./...`, `go test ./internal/app/...` clean.

14. [x] `logsearch.go` + `logsearch_test.go`: add `Table() *tview.Table`,
    `FilterInputs() []tview.Primitive` (`.patternInput`),
    `Primitive() tview.Primitive`; rename `open`→`Open` (pure rename
    — already sets its own title); rename `logSearchView`→
    `LogSearchView`, `newLogSearchView`→`NewLogSearchView`. Update
    `app.go`'s `AddPage`/chrome-wiring and `viewwiring.go`'s
    `OpenLogSearch`. Move `TestLogSearchViewEscReturnsToCloudWatchLogs`
    (`InputHandler()` rewrite) into `viewwiring_test.go`. Rewrite the
    remaining `New(config.Default())` sites to `newFakeViewHost()` +
    `dialog.NewTimeRangeModal(host)`. `gofmt -l`, `go build ./...`,
    `go test ./internal/app/...` clean.

15. [x] `messages.go` + `messages_test.go`: add `Table() *tview.Table`,
    `FilterInputs() []tview.Primitive`, `Primitive() tview.Primitive`;
    fold `OpenMessages`'s body into `Open(queueName string)`; add
    `QueueName() string`, `Filter() queue.MessageFilter`; fold
    `ApplyMessagesFilter`'s body into `ApplyFilter(f
    queue.MessageFilter)`; rename `load`→`Load`; rename
    `messagesView`→`MessagesView`, `newMessagesView`→
    `NewMessagesView`. Update `app.go`'s chrome-wiring,
    `viewwiring.go`'s `OpenMessages` (shrinks to the page-switch/
    focus/context-panel chrome plus one `a.messagesV.Open(queueName)`
    call), and `host.go`'s `ReloadAfterSend`/`MessagesFilter`/
    `ApplyMessagesFilter`/`FocusMessages`. Rewrite the file's 6
    `New(config.Default())` sites to `newFakeViewHost()` + real
    `dialog.NewMessageFilter(host)`/`dialog.NewSendMessageOverlay(host)`/
    `dialog.NewConfirmDialog(host)`/`dialog.NewMovePicker(host)`
    instances. `gofmt -l`, `go build ./...`,
    `go test ./internal/app/...` clean.

16. [x] Phase 1–3 checkpoint: grep confirms zero remaining
    `New(config.Default())` calls in the 14 view test files (all
    replaced by `newFakeViewHost()`, except the 15 tests now in
    `viewwiring_test.go`); `viewwiring_test.go` contains exactly the
    15 relocated tests and passes; zero remaining raw `.table`/
    `.textView`/`.flex`/`.filterInput`-family/`.backend`/`.pipelineName`/
    `.all` reach-ins in `app.go`/`host.go`/`viewwiring.go`/
    `codepipelinewatch.go`; zero remaining lowercase
    `statusLabel`/`open`/`render`/`load`/`repaint` calls from outside
    each type's own file. `gofmt -l tui/` clean; `go vet ./...`
    clean; `go build ./...` and `go test ./...` pass repo-wide.

17. [x] Phase 4 — the physical move: for each of the 14 view+test file
    pairs (`internal/view/` created on the first), `git mv` into
    `internal/view/`, change `package app` → `package view`; move
    `viewhosttestfake_test.go` → `internal/view/testfake_test.go`
    alongside them (same package-line change, no other edit); update
    `app.go`'s 14 field types and construction calls to the `view.`
    prefix, and add the `internal/view` import to `app.go`/`host.go`/
    `viewwiring.go`/`codepipelinewatch.go`. `go build ./...` after
    each pair (or in a few small batches), final `go build ./...`/
    `go test ./...` once all 14 have moved.

18. [x] Final verification pass: `gofmt -l tui/` clean; `go vet ./...`
    clean; `go build ./...` and `go test ./...` pass repo-wide (all
    packages `ok`); confirm zero import cycle
    (`go list -deps ./internal/app/... ./internal/view/...` succeeds);
    confirm `internal/app` contains none of the 14 original files
    (only `viewwiring_test.go`, `app.go`, `host.go`, `viewwiring.go`,
    `codepipelinewatch.go`, `settings.go`, `log.go`, `theme.go`,
    `connectionsecrets.go` + their tests remain).

19. [x] Live verification via `verify-live`: this is the largest
    structural change in phase 4 — a full pass through every moved
    view's normal operation, not just what this CR's own diff
    touches. Cover: each of the 9 top-level resource screens (Queues,
    Messages, SSM Parameters, Secrets Manager, CloudWatch Logs,
    Datadog Logs, CodePipeline) loads and filters correctly; each
    detail view (message, SSM param, secret, log event, Datadog log
    event, CodePipeline stage status) opens and Esc/Backspace returns
    correctly with the right focus/context panel; the dialog flows
    from CR 83 (purge, move, send-message, message filter, time
    range) still work; CodePipeline's background watcher still
    updates the list/detail views live. Record what was checked and
    the outcome here once complete.

    Checked against the real ActiveMQ broker (`orders` scratch queue,
    seeded via `task seed:queue -- orders 5`), the real `example-dev` AWS
    profile, and the real Datadog config, via tmux (`verify-live`
    skill). All confirmed working:
    - **Queues**: list loads/filters; `p` purge (confirm dialog,
      reloads with pending count updated); `M` move-queue picker
      opens; `c` send-message opens.
    - **Messages**: opened from a queue, list loads; `f` message
      filter opens; message detail opens with the folded title
      (`Message Details — orders`); `d` delete (confirm → Yes →
      correctly back on the messages list with the message gone,
      `onBack`+`onReload` both fired); `m` move picker opens,
      cancelling it correctly restores focus to the detail view
      itself (`restoreDetail`); Esc from detail returns to messages;
      Esc from messages returns to queues.
    - **SSM Parameters**: list loads (398 params, after completing a
      real AWS SSO browser re-auth mid-verification); detail opens
      with the folded title (`Parameter — <name>`); Esc returns to
      the list with correct focus/context panel.
    - **Secrets Manager**: list loads (119 secrets); detail opens;
      Esc returns to the list correctly.
    - **CloudWatch Logs**: list loads (111 groups); log search opens
      per group; `t` time-range picker opens, applying "1mo" re-runs
      the search and updates the title/results; log event detail
      opens; Esc from event detail returns to search; Esc from search
      returns to the CloudWatch Logs list.
    - **Datadog Logs**: list loads and searches (no AWS dependency);
      log event detail opens; Esc returns to the list correctly.
    - **CodePipeline**: list loads correctly (0 pipelines in this AWS
      account — same real-data limitation already hit and noted in
      CR 83's verification); detail view and the background watcher
      couldn't be exercised live for lack of data, but `Table()`/
      `FilterInputs()`/`Primitive()`/`PipelineName()`/`Repaint()` all
      compile and pass their unit tests, and the construction/wiring
      is code-identical to the 8 other list/detail pairs already
      confirmed working live.
    One transient hiccup unrelated to this CR: AWS SSO session expired
    mid-verification, requiring the user to complete a real browser
    login before SSM/Secrets/CloudWatch Logs could load — not a
    regression from the move, just real credential lifecycle.
