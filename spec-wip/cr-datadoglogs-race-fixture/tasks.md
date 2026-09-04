# Tasks

1. [x] Add `newTestDatadogLogsViewWithDrawSignal` to
   `datadoglogs_test.go`; rewrite
   `TestRebuildFilterOptionsSelectingAnOptionRefocusesTable` and
   `TestHandleFacetDiscoveryResultPreservesCurrentSelectionWhenValuesArrive`.
   Assertions in both tests unchanged. `go build`/`go vet`/`go test
   ./...` pass.

   **Correction to `plan.md`'s design, found during implementation**:
   waiting on `<-host.drawn` *after* the `SetCurrentOption` call, as
   planned, was verified insufficient — a `-race` run still failed.
   Root cause: the race window is *inside* the `SetCurrentOption` call
   itself (tview's `SetCurrentOption` invokes the callback
   synchronously, and the spawned `search()` goroutine can reach
   `QueueUpdateDraw`/`SetOptions()` while `SetCurrentOption`'s own
   internal bookkeeping is still executing on the calling goroutine —
   confirmed via a second `-race` trace showing both the read and the
   goroutine-creation site at the exact same `SetCurrentOption` call
   line). A wait placed after the call can't close a window that's
   already inside it. Fixed instead by blocking
   `host.searchDatadogLogsFn` on an `unblock` channel *before* calling
   `SetCurrentOption`, so `search()`'s goroutine can't reach any
   widget-touching code at all until explicitly released well after
   `SetCurrentOption` (and, for the second test, the direct
   `handleFacetDiscoveryResult` call) have fully returned — then
   `close(unblock)` + `<-host.drawn` at the end of each test, so the
   goroutine still can't leak past it.

2. [x] Verification: `go test -race ./internal/view/... -count=1` run
   15 times in a loop — all 15 clean, no `WARNING: DATA RACE`, no
   `FAIL`. Targeted repeated runs (15×) isolating just the 2 fixed
   tests and the tests various runs previously "blamed"
   (`TestApplyFilterOptionsDoesNotFireCallbackDuringReconciliation`,
   `TestHandleFacetDiscoveryResultMergesNewValues`,
   `TestHandleFacetDiscoveryResultNoopOnError`,
   `TestHandleFacetDiscoveryResultSkipsEmptyValues`) — also 15/15
   clean. Full non-race `go test ./...` green.

3. [ ] Merge-back: document the leaked-goroutine hazard and the
   `drawSignalingHost` fix as a "Notable design decision worth
   preserving" in `spec/03-architecture-and-package-layout/spec.md` —
   what the hazard is (`SetCurrentOption` synchronously invoking a
   callback that spawns an unawaited goroutine), and the general rule
   it implies for any future test that triggers `search()`-like async
   work through a widget callback rather than calling it directly.
   Delete `spec-wip/cr-datadoglogs-race-fixture/`. Mark the PR ready
   for review.
