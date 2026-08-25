# Tasks

1. [ ] `MessagesView.AllMessages()` getter (`tui/internal/view/messages.go`).
2. [ ] `Host.LoadedJMSTypes`/`Host.ScanJMSTypes` — interface methods in
   `tui/internal/ui/host.go`, implementations + `distinctJMSTypes` helper
   in `tui/internal/app/host.go`, unit tests in
   `tui/internal/app/host_test.go`, stub/fake methods in
   `tui/internal/dialog/hosttest_test.go` and
   `tui/internal/view/testfake_test.go`.
3. [ ] Wire the two-tier autocomplete into
   `tui/internal/dialog/messagefilter.go` (styling, sentinel entry,
   `SetChangedFunc` scan trigger), plus new
   `tui/internal/dialog/messagefilter_test.go`.
4. [ ] Manual verification via the `verify-live` skill against a real
   broker (both backends) — confirm the two-tier suggestion behavior,
   the scan status message, and that applying a suggested filter
   actually filters correctly (record what was checked here).
5. [ ] Merge-back: update `spec/08-message-browser-and-detail/spec.md`
   to document the JMS Type autocomplete (both tiers) and its known
   completeness limitation; delete
   `spec-wip/fe-jmstype-filter-suggestions/`.
