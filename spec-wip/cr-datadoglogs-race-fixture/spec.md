# CR: fix leaked-goroutine races in datadoglogs_test.go

Date: 2026-09-04

## Purpose

The 2026-09-04 architectural review flagged (`BACKLOG.md`, now
removed as this work starts) that `go test -race
./internal/view/...` reports real data races in `datadoglogs_test.go`,
hypothesized at the time to be caused by `fakeViewHost.QueueUpdateDraw`
running its callback inline instead of serializing onto one goroutine
the way real tview's `Application.QueueUpdateDraw` does.

A deeper investigation for this CR confirms the mechanism precisely,
and it's narrower than that framing suggested — not a systemic gap in
the shared fake affecting the whole test suite, but a **classic
leaked-goroutine bug in exactly 2 tests**:

- `TestRebuildFilterOptionsSelectingAnOptionRefocusesTable`
  (`datadoglogs_test.go:138`) and
  `TestHandleFacetDiscoveryResultPreservesCurrentSelectionWhenValuesArrive`
  (`datadoglogs_test.go:595`) both call `dv.serviceFilterDD.
  SetCurrentOption(1)` on a dropdown already wired (via
  `rebuildFilterOptions`/`refreshFilterDropdowns`) with the *real*
  `onSelect` callback — the one that calls `dv.search()`.
- tview's `DropDown.SetCurrentOption` invokes that callback
  synchronously, so `search()` runs *while `SetCurrentOption`'s own
  call is still on the stack* — it spawns a goroutine (the same
  `go func() { ...; host.QueueUpdateDraw(...) }()` shape every
  `search()`/`discoverFacetValuesFor()` call uses) and returns
  immediately.
- Neither test waits for that goroutine. It keeps running after the
  test function returns — because `fakeViewHost.QueueUpdateDraw` runs
  its callback inline (on whatever goroutine calls it, immediately),
  that leaked goroutine very quickly calls back into
  `handleSearchResult` → `rebuildFilterOptions` →
  `refreshFilterDropdowns` → `applyFilterOptions` →
  `tview.DropDown.SetOptions()`, mutating the same dropdown object
  the test (and tview's own `SetCurrentOption` internals) already
  touched — with no synchronization connecting the two, which is
  exactly what `-race` is built to catch, regardless of which test
  happens to be executing in the same process when the detector
  reports it (confirmed live: the two tests above are the actual
  culprits, but *other*, uninvolved tests running immediately
  afterward in the same process were the ones `-race` reported as
  failing in different runs — a leaked goroutine from an already-passed
  test, not a bug in the one shown failing).
- In real tview this exact interleaving is structurally impossible:
  `QueueUpdateDraw` always defers the callback to a later point on the
  single event-loop goroutine, which can never overlap with
  `SetCurrentOption`'s own still-executing call on that same goroutine.
  So this is confirmed test-only, not a production bug — consistent
  with the review's original suspicion, just now root-caused to 2
  specific tests rather than the fake's general behavior.
- The third test that looked related on the surface,
  `TestApplyFilterOptionsDoesNotFireCallbackDuringReconciliation`,
  turns out to be a false lead: it already passes its own local
  closure to `applyFilterOptions` directly (bypassing
  `refreshFilterDropdowns`'s real, `search()`-calling callback
  entirely) — its own doc comment already explains why, deliberately.
  It's simply the test that happened to be running when a *leaked*
  goroutine from the *previous* test got caught by the race detector
  in some runs.

## Scope

- Fix the 2 actual leak points: give both tests a `drawSignalingHost`
  (already used package-wide for exactly this "don't let an async
  goroutine outlive the test" purpose, e.g. `queues_test.go`,
  `pipelinewatcher_test.go`) instead of a plain `fakeViewHost`, and
  wait on `<-host.drawn` right after the `SetCurrentOption` call that
  triggers `search()`, so the goroutine's `QueueUpdateDraw` call is
  guaranteed to have completed before the test function returns.
- New `newTestDatadogLogsViewWithDrawSignal` helper in
  `datadoglogs_test.go`, mirroring the same-named-pattern helpers
  already in `ssmparams_test.go`/`secrets_test.go`/etc.
- Verify with repeated `go test -race ./internal/view/...` runs
  (a single clean run isn't proof for a timing-dependent race — see
  `plan.md`'s testing section for how many/how this is checked).

## Out of scope

- **No change to `fakeViewHost.QueueUpdateDraw`'s inline-execution
  behavior.** The investigation found this specific race doesn't
  require changing it — the fix is at the 2 leaking call sites, not
  the shared fake. Changing the fake's core semantics (e.g. making it
  genuinely deferred/serialized) would be a much larger, riskier
  change touching timing assumptions across dozens of other tests in
  the package for no benefit this specific bug needs.
- **No production code changes** — confirmed this cannot happen in
  the real app; `datadoglogs.go` itself is untouched.
- No change to `TestApplyFilterOptionsDoesNotFireCallbackDuringReconciliation`
  or any other test — they were never the actual problem.

## Data & config

No new files. Touches `tui/internal/view/datadoglogs_test.go` only.
