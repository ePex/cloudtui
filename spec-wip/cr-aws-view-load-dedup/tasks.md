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

3. [ ] `SSMParamsView.load()` rewritten to call `runAWSLoad`, per
   `plan.md`'s example. `ShowReauthWaiting`/`ShowReauthDone` stay,
   unchanged in behavior. `ssmparams_test.go` expected to need no
   changes — confirm all its existing assertions still pass
   unmodified; if any turn out to have been coupled to the old
   internal shape, fix that coupling as part of this task. `go
   build`/`go vet`/`go test ./...` pass.

4. [ ] `SecretsView.load()`: same rewrite. `secrets_test.go` unchanged
   and passing. `go build`/`go vet`/`go test ./...` pass.

5. [ ] `LogsView.load()`: same rewrite. `logs_test.go` unchanged and
   passing. `go build`/`go vet`/`go test ./...` pass.

6. [ ] `CodePipelineListView.load()`: same rewrite.
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
