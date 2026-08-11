# Tasks — Bugfix 40

1. [x] Add `a.datadogEditorVisible` to `onGlobalKey`'s overlay-exemption
   block in `internal/app/app.go`.
2. [x] Add `a.datadogEditorVisible` to `onPromptDone`'s parallel
   focus-restoration check.
3. [x] Regression test:
   `TestOnGlobalKeyPassesThroughWhenDatadogEditorVisible` in
   `internal/app/app_test.go`.
4. [x] Full suite (`go build`, `go vet`, `go test ./...`) passes.
