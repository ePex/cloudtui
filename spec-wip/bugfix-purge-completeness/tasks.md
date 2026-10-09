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

4. [x] **TUI: proxy client repeats bulk delete/move until zero** (plan
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
   *Done 2026-10-10.* The context check is unit-tested on the round
   helper directly, since cancelling from an `httptest` handler races
   the client reading that response. Mutation-checked (single request
   again fails 7 tests). Live: purging a 10-message queue from the queue
   list made two `delete-messages` requests in the `mq-proxy` log and
   left it empty. Each takes ~2 s (`mq-proxy`'s receive timeout), so the
   confirming request adds ~2 s to a purge against `mq-proxy`. The
   manual check against a per-call-capped server is still open (user).

5. [x] **TUI: one purge flow for both entry points, with a result
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
   *Done 2026-10-10.* The goroutine's result handling is split into
   `purgeFlow.finish` so it's tested directly. The regression test
   fails against the old in-queue behavior. Live: both paths showed the
   JMS Type prompt, the same confirm wording and `Purged 1600
   message(s) from "cloudtui.test.purge"`, leaving the queue empty; a
   `TypeA` purge inside the queue showed `Purged 800 TypeA message(s)
   …` and left the 800 TypeB. Found while doing so, not part of this
   bugfix: text typed into the JMS Type prompt before its automatic
   scan finishes is wiped when the scan completes
   (`JMSTypePrompt.handleScanResult` clears the field).

6. [x] **TUI: warning after filtered purge/move-all** (plan §5 "Result
   line" second paragraph). After a filtered purge or move-all, read the
   pending count via `List`; above 200 remaining, append `— <m>
   message(s) remain; matching messages further back may not have been
   reached`. Tests: above/at/below the threshold, `List` failure (no
   suffix, no error), unfiltered operations never warn.
   *Live check:* purge `TypeB` → `Purged 0 TypeB message(s) … — 1600
   message(s) remain; …`; purge `TypeA` → `Purged 800 TypeA message(s)
   …` with the warning (800 remain); then purge `TypeB` → `Purged 800
   …` and no warning.
   *Done 2026-10-10.* Threshold boundary mutation-checked (`>=` fails
   the "at the window" case). Live, all three exactly as above, e.g.
   `Purged 0 TypeB message(s) from "cloudtui.test.purge" — 1600
   message(s) remain; matching messages further back may not have been
   reached`. Also live, since move-all's wiring has no unit test:
   move-all `TypeB` to another queue → `Moved 0 message(s) from … to … —
   1600 message(s) remain; …`.

7. [x] **TUI: message browser says when it's incomplete** (plan §5
   "Message browser title"). Unfiltered loads fetch the pending count
   concurrently and show `showing <n> of <pending>` when `n < pending`;
   filtered loads with `hasMore` show `first <n> browsed, more on
   queue`; fallbacks when `List` fails. Tests for each wording and the
   fallbacks, rendered via `tcell.NewSimulationScreen`.
   *Live check:* opening the queue shows `showing 400 of 1600`; filtering
   by `TypeB` shows an empty list titled `first 0 browsed, more on
   queue`; a 10-message queue shows no suffix.
   *Done 2026-10-10.* Load's result handling is split into `applyLoad`
   for tests; `go test -race` clean on the concurrent pending-count
   fetch. Pending-comparison mutation fails 5 tests. Live: the
   1,600-message queue showed `(filter: max=500) (showing 400 of
   1600)`; the 10-message queue no note. Filtered: checked with
   `TypeA` instead of `TypeB` → `(filter: type=TypeA max=500) (first
   400 browsed, more on queue)`. `TypeB` couldn't be applied from the
   keyboard: with it not among the suggestions, Enter in the filter's
   JMS Type field picks the drop-down's scan entry and clears the field
   (pre-existing; recorded with the follow-up bugfix from task 5).

8. [x] **TUI: JMS Type scans report the limit** (plan §5 "JMS Type
   scans"). `ScanJMSTypes` returns `hasMore`; `JMSTypePrompt` and
   `MessageFilter` show `Scanned the first <n> messages — the queue
   holds more; types further back aren't listed` when it's true. Tests
   for both dialogs, with and without `hasMore`.
   *Live check:* `p` on the queue offers only `TypeA` and the status
   line shows the scan message; the same in the message filter's JMS
   Type field (`f` inside the queue).
   *Done 2026-10-10.* Deviation from plan §5: `ScanJMSTypes` returns a
   `ui.JMSTypeScan{Types, Scanned, HasMore}` instead of `(types,
   hasMore, err)`, since the message needs the scanned count too.
   Mutation-checked (an always-empty status fails both new tests).
   Live: the purge prompt offered only `TypeA` with the status line
   `Scanned the first 400 messages — the queue holds more; types
   further back aren't listed`; the message filter's opt-in 5,000
   scan (`maxCount=5000` in the `mq-proxy` log) showed the same.
   Found while doing so, not part of this bugfix: `go test -race
   ./internal/dialog/` reports data races in a theming test that opens
   the JMS Type prompt and lets its automatic scan run; `main` has the
   same 10 races, so it predates this branch.

9. [x] **Regression run.** `task test` (all modules), `task
   smoke:test`, and a final pass of the live checks from tasks 5–8 in
   one session. Remove the scratch queue afterwards.
   *Done 2026-10-10.* `task test`: exit 0 (Go, mq-proxy, mq-proxy-web
   43/43). Smoke test: PASS, run with a scratch `HOME` holding only a
   local-broker connection, because the developer's real active
   connection pointed at a shared remote environment. Note for running
   it that way: tmux must be a separate server (`tmux -L <name>`) —
   an already-running tmux server starts the TUI with its own
   environment, so a `HOME` override doesn't reach it. A first attempt
   hit exactly that; it only listed queues on the real connection
   (read-only) before timing out. Final pass in one session on 800 ×
   TypeA then 800 × TypeB: purge prompt offered only `TypeA` with the
   scan note; purging `TypeB` from the queue list → `Purged 112 TypeB
   message(s) … — 1488 message(s) remain; …` (112, not 0 as in task 6:
   after the earlier browsing the broker had more of the queue paged
   in, which is why the warning says "may"); inside the queue the
   title read `(showing 400 of 1488)`, and `p` with a blank type →
   JMS Type prompt → `Purged 1488 message(s) …`, queue empty. Both
   scratch queues and their two `ActiveMQ.Statistics.Destination.*`
   request queues removed from the local broker.
