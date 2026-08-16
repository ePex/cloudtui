# Tasks — CR 64: move chrome files into `internal/ui`

1. [x] Create `internal/ui/topbar.go`, `internal/ui/statusbar.go`,
   `internal/ui/help.go`, `internal/ui/notify.go` — package `ui`, content
   moved from the corresponding `internal/app` files with the exported
   surface from plan.md (`TopBar` struct + all 7 fields, `NewTopBar`,
   `NewStatusBar`, `NewHelpModal`, `HelpModalWidth`, `HelpModalHeight`,
   `Centered`, `DesktopNotify`, `InfoPanelText`, `ShortcutPanelRows`);
   `newDivider`, `newInfoPanel`, `newLogoPanel`, `logoWidth`, `maxInt`
   stay unexported. Also move `topbar_test.go`, `statusbar_test.go`,
   `help_test.go` into `internal/ui`, updating their references to the
   now-exported names. `gofmt -l`, `go build ./...` (package `ui` alone
   won't compile yet since `internal/app` still has the old files too —
   that's fine, resolved in task 2).

2. [x] Delete the six original files from `internal/app`
   (`topbar.go`, `statusbar.go`, `help.go`, `notify.go`,
   `topbar_test.go`, `statusbar_test.go`, `help_test.go`). Update all
   cross-package call sites: `app.go` (`newTopBar`, `tb.*` field access,
   `newStatusBar`, `desktopNotify`, all 10 `centered(...)` calls,
   `newHelpModal`/`helpModalWidth`/`helpModalHeight`, `infoPanelText`),
   `connections.go` and `awsprofiles.go` and `theme.go` (`infoPanelText`
   → `ui.InfoPanelText`, adding the `internal/ui` import to each),
   `messages_test.go` (`shortcutPanelRows` → `ui.ShortcutPanelRows`,
   adding the `internal/ui` import). `gofmt -l`, `go vet ./...`,
   `go build ./...`, `go test ./...` all clean.

3. [x] Final verification pass: confirm `internal/app` is four
   (non-test) files smaller, `internal/ui` gained exactly the four
   source + three test files, `gofmt -l tui/` and `go vet ./...` clean
   repo-wide, `go build ./...` and `go test ./...` pass repo-wide, and a
   quick manual launch (`task run` or equivalent) shows the top bar,
   status bar, help overlay (`?`), and info panel unchanged. No commit
   needed unless this surfaces something to fix.

   Verified live via tmux (`verify-live` skill's driving pattern): built
   the binary, launched it, confirmed the top bar (info panel, divider,
   context panel, CLOUDTUI logo) renders correctly on the Home screen,
   pressed `?` and confirmed the help overlay renders centered with all
   keybindings, closed cleanly with Escape then `q`. No visual
   regressions found.
