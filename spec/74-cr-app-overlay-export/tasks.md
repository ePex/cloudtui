# Tasks — CR 74: export the 10 overlay types, constructors, `Show` methods

1. [x] Rename `confirmDialog` → `ConfirmDialog`, `newConfirmDialog` →
   `NewConfirmDialog`, `show` → `Show` in `confirm.go` (declaration,
   receivers, doc comments, `var _ ui.Themeable`). Update the one
   caller in `app.go`'s `New()`. `gofmt -l`, `go build ./...` will
   still fail here — external `.confirm.show(` call sites in
   `message_detail.go`/`queues.go`/`messages.go` and test files aren't
   fixed yet; that's expected until task 11. Just confirm the errors
   are only in those known files, nothing unexpected.

   Note: also found (and fixed on the spot, since it blocked the
   build) `connManager.confirm *confirmDialog` in `connections.go` and
   `app.go`'s `confirm *confirmDialog` field — plan.md deferred field
   type updates to task 10, but Go requires the type to exist wherever
   it's referenced, so each field-type reference to a renamed type had
   to be fixed in the same task as the rename, not batched at the end.
   Also found one external `.show()` call site missed by `spec/70`'s
   original audit: `connections.go:154`, `cm.confirm.show(...)` inside
   `connManager.delete()` — fixed in task 4 alongside the rest of that
   file.

2. [x] Rename `movePicker` → `MovePicker`, `newMovePicker` →
   `NewMovePicker`, `show` → `Show` in `movepicker.go`. Update the
   constructor call and field type in `app.go`. Confirmed remaining
   build errors were only the known external call sites.

3. [x] Rename `sendMessageOverlay` → `SendMessageOverlay`,
   `newSendMessageOverlay` → `NewSendMessageOverlay`, `show` → `Show`
   in `sendmessage.go`. Update the constructor call and field type in
   `app.go`. Confirmed remaining build errors were only the known
   external call sites.

4. [x] Rename both `connManager` → `ConnManager` and `connEditor` →
   `ConnEditor` in `connections.go` (constructors `NewConnManager`/
   `NewConnEditor`, both `show` → `Show`), keeping the
   `manager`/`editor` field names as-is. Updated both constructor
   calls and field types in `app.go`, plus `onPromptDone`'s
   `a.connManager.Show()`. Fixed the extra `cm.confirm.show(` call
   site found in task 1's note. Confirmed remaining build errors were
   only the known external call sites.

5. [x] Rename `messageFilter` → `MessageFilter`, `newMessageFilter` →
   `NewMessageFilter`, `show` → `Show` in `messagefilter.go`. Update
   the constructor call and field type in `app.go`; fixed one doc
   comment in `host.go` referencing `messageFilter.apply`. Confirmed
   remaining build errors were only the known external call sites.

6. [x] Rename `timeRangeModal` → `TimeRangeModal`, `newTimeRangeModal`
   → `NewTimeRangeModal`, `show` → `Show` in `timerangemodal.go`.
   Update the constructor call and field type in `app.go`. Confirmed
   remaining build errors were only the known external call sites.

7. [x] Rename `datadogEditor` → `DatadogEditor`, `newDatadogEditor` →
   `NewDatadogEditor`, `show` → `Show` in `datadogsettings.go`. Update
   the constructor call and field type in `app.go`. Confirmed
   remaining build errors were only the known external call sites.

8. [x] Rename `themePicker` → `ThemePicker`, `newThemePicker` →
   `NewThemePicker`, `show` → `Show` in `themepicker.go`. Update the
   constructor call and field type in `app.go`. Confirmed remaining
   build errors were only the known external call sites.

9. [x] Rename `awsProfilesPicker` → `AWSProfilesPicker`,
   `newAWSProfilesPicker` → `NewAWSProfilesPicker`, `show` → `Show` in
   `awsprofiles.go`. Update the constructor call and field type in
   `app.go`, plus `onPromptDone`'s `a.awsProfiles.Show()`. Confirmed
   remaining build errors were only the known external call sites.

10. [x] Update `app.go`'s remaining references. In practice this was
    already done incrementally in tasks 1–9 (Go requires a field's
    type to exist at the point of each rename, so field declarations
    and constructor calls couldn't be batched separately from their
    type's rename). Verified at this point: zero lowercase overlay
    type identifiers remain anywhere in `app.go`; all 10 field
    declarations, constructor calls, and both `onPromptDone` calls use
    the exported names.

11. [x] Updated `settings.go`: all 8 `.show()` → `.Show()` call sites
    (4 in `newSettingsView`, 4 in `refreshSettingsList`). Updated the 5
    external view files' `.show(` → `.Show(` call sites:
    `message_detail.go` (2), `queues.go` (3), `messages.go` (5,
    including `messageFilter.Show()` with no args),
    `logsearch.go` (1), `datadoglogs.go` (1). `go build ./...` passes
    (production code only — tests not yet fixed at this point).

12. [x] Updated test files with `.show(` calls: `app_test.go` (1 site,
    `datadogEditor.Show()`), `connections_test.go`,
    `datadogsettings_test.go`, `timerangemodal_test.go`,
    `awsprofiles_test.go` — the last of these wasn't on plan.md's
    original list of 7 (it references overlays only via the `a.X`
    field path, not the bare type name, so the earlier type-name grep
    missed it); found via `go vet`'s first failure after the
    production-code pass and fixed the same way. `.close(` calls left
    unchanged (method stays unexported). `messages_test.go` and
    `logsearch_test.go`/`datadoglogs_test.go` needed no changes — they
    only read fields (`.visible`, `.relativeList`, `.onApply`, etc.)
    via the unchanged field path, never the bare type name or a
    `.show(`/`.close(` call. `go build ./...` and `go test ./...` pass
    repo-wide.

13. [x] Final verification pass: `gofmt -l tui/` clean; `go vet ./...`
    clean; grep confirms zero remaining lowercase overlay type
    declarations/pointers (`type x struct` / `*x` for all 10 old
    names) and zero remaining `.show(` calls anywhere in `internal/app`;
    `go build ./...` and `go test ./...` pass repo-wide (all packages
    `ok`). No live verification needed — pure rename, per spec.md.
