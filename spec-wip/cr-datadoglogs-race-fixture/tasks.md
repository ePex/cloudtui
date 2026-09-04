# Tasks

1. [ ] Add `newTestDatadogLogsViewWithDrawSignal` to
   `datadoglogs_test.go`; rewrite
   `TestRebuildFilterOptionsSelectingAnOptionRefocusesTable` and
   `TestHandleFacetDiscoveryResultPreservesCurrentSelectionWhenValuesArrive`
   to use it and wait on `<-host.drawn` after their `SetCurrentOption`
   call, per `plan.md`. Assertions in both tests stay unchanged. `go
   build`/`go vet`/`go test ./...` pass; a single `go test -race
   ./internal/view/...` run is clean.

2. [ ] Verification: `go test -race ./internal/view/... -count=1` run
   at least 10 times in a loop, plus targeted repeated runs isolating
   the 2 fixed tests and the tests various runs previously "blamed"
   (`TestApplyFilterOptionsDoesNotFireCallbackDuringReconciliation`,
   `TestHandleFacetDiscoveryResultMergesNewValues`,
   `TestHandleFacetDiscoveryResultNoopOnError`,
   `TestHandleFacetDiscoveryResultSkipsEmptyValues`) — all clean, no
   `WARNING: DATA RACE`. Record the exact commands run and pass count
   in the commit/task note as evidence, since a timing-dependent race
   fix can't be proven by a single green run.

3. [ ] Merge-back: document the leaked-goroutine hazard and the
   `drawSignalingHost` fix as a "Notable design decision worth
   preserving" in `spec/03-architecture-and-package-layout/spec.md` —
   what the hazard is (`SetCurrentOption` synchronously invoking a
   callback that spawns an unawaited goroutine), and the general rule
   it implies for any future test that triggers `search()`-like async
   work through a widget callback rather than calling it directly.
   Delete `spec-wip/cr-datadoglogs-race-fixture/`. Mark the PR ready
   for review.
