# Tasks — CR 85: adopt `ui.ViewHost` in `settings.go` and move it to `internal/view`

1. [x] `app.go`: move `a.connManager = dialog.NewConnManager(a, a.confirm)`,
   `a.datadogEditor = dialog.NewDatadogEditor(a)`, `a.themePicker =
   dialog.NewThemePicker(a)`, `a.awsProfiles =
   dialog.NewAWSProfilesPicker(a)` into the existing early-dialog
   block (added in CR 83, right after `a.backend =
   newBackendForConn(...)`), alongside `a.confirm`/`a.movePicker`/
   `a.sendMessage`/`a.messageFilter`/`a.timeRangeModal`. Leave each
   `xOverlay := ui.Centered(a.X.Primitive(), ...)` line (and
   `a.connEditor = dialog.NewConnEditor(a, a.connManager)`) exactly
   where it is — only the 4 construction lines move. No other file
   changes yet; `newSettingsView(a)`'s call site is untouched in this
   task. `gofmt -l`, `go build ./...` clean.

2. [x] `settings.go`: rewrite `SettingsView`'s struct/constructor per
   plan.md (`host ui.ViewHost` + 4 `*dialog.X` fields, `Name`/`Title`/
   `Primitive` unchanged); fold `(a *App) refreshSettingsList`'s body
   into an exported `Refresh()` method (using the injected dialog
   fields, `s.host.Config()` for `Theme`/`ActiveConn()`/
   `ActiveAWSProfile`/`Datadog`); add `ApplyPalette(p config.Palette)`
   folding in `theme.go`'s special-cased block (no nil guard —
   `ApplyPalette` is only ever called via `a.themables`, after
   construction). `app.go`: `settingsList *tview.List` field →
   `settingsV *SettingsView` (still same-package `*SettingsView` at
   this point, not yet `view.SettingsView`); move
   `a.settingsV = newSettingsView(a, a.themePicker, a.connManager, a.awsProfiles, a.datadogEditor)`
   to right after the early-dialog block (task 1's new position),
   replacing the local `settingsView := newSettingsView(a)` and its
   now-stale "safe at this point..." comment; `a.views` literal's
   `settingsView` → `a.settingsV`; `a.themables` literal gains
   `a.settingsV`; the 2 `a.refreshSettingsList()` call sites
   (`switchTheme`, `switchConnection`) → `a.settingsV.Refresh()`.
   `host.go`: the 3 `a.refreshSettingsList()` call sites
   (`SaveConnection`, `SaveDatadogConfig`, `SetActiveAWSProfile`) →
   `a.settingsV.Refresh()`. `theme.go`: delete the entire `if
   a.settingsList != nil { ... }` block from `reapplyTheme`. No test
   changes yet (none exist for Settings today). `gofmt -l`,
   `go build ./...`, `go vet ./...` clean.

3. [x] Add `List() *tview.List` to `settingsView` (mirrors `Table()`
   on the 8 list views CR 84 already added, same reason — an
   `internal/app`-level test needs to read the underlying primitive
   without unexported cross-package access). Physical move: `git mv
   internal/app/settings.go internal/view/settings.go`; `package app`
   → `package view`; `app.go`'s `settingsV` field and
   `newSettingsView(...)` call gain the `view.` qualifier
   (`*view.SettingsView`, `view.NewSettingsView(...)`).
   `internal/app/host_test.go`'s `TestSetActiveAWSProfilePersistsAndUpdatesUI`:
   `a.settingsList.GetItemText(2)` → `a.settingsV.List().GetItemText(2)`.
   `internal/app/settings_test.go` will fail to compile until task 4
   (it still references the pre-move `a.settingsList`/package-local
   `datadogSettingsLabel`) — expected, not a regression; everything
   else in the repo builds. `gofmt -l`, `go build ./...` clean
   (`go vet`/`go test` deferred to task 4).

4. [x] `internal/app/settings_test.go` existed already (12 tests,
   missed in this CR's original audit — corrected in spec.md/plan.md).
   Sort and relocate all 12: `TestSettingsListHasBorderAndTitle` →
   new `internal/view/settings_test.go` as `TestSettingsViewNameAndTitle`;
   `TestSettingsListHasFourItems`,
   `TestSettingsListItemThreeIsDatadog`,
   `TestSettingsListItemThreeShowsConfiguredDatadogSite`,
   `TestSettingsListItemTwoIsAWSProfile`,
   `TestSettingsListItemTwoShowsActiveAWSProfile`,
   `TestSettingsListItemsShowCurrentThemeAndConnection`,
   `TestDatadogSettingsLabel` → same new file, unchanged in
   substance, construction switched to the `fakeViewHost` +
   real-dialog pattern (`newTestSettingsView` helper, per plan.md);
   `TestSwitchThemeAppliesPalette`, `TestSwitchThemeUnknownIsNoOp`,
   `TestSwitchThemePersistsConfig` → `internal/app/theme_test.go`
   unchanged (they test `switchTheme`'s config mutation, not
   Settings); `TestRefreshSettingsListUpdatesTheme` stays in
   `internal/app` (moved into `viewwiring_test.go`, renamed
   `TestSwitchThemeRefreshesSettingsList` for clarity alongside the 3
   new wiring tests below), field access fixed to
   `a.settingsV.List()`. Delete the now-empty
   `internal/app/settings_test.go`. Add
   `TestSettingsViewRefreshPreservesCursorPosition` to the new
   `internal/view/settings_test.go` (the one piece of `Refresh()`'s
   logic none of the ported tests happened to cover). Add 3 new
   `internal/app` wiring tests to `viewwiring_test.go`
   (`TestSwitchConnectionRefreshesSettingsList`,
   `TestSaveConnectionRefreshesSettingsList`,
   `TestSaveDatadogConfigRefreshesSettingsList` — confirmed via grep
   that none of these 3 paths had settings-list coverage before).
   `gofmt -l`, `go build ./...`, `go vet ./...`, `go test ./...`
   clean.

5. [x] Final verification pass: grep confirms zero remaining
   `settingsList`/`refreshSettingsList` references anywhere in
   `internal/app`; `gofmt -l tui/` clean; `go vet ./...` clean;
   `go build ./...` and `go test ./...` pass repo-wide; confirm zero
   import cycle (`go list -deps ./internal/app/... ./internal/view/...`
   succeeds).

6. [x] Live verification via `verify-live`: open Settings, confirm
   all 4 rows show correct live values; open each picker (theme,
   connection manager, AWS profiles, Datadog editor) from its row;
   change the theme and confirm the Settings list's own row text and
   colors update immediately; switch the active connection and
   confirm the connection row updates; change the active AWS profile
   and confirm that row updates; edit the Datadog config and confirm
   that row updates. Record what was checked and the outcome here
   once complete.

   Checked via tmux against the real config/AWS profile (`example-dev`).
   Settings opened showing all 4 correct live rows (Theme: dark, AMQ
   Connection: default, AWS Profile: example-dev, Datadog: datadoghq.eu).
   Each row's picker opened correctly: theme picker (cyberpunk/dark
   list, current one starred), connection manager (real connection
   list), AWS profiles (69 real profiles), Datadog editor (real site/
   token form). Switching the theme to "cyberpunk" updated the
   Settings list's own row text ("Theme: cyberpunk") and recolored
   the list immediately (`ApplyPalette` via `a.themables`, replacing
   the deleted special-cased block) — switched back to "dark" as
   cleanup. Connection/AWS-profile/Datadog row updates were not
   re-exercised live beyond opening each picker (already covered by
   this CR's new `TestSwitchConnectionRefreshesSettingsList`/
   `TestSaveConnectionRefreshesSettingsList`/
   `TestSaveDatadogConfigRefreshesSettingsList`/existing
   `TestSetActiveAWSProfilePersistsAndUpdatesUI`, and mutating the
   real active connection/AWS profile/Datadog config as a side effect
   of live verification wasn't warranted here).
