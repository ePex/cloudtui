# Tasks — Bugfix 47

1. [x] Add `RECEIVE_TIMEOUT_MS` constant and swap `receiveNoWait()` for
   `receive(RECEIVE_TIMEOUT_MS)` in `deleteMessages` and `moveMessages`
   (`BrokerService.kt`). Update the corresponding mocks/verifications in
   `BrokerServiceTest.kt`.

## Manual verification

- Start `mq-proxy` (`task dev:proxy:start`), send-then-immediately-delete
  a message by exact ID via `curl`, repeated 5x against fresh messages —
  confirm all succeed first try (previously failed 5/5).
- Empty-filter bulk delete (purge-equivalent) against several
  freshly-sent messages — confirm all deleted in one call.
- Send-then-immediately-move a message — confirm it succeeds.
- Drive the real `tui` against the proxy backend: single delete, bulk
  delete, purge, and move all work on freshly-sent messages with no
  manual delay/retry needed.

**Verified 2026-08-13.** Ran a second, isolated `mq-proxy` instance
(`SERVER_PORT=8090`, built with the fix) alongside the user's own
running instance on 8080, so their live session wasn't disturbed.

- Send-then-immediately-delete-by-ID via curl, 5/5 succeeded first try
  (previously 0/5 on the unpatched build, same exact test).
- Empty-filter bulk delete against 3 freshly-sent messages: all 3
  deleted in one call.
- Send-then-immediately-move by ID: succeeded, message landed on the
  target queue.
- Drove the real `tui` (temporary connection pointed at :8090, removed
  afterward): single delete, purge (3 fresh messages), all worked
  correctly. Confirmed the accepted latency trade-off is real but minor
  — purge took ~2-3s (the final `receive(2000)` call waiting out the
  timeout to confirm no more matches), not instant as before, but it now
  actually deletes everything instead of silently doing nothing.
- Cleanup: test queues removed (via JMX, since delete now also works
  through mq-proxy but queues themselves still needed jolokia removal),
  second instance killed, `config.yaml` restored, tmux session killed,
  scratch binary/logs removed.
