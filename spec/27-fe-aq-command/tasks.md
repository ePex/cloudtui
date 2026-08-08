# Tasks — FE 27: `:aq` command prompt shortcut

Plan: [plan.md](plan.md)

1. [x] Add `aq`/`connections` case to `onPromptDone`.
2. [x] Guard the deferred `SetFocus(a.pages)` so it doesn't steal focus
   from an overlay the command just opened.
3. [x] Tests: `:aq` and `:connections` open the manager with correct
   focus; `:aq` works from a non-Settings view.
4. [x] `go build ./...`, `go vet ./...`, `go test ./...`.
5. [x] Manual verification: `:aq` from the Log view, confirmed via `n`
   (new connection) that keyboard focus actually reached the manager, not
   the log view underneath.
