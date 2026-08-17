# Tasks — CR 71: promote 4 shared helpers to `internal/ui`

1. [x] Create `internal/ui/style.go` (`StyleList`, `StyleDropDown`) and
   `internal/ui/style_test.go` (moved `TestStyleListAppliesSelectionColors`,
   updated to call `StyleList`) per plan.md. `gofmt -l`, `go build ./...`,
   `go test ./internal/ui/...` clean — this task alone builds fine since
   it's pure addition, no removal yet.

2. [x] Create `internal/ui/filter.go` (`ParseFilterDate`,
   `ParseMessageFilterForm`, `filterDateLayout`, `filterDateTimeLayout`)
   and `internal/ui/filter_test.go` (moved `TestParseMessageFilterForm`,
   updated to call `ParseMessageFilterForm`) per plan.md. `gofmt -l`,
   `go build ./...`, `go test ./internal/ui/...` clean — same as task
   1, pure addition.

3. [x] Remove `styleList` from `theme.go`, `styleDropDown` from
   `settings.go`, and `parseFilterDate`/`parseMessageFilterForm`/
   `filterDateLayout`/`filterDateTimeLayout` from `messages.go`.
   Update every caller listed in plan.md's table
   (`theme.go`, `confirm.go`, `movepicker.go`, `sendmessage.go`,
   `timerangemodal.go`, `connections.go`, `datadoglogs.go`,
   `messagefilter.go`) to the `ui.`-qualified name. Remove the two now-
   superseded tests from `theme_test.go`/`messages_test.go`. `gofmt -l`,
   `go vet ./...`, `go build ./...`, `go test ./...` all clean.

   Also found (not in plan.md's table, caught by the build error):
   `settings.go`'s `themePicker` section calls `styleList` twice
   (`newSettingsView`'s own list styling, and `themePicker.ApplyPalette`)
   — both updated to `ui.StyleList` too. `messages.go` needed `strconv`
   and `time` imports trimmed (only used by the two removed functions).

4. [x] Final verification pass: confirm none of `theme.go`/
   `settings.go`/`messages.go` defines `styleList`/`styleDropDown`/
   `parseFilterDate`/`parseMessageFilterForm` anymore (`grep -n 'func
   styleList\|func styleDropDown\|func parseFilterDate\|func
   parseMessageFilterForm' tui/internal/app/*.go` returns nothing);
   `gofmt -l tui/` and `go vet ./...` clean repo-wide; `go build ./...`
   and `go test ./...` pass repo-wide. No commit needed unless this
   surfaces something to fix.

   Confirmed: zero matches, all checks clean repo-wide.
