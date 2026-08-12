# Tasks — FE 43

1. [x] Add dependencies: `github.com/aws/aws-sdk-go-v2/service/codepipeline`
   and `github.com/gen2brain/beeep` to `tui/go.mod`.
2. [x] `internal/awscodepipeline`: `Pipeline`/`StageStatus` types,
   `ListPipelines`/`GetPipelineState`, `buildStageStatuses` split out
   for testing. Unit tests (no real API calls).
3. [x] `internal/app`: `App.notify` func field + `desktopNotify` real
   implementation (`internal/app/notify.go`); `listPipelines`/
   `getPipelineState` func fields wired in `New()`.
4. [x] `internal/app/codepipelinewatch.go`: `watchedPipelines`/
   `lastPipelineStages` fields, `startWatchingPipeline`/
   `stopWatchingPipeline`/`pollPipeline`/`handlePipelinePoll`, and the
   pure helpers `stageTransitions`/`snapshotStages`/`pipelineFinished`.
   Unit tests per plan.md's Testing section.
5. [x] `internal/app/codepipelinelist.go`: list view (table, filter,
   watching indicator, `w` toggle, `Enter` opens detail), registered in
   Home's "Apps" section. Unit tests mirroring the other list views'
   conventions (Name/Title, header labels, filter, showError).
6. [x] `internal/app/codepipelinedetail.go`: detail view (stage table,
   `w` toggle, `r` refresh, `Esc` back). Unit tests mirroring
   `logsearch_test.go`'s conventions.
7. [ ] Manual verification (per `tui/CLAUDE.md`) — see plan.md's
   Testing section for the checklist. Record what was checked here once
   done.
   - 2026-08-12: watched a real pipeline mid-run; got a premature
     "Pipeline finished" notification while it was still actively
     running. Root cause + fix recorded in `plan.md` ("Bugfix" note under
     `pipelineFinished`). Re-verify against a real in-progress pipeline
     with the fix before checking this task done.
