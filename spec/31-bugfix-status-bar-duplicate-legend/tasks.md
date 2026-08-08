# Tasks — Bugfix 31: status bar duplicated the legend; Home listed itself

1. [x] `statusbar.go`: remove `readyStatusText`; `newStatusBar` sets no
   default text.
2. [x] `theme.go`: `reapplyTheme` recolors the status bar only, no longer
   resets its text.
3. [x] `home.go`: drop `h`/`home` from `Shortcuts()`.
4. [x] Update/replace tests: `statusbar_test.go` (blank at idle, dropped
   the `readyStatusText` test), `views_test.go` (5-entry list, explicit
   "no `h` entry" test), `app_test.go` (context panel check updated to
   the 5-entry set and asserts "home" is absent).
5. [x] `go build ./...`, `go vet ./...`, `go test ./...` — all pass.
6. [x] Manual verification: status bar blank at idle on both Home and Log
   (confirms it's not Home-specific special-casing); Home's context panel
   shows `?`/`l`/`s`/`q`/`:` only.
