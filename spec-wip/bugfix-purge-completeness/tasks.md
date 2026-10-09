# Tasks: purge and browse don't reach every message on the proxy backend

See `spec.md` and `plan.md`. Each task needs its own approval before it's
implemented, and gets checked off only once implemented, tested and
pushed.

Live checks use the `verify-live` skill: this repo's `mq-proxy` against
the local broker, a disposable queue (here `cloudtui.test.purge`), seeded
with **800 × TypeA followed by 800 × TypeB** unless a task says
otherwise.

1. [x] **mq-proxy: honest `hasMore` on `list-messages`** (plan §1).
   `doBrowse` sets `hasMore` when the enumeration ended early but the
   queue depth (statistics plugin) is larger than what the browser saw;
   filtered browses use a count-only unfiltered pass for that. No stats →
   unchanged behavior plus a warning log. Unit tests in
   `BrokerServiceTest.kt`.
   *Live check:* `list-messages` with `filter.maxCount=5000` returns 400
   messages and `hasMore: true`; the same with `filter.jmsType=TypeB`
   returns 0 messages and `hasMore: true`; a queue with 10 messages
   returns 10 and `hasMore: false`.
   *Done 2026-10-10.* Unit tests mutation-checked (disabling the depth
   check fails both `hasMore: true` tests). Live, 800 × TypeA then
   800 × TypeB: unfiltered → 400, `hasMore: true`; `TypeA` → 400,
   `hasMore: true`; `TypeB` → **48** (not 0: with a selector the broker
   scanned slightly past 400 this time; stable across repeats),
   `hasMore: true`; `maxCount=100` → 100, `hasMore: true`. 10-message
   queue: unfiltered and `TypeA` → 10, `hasMore: false`; `TypeB` → 0,
   `hasMore: false`; `maxCount=5` → 5, `hasMore: true`.

2. [x] **TUI: `BrowseMessages` returns `hasMore`** (plan §2, §3 first
   bullet, §4 last bullet). Signature becomes `([]Message, bool, error)`
   on `queue.Backend`, the proxy client decodes `hasMore` from the
   envelope, Jolokia returns `false`, and `secretbackend` passes it
   through. Callers and test fakes are updated without any visible
   behavior change yet (the views ignore the flag until tasks 6–7).
   Tests: proxy client passes `hasMore` through; `secretbackend`
   forwards it.

3. [x] **TUI: `PurgeQueue` returns the deleted count** (plan §2, §4).
   Signature becomes `(int, error)`. Proxy: the count from the response.
   Jolokia: `QueueSize` read before tier 1, the returned count for tier
   2, counted removals for tier 3. `secretbackend` passes it through.
   Views ignore the count until task 5. Tests per Jolokia tier, proxy
   and `secretbackend`.
   *Done 2026-10-10.* Tier 1 is skipped when `QueueSize` can't be read,
   so the count is never made up; on error, the count deleted so far is
   returned alongside it. Tests mutation-checked. Live: the Jolokia
   `QueueSize` read returns `{"status":200,"value":10}` on a 10-message
   queue, the shape the code decodes.

4. [ ] **TUI: proxy client repeats bulk delete/move until zero** (plan
   §3). Private round helper used by `PurgeQueue`, `MoveAllMessages` and
   `DeleteMessages`/`MoveMessages` when `MaxCount == 0`; sums the
   counts; stops on a 0 round; `maxBulkRounds` (1,000) guard with an
   "incomplete" error; context checked between rounds; partial count in
   any error. `MaxCount > 0` (incl. single-message calls) stays one
   request. Tests as listed in plan §3.
   *Live check:* none needed against `mq-proxy` (it already empties in
   one round, so this is one request plus one returning 0, visible in
   the `mq-proxy` log). *Manual check by the user*, if a server that
   caps deletes per call is available: purge a 1,000+ message queue
   from the TUI and confirm it ends empty.

5. [ ] **TUI: one purge flow for both entry points, with a result
   line** (plan §5 "Shared purge flow" and "Result line" first
   paragraph). New `internal/view/purge.go` holds the JMS Type prompt →
   confirm → backend call → status flow, used by `QueuesView` and
   `MessagesView`; `MessagesView` gets a `JMSTypePrompt`. Success shows
   `Purged <n> message(s) from "<queue>"` (or `Purged <n> <type>
   message(s) …`). Tests: the helper (blank vs typed, count in status,
   error path); `p` in `MessagesView` opens the JMS Type prompt
   (regression test for defect 3).
   *Live check:* `p` in the queue list and `p` inside the queue both
   show the JMS Type prompt and the same confirm wording; each purges
   1,600 → 0 and shows `Purged 1600 message(s) from
   "cloudtui.test.purge"`.

6. [ ] **TUI: warning after filtered purge/move-all** (plan §5 "Result
   line" second paragraph). After a filtered purge or move-all, read the
   pending count via `List`; above 200 remaining, append `— <m>
   message(s) remain; matching messages further back may not have been
   reached`. Tests: above/at/below the threshold, `List` failure (no
   suffix, no error), unfiltered operations never warn.
   *Live check:* purge `TypeB` → `Purged 0 TypeB message(s) … — 1600
   message(s) remain; …`; purge `TypeA` → `Purged 800 TypeA message(s)
   …` with the warning (800 remain); then purge `TypeB` → `Purged 800
   …` and no warning.

7. [ ] **TUI: message browser says when it's incomplete** (plan §5
   "Message browser title"). Unfiltered loads fetch the pending count
   concurrently and show `showing <n> of <pending>` when `n < pending`;
   filtered loads with `hasMore` show `first <n> browsed, more on
   queue`; fallbacks when `List` fails. Tests for each wording and the
   fallbacks, rendered via `tcell.NewSimulationScreen`.
   *Live check:* opening the queue shows `showing 400 of 1600`; filtering
   by `TypeB` shows an empty list titled `first 0 browsed, more on
   queue`; a 10-message queue shows no suffix.

8. [ ] **TUI: JMS Type scans report the limit** (plan §5 "JMS Type
   scans"). `ScanJMSTypes` returns `hasMore`; `JMSTypePrompt` and
   `MessageFilter` show `Scanned the first <n> messages — the queue
   holds more; types further back aren't listed` when it's true. Tests
   for both dialogs, with and without `hasMore`.
   *Live check:* `p` on the queue offers only `TypeA` and the status
   line shows the scan message; the same in the message filter's JMS
   Type field (`f` inside the queue).

9. [ ] **Regression run.** `task test` (all modules), `task
   smoke:test`, and a final pass of the live checks from tasks 5–8 in
   one session. Remove the scratch queue afterwards.
