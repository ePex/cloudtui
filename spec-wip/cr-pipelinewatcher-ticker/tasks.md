# Tasks

1. [x] `pollTicker`/`realPollTicker` + `PipelineWatcher.newTicker`
   field, defaulted in `NewPipelineWatcher`; `pollPipeline` switched to
   call `w.newTicker(pipelinePollInterval)` instead of
   `time.NewTicker` directly, per `plan.md`. No test changes yet —
   confirm the full existing `pipelinewatcher_test.go` suite still
   passes unmodified (this task is pure wiring, behavior-preserving by
   construction). `go build`/`go vet`/`go test ./...` pass.

2. [ ] New `fakeTicker` test double + the loop test(s) in
   `pipelinewatcher_test.go` described in `plan.md`: start a watch
   with `w.newTicker` overridden, send synthetic ticks, assert via
   `drawSignalingHost` that `pollPipeline`'s `QueueUpdateDraw`
   dispatch actually lands and a poll happened (twice, to prove the
   loop continues, not just fires once); assert `Stop()` is called on
   the fake after `StopWatchingPipeline`. This is the actual point of
   the CR — the gap `TestStartStopWatchingPipeline`'s own comment
   flags. `go build`/`go vet`/`go test ./...` pass, plus a `-race` run
   of this package specifically (concurrency-adjacent code).

3. [ ] Merge-back: document the `pollTicker` injection pattern as a
   "Notable design decision worth preserving" in
   `spec/03-architecture-and-package-layout/spec.md`, near or appended
   to the existing `PipelineWatcher` paragraph — what problem it
   solves (deterministic testing of a ticker-driven loop without real
   wall-clock waits) and where the pattern would apply again if
   another background poller is ever added. Delete
   `spec-wip/cr-pipelinewatcher-ticker/`. Mark the PR ready for
   review.
