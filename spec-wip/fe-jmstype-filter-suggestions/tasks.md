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
4. [ ] Manual verification via the `verify-live` skill against a real
   broker (both backends) — confirm the two-tier suggestion behavior,
   the scan status message, and that applying a suggested filter
   actually filters correctly (record what was checked here).
5. [ ] Merge-back: update `spec/08-message-browser-and-detail/spec.md`
   to document the JMS Type autocomplete (both tiers) and its known
   completeness limitation; delete
   `spec-wip/fe-jmstype-filter-suggestions/`.
