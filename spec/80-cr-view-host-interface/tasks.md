# Tasks — CR 80: `ui.ViewHost`

1. [x] Created `internal/ui/viewhost.go` with the `ViewHost` interface
   (embeds `Host`, declares the 28 new methods), doc comments per
   group. `gofmt -l`, `go build ./internal/ui/...` clean.

2. [x] Created `internal/app/viewhost.go` with the 14 genuinely-new
   wrapper methods (`SwitchToPage`, `SetPendingCloudWatchPattern`, and
   the 12 data-fetch methods), each delegating to the like-named
   unexported field. `gofmt -l`, `go build ./...` clean.

3. [x] Renamed `switchTo`→`SwitchTo` in `app.go`.

   **Correction during implementation**: plan.md's call-site table
   only covered production files; same-package test files reaching
   into the unexported method (`app_test.go`, 14 sites) weren't
   audited and initially broke `go vet`/`go test` (which `go build`
   alone doesn't catch, since it skips test files). Fixed by extending
   the rename to `app_test.go`. Also swept every doc-comment mention
   of `switchTo` across the package (11 files) to keep prose accurate
   now that the method is exported — not required for compilation,
   but left stale otherwise. `gofmt -l`, `go build ./...`,
   `go vet ./...`, `go test ./...` clean.

4. [x] Renamed `updateContextPanel`→`UpdateContextPanel` in `app.go`;
   fixed all 7 production call sites (`app.go`, `theme.go`,
   `codepipelinedetail.go`, `datadoglogdetail.go`, `logsearch.go`,
   `secretdetail.go`, `paramdetail.go`) — no test-file references this
   time (confirmed by grep). `gofmt -l`, `go build ./...`,
   `go vet ./...` clean.

5. [x] Renamed `copyToClipboard`→`CopyToClipboard` in `app.go`; fixed
   4 production call sites plus `app_test.go` (2 sites, same
   same-package-test gap as task 3). `gofmt -l`, `go build ./...`,
   `go vet ./...` clean.

6. [x] Renamed `isWatchingPipeline`→`IsWatchingPipeline`,
   `startWatchingPipeline`→`StartWatchingPipeline`,
   `stopWatchingPipeline`→`StopWatchingPipeline` in
   `codepipelinewatch.go`; fixed all call sites including
   `codepipelinelist_test.go`, `codepipelinedetail_test.go`,
   `codepipelinewatch_test.go` (found via `grep -rl` across all `*.go`
   from the start this time, per task 3's lesson). `gofmt -l`,
   `go build ./...`, `go vet ./...` clean.

7. [x] Renamed all 8 `open*`→`Open*` trampolines in `viewwiring.go`.

   **Correction during implementation**: plan.md estimated "8 internal
   call sites" (each `wireXOpensY` calling its paired `openX` once
   within `viewwiring.go`) — but every one of the 8 `Open*` methods
   turned out to have its own dedicated test coverage in `internal/app`
   (`app_test.go`, `codepipelinedetail_test.go`,
   `datadoglogdetail_test.go`, `logdetail_test.go`,
   `paramdetail_test.go`, `secretdetail_test.go` — ~25 call sites
   total) plus doc-comment references in 6 production files
   (`app.go`, `codepipelinedetail.go`, `datadoglogdetail.go`,
   `logdetail.go`, `logsearch.go`, `message_detail.go`, `messages.go`,
   `paramdetail.go`, `secretdetail.go`). First rename pass (scoped to
   `viewwiring.go` only, per the plan) left the build silently broken
   for `go vet`/`go test` — caught immediately since task 3 already
   established the "always grep the whole package, not just the
   planned files" check. Fixed by re-running the rename across all of
   `internal/app`. `gofmt -l`, `go build ./...`, `go vet ./...`,
   `go test ./...` clean — all pre-existing `TestOpen*` tests still
   pass unmodified (mechanical rename only, no behavior change).

8. [x] Added `var _ ui.ViewHost = (*App)(nil)` to
   `internal/app/viewhost.go` (plus the `internal/ui` import it
   needs). `go build ./...` passed immediately — confirms all 28
   methods are correctly implemented with matching signatures.

9. [x] Final verification pass: grep confirms zero remaining
   lowercase references to any of the 14 renamed methods anywhere in
   `internal/app`; `gofmt -l tui/` clean; `go vet ./...` clean;
   `go build ./...` and `go test ./...` pass repo-wide (all packages
   `ok`). No live verification needed — additive/rename only, per
   spec.md.
