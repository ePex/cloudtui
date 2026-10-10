# Bugfix: purge and browse don't reach every message on the proxy backend

Date: 2026-10-10

## Problem

Reported against the proxy backend: purging a queue with more than ~1,000
messages leaves most of them behind (only ~400 deleted); results get
worse when the queue holds several JMS types; and purging from the
queue list behaves differently from purging inside an open queue.

Reproduced live (local ActiveMQ, a disposable scratch queue, this
repo's `mq-proxy`). Three separate defects combine into
the reported behavior, and a fourth (browsing) was found while
reproducing them:

### 1. The TUI trusts a single `delete-messages` call (TUI, proxy backend)

`proxy.Client.PurgeQueue` sends one `delete-messages` request with an
empty filter and no `maxCount`, and treats any non-error response as
"queue is empty". `DeleteMessages`/`MoveMessages` without a `maxCount`
(used for filtered purge and filtered move-all) do the same.

Nothing in spec/11's wire contract guarantees that one call acts on
every matching message. A server that implements delete as "browse,
then consume each browsed message by ID" is capped per call: a JMS
`QueueBrowser` on ActiveMQ never returns more than the broker's
`maxBrowsePageSize` (default **400**) messages, regardless of the
requested limit — which matches the reported "only ~400 deleted". This
repo's `mq-proxy` consumes instead of browsing, so an *unfiltered* purge
empties the queue in one call (verified: 1,202 / 1,600 / 5,004
messages, mixed types, all deleted). The TUI must not depend on how the
server behind the contract is implemented.

### 2. Filtered delete/move starves behind non-matching messages (mq-proxy)

`BrokerService.deleteMessages`/`moveMessages` open a consumer with a JMS
selector (e.g. `JMSType = 'TypeB'`) and stop at the first 2 s receive
timeout. ActiveMQ only evaluates a selector consumer against the
messages it has paged into memory (default `maxPageSize` 200, from the
head of the queue). If none of those match, nothing is ever dispatched
and the call returns 0, even though matching messages exist further
back. Verified with 800 × TypeA followed by 800 × TypeB:

| Filter | Deleted | Remaining |
|---|---|---|
| `TypeB` | **0** | 1,600 |
| `TypeA` | 800 | 800 |
| `TypeB` (now at the head) | 800 | 0 |

This affects filtered purge and filtered move-all alike.

### 3. Purge inside an open queue skips the JMS Type prompt (TUI)

spec/09 says both `p` entry points (queue list and messages list) open
the JMS Type filter prompt, then confirm. Verified live: the queue list
does; the messages list (`internal/view/messages.go`) goes straight to
the "All messages will be deleted" confirm and always calls
`PurgeQueue`. That's the "behaves differently" part of the report — the
same action offers different options depending on where it's started.

### 4. Browsing caps at 400 and claims there is nothing more (mq-proxy, TUI)

`list-messages` uses a JMS `QueueBrowser`, so it hits the same
`maxBrowsePageSize` cap as defect 1: browsing a 1,600-message queue with
`filter.maxCount=5000` returns 400 messages **and `hasMore: false`**.
`mq-proxy` derives `hasMore` from "did the browser enumeration yield
another message", which is false once the broker stops at 400 — so the
flag lies exactly when it matters.

Consequences in the TUI:

- The message browser silently shows at most 400 messages, with no hint
  that the queue holds more.
- The JMS Type scan ("Scan up to 5,000 messages for JMS types", and the
  prompt's automatic 500-message scan) never sees types beyond the first
  400. Verified live: a queue with 800 × TypeA then 800 × TypeB only
  offered `TypeA` — so a user can't even pick the type they'd need for
  a filtered purge.

### What JMS can and can't reach

Both broker limits are broker policy (`maxPageSize`, default 200, for
consumers; `maxBrowsePageSize`, default 400, for browsers), and JMS
offers no side-effect-free way around them. Verified live:

- **Unfiltered consuming reaches everything** (plain purge, move-all).
- **Browsing sees only the first 400 messages, filtered or not**: a
  browse with `jmsType=TypeB` on 800 × TypeA then 800 × TypeB returned
  0 messages.
- **Consuming everything and acknowledging only matching messages**
  (ActiveMQ's individual-acknowledge mode) doesn't work around it
  either: it stalled after 800 of 1,600 messages, and every message it
  handed back came back flagged as redelivered (`JMSRedelivered=true`,
  redelivery counter incremented) and out of order — unacceptable on
  real data.
- The other workaround — move everything to a temporary queue and move
  the non-matching messages back — would give every kept message a new
  message ID and timestamp. Rejected for the same reason.

JMX would bypass these limits, but Amazon MQ doesn't expose it, which is
the reason `mq-proxy` exists. So for filtered operations and browsing,
this fix makes the limits **visible** instead of silently pretending to
be complete. The remedy for a queue that really needs more is raising
the broker's `maxPageSize`/`maxBrowsePageSize`, which the spec
documents.

## Fix (expected behavior)

- **Unfiltered purge and unfiltered move-all on the proxy backend act on
  every message**, regardless of server-side per-call limits — the
  client keeps issuing calls until the server reports nothing left to
  act on. **Filtered** purge/move-all repeat the same way, so they reach
  everything the broker lets a filtered consumer see.
- That repetition **always terminates**: it stops when a call acts on 0
  messages, and is also bounded so a server that misreports progress
  can't loop the TUI forever. If the bound is hit while messages still
  remain, the user gets an error saying the purge/move was incomplete,
  rather than a silent partial success.
- **A filtered purge/move-all that may have missed matches says so.**
  When more messages remain on the queue afterwards than the broker's
  default consumer window (200), the result line adds a warning that
  matching messages further back may not have been reached. The
  warning is a "may", not a certainty: the TUI can't tell whether the
  remaining messages include matches.
- **`p` inside an open queue goes through the same JMS Type prompt and
  confirm wording as `p` in the queue list**, and performs the same
  backend call for the same input.
- **Purge reports how many messages it deleted**: on success the status
  line shows `Purged <n> message(s) from "<queue>"` (mirroring move-all's
  existing `Moved <n> message(s) from "<src>" to "<dst>"`), for both
  entry points, filtered or not, on both backends. This requires
  `queue.Backend.PurgeQueue` to return the count; where a backend's
  native purge can't report an exact count (Jolokia's `purgeQueue()`
  JMX operation), the plan decides how the count is obtained (e.g. the
  queue's pending count read just before purging).
- **Browsing tells the truth about being incomplete**:
  - `mq-proxy`'s `list-messages` gains **`truncated: Boolean`**, true
    when the broker cut the browse short — detected when the queue holds
    more messages than the browser saw. `hasMore` keeps its meaning,
    "another page can be fetched" (`maxCount` reached with more
    behind it), which the web console's "Load more" (spec/21) relies
    on. *Amended 2026-10-10 during merge-back:* the first version set
    `hasMore` itself for a cut-short browse, which left the web
    console's "Load more" visible forever on any queue over 400
    messages, every click returning an empty page (verified live).
  - The TUI treats either flag as "incomplete".
  - The message browser shows that in its title, e.g.
    `showing 400 of 1,600` when unfiltered, or `first 400 browsed, more
    on queue` when filtered (the pending count isn't comparable to a
    filtered result).
  - A JMS Type scan that hit the limit says so (e.g. `Scanned the first
    400 of 1,600 messages — types further back aren't listed`), in both
    the purge/move-all prompt and the message filter.
  - Paging with `filter.afterMessageId` doesn't help: the server finds
    the cursor by browsing, which stops at the same 400. The TUI doesn't
    attempt it.
- Single-message delete/move (`maxCount: 1`) are unchanged.
- The Jolokia backend only changes to report the purge count; its purge
  and browse paths otherwise stay as they are (not reported as
  affected).

## Out of scope

- Progress indicators for long purges/moves (already out of scope in
  spec/09).
- Changing broker configuration (`maxPageSize`/`maxBrowsePageSize`).
  The spec documents it as the remedy; neither the TUI nor `mq-proxy`
  changes or depends on it.
- Reaching messages beyond the broker's limits for filtered operations
  or browsing (see "What JMS can and can't reach").
- Changing the TUI's own browse/scan limits (500 default, 5,000 scan).
- The Jolokia backend's browsing (its own `browse()` limits are
  documented in spec/08 and weren't reported as affected).
