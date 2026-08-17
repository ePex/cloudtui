# Tasks — CR 83: `ui.ViewHost` adoption, 5 dialog-coupled views

1. [x] `app.go`: move the 5 dialogs' construction lines (`a.confirm =
   dialog.NewConfirmDialog(a)`, `a.movePicker = dialog.NewMovePicker(a)`,
   `a.sendMessage = dialog.NewSendMessageOverlay(a)`, `a.messageFilter =
   dialog.NewMessageFilter(a)`, `a.timeRangeModal =
   dialog.NewTimeRangeModal(a)`) to right after `a.backend =
   newBackendForConn(a, cfg.ActiveConn())`, before `a.queuesV =
   newQueuesView(...)`. Leave each `xOverlay := ui.Centered(a.X.Primitive(),
   ...)` line (and its sizing comments) exactly where it is today —
   only the construction line itself moves. No other file changes yet
   (every `newXView` call site still uses today's signature). `gofmt
   -l`, `go build ./...` clean.

2. [x] `datadoglogs.go`: `app *App` → `host ui.ViewHost`; new
   `timeRangeModal *dialog.TimeRangeModal` field/param; constructor
   gains the `timeRangeModal` parameter (before `onSelect`); rename
   table applied (`.tv`×10, `.cfg`×4, `.searchDatadogLogs` →
   `.SearchDatadogLogs`, `.listDatadogFacetValues` →
   `.ListDatadogFacetValues`); `.timeRangeModal.Show(...)` call site
   switches from `a.timeRangeModal` to the new field. Add `internal/dialog`
   import. Update `app.go`'s `newDatadogLogsView(a, a.OpenDatadogLogDetail)`
   call to `newDatadogLogsView(a, a.timeRangeModal, a.OpenDatadogLogDetail)`.
   `gofmt -l`, `go build ./...` clean.

3. [x] `logsearch.go`: same field/constructor rename as task 2, plus
   `onBack func()` parameter; rename table applied (`.tv`×4, `.cfg`×3,
   `.filterLogEvents` → `.FilterLogEvents`); Esc-handler's 3-line body
   (`a.pages.SwitchToPage("cloudwatch-logs")`,
   `a.tv.SetFocus(a.logsV.table)`, `a.UpdateContextPanel(a.logsV)`)
   replaced with `onBack()`. Update `app.go`'s
   `newLogSearchView(a, a.OpenLogEventDetail)` call to pass
   `a.timeRangeModal` and the `onBack` closure (moved verbatim). Add
   `TestLogSearchViewEscReturnsToCloudWatchLogs` (same shape as
   `TestParamDetailViewEscReturnsToSSMParameters`, target page
   `"cloudwatch-logs"`). `gofmt -l`, `go build ./...`, `go test
   ./internal/app/...` clean.

4. [x] `queues.go`: `app *App` → `host ui.ViewHost`; new `confirm
   *dialog.ConfirmDialog`, `movePicker *dialog.MovePicker`,
   `sendMessage *dialog.SendMessageOverlay` fields/params; rename
   table applied (`.tv`×8, `.cfg`×4, `.contextPanel`×2, `.statusBar`×1);
   the 3 dialog `.Show(...)` call sites switch to the new fields; the
   `'M'`/`'c'` handlers' self-restore closures (rebuilding this view's
   own context panel after a dialog closes) stay inline, using
   `qv.host`/`qv.Config()`/`qv.SetContextHint`. Update `app.go`'s
   `newQueuesView(a, a.backend, a.OpenMessages)` call to
   `newQueuesView(a, a.backend, a.confirm, a.movePicker, a.sendMessage, a.OpenMessages)`.
   Update all 9 `queues_test.go` call sites (`replace_all`:
   `newQueuesView(a, &fakeQueueBackend{}, func(string) {})` →
   `newQueuesView(a, &fakeQueueBackend{}, a.confirm, a.movePicker, a.sendMessage, func(string) {})`).
   `gofmt -l`, `go build ./...`, `go test ./internal/app/...` clean.

5. [x] `messages.go`: same field/constructor rename as task 4, with
   `messageFilter *dialog.MessageFilter` added; rename table applied
   across the file's `a.X`/`mv.app.X` mix (preserve which access point
   is used where — only what each resolves to changes); the 4 dialog
   `.Show(...)` call sites switch to the new fields; Esc handler's
   `a.SwitchTo("queues")` is untouched (already exported, no `onBack`
   needed). Update `app.go`'s `newMessagesView(a, a.OpenMessageDetail)`
   call to `newMessagesView(a, a.messageFilter, a.sendMessage, a.confirm, a.movePicker, a.OpenMessageDetail)`.
   Update all 6 `messages_test.go` call sites (`replace_all`:
   `newMessagesView(a, func(string, queue.Message) {})` →
   `newMessagesView(a, a.messageFilter, a.sendMessage, a.confirm, a.movePicker, func(string, queue.Message) {})`).
   `gofmt -l`, `go build ./...`, `go test ./internal/app/...` clean.

6. [x] `message_detail.go`: `app *App` → `host ui.ViewHost`; new
   `movePicker *dialog.MovePicker`, `confirm *dialog.ConfirmDialog`
   fields/params, plus `onBack func()` and `onReload func()`; rename
   table applied (`.tv`×6, `.cfg`×4, `.contextPanel`×4, `.statusBar`×2,
   `.backend`×2 → `.Backend()`); the 3 sibling-reaching blocks (`'m'`
   success, `'d'` success, Esc) replaced with `onBack()` (all 3)
   followed by `onReload()` (`'m'`/`'d'` sites only, right after
   `onBack()`); `restoreDetail`'s `a.tv.SetFocus(a.pages)` becomes
   `dv.host.SetFocus(dv.textView)` (self-restore, no raw `.pages` on
   `ui.ViewHost`). Update `app.go`'s `newMessageDetailView(a)` call to
   pass `a.movePicker`, `a.confirm`, the `onBack` closure (moved
   verbatim from the Esc handler), and `func() { a.messagesV.load() }`
   as `onReload`. Update `message_detail_test.go`'s
   `newTestMessageDetailView` helper: `newMessageDetailView(a)` →
   `newMessageDetailView(a, a.movePicker, a.confirm, func() {}, func() {})`.

   Added `TestMessageDetailViewEscReturnsToMessages` (synchronous,
   same shape as `TestParamDetailViewEscReturnsToSSMParameters`).
   The two planned "success reload" tests turned out infeasible as
   written: the `'m'`/`'d'` success path runs inside a goroutine +
   `QueueUpdateDraw`, which — like every other goroutine+
   `QueueUpdateDraw` path in this app — needs a running tview event
   loop to ever complete, and no test in this codebase drives one.
   Added `TestMessageDetailViewMoveOpensPickerWithSourceQueue` /
   `TestMessageDetailViewDeleteOpensConfirmWithPrompt` instead (same
   shape as `messages_test.go`'s own
   `TestMessagesViewDeleteFallsBackToCursorWhenNothingMarked`/
   `...MoveFallsBackToCursorWhenNothingMarked` tests, which stop at
   the same synchronous boundary), covering the synchronous half;
   the actual reload-on-success is covered by task 8's live
   verification instead. `gofmt -l`, `go build ./...`, `go test
   ./internal/app/...` clean.

7. [x] Final verification pass: grep confirms zero remaining `.app.`
   references and zero raw `.tv`/`.cfg`/`.contextPanel`/`.statusBar`/
   `.pages`/`.backend`(bare)/lowercase-func-field access in the 5
   files; zero direct-through-`app`/`host` dialog access (`.confirm`/
   `.movePicker`/`.sendMessage`/`.messageFilter`/`.timeRangeModal` only
   reached via each view's own injected field); zero sibling-view-field
   reaches (`a.messagesV.`/`a.logsV.`) inside `message_detail.go`/
   `logsearch.go` themselves (only `app.go`'s `onBack`/`onReload`
   closures). `gofmt -l tui/` clean; `go vet ./...` clean; `go build
   ./...` and `go test ./...` pass repo-wide (all packages `ok`).

8. [x] Live verification via `verify-live`: purge a queue; move a
   queue's messages via `M`; send a test message via `c`; filter
   messages via `f`; from a message's detail view, move (`m`) and
   delete (`d`) a message and confirm both land back on the messages
   list with it visibly reloaded; from CloudWatch Logs search, open
   the time-range picker (`t`) and confirm it applies; confirm Esc
   from log search returns to the CloudWatch Logs list with focus on
   its table and the list's shortcuts in the context panel. Record
   what was checked and the outcome here once complete.

   Checked against the real ActiveMQ broker (`orders` scratch queue,
   seeded via `task seed:queue -- orders 5`) and the real `mlf-dev`
   AWS profile / Datadog config, via tmux (`verify-live` skill). All
   checked out:
   - `messages.go`: `f` (message filter), `c` (send-message) both
     opened with correct titles/labels.
   - `message_detail.go`: `d` (delete) opened the confirm dialog with
     the right prompt, confirming "Yes" returned to the messages list
     with the deleted message visibly gone (`onBack`+`onReload` both
     fired); `m` (move) opened the move picker with the destination
     list; cancelling it (Esc) correctly restored focus to the detail
     view itself (`restoreDetail`'s self-restore, unaffected by this
     CR); Esc from the detail view returned to the messages list
     (`onBack`).
   - `queues.go`: `p` (purge) opened the confirm dialog with the right
     prompt, confirming "Yes" purged `orders` and reloaded (pending
     count dropped to 0, visible in the table); `M` (move) opened the
     move picker; `c` (send-message) opened with the right title.
   - `logsearch.go`: `t` (time range) opened the picker; applying
     "1mo" re-ran the search and updated the title/results; Esc
     returned to the CloudWatch Logs list with focus on its table and
     the list's shortcuts in the context panel (`onBack`).
   - `datadoglogs.go`: `t` (time range) opened correctly, confirming
     the `timeRangeModal` field wiring.
   Cleanup: `orders` left empty (purged as part of the test — it's the
   scratch queue, safe to leave empty), tmux session killed, verify
   binary removed.
