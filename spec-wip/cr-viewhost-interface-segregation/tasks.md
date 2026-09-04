# Tasks

Each task is an independent, isolated diff — mirrors how the earlier
5-view loading-indicator CR was broken down. **Every task keeps the
build green**: the new interfaces are added *additively* in task 1
(the old `ViewHost` interface stays defined, still satisfied by
`*App`, until nothing references it), each cluster migrates on its own
task, and only the final cleanup task removes `ViewHost` once it's
provably unused. `go build ./...`/`go vet ./...`/`go test ./...` must
pass after every single task, no exceptions; `gofmt` before every
commit.

1. [x] Add `AWSAuthHost`, `SSMParamsHost`, `SecretsHost`,
   `CloudWatchLogsHost`, `DatadogLogsHost`, `CodePipelineHost`,
   `MessagesHost` to `internal/ui/viewhost.go` **alongside** the
   existing `ViewHost` (not touched yet). Narrow
   `internal/view/awsload.go`'s `runAWSLoad` `host` param to a new
   unexported `awsLoadHost` interface (`ui.Host`+`ui.AWSAuthHost`) —
   safe immediately, since every current caller passes a
   `ui.ViewHost`-typed value, which already structurally satisfies the
   narrower parameter type. `go build ./...`/`go vet ./...`/`go test
   ./...` pass, full suite, no exceptions — this task is purely
   additive.

2. [x] `ssmparams.go`, `paramdetail.go`: `host`/`a` type
   `ui.ViewHost` → `ui.SSMParamsHost`. `*App` already structurally
   satisfies it (no assertion needed yet — added in task 9), so
   `app.go`'s `NewSSMParamsView(a, ...)` call site needs no change.
   Full suite green.

3. [x] `secrets.go`, `secretdetail.go` → `ui.SecretsHost`. Full suite green.

4. [x] `logs.go`, `logsearch.go`, `logdetail.go` →
   `ui.CloudWatchLogsHost`. Full suite green.

5. [x] `datadoglogs.go`, `datadoglogdetail.go` → `ui.DatadogLogsHost`.
   Full suite green.

6. [x] `codepipelinelist.go`, `codepipelinedetail.go`,
   `pipelinewatcher.go` → `ui.CodePipelineHost`. Full suite green.

7. [x] `messages.go` → `ui.MessagesHost`. Full suite green.

8. [x] `queues.go`, `message_detail.go`, `settings.go` → plain
   `ui.Host` (`log.go` needed no change — confirmed it takes no host
   parameter at all). Full suite green.

9. [x] Cleanup: confirmed via grep that no file referenced
   `ui.ViewHost` any more (every view migrated in tasks 2-8) except
   its own definition/assertion, then deleted the `ViewHost` interface
   from `internal/ui/viewhost.go` (and its now-unused `queue` import).
   Replaced `internal/app/viewhost.go`'s single `var _ ui.ViewHost =
   (*App)(nil)` assertion with one per new interface
   (`SSMParamsHost`/`SecretsHost`/`CloudWatchLogsHost`/
   `DatadogLogsHost`/`CodePipelineHost`/`MessagesHost`), per `plan.md`
   step 4.

   **Extra cleanup beyond `plan.md`'s stated scope, found during
   implementation**: `testfake_test.go`'s `fakeViewHost` had its own
   `var _ ui.ViewHost = (*fakeViewHost)(nil)` assertion (missed by the
   grep in `spec.md`'s research pass, since it's a `_test.go` file) —
   replaced with one assertion per new interface, same as `*App`. Its
   `SwitchToPage`, `UpdateContextPanel`, and 8 `OpenX` stub methods
   were also removed — confirmed via grep they're never called by any
   test and are no longer required by any of the 7 new interfaces, so
   keeping them would be dead code (`CLAUDE.md`'s "no dead code" rule)
   rather than the "no test file needs changes" spec.md predicted for
   the *split-the-fake* question specifically — this is a smaller,
   different kind of test-file touch (trimming genuinely dead stubs,
   not restructuring the fake), consistent with that decision's intent.
   `SwitchTo`/`CopyToClipboard`/watch-toggle/data-fetcher methods all
   stay, still required by one or more of the new interfaces.

   Full suite green (`go build`/`go vet`/`go test ./...`) — this is
   the task that finally proves, via the compiler, that the old god
   interface is completely gone and every replacement is complete. A
   `-race` run surfaces the pre-existing, separately-tracked
   `datadoglogs_test.go` race (`BACKLOG.md`) — unrelated to this
   change, same root cause already documented, not introduced here.

10. [ ] Merge-back: update
    `spec/03-architecture-and-package-layout/spec.md`'s `ViewHost`
    section (currently documents the single wide interface) to
    describe the new 7-interface shape and why it replaced the god
    interface — reference the 2026-09-04 architectural review and the
    zero-usage findings for the 10 removed methods. Delete
    `spec-wip/cr-viewhost-interface-segregation/`. Mark the PR ready
    for review.
