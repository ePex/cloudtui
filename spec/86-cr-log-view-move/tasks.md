# Tasks — CR 86: move `log.go` into `internal/view`

1. [x] `log.go`: rename `logView` → `LogView`; drop the `app *App`
   field entirely (struct becomes `{ textView *tview.TextView; path
   string }`); `newLogView(a *App)` → `NewLogView()`,
   `newLogViewWithPath(a *App, path string)` →
   `NewLogViewWithPath(path string)`, both dropping the `a`/`lv.app =
   a` assignment; `Name`/`Title`/`Primitive`/`Shortcuts`/`Activate`/
   `load`/`colorizeLog`/`logLevelColor`/`ApplyPalette` unchanged except
   receiver type. Still `package app` at this point — no move yet.
   `gofmt -l`, `go vet ./...` clean (will not compile yet — `app.go`
   and `log_test.go` still call the old signatures; that's expected,
   fixed in the next two tasks).

2. [x] `app.go`: field `logV *logView` → `logV *LogView`; construction
   call `a.logV = newLogView(a)` → `a.logV = NewLogView()`. `a.views`
   and `a.themables` literals untouched (both already reference the
   `a.logV` field, not a local var). `gofmt -l`, `go build ./...`,
   `go vet ./...` clean.

3. [x] `log_test.go`: fix the two `newLogViewWithPath(a, ...)` call
   sites in `TestLogViewActivateWithMissingFile`/
   `TestLogViewActivateLoadsFile` to `NewLogViewWithPath(...)` (drop
   the first argument, drop the now-unused `a := New(config.Default())`
   in each). `TestLogLevelColor`/`TestColorizeLogWrapsRecognizedLevels`/
   `TestColorizeLogEscapesLiteralBrackets` unchanged (no `*App`
   involved). Move `TestLogViewName`, `TestLogViewImplementsShortcuttable`,
   `TestLogViewShortcutsIncludeR` out of `log_test.go` into
   `viewwiring_test.go` unchanged in body (still exercise `a.logV` on
   a real `New(config.Default())`); rename `TestLogViewName` →
   `TestLogViewIsWiredAsLogPage` for clarity there. `gofmt -l`,
   `go build ./...`, `go vet ./...`, `go test ./...` clean.

4. [x] Physical move: `git mv internal/app/log.go internal/view/log.go`;
   `git mv internal/app/log_test.go internal/view/log_test.go`
   (now containing only the 3 pure-`LogView` tests left after task 3);
   `package app` → `package view` in both. `app.go`'s `logV` field and
   `NewLogView()` call gain the `view.` qualifier (`*view.LogView`,
   `view.NewLogView()`). `gofmt -l`, `go build ./...`, `go vet ./...`,
   `go test ./...` clean.

5. [x] Final verification pass: grep confirms zero remaining
   `logView`/`newLogView`/`newLogViewWithPath` references anywhere in
   `internal/app`; `gofmt -l tui/` clean; `go vet ./...` clean;
   `go build ./...` and `go test ./...` pass repo-wide; confirm zero
   import cycle (`go list -deps ./internal/app/... ./internal/view/...`
   succeeds).

   One stray reference found beyond code: `app.go`'s `activatable`
   interface doc comment ("e.g. logView reloads the log file on
   SwitchTo") named the now-moved type by its old unqualified name —
   updated to `view.LogView`. Everything else was clean on the first
   pass.

6. [x] Live verification via `verify-live`: open Log (`l`), confirm
   content loads (or "No log file found." if the file is absent) with
   level-based coloring; press `r` to refresh; switch theme and
   confirm the Log view's border/title recolor live via `ApplyPalette`.
   Record what was checked and the outcome here once complete.

   Checked via tmux against the real local config/log file. Opened Log
   (`l`): the real startup log line rendered
   (`time=... level=INFO msg=startup ...`), colored `[96m` (aqua) per
   its INFO level, confirming `colorizeLog`/`logLevelColor` are intact
   post-move. Pressed `r`: refreshed without error, same content
   redisplayed. Switched theme to "cyberpunk" via Settings, then
   reopened Log: the border/title recolored live to cyberpunk's accent
   (`rgb(255,0,60)`), confirming `ApplyPalette` still fires correctly
   through `a.themables` for the relocated view. Switched the theme
   back to "dark" as cleanup. Quit with `q`, killed the tmux session,
   removed the verify binary. `config.yaml` (gitignored) untouched
   beyond the theme round-trip; `git status` after showed only the
   intended source changes.
