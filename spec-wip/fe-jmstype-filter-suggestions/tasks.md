# Tasks

1. [x] `MessagesView.AllMessages()` getter (`tui/internal/view/messages.go`).
2. [x] `Host.LoadedJMSTypes`/`Host.ScanJMSTypes` — interface methods in
   `tui/internal/ui/host.go`, implementations + `distinctJMSTypes` helper
   in `tui/internal/app/host.go`, unit tests in
   `tui/internal/app/host_test.go`, stub/fake methods in
   `tui/internal/dialog/hosttest_test.go` and
   `tui/internal/view/testfake_test.go`.
3. [x] Wire the two-tier autocomplete into
   `tui/internal/dialog/messagefilter.go` (styling, sentinel entry,
   `SetChangedFunc` scan trigger), plus new
   `tui/internal/dialog/messagefilter_test.go`.

   **Bug found and fixed during implementation**: `App.New()` builds
   `a.messageFilter` (line 227) before `a.messagesV` (line 237) —
   `NewMessagesView` takes the already-built `messageFilter` as a
   constructor argument, so this ordering can't simply flip.
   `NewMessageFilter`'s `SetAutocompleteFunc` call eagerly invokes
   `Autocomplete()` once immediately (the same gotcha already documented
   for the `:` prompt — see `spec/01`'s "must be called after
   StyleInputFieldAutocomplete" note), which called through to
   `LoadedJMSTypes` → `a.messagesV.AllMessages()` while `a.messagesV` was
   still nil, panicking (`TestNewRegistersViewsWithHomeDefault` caught
   this immediately). Fixed with the same `if a.messagesV != nil`
   guard `ReloadAfterSend` already uses in the same file for the same
   reason. `TestLoadedJMSTypesNilMessagesViewDoesNotPanic` pins it
   directly rather than relying on incidental coverage from every other
   test that constructs an `App`.
4. [x] Manual verification via the `verify-live` skill against a real
   broker (both backends) — confirm the two-tier suggestion behavior,
   the scan status message, and that applying a suggested filter
   actually filters correctly (record what was checked here).

   **Two more bugs found and fixed during this step** (both now covered
   by regression tests):

   1. **Stale suggestions on fresh open.** Opening the filter dialog for
      the first time after selecting a queue showed *only* the scan
      sentinel, never the real loaded types — until a keystroke forced a
      refresh. Same root cause as the `:` prompt's documented gotcha
      (spec/01): `SetAutocompleteFunc` eagerly builds and caches the
      drop-down once at construction time, and `SetText` (which `Show()`
      uses to prefill the field) doesn't itself refresh that cache. Fixed
      by calling `mf.jmsTypeItem.Autocomplete()` explicitly in `Show()`,
      mirroring the `:` prompt's own fix. Covered by
      `TestShowRefreshesStaleAutocomplete` (confirmed to actually fail
      without the fix, not just pass trivially).
   2. **Reentrant `SetText` corrupted the field's text buffer.**
      Originally `onJMSTypeChanged` called `mf.jmsTypeItem.SetText("")`
      synchronously upon detecting the sentinel — but that detection
      itself runs *from inside* `tview.InputField`'s own
      `SetText`-triggered change notification (accepting the sentinel via
      Enter calls `SetText(sentinel)`, which invokes our `SetChangedFunc`
      synchronously, from which we called `SetText` again on the same
      field). Observed live as visibly garbled/duplicated text
      (`"...for JMS typesfor JMS typesfor JMS types..."`) — tview's
      underlying text buffer isn't safe to mutate reentrantly from its
      own change callback. Fixed by moving the clear into
      `handleScanResult` (which runs from a completed goroutine's
      `QueueUpdateDraw`, genuinely outside the original input handler's
      call stack) instead of `onJMSTypeChanged`. Also added a guard in
      `apply()` refusing to submit while a scan is in flight, since the
      field visibly holds the sentinel text for that window now. Covered
      by updated `TestHandleScanResultMergesIntoSuggestions`/
      `TestHandleScanResultErrorSetsStatusAndClearsFlag` (assert the
      field is cleared) and new `TestApplyRefusesWhileScanning`.

   Verified end-to-end against a real local ActiveMQ broker
   (`localhost:8161`), on **both backends** using the same underlying
   broker: Jolokia (the `default` connection) and mq-proxy (the
   pre-existing `local-mq-proxy` connection, via `task dev:proxy:start`)
   — the backend distinction matters here since mq-proxy's
   `list-messages` endpoint requires a positive `maxCount` on every call.
   Seeded a scratch queue (`jmstype-verify-queue`, via
   `task test:queue:add`/`remove`) with 4 messages across 3 real JMS
   types (`OrderCreated` ×2, `OrderCancelled`, `PaymentFailed`, sent via
   direct Jolokia `sendTextMessage` JMX calls with a `JMSType` header,
   since neither `task seed:queue` nor the TUI's own "create message"
   sets one). On both backends: opening the filter fresh showed all
   three real types plus the sentinel; typing narrowed by prefix;
   selecting the sentinel scanned successfully (mq-proxy's `maxCount`
   requirement satisfied) and cleared the field without corruption;
   suggestions still worked after the scan; accepting a suggestion and
   pressing Apply correctly filtered the messages view to just the
   matching type. Cleanup: removed the scratch queue, stopped mq-proxy,
   and restored `config.yaml`'s `activeConnection` to `default` (switching
   connections during the mq-proxy check changed it).
5. [ ] Merge-back: update `spec/08-message-browser-and-detail/spec.md`
   to document the JMS Type autocomplete (both tiers) and its known
   completeness limitation; delete
   `spec-wip/fe-jmstype-filter-suggestions/`.
