# Plan — Bugfix 47

## Approach

Minimal, localized change: swap the receive call in both loops, nothing
else about `deleteMessages`/`moveMessages` changes (selector building,
`maxCount` handling, transaction semantics for move all stay as-is).

1. Add a private constant `RECEIVE_TIMEOUT_MS = 2000L` near the top of
   `BrokerService` (`mq-proxy/src/main/kotlin/com/github/epex/mqproxy/service/BrokerService.kt`),
   next to the class declaration — shared by both loops so the timeout is
   defined once, not duplicated as a magic number.
2. `deleteMessages`: `consumer.receiveNoWait()` → `consumer.receive(RECEIVE_TIMEOUT_MS)`.
3. `moveMessages`: same substitution.
4. `BrokerServiceTest.kt`: update every `every { consumer.receiveNoWait() ... }`
   / `verify(exactly = ...) { consumer.receiveNoWait() }` for these two
   functions to `consumer.receive(RECEIVE_TIMEOUT_MS)` (test constant
   mirrors the production one, imported or duplicated as a literal —
   whichever keeps the test file's existing style, which already inlines
   `3_000` for the `fetchStats` tests rather than importing a constant).
5. No new tests needed beyond updating the mocks — the existing test
   names/assertions (`deleteMessages stops after maxCount matches`,
   `deleteMessages builds a selector combining...`, etc.) already cover
   the loop's behavior; they just need to mock the right method now.

## Files touched

- `mq-proxy/src/main/kotlin/com/github/epex/mqproxy/service/BrokerService.kt`
- `mq-proxy/src/test/kotlin/com/github/epex/mqproxy/service/BrokerServiceTest.kt`
- `spec/47-bugfix-mq-proxy-delete-move-receive-race/tasks.md` (next gate)

## Key decisions

- **2000ms**, not a shorter or longer value. Confirmed sufficient in live
  testing against a real broker; short enough that a genuinely-empty
  match (the common "nothing left to delete" exit case) doesn't stall
  callers for an unreasonable time, matching the same order of magnitude
  as the existing `fetchStats` precedent (3000ms) in this file.
- **One shared constant for both methods**, not two separate ones or an
  inline literal per call site — they have identical reasoning (same
  race, same fix), so a single named constant keeps that obvious and
  avoids the two timeouts silently drifting apart later.
- **No change to `moveMessages`'s transactional structure** (it stays
  `SESSION_TRANSACTED`, committing once at the end) — `receive(timeout)`
  drops in as a straight replacement for `receiveNoWait()` inside the
  existing loop without touching that.
- **No client-side change** (Go proxy client, `PurgeQueue`'s count
  check) — confirmed out of scope in `spec.md`; the root cause is fully
  server-side.

## Manual verification

Unit tests confirm the mock call shape but can't catch the actual
race (that's precisely what made this bug invisible to the existing
tests before). Before checking off the implementation task:

- Start a real `mq-proxy` instance (`task dev:proxy:start`) against the
  dev broker.
- Send a message to a disposable test queue, then immediately (no
  artificial delay) delete it by exact ID via `curl` against
  `delete-messages` — confirm it succeeds first try, repeated at least
  5 times against fresh messages (the original bug failed 5/5 under this
  exact test).
- Repeat with an empty-filter bulk delete (purge-equivalent) against a
  queue with several messages sent immediately beforehand — confirm all
  are deleted in one call.
- Sanity-check `moveMessages` the same way (send, immediately move) —
  not confirmed broken live, but exercised here since it shares the fix.
- Confirm via `tui` itself (proxy backend): single delete, bulk delete
  (mark + `d`), purge (`p`), and move (`m`) all work against freshly-sent
  messages without needing a manual delay/retry.
