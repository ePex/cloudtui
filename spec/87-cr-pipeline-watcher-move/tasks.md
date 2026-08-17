# Tasks — CR 87: extract the CodePipeline background watcher out of `internal/app`

1. [x] Create `internal/view/pipelinewatcher.go` with `PipelineWatcher`
   (struct + `NewPipelineWatcher`), `IsWatchingPipeline`/
   `StartWatchingPipeline`/`StopWatchingPipeline`, `pollPipeline`,
   `handlePipelinePoll`, `stageTransitions`, `snapshotStages`,
   `pipelineFinished`, and the `pipelinePollInterval` const — all per
   plan.md, `a.X`/`w.host.X` calls updated, `w.detailV`/`w.listV`
   reach-ins with no nil guard. This is a new, self-contained file;
   `internal/app/codepipelinewatch.go` is untouched in this task, so
   both the old and new implementations exist side by side briefly.
   `gofmt -l`, `go build ./...`, `go vet ./...` clean.

2. [x] Create `internal/view/pipelinewatcher_test.go`: port
   `TestStageTransitionsNilPrevReturnsNoMessages`,
   `TestStageTransitionsReportsChangedStages`,
   `TestStageTransitionsNeverRunStageLabel`, `TestSnapshotStages`,
   `TestPipelineFinished` unchanged; add `fakeNotifier` (moved
   unchanged) and the `newTestPipelineWatcher(t)` helper per plan.md;
   rewrite `TestHandlePipelinePollFirstPollEstablishesSilentBaseline`,
   `TestHandlePipelinePollNotifiesOnChangedStage`,
   `TestHandlePipelinePollErrorStopsWatchAndNotifiesOnce`,
   `TestHandlePipelinePollStopsWatchWhenFinished`,
   `TestStartStopWatchingPipeline` against `PipelineWatcher` + the new
   helper (same assertions, new construction); add
   `TestHandlePipelinePollRendersOpenDetailView` (new coverage for the
   previously-unasserted detail-view-refresh path, per plan.md).
   `gofmt -l`, `go build ./...`, `go vet ./...`,
   `go test ./internal/view/...` clean.

3. [x] Reduce `internal/app/codepipelinewatch.go` to the 3 trampoline
   methods (`IsWatchingPipeline`/`StartWatchingPipeline`/
   `StopWatchingPipeline`, each forwarding to `a.pipelineWatcher`) per
   plan.md; delete everything else from the file (moved to
   `internal/view/pipelinewatcher.go` in task 1, not duplicated).
   Delete `internal/app/codepipelinewatch_test.go` (all 9 tests
   relocated/rewritten in task 2). `gofmt -l` clean; `go build ./...`
   will fail (`a.pipelineWatcher` doesn't exist on `*App` yet) — that's
   expected, fixed in task 4.

4. [x] `app.go`: remove the `watchedPipelines`/`lastPipelineStages`
   fields and their 2 init lines; add `pipelineWatcher
   *view.PipelineWatcher` field; construct it right after
   `codePipelineListV`/`codePipelineDetailV` per plan.md
   (`a.pipelineWatcher = view.NewPipelineWatcher(a, a.notify,
   a.codePipelineListV, a.codePipelineDetailV)`). `gofmt -l`,
   `go build ./...`, `go vet ./...`, `go test ./...` clean.

5. [x] `internal/ui/viewhost.go`: update the "CodePipeline background
   watcher (implemented by App's codepipelinewatch.go)" doc comment to
   reflect `view.PipelineWatcher` as the actual owner, per plan.md —
   no signature change. `gofmt -l`, `go build ./...`, `go vet ./...`
   clean.

6. [x] Final verification pass: grep confirms zero remaining
   `watchedPipelines`/`lastPipelineStages` references anywhere in
   `internal/app`, and that `codepipelinewatch.go` holds only the 3
   trampolines; `gofmt -l tui/` clean; `go vet ./...` clean;
   `go build ./...` and `go test ./...` pass repo-wide; confirm zero
   import cycle (`go list -deps ./internal/app/... ./internal/view/...`
   succeeds).

   All clean on the first pass — no stray references, no additional
   fixes needed this time (unlike CR 86, which turned up one stale doc
   comment). Also grepped every other `spec/*.md` referencing
   `codepipelinewatch.go` (13 hits, in `spec/43`, `spec/64`, `spec/79`,
   `spec/80`, `spec/81`, `spec/84`, `spec/85`, `spec/86`) — all
   historical documents describing state at time of writing, same
   precedent as CR 85/86; none updated.

7. [x] Live verification via `verify-live`: start a watch on a real
   pipeline from the list view (`w`) and confirm the WATCHING column
   updates immediately; open that pipeline's detail view and confirm
   its title also shows watching; stop the watch from the detail view
   (`w`) and confirm both the detail title and the list's WATCHING
   column clear; repeat starting from the detail view instead, stopping
   from the list. If no pipeline in the connected AWS account is
   actively running during verification, note explicitly which parts
   (stage-transition notification, auto-stop on finish) couldn't be
   observed live, matching CR 83/84's precedent for CodePipeline's
   empty-account limitation. Record what was checked and the outcome
   here once complete.

   Checked via tmux against the real AWS profile (`example-dev`). Navigated
   to `codepipeline` — the list loaded cleanly with 0 pipelines in this
   account (same empty-account situation CR 83/84 already hit), which
   confirms the real `ListPipelines` call succeeded through
   `ui.ViewHost` post-move without erroring. Pressed `w` on the empty
   list: no-op, no panic (`toggleWatchSelected`'s `idx < 0` guard).
   Pressed `r` to refresh: succeeded cleanly, still 0 pipelines,
   confirming another real AWS round-trip through the same wiring.
   Quit with `q`, killed the tmux session, removed the verify binary.

   Not observable live, same limitation as CR 83/84: starting/stopping
   a watch on a real pipeline, the WATCHING column/title updating,
   stage-transition notifications, and auto-stop on finish, since the
   account has no pipelines to watch. These paths are covered instead
   by this CR's unit tests
   (`TestStartStopWatchingPipeline`/`TestHandlePipelinePoll*`/
   `TestHandlePipelinePollRendersOpenDetailView`) plus the pre-existing
   `codepipelinelist_test.go`/`codepipelinedetail_test.go` coverage of
   `toggleWatchSelected`/`toggleWatch` calling through to
   `host.IsWatchingPipeline`/`StartWatchingPipeline`/
   `StopWatchingPipeline`, which are unchanged by this CR.
