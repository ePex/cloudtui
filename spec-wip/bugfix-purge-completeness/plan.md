# Plan: purge and browse don't reach every message on the proxy backend

See `spec.md` for the defects and the expected behavior. This plan covers
both modules: `mq-proxy/` (Kotlin) and `tui/` (Go).

## 1. mq-proxy: honest `hasMore` on `list-messages`

`BrokerService.doBrowse` (`mq-proxy/.../service/BrokerService.kt`) sets
`hasMore` only when it stops early because `maxCount` was reached. When the
browser enumeration ends by itself, it can't tell "queue exhausted" from
"broker stopped at `maxBrowsePageSize`".

Change: when the enumeration ends before `maxCount`, compare how many
messages the broker let the browser see with the queue depth:

- **Unfiltered browse:** the number of messages enumerated *is* the browse
  window. If the queue depth is larger, set `hasMore = true`.
- **Filtered browse (selector):** the broker applies the selector itself,
  so the enumerated count says nothing about how far it scanned. Run a
  second, unfiltered browse pass that only counts (no body, no summary
  mapping; at most `maxBrowsePageSize` messages, so cheap) and compare that
  count with the depth.
- **Queue depth** comes from the existing `fetchStats` (statistics broker
  plugin), which `list-queues` already depends on. If the plugin doesn't
  answer, keep today's behavior (`hasMore = false`) and log a warning; the
  TUI's own check against the pending count (section 4) still applies to
  unfiltered browses.
- The extra work only happens in the "enumeration ended early" case, and
  the count pass only for filtered browses, so the common small-queue path
  costs one stats round trip.

The cursor (`afterMessageId`) handling stays as it is.

`deleteMessages`/`moveMessages` don't change: their receive loop already
reaches everything an unfiltered consumer can, and filtered consumers
can't be helped (spec, "What JMS can and can't reach").

**Tests** (`BrokerServiceTest.kt`, mockk-based like the existing ones):
enumeration ends below `maxCount` with depth > enumerated → `hasMore`;
depth == enumerated → no `hasMore`; filtered browse uses the counting pass;
stats unavailable → `hasMore = false`; `maxCount` reached → `hasMore`
unchanged from today.

## 2. TUI `queue.Backend` interface changes

Two signature changes in `tui/internal/queue/backend.go`:

- `PurgeQueue(ctx, queueName) (int, error)` — the number of messages
  deleted, matching `MoveAllMessages`' existing `(int, error)`.
- `BrowseMessages(ctx, queueName, filter) ([]Message, bool, error)` — the
  `bool` is `hasMore`, following the existing
  `SearchDatadogLogs(...) (events, hasMore, err)` convention in this
  codebase rather than introducing a result struct.

Ripple, all mechanical: `jolokia`, `proxy` and `secretbackend` (the
decorator passes both through), `app.ScanJMSTypes`, the views, and every
test fake (about 50 call sites, mostly tests).

## 3. TUI proxy client (`internal/queue/proxy/proxy.go`)

- **`BrowseMessages`** returns the response's `hasMore`. `listEnvelope`
  doesn't decode it yet, so it gains a `HasMore bool` field.
- **Repeat-until-zero** for every call that acts on "all matching": a
  private helper drives `delete-messages`/`move-messages` in rounds. It
  sums each round's count and stops when a round returns 0. It is used by
  `PurgeQueue`, `MoveAllMessages`, and by `DeleteMessages`/`MoveMessages`
  **only when `filter.MaxCount == 0`**. Single-message calls (`MaxCount:
  1`) and any explicit `MaxCount` keep their single request, so
  `RemoveMessage`/`MoveMessage` are untouched.
- **Termination guard:** at most `maxBulkRounds` rounds (constant, 1,000;
  at ~400 per round that's 400,000 messages). Hitting it returns an error
  wrapping the count so far: `purge "<q>" incomplete after 1000 rounds
  (<n> deleted); the server kept reporting progress`. A server that counts
  messages it didn't really delete can't loop the TUI forever.
- The context is checked between rounds, so a cancelled context stops the
  loop.
- POSTs still never retry on transport errors (spec/11); a round that
  fails ends the loop with an error that includes the count so far.

**Tests** (`proxy_test.go`, `httptest` server like the existing ones):
server caps at N per call → client loops and sums; first round 0 → one
request; filtered `MaxCount: 0` loops; `MaxCount: 1` sends exactly one
request; a server that never returns 0 → stops at `maxBulkRounds` with the
"incomplete" error; error mid-loop → error with partial count; `hasMore`
passed through from the envelope.

## 4. TUI Jolokia client (`internal/queue/jolokia/`)

- `PurgeQueue` returns a count per tier:
  - Tier 1 (`purgeQueue()`): returns nothing, so read the queue's
    `QueueSize` attribute just before purging and report that. A message
    arriving in between makes it off by a few; acceptable for a status
    line, and noted in a comment.
  - Tier 2 (`removeMatchingMessages("TRUE")`): already returns the
    number removed; use it.
  - Tier 3 (browse-and-remove): count successful removals.
- `BrowseMessages` returns `hasMore = false`. Its own browse limits are out
  of scope (spec).

**Tests:** extend the existing Jolokia `httptest` fakes per tier.

## 5. TUI views

### Shared purge flow (`internal/view/`)

Move the queue list's purge flow (JMS Type prompt → confirm → backend
call → result) out of `QueuesView` into a small helper in a new file
`internal/view/purge.go`, used by both `QueuesView` and `MessagesView`.
This fixes defect 3 by construction: one code path, so the two entry
points can't drift again. `MessagesView` gains a `JMSTypePrompt`, wired up
the same way `QueuesView` already does it.

`doPurge` keeps its dispatch (blank type → `PurgeQueue`, otherwise
`DeleteMessages` with the type) and now returns the count.

### Result line

On success, both views set the status to `Purged <n> message(s) from
"<queue>"` (with a type: `Purged <n> <type> message(s) from "<queue>"`),
then reload.

For a **filtered** purge or move-all, the helper then reads the queue's
pending count via `Backend.List`. If more than `filteredWindowWarnAt`
(200, ActiveMQ's default `maxPageSize`) messages remain, it appends: `—
<m> message(s) remain; matching messages further back may not have been
reached`. Move-all's existing `Moved <n> message(s)…` line gets the same
suffix for filtered moves.

### Message browser title

`MessagesView.Load` keeps `hasMore`, and the suffix goes into
`updateTitle` next to the existing filter description:

- **Unfiltered** (no JMS type/date filter): `Load` also fetches the
  pending count via `Backend.List`, concurrently with the browse, and
  titles the view `showing <n> of <pending>` whenever `n < pending`.
  This doesn't depend on `hasMore`, so it also covers a server whose
  `hasMore` still lies (e.g. stats plugin unavailable). If `List` fails,
  fall back to the filtered wording when `hasMore` is true, and to no
  suffix otherwise.
- **Filtered:** `first <n> browsed, more on queue` when `hasMore` is
  true.

The cost is one extra `List` call per unfiltered load.

### JMS Type scans

`ScanJMSTypes` (`internal/app/host.go`, `ui.Host`) returns
`(types []string, hasMore bool, err error)`. `JMSTypePrompt` and
`MessageFilter` both show `Scanned the first <n> messages — the queue
holds more; types further back aren't listed` in the status line when
`hasMore` is true.

**Tests:** shared purge helper (blank vs typed, count in status, warning
threshold above/below 200, error path); `MessagesView` `p` opens the JMS
Type prompt (the regression test for defect 3); title wording for
`hasMore` filtered/unfiltered and the `List` failure fallback; scan status
message on `hasMore`. Rendering checks via `tcell.NewSimulationScreen`
where a test needs the screen.

## 6. Specs (merge-back)

- `spec/09-queue-message-actions`: shared purge flow, purge count,
  filtered-operation warning, broker limits and the
  `maxPageSize`/`maxBrowsePageSize` remedy.
- `spec/11-mq-proxy-backend-integration`: `hasMore` semantics, the
  client's repeat-until-zero rule and its round limit, the new
  `PurgeQueue`/`BrowseMessages` signatures.
- `spec/08-message-browser-and-detail`: title suffix and scan message.
- `spec/10-mq-proxy-service`: one line on the browse window and stats
  dependency.

## Key decisions and trade-offs

- **Make limits visible rather than work around them.** Every workaround
  either stalls or changes the kept messages (see the spec), so no
  workaround is attempted.
- **Repeat in the client, not only in `mq-proxy`.** `mq-proxy` already
  empties a queue in one call, but the TUI can't rely on how the server
  behind the wire contract is implemented.
- **`(…, bool, error)` instead of a result struct** for `BrowseMessages`,
  to match the existing Datadog search signature. A struct would be
  easier to extend later, but nothing else is planned for it.
- **The 200 warning threshold is a heuristic.** It matches ActiveMQ's
  default window, and the wording says "may". On a broker with a larger
  window it warns more often than needed; with a smaller one it can miss.
- **Extra `List` calls**: one per unfiltered message-browser load and one
  after a filtered purge/move-all. `List` is one request, which the queue
  list already makes on every refresh.

## Manual verification

Unit tests can't cover broker behavior, so each task touching it gets a
live check with the `verify-live` skill: our `mq-proxy` against the local
broker, a disposable queue with 800 × TypeA then 800 × TypeB. The
concrete checks go into `tasks.md`.

No new dependencies.
