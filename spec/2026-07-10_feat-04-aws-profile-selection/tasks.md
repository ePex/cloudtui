# Tasks — AWS profile selection

Plan: [plan.md](plan.md)

Each task below needs explicit manual approval before it is implemented.

1. [x] **`tui/internal/awsprofile/awsprofile.go` + `awsprofile_test.go`** —
   `List()`/`ListFrom(configPath, credentialsPath)`, `scanConfigProfiles`/
   `scanCredentialsProfiles`; tests against temp fixture files covering
   `[default]`, `[profile foo]`, excluded `[sso-session x]`, `[bar]` in
   credentials, cross-file de-dup, and both files missing.
   Status: done.

2. [x] **`tui/internal/config/config.go` + `config_test.go`** — `AWSConfig`
   (`Profile` field), `Save`/`SaveDefault`; tests for the save/load
   round-trip and the new field's default.
   Status: done.

3. [x] **`tui/internal/ui/views/settings.go`** (deleted) + **`views_test.go`**
   (updated) — remove the `settings` placeholder and its test case, since
   settings moves to `internal/app`.
   Status: done.

4. [x] **`tui/internal/app/topbar.go` + `topbar_test.go`** — extract
   `infoPanelText(cfg) string`; `topBar` gains an `info *tview.TextView`
   field; update existing tests and add one for `infoPanelText` with an
   `AWS.Profile` set vs. unset.
   Status: done. (Full package test run deferred to task 6 — `app.go`
   still references the deleted `views.NewSettings()` until then.)

5. [x] **`tui/internal/app/settings.go` + `settings_test.go`** —
   `newSettingsView(a *App) ui.View`; `openProfilePicker`/
   `selectProfile`/`closeProfilePicker` on `App`; tests using `t.Setenv`
   for `AWS_CONFIG_FILE`/`AWS_SHARED_CREDENTIALS_FILE` against fixtures
   (settings list reflects current profile; picker population and
   pre-selection; select persists + updates UI; Escape cancels
   cleanly).
   Status: done. Depends on `App.settingsList`/`infoPanel`/
   `refreshInfoPanel` from task 6 to compile/pass — not run standalone.

6. [x] **`tui/internal/app/app.go` + `app_test.go`** — wire
   `newSettingsView(a)` into the views slice, add the `"profile-picker"`
   `rootPages` page, call `refreshInfoPanel()` at startup; update the
   existing views-slice/default-view tests for the new construction
   path.
   Status: done. Also fixed a latent focus bug uncovered by this
   feature: `switchTo` now re-calls `SetFocus(a.pages)` after
   `SwitchToPage`, since `tview.Pages` only re-delegates focus to its
   front item when `Focus()` is (re-)called, not automatically on
   `SwitchToPage` — needed for the settings list's own key handling to
   actually receive input after pressing 's'. `app_test.go`'s existing
   5-view/default-view test already covered the new construction
   unmodified — no edit needed there. Full test suite passes.

7. [x] **`tui/config.example.yaml`** — document `aws.profile` (normally set
   via the picker, not hand-edited).
   Status: done.

8. [x] **Verify** — `gofmt -l .`, `go vet ./...`, `go test ./...` clean/
   passing; `go build ./cmd/tui` succeeds; best-effort run check (same
   tty/tmux caveat as before).
   Status: done for the automatable parts — all clean/passing/succeeding.
   5s run produced no error output; confirmed no stray `config.yaml` was
   written into the real `tui/` working directory by the test suite
   (`t.Chdir` isolation worked as intended). Interactive/visual
   confirmation still not possible from this sandboxed shell; recommend
   `task run:tui` to try the profile picker yourself.
