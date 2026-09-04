# Plan

## Approach

1. **A `pollTicker` interface** (`internal/view/pipelinewatcher.go`)
   wrapping `*time.Ticker`'s `C`/`Stop` surface:

   ```go
   // pollTicker abstracts *time.Ticker's C/Stop surface so
   // pollPipeline's loop can be driven deterministically in tests
   // instead of waiting on real wall-clock time.
   type pollTicker interface {
       C() <-chan time.Time
       Stop()
   }

   // realPollTicker adapts *time.Ticker to pollTicker — time.Ticker's
   // C is a field, not a method, so it can't satisfy the interface
   // directly.
   type realPollTicker struct {
       *time.Ticker
   }

   func (t *realPollTicker) C() <-chan time.Time { return t.Ticker.C }
   ```

2. **`PipelineWatcher` gains a `newTicker func(time.Duration)
   pollTicker` field**, defaulted in `NewPipelineWatcher` (not taken as
   a constructor parameter — `watched`/`lastStages` are already
   defaulted the same way inside the constructor rather than passed
   in, and there's no meaningful alternative "real" implementation the
   way `notify` has one, so a constructor param would just be
   boilerplate every non-test caller repeats identically):

   ```go
   func NewPipelineWatcher(...) *PipelineWatcher {
       return &PipelineWatcher{
           ...
           newTicker: func(d time.Duration) pollTicker {
               return &realPollTicker{time.NewTicker(d)}
           },
       }
   }
   ```

   A test overrides `w.newTicker` directly on the constructed value —
   same shape as overriding a `fakeViewHost` fetch func field.

3. **`pollPipeline` calls `w.newTicker(pipelinePollInterval)`** instead
   of `time.NewTicker(pipelinePollInterval)` directly; everything else
   in the loop (the `select` on `stop`/`ticker.C()`, the fetch, the
   `QueueUpdateDraw` dispatch to `handlePipelinePoll`) is unchanged.

4. **New test** exercising the real loop, added to
   `pipelinewatcher_test.go`:
   - Build a `fakeTicker` (a buffered `chan time.Time` behind `C()`,
     plus a `stopped bool` `Stop()` sets) local to the test file,
     matching this package's existing small-test-double style (e.g.
     `fakeNotifier`).
   - Use `drawSignalingHost` (already shared package-wide, from
     `queues_test.go`) so the test can block until `pollPipeline`'s
     `QueueUpdateDraw` call has actually landed, instead of a sleep.
   - Set `w.newTicker = func(time.Duration) pollTicker { return
     fakeTicker }` *before* calling `w.StartWatchingPipeline(name)` —
     safe without a mutex because everything written before a `go`
     statement happens-before the spawned goroutine per the Go memory
     model, same reasoning already documented on `StartWatchingPipeline`
     for `profile`'s capture-by-value.
   - Send one value on `fakeTicker`'s channel, `<-host.drawn`, then
     assert a poll actually happened (e.g. `host.getPipelineStateFn`
     called once, or `listV`'s repainted state) — proving the loop
     itself ticks and dispatches, not just `handlePipelinePoll` in
     isolation.
   - A second send + a second `<-host.drawn` to confirm the loop
     continues (doesn't fire once and stop).
   - `StopWatchingPipeline` then a send on the *closed* `stop` channel
     path already covered structurally by `select`'s existing
     `case <-stop: return` — confirm `fakeTicker.stopped` is true
     after stopping, proving `defer ticker.Stop()` actually runs
     through the interface.

## Files touched

- `tui/internal/view/pipelinewatcher.go` — add `pollTicker`,
  `realPollTicker`, the `newTicker` field, and switch `pollPipeline` to
  use it.
- `tui/internal/view/pipelinewatcher_test.go` — add `fakeTicker` and
  the new loop test(s) above. Existing tests
  (`TestHandlePipelinePoll*`, `TestStartStopWatchingPipeline`) are
  expected to need no changes — `TestStartStopWatchingPipeline`'s
  existing comment about only checking pre-first-tick state stays
  accurate (it's testing `Start`/`Stop`'s synchronous bookkeeping, not
  the loop) and can stay as-is; the new test is additive coverage of
  the loop it explicitly says it doesn't cover.

## Testing

- The new loop test above is the actual point of this CR — it's the
  concrete gap being closed, not incidental coverage.
- Full existing suite (`TestHandlePipelinePoll*`, transition/finished
  helpers, `TestStartStopWatchingPipeline`) must keep passing
  unchanged.
- `go build`/`go vet`/`go test ./...` (and a `-race` run specifically
  for this package, given the subject matter) after the task, `gofmt`
  before commit.

## Key decisions / trade-offs

- **Interface wraps `C()`/`Stop()` as methods, not a struct mirroring
  `time.Ticker`'s public field.** `time.Ticker.C` is a field
  (`<-chan Time`), which can't be part of a Go interface directly —
  wrapping it in a `C()` method on the adapter is the standard idiom
  for making a stdlib type with a field-shaped API satisfy an
  interface (same shape as e.g. wrapping `net.Conn` reads/writes when
  only part of the surface is needed).
- **Default supplied internally in the constructor, override by direct
  field assignment in tests** — see point 2 above; keeps
  `NewPipelineWatcher`'s signature unchanged for its one real caller
  (`app.go`), and matches how `watched`/`lastStages` are already
  defaulted rather than passed in.
- **Scope stays limited to `PipelineWatcher`** — it's the only
  background-ticker poll loop in the codebase today (confirmed via
  `grep -rln Ticker internal/`); no other view has an equivalent gap
  to fix in the same pass.
