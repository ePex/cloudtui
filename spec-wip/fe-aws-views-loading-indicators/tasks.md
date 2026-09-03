# Tasks

Each task applies the identical pattern from `plan.md` to one view —
independent of the others, so each can be reviewed/committed on its
own.

1. [x] `SSMParamsView`: `loadingParametersStatus` constant, `loadSeq`
   field, `load()` restructured to show the placeholder immediately
   and guard its `QueueUpdateDraw` callback, `ShowReauthWaiting`/
   `ShowReauthDone` implementing `ui.ReauthStatusShower`. Tests:
   `TestSSMParamsViewShowReauthWaitingThenDone`,
   `TestSSMParamsViewLoadShowsLoadingStatusImmediately`,
   `TestSSMParamsViewLoadDiscardsStaleResponse` (the latter via a new
   `newTestSSMParamsViewWithDrawSignal` helper reusing queues_test.go's
   `drawSignalingHost`, same package). `go build`/`go vet`/`go test
   ./...` all pass.

   **FYI, not fixed here (pre-existing, unrelated)**: `go test -race
   ./internal/view/...` reports data races in several unrelated tests
   (favorites sorting, Datadog logs, CodePipeline poll, etc.) —
   confirmed present on a clean `main` checkout too, before this
   change. Neither CI nor `Taskfile.yml` run tests with `-race`, so
   this doesn't block anything here; flagged for a separate
   `BACKLOG.md` item rather than fixed as a drive-by.

2. [x] `SecretsView`: same pattern (`loadingSecretsStatus`). Tests:
   `TestSecretsViewShowReauthWaitingThenDone`,
   `TestSecretsViewLoadShowsLoadingStatusImmediately`,
   `TestSecretsViewLoadDiscardsStaleResponse`. `go build`/`go vet`/
   `go test ./...` all pass.

3. [x] `LogsView`: same pattern (`loadingLogGroupsStatus`). Tests:
   `TestLogsViewShowReauthWaitingThenDone`,
   `TestLogsViewLoadShowsLoadingStatusImmediately`,
   `TestLogsViewLoadDiscardsStaleResponse`. `go build`/`go vet`/`go
   test ./...` all pass.

4. [x] `CodePipelineListView`: same pattern
   (`loadingPipelinesStatus`). Tests:
   `TestCodePipelineListViewShowReauthWaitingThenDone`,
   `TestCodePipelineListViewLoadShowsLoadingStatusImmediately`,
   `TestCodePipelineListViewLoadDiscardsStaleResponse`. This view has
   no favorite/star column, so its status/error/loading cell is column
   0, not column 1 like the other three views. `go build`/`go vet`/`go
   test ./...` all pass, including `-race` for this view's package.

5. [x] `CodePipelineDetailView`: same pattern, but the loading message
   is per-pipeline (`fmt.Sprintf("Loading %s…", dv.pipelineName)`, no
   constant) and `load()` is called from `Open()`. Tests:
   `TestCodePipelineDetailViewShowReauthWaitingThenDone`,
   `TestCodePipelineDetailViewLoadShowsLoadingStatusImmediately`,
   `TestCodePipelineDetailViewLoadDiscardsStaleResponse`. Same
   column-0 cell layout as `CodePipelineListView`. `go build`/`go
   vet`/`go test ./...` all pass, including `-race` for this view's
   package.

6. [ ] Merge-back: document the immediate loading placeholder +
   `ui.ReauthStatusShower` support in `spec/15-aws-parameter-store`
   (`SSMParamsView`), `spec/16-aws-secrets-manager` (`SecretsView`),
   `spec/17-aws-cloudwatch-logs` (`LogsView`), and
   `spec/20-aws-codepipeline-monitor` (`CodePipelineListView` +
   `CodePipelineDetailView`) — 4 spec areas, one view (or two, for
   CodePipeline) each. Remove the now-done "Loading indicators + SSO
   re-auth status for the remaining AWS views" item from
   `BACKLOG.md`. Delete `spec-wip/fe-aws-views-loading-indicators/`.
   Mark the PR ready for review.
