# CR: deterministic testing for CodePipeline's poll loop

Date: 2026-09-04

## Purpose

`PipelineWatcher.pollPipeline` (`internal/view/pipelinewatcher.go`)
drives the CodePipeline background watch (spec/20) off a raw
`time.NewTicker(pipelinePollInterval)` (20s, fixed). Every poll's
*handling* (`handlePipelinePoll`) is well covered by direct unit tests
(`TestHandlePipelinePollFirstPollEstablishesSilentBaseline`,
`...NotifiesOnChangedStage`, `...ErrorStopsWatchAndNotifiesOnce`,
`...StopsWatchWhenFinished`, `...RendersOpenDetailView`), but the poll
*loop itself* — the part that actually waits on the ticker and decides
when to call `handlePipelinePoll` vs. stop — has no test coverage.
`TestStartStopWatchingPipeline`'s own comment says as much: it "only
asserts on the synchronous map/channel state `StartWatchingPipeline`
itself mutates before the goroutine's first tick could ever fire,"
since the first real tick is a full `pipelinePollInterval` away and
nothing waits around for it.

This was flagged in a 2026-09-04 architectural review (see
`BACKLOG.md`'s "From the 2026-09-04 architectural review" section),
which found the independent `cloudtui-go` reimplementation solves the
identical problem in its equivalent poll loop
(`codepipelinewatch.go`) by abstracting the ticker behind a small
2-method interface (`C() <-chan time.Time; Stop()`), letting a test
drive ticks deterministically via a fake channel instead of waiting on
real wall-clock time or only checking pre-first-tick state.

## Scope

- A small interface (exact name/shape decided in `plan.md`) wrapping
  `*time.Ticker`'s `C`/`Stop` surface, with a real implementation
  backed by `time.NewTicker` and a fake/test implementation whose
  channel a test can send on directly.
- `pollPipeline` constructs its ticker through an injectable
  constructor (a func field on `PipelineWatcher`, following this
  codebase's existing "injectable func field, real impl wired in
  `New()`, fakeable in tests" convention — e.g. how CodePipeline's
  desktop-notification call is already injectable per spec/20) so
  tests can substitute the fake.
- A new test exercising the actual loop: start a watch, send a
  synthetic tick, assert `handlePipelinePoll` fired (observable via
  the same fakes `TestHandlePipelinePoll*` already use — a fetch call
  landing, or the list/detail view repainting) — without waiting 20
  real seconds.
- `pipelinePollInterval` itself stays fixed at 20s (spec/20 says "fixed,
  not user-configurable") — this CR only makes the loop *around* it
  testable, not configurable.

## Out of scope

- No change to the 20s interval, the notification behavior, or any of
  `handlePipelinePoll`'s already-tested logic.
- No change to `QueuesView` or any other view — this is specific to
  `PipelineWatcher`, the only background-ticker poll loop in the
  codebase today.
- The other two items from the same review round
  (`ViewHost` interface segregation, `showError`/`showStatus`
  boilerplate dedup) are unrelated code paths, not addressed here —
  see `BACKLOG.md`.
- No user-visible behavior change.

## Data & config

No new files beyond the ticker interface/fake. Touches
`tui/internal/view/pipelinewatcher.go` + `pipelinewatcher_test.go`.
