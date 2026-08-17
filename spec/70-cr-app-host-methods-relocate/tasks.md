# Tasks — CR 70: relocate 4 `App` methods into `host.go`

1. [x] Move `SaveConnection` and `DeleteConnection` from `connections.go`
   into `host.go` (appended after `FocusMessages()`), verbatim. Trim
   any now-unused imports in `connections.go`. `gofmt -l`,
   `go vet ./...`, `go build ./...`, `go test ./...` all clean.

2. [x] Move `SaveDatadogConfig` from `datadogsettings.go` into
   `host.go`, verbatim. Trim any now-unused imports in
   `datadogsettings.go`. `gofmt -l`, `go vet ./...`, `go build ./...`,
   `go test ./...` all clean.

3. [x] Move `SetActiveAWSProfile` from `awsprofiles.go` into `host.go`,
   verbatim. Trim any now-unused imports in `awsprofiles.go`.
   `gofmt -l`, `go vet ./...`, `go build ./...`, `go test ./...` all
   clean.

4. [x] Final verification pass: confirm `connections.go`,
   `datadogsettings.go`, `awsprofiles.go` define no `(a *App)` method
   (`grep -n 'func (a \*App)'` on each returns nothing); `gofmt -l tui/`
   and `go vet ./...` clean repo-wide; `go build ./...` and
   `go test ./...` pass repo-wide. No commit needed unless this
   surfaces something to fix.

   Confirmed: zero `(a *App)` methods in any of the three donor files;
   all checks clean. Per spec.md, pure code motion of already-tested
   logic — no live re-verification needed, `go test ./...` passing is
   sufficient.
