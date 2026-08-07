# Tasks — FE 22: Named connections

1. [x] Update `tui/internal/config/config.go`: add `Connection` struct; replace top-level `Backend`/`Queue`/`Proxy` with `Connections []Connection` / `ActiveConnection string`; update `Default()`; add `ActiveConn()` helper; update `Load()` with migration and new env-var injection.
2. [x] Update `tui/internal/config/config_test.go`: remove/update broken tests, add migration / `ActiveConn` / new-shape tests.
3. [x] Update `tui/internal/app/topbar.go`: `infoPanelText()` adds `Connection: <alias>` line.
4. [x] Update `tui/internal/app/topbar_test.go`: add test for Connection alias in info panel.
5. [x] Update `tui/internal/app/app.go`: add `newBackendForConn()`; `switchConnection()`; conn overlay fields on `App`; update `New()` backend init and `rootPages` wiring; guard `connManagerVisible`/`connEditorVisible` in `onGlobalKey`.
6. [x] Update `tui/internal/app/settings.go`: replace `tview.Form` with `tview.List`; add `refreshSettingsList()`, `showThemePicker()`, `closeThemePicker()`; theme picker overlay wired in `app.go`; `reapplyTheme` updated in `theme.go`.
7. [x] Create `tui/internal/app/connections.go`: `showConnectionManager()`, `closeConnManager()`, `populateConnManagerList()`, `showConnEditor()`, `closeConnEditor()`, `saveConnEditor()`, `deleteConnFromManager()`.
8. [x] Update `tui/config.example.yaml`: replace legacy `backend`/`queue`/`proxy` fields with `activeConnection` / `connections` shape.
9. [x] Run `go build ./...` and `go test ./...` in `tui/` — all must pass.
