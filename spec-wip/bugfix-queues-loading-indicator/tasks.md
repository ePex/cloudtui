# Tasks

1. [x] **Implement the loading indicator and stale-response guard.** Added
   `loadSeq int` to `QueuesView`, `showStatus(msg string)` mirroring
   `showError`'s shape, rewrote `Load()` per `plan.md`. Added
   `fakeQueueBackend.listFn` (injectable override) and 4 unit tests:
   loading row appears immediately, success/error still repaint/show
   correctly from the loading state, and the key regression test —
   `TestQueuesViewLoadDiscardsStaleResponse` — proving a slower, earlier
   `Load()`'s result is discarded once a newer one has already rendered
   (deterministic via a `drawSignalingHost` test double, no actual data
   race). `go test -race ./internal/view/...` passes for all 4 new
   tests; the one `-race` failure in that package
   (`TestHandleFacetDiscoveryResultPreservesCurrentSelectionWhenValuesArrive`
   in `datadoglogs_test.go`) is the pre-existing, already-known failure
   from before this branch, confirmed unrelated.

2. [x] **Manual verification.** Drove the real TUI in tmux per the
   `verify-live` skill: created a disposable connection pointing at a
   blackhole IP (`192.0.2.1:9999`, guaranteed to hang rather than
   instantly refuse, unlike `localhost` with nothing listening) to get
   an observable window. Confirmed live: "Loading queues…" appears
   immediately on switching to it; switching to the real `default`
   connection while the blackhole request is still in flight shows
   `default`'s real queues immediately (its own "Loading queues…" first,
   then real data); after waiting out the full ~60s proxy timeout
   window, the table still showed `default`'s queues, unclobbered by the
   stale blackhole connection's eventual failure. Deleted the test
   connection and confirmed `~/.cloudtui/config.yaml` was restored
   byte-identical to before.

3. [ ] **Merge-back.** Update `spec/07-activemq-queue-list/spec.md`'s
   Behavior section (the "Reload"/"Auto-refresh on activate" bullets)
   to document the loading indicator and the stale-response guard as
   current, shipped behavior. Delete
   `spec-wip/bugfix-queues-loading-indicator/`.
