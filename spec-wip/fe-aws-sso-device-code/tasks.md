# Tasks

1. [ ] **Core `awsauth` package.** Rewrite `Login` to stream via
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
   changes land in tasks 2–3).

2. [ ] **Interface plumbing, secretbackend, and QueuesView.** Update
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
   again).

3. [ ] **The five remaining `WithReauth` call sites.** Update
   `ssmparams.go`, `secrets.go`, `logs.go`, `codepipelinedetail.go`,
   `codepipelinelist.go` (local `reauthWaitingMsg` const + `onCode`
   argument each) and `pipelinewatcher.go` (`onCode: nil`). Add one new
   test per updated view file confirming its `onCode` callback updates
   the status message correctly. `go build`/`go vet`/`go test ./...`
   clean.

4. [ ] **Merge-back.** Update `spec/14-aws-profiles/spec.md` (the
   canonical spec for this shared mechanism — confirmed via grep that
   the individual SSM/Secrets/Logs/CodePipeline specs don't duplicate
   it): the "Automatic SSO re-authentication" bullet and `Login`'s
   signature in Implementation notes. Also touch up
   `spec/12-named-connections/spec.md` and
   `spec/07-activemq-queue-list/spec.md`'s existing SSO-reauth
   descriptions to mention the code/URL. Delete
   `spec-wip/fe-aws-sso-device-code/`.
