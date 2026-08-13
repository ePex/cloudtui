# Bugfix 47 — mq-proxy delete/move unreliably consume 0 messages

Date: 2026-08-13

## What

`BrokerService.deleteMessages`/`moveMessages` (`mq-proxy`) use
`consumer.receiveNoWait()` in a loop to pull messages matching a JMS
selector off a freshly-created consumer. This is unreliable: the ActiveMQ
broker dispatches matching messages to a new consumer's prefetch buffer
*asynchronously*, and `receiveNoWait()` gives up immediately if that
dispatch hasn't landed yet — even against messages that have existed on
the queue for seconds and are confirmed present via `list-messages`.

## Why

Reported live (via `tui`, proxy backend):

- **Single/bulk delete fails with "not found"** even though the target
  message genuinely exists — `RemoveMessage`/`DeleteMessages` (the Go
  client) checks the deleted count and surfaces an error when it's 0.
- **Purge silently does nothing** — `PurgeQueue` hits the exact same
  `deleteMessages` code path (empty filter) but the Go client doesn't
  check the returned count, so it reports success while deleting 0
  messages.
- **Move appeared to work**, but `moveMessages` has the identical
  `receiveNoWait()` pattern — same underlying race, just not yet hit by
  timing luck in that testing.

**Confirmed the mechanism directly**: sending a message and immediately
trying to delete it by exact ID failed 5/5 times, including after
explicit 1–2 second delays and repeated retries — ruling out a simple
"just needs a moment" race and pointing at `receiveNoWait()`'s
known-unreliable-right-after-consumer-creation behavior compounding
across attempts (a message can be briefly dispatched to a consumer that
then closes without receiving it, triggering redelivery, which the next
attempt's fresh consumer races against again). Swapping
`consumer.receiveNoWait()` for `consumer.receive(2000)` in
`deleteMessages` (tested in isolation, not yet applied for real) made
both single-ID delete and empty-filter bulk delete reliably succeed
against the same previously-stuck messages.

## Scope

`mq-proxy` (Kotlin), `BrokerService.kt`:

1. `deleteMessages`: `consumer.receiveNoWait()` → `consumer.receive(timeout)`.
2. `moveMessages`: same change, same reasoning — not yet independently
   confirmed broken live, but it's the same code pattern with the same
   root cause, so leaving it as `receiveNoWait()` once the mechanism is
   understood isn't defensible.
3. A shared timeout constant (`receive(2000)` confirmed sufficient in
   testing; exact value is a `plan.md` decision).
4. Unit tests in `BrokerServiceTest.kt` updated/added to assert
   `receive(timeout)` is called, not `receiveNoWait()`.

No wire-contract change — this is a pure implementation fix inside
`BrokerService`, not a DTO/route/envelope change, so `openapi.yaml`
doesn't need regenerating.

## Out of scope

- **The added tail latency** on a delete/purge/move call whose filter
  matches nothing (or fewer than requested): the final loop iteration
  now waits up to the timeout before concluding "no more matches,"
  instead of returning instantly. Accepted trade-off for correctness —
  matches the existing `fetchStats` precedent (`consumer.receive(3_000)`)
  in this same file.
- **`PurgeQueue`'s Go client not checking the deleted count.** Once
  `deleteMessages` reliably deletes everything matching an empty filter,
  `PurgeQueue` reporting success is correct again — the root cause is
  server-side, not a missing client-side check. Worth a follow-up
  robustness pass later, not blocking this fix.
- **`browseMessages`/`sendMessage`** are unaffected — they don't use
  `receiveNoWait()`.
