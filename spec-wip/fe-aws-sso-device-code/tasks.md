# Tasks

1. [x] **Core `awsauth` package.** Rewrite `Login` to stream via
   `io.Pipe` + `scanForDeviceCode`; add the `onCode` parameter to
   `WithReauth`. Update `TestLoginAWSCLINotOnPath`; add
   `TestScanForDeviceCodeParsesRealCLIOutputShape`,
   `TestScanForDeviceCodeNilCallbackSafe`,
   `TestScanForDeviceCodeNoMatchNeverCallsOnCode`,
   `TestLoginStreamsDeviceCodeBeforeCompleting` (skipped on Windows),
   `TestWithReauthPassesOnCodeThroughToLogin`; update the other
   `retry_test.go` stubs for the new `login` signature.
   `go build`/`go vet`/`go test ./internal/awsauth/...` clean (this task
   alone won't compile the rest of the module — the interface/call-site
   changes land in tasks 2–3). All 11 tests pass under `-race`,
   including `TestLoginStreamsDeviceCodeBeforeCompleting`'s real
   subprocess (fake `aws` script) end-to-end check.

2. [x] **Interface plumbing, secretbackend, and QueuesView.** Update
   `ui.Host`/`ui.ViewHost`'s `AWSSSOLogin`, `ui.ReauthStatusShower`'s
   `ShowReauthWaiting`, `app.go`/`viewhost.go`'s wiring (including the
   `NewSecretResolver` call site's new `onCode` closure),
   `secretbackend.SecretResolver`, and `QueuesView.ShowReauthWaiting`.
   Update `fakeViewHost.awsSSOLoginFn`/`AWSSSOLogin` in
   `testfake_test.go`, `secretbackend_test.go`'s constructor calls, and
   `app_test.go`'s `showReauthWaiting` calls; add
   `TestShowReauthWaitingIncludesDeviceCodeAndURLWhenProvided`.
   `go build`/`go vet`/`go test ./...` clean across the whole module at
   the end of this task (this is the point everything compiles together
   again). To get there, the five remaining `WithReauth` call sites
   (task 3's actual scope) needed a placeholder `nil` argument added for
   the new `onCode` parameter — Go requires the whole module to compile
   together, so this couldn't wait for task 3; each is marked with a
   `// TODO(fe-aws-sso-device-code task 3)` comment and gets its real
   implementation there. All tests pass; the one `-race` failure
   (`TestHandleFacetDiscoveryResultPreservesCurrentSelectionWhenValuesArrive`
   in `datadoglogs_test.go`) is the pre-existing, already-known one,
   confirmed unrelated.

3. [x] **The five remaining `WithReauth` call sites.** Updated
   `ssmparams.go`, `secrets.go`, `logs.go`, `codepipelinedetail.go`,
   `codepipelinelist.go` (local `reauthWaitingMsg` const + real `onCode`
   argument each, replacing task 2's placeholder `nil`) — already had
   `pipelinewatcher.go`'s permanent `onCode: nil` from task 2. Added one
   new test per updated view file
   (`TestXShowStatusRendersDeviceCodeMessage`) confirming `showStatus`
   renders the combined "waiting + code + URL" message correctly — same
   shape/spirit as each file's existing `TestXShowStatusRendersMessage`,
   which already established that `load()`'s own goroutine+
   `QueueUpdateDraw` plumbing isn't directly driven at this level (needs
   a running tview event loop); the wiring itself (does `onCode` get
   called and does it build the right string) is covered by
   `internal/app`'s `TestShowReauthWaitingIncludesDeviceCodeAndURLWhenProvided`
   and `internal/awsauth`'s `TestWithReauthPassesOnCodeThroughToLogin`.
   `go build`/`go vet`/`go test ./...` clean; all 5 new tests pass.

4. [ ] **Merge-back.** Update `spec/14-aws-profiles/spec.md` (the
   canonical spec for this shared mechanism — confirmed via grep that
   the individual SSM/Secrets/Logs/CodePipeline specs don't duplicate
   it): the "Automatic SSO re-authentication" bullet and `Login`'s
   signature in Implementation notes. Also touch up
   `spec/12-named-connections/spec.md` and
   `spec/07-activemq-queue-list/spec.md`'s existing SSO-reauth
   descriptions to mention the code/URL. Delete
   `spec-wip/fe-aws-sso-device-code/`.
