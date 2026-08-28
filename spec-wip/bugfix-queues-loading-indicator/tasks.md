# Tasks

1. [ ] **Implement the loading indicator and stale-response guard.** Add
   `loadSeq int` to `QueuesView`, add `showStatus(msg string)` mirroring
   `showError`'s shape (accent-colored, `SetExpansion(5)`), rewrite
   `Load()` per `plan.md`. Add unit tests: loading row appears
   immediately (backend gated on a channel), a stale response from an
   earlier `Load()` is discarded once a newer one has already rendered,
   and the existing repaint/showError transitions still work correctly
   from the new loading state. Run `go test -race ./internal/view/...`
   specifically for the new tests, per `plan.md`'s note on this
   codebase's history with async-test gating bugs.

2. [ ] **Manual verification.** Per `plan.md` — drive the real TUI in
   tmux (or via the `verify-live` skill) against at least two
   connections, confirm: the loading row appears immediately on
   connection switch and on `r` refresh; switching connections rapidly
   (before the first fetch resolves) ends with the table showing the
   *last*-selected connection's queues, never a stale/superseded one;
   an unreachable/misconfigured connection eventually shows the existing
   error row (unchanged behavior) rather than hanging with a stale list.

3. [ ] **Merge-back.** Update `spec/07-activemq-queue-list/spec.md`'s
   Behavior section (the "Reload"/"Auto-refresh on activate" bullets)
   to document the loading indicator and the stale-response guard as
   current, shipped behavior. Delete
   `spec-wip/bugfix-queues-loading-indicator/`.
