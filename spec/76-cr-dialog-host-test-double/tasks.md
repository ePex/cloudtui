# Tasks — CR 76: `ui.Host` test double

1. [x] Create `internal/app/hosttest_test.go` with the `testHost` type,
   `savedConnectionCall`, `newTestHost()`, all 20 `ui.Host` method
   implementations, and `var _ ui.Host = (*testHost)(nil)`, per
   plan.md's template. `gofmt -l`, `go build ./...`,
   `go vet ./...` clean (pure addition — nothing calls it yet).

2. [x] Converted `connections_test.go`: added `newTestConnEditor(t)`,
   rewrote both `TestConnEditorEscapeCloses`/
   `TestConnEditorOtherKeysPassThrough` to use it instead of
   `New(config.Default())`. `gofmt -l`, `go build ./...`,
   `go test ./internal/app/... -run TestConnEditor` pass.

3. [x] Converted `datadogsettings_test.go`: added
   `newTestDatadogEditor(t)`, rewrote
   `TestDatadogEditorEscapeCloses`/`TestDatadogEditorOtherKeysPassThrough`/
   `TestDatadogEditorPrefillsFromConfig` to use it. Left
   `TestSaveDatadogEditorRoundTrip` untouched. `gofmt -l`,
   `go build ./...`,
   `go test ./internal/app/... -run TestDatadogEditor` pass.

4. [x] Moved `TestDatadogSettingsLabel` from `datadogsettings_test.go`
   into `settings_test.go`.

   Correction during implementation: `settings_test.go` already
   existed (11 unrelated tests covering the settings list/theme
   switching, e.g. `TestSettingsListHasFourItems`,
   `TestSwitchThemeAppliesPalette`) — plan.md's "new file" was wrong;
   an earlier check (`ls confirm_test.go movepicker_test.go
   sendmessage_test.go messagefilter_test.go themepicker_test.go`)
   never actually checked `settings_test.go` itself. Caught
   immediately after `git add` showed it as modified, not added,
   before any commit — recovered the original content with `git
   checkout HEAD -- settings_test.go` and re-applied
   `TestDatadogSettingsLabel` as an append instead of a file
   replacement. All 11 original tests plus the moved one verified
   passing. `gofmt -l`, `go build ./...`,
   `go test ./internal/app/... -run 'TestSettings|TestSwitchTheme|TestRefreshSettingsList|TestDatadogSettingsLabel'`
   all pass.

5. [x] Converted `timerangemodal_test.go`: added
   `newTestTimeRangeModal(t)`, rewrote all 13 tests to use it,
   including replacing `TestApplyTimeRangeAbsoluteInvalidDate`'s
   `a.statusBar.GetText(true)` check with an assertion against
   `host.status`. Rewrote the file in full (via `Write`, same
   reasoning as CR 75's equivalent rewrite — the volume of mechanical
   `a.timeRangeModal.X` → `tm.X` substitutions across 13 tests made
   per-line `Edit` calls impractical given many near-duplicate lines).
   `gofmt -l`, `go build ./...` clean; all 13 renamed/kept test
   functions pass (`TestShowTimeRangeModal*`, `TestCloseTimeRangeModal`,
   `TestTimeRangeModal*`, `TestSwitchTimeRangeTab*`, `TestApplyTimeRange*`).

6. [x] Final verification pass: confirmed zero `New(config.Default())`
   calls remain in `connections_test.go`/`timerangemodal_test.go`, and
   exactly one remains in `datadogsettings_test.go`
   (`TestSaveDatadogEditorRoundTrip`, expected — the one left-alone
   integration test). `gofmt -l tui/` clean; `go vet ./...` clean;
   `go build ./...` and `go test ./...` pass repo-wide (all packages
   `ok`). No live verification needed — test-infrastructure only, no
   production-code behavior change.
