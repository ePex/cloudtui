# Tasks

1. [x] `awsauth.Do[T any]` (`internal/awsauth/retry.go`): resolves
   `AuthType` internally then delegates to `WithReauth`, per
   `plan.md`. Tests in `retry_test.go`, mirroring the existing
   `TestWithReauthXxx` style: success-first-try, retries-after-login,
   `authTypeFor`'s error is ignored (call still proceeds, matching
   today's `_ :=` discard at every call site), and `authTypeFor`'s
   returned `AuthType` is what's actually passed through to
   `NeedsReauth` (e.g. `AuthStaticKeys` never triggers reauth, same as
   `TestWithReauthNotNeeded`). `go build`/`go vet`/`go test ./...`
   pass; no other file touched yet.

2. [x] `runAWSLoad[T any]` (new `internal/view/awsload.go`): the
   shared load/reauth/staleness-guard shape, per `plan.md`, built on
   top of `awsauth.Do` from task 1. New `awsload_test.go` testing it
   directly against `fakeViewHost`: empty-profile guard, loading
   placeholder shown synchronously, stale response discarded via
   `loadSeq`, reauth waiting/done messages shown in order. Not yet
   called from any view. `go build`/`go vet`/`go test ./...` pass.

3. [x] `SSMParamsView.load()` rewritten to call `runAWSLoad`, per
   `plan.md`'s example. `ShowReauthWaiting`/`ShowReauthDone` stay,
   unchanged in behavior. `ssmparams_test.go` needed no changes — all
   20 existing assertions pass unmodified.

   **Deviation from `plan.md` found during implementation**: the
   original `load()` only called `slog.Error` on the *fetch*-failure
   path, not the empty-profile guard. `runAWSLoad` uses one `showError`
   callback for both, so this view's `showError` argument is now a
   small closure (`func(err error) { slog.Error(...); pv.showError(err)
   }`) instead of `pv.showError` directly — meaning the empty-profile
   case now also logs (at ERROR level, same message). Not user-visible
   (nothing renders logs in the TUI), not asserted by any test, and
   arguably more consistent (every `showError` call now logs its
   cause). Judged not worth a `showError`/`onFetchError` parameter
   split in `runAWSLoad` just to preserve that one asymmetry — noting
   it here since it's a real, if minor, behavior change. The same
   pattern applies to the remaining 4 views (tasks 4-7).

   `go build`/`go vet`/`go test ./...` pass.

4. [x] `SecretsView.load()`: same rewrite, including the same
   logging-closure adjustment noted in task 3. `secrets_test.go` unchanged
   and passing. `go build`/`go vet`/`go test ./...` pass.

5. [x] `LogsView.load()`: same rewrite, including the same
   logging-closure adjustment. `logs_test.go` unchanged and
   passing. `go build`/`go vet`/`go test ./...` pass.

6. [x] `CodePipelineListView.load()`: same rewrite, including the same
   logging-closure adjustment.
   `codepipelinelist_test.go` unchanged and passing. `go build`/`go
   vet`/`go test ./...` pass.

7. [ ] `CodePipelineDetailView.load()`: same rewrite, with
   `loadingMsg` computed per-call
   (`fmt.Sprintf("Loading %s…", dv.pipelineName)`) instead of a shared
   constant, per `plan.md`. `codepipelinedetail_test.go` unchanged and
   passing. `go build`/`go vet`/`go test ./...` pass.

8. [ ] Merge-back: document `runAWSLoad`/`awsauth.Do` as a "Notable
   design decision worth preserving" in
   `spec/03-architecture-and-package-layout/spec.md` (same section
   that documents `ui.SetInputFieldText`) — what the shared shape is,
   which 5 views use it, and that `QueuesView` deliberately doesn't
   (different reauth-dispatch mechanism, see spec/03's existing
   `PipelineWatcher` paragraph and spec/07/spec/12 for why). Delete
   `spec-wip/cr-aws-view-load-dedup/`. Mark the PR ready for review.
