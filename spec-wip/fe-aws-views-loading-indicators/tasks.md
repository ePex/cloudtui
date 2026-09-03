# Tasks

Each task applies the identical pattern from `plan.md` to one view —
independent of the others, so each can be reviewed/committed on its
own.

1. [ ] `SSMParamsView`: `loadingParametersStatus` constant, `loadSeq`
   field, `load()` restructured to show the placeholder immediately
   and guard its `QueueUpdateDraw` callback, `ShowReauthWaiting`/
   `ShowReauthDone` implementing `ui.ReauthStatusShower`. Tests:
   `TestSSMParamsViewShowReauthWaitingThenDone`,
   `TestSSMParamsViewLoadShowsLoadingStatusImmediately`,
   `TestSSMParamsViewLoadDiscardsStaleResponse`.

2. [ ] `SecretsView`: same pattern (`loadingSecretsStatus`). Tests:
   `TestSecretsViewShowReauthWaitingThenDone`,
   `TestSecretsViewLoadShowsLoadingStatusImmediately`,
   `TestSecretsViewLoadDiscardsStaleResponse`.

3. [ ] `LogsView`: same pattern (`loadingLogGroupsStatus`). Tests:
   `TestLogsViewShowReauthWaitingThenDone`,
   `TestLogsViewLoadShowsLoadingStatusImmediately`,
   `TestLogsViewLoadDiscardsStaleResponse`.

4. [ ] `CodePipelineListView`: same pattern
   (`loadingPipelinesStatus`). Tests:
   `TestCodePipelineListViewShowReauthWaitingThenDone`,
   `TestCodePipelineListViewLoadShowsLoadingStatusImmediately`,
   `TestCodePipelineListViewLoadDiscardsStaleResponse`.

5. [ ] `CodePipelineDetailView`: same pattern, but the loading message
   is per-pipeline (`fmt.Sprintf("Loading %s…", dv.pipelineName)`, no
   constant) and `load()` is called from `Open()`. Tests:
   `TestCodePipelineDetailViewShowReauthWaitingThenDone`,
   `TestCodePipelineDetailViewLoadShowsLoadingStatusImmediately`,
   `TestCodePipelineDetailViewLoadDiscardsStaleResponse`.

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
