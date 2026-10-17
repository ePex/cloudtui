# mq-proxy wire contract and the TUI's proxy backend

_Condensed from spec/21, spec/44, spec/45, spec/47, spec/48, spec/49, spec/50, spec/51, spec/58 — see those folders for the incremental history. The wire contract went through several corrections (spec/44 → 45 → 48 → 49 → 51); this document describes ONLY the final, current shape. Do not build against any REST route table in spec/20 or spec/21 — those are superseded._

## Purpose

`tui/internal/queue/proxy/` implements `queue.Backend` (spec/07) by
calling `mq-proxy` (spec/10) over REST/JSON, so the TUI can talk to
brokers that don't expose Jolokia (notably AWS Amazon MQ). Which backend a
connection uses (`jolokia` vs `proxy`) is a per-connection config field —
see spec/12 (Named connections) for that config shape; this document
covers the wire contract itself and the Go client's behavior.

## REST API shape (current, final)

- **Command-style routing**, not resource-style:
  `/api/management/command/<verb>`, e.g. `list-queues`, `list-messages`,
  `send-message`, `delete-messages`, `move-messages`. This matches a
  reference internal queue-management API's convention — `mq-proxy` is the
  reference implementation going forward (the intent is the reverse
  service converges to mq-proxy's shape, not the other way around).
- **Response envelope**: every response wraps its payload as `{ data,
  errors }` (or `{ data, error }` for single-result operations), rather
  than a bare array/object with errors only as HTTP status.
- **Auth**: HTTP Basic on every `/api/**` endpoint, unchanged from
  spec/10.

### `list-queues`

Returns queue summaries: `name`, pending count, consumer count, enqueue
count, dequeue count, **and `producerCount`**.

### `list-messages` (GET)

Query params bind through a wrapper object (`ListMessagesQuery` on the
Kotlin side, `@ModelAttribute`), which is why the shape is:

- `sourceQueue` — **required**, top-level.
- `returnBody` — optional, top-level, default `false` (message bodies are
  *not* included unless explicitly requested).
- `filter.*` — nested, not flat: `filter.jmsType`, `filter.messageId`,
  `filter.fromDate`, `filter.toDate`, `filter.maxCount`,
  `filter.afterMessageId`.
- **`filter.maxCount` is required and must be a positive integer** — a
  request without it, or with `maxCount <= 0`, is rejected with 400. This
  is the one endpoint where the cap is non-optional (see Gotchas).
- **`filter.afterMessageId`** (optional): a pagination cursor, distinct
  from `filter.messageId` (which means "return only this one message").
  Unset behaves as before (start from the beginning of the queue); set,
  the server skips every message up to and including a match, then
  returns up to `filter.maxCount` messages starting after it. A cursor
  that no longer matches any message on the queue (e.g. a concurrent
  purge/consume raced the page) silently falls back to starting from the
  beginning rather than erroring.

Each returned message includes the real JMS `jmsType` header (not
inferred). The response also carries two booleans, shared on
`mq-proxy`'s `ListResponse<T>` (also used by `list-queues` and
delete/move, where they're simply unused and default `false`) rather
than forking a message-specific response type:

- **`hasMore`** — another page can be fetched: `maxCount` was reached
  and at least one more matching message follows. This is what a "Load
  more" pages on (`mq-proxy-web`, spec/21).
- **`truncated`** — the browse ended before `maxCount` only because the
  broker cut it short: the queue holds messages this browse couldn't
  see, and paging can't reach them either.

A JMS `QueueBrowser` on ActiveMQ never sees more than the broker's
`maxBrowsePageSize` (default **400**) messages, filtered or not, and its
enumeration just ends there — indistinguishable from the end of the
queue. So when `mq-proxy`'s enumeration ends before `maxCount`, it
compares the browse window with the queue depth from the statistics
plugin (the same `fetchStats` `list-queues` uses): an unfiltered pass's
enumerated count *is* the window; a filtered pass's isn't (the broker
applies the selector itself), so it counts the window with a second,
unfiltered, count-only browse. Depth larger than the window →
`truncated: true`. Without the statistics plugin it can't tell, keeps
`false`, and logs a warning — and every browse that ends early then
waits up to `fetchStats`' 3 s reply timeout first. Browsing never
returns messages beyond the window; `truncated` only makes the cut
visible.

**Why two fields:** reporting the cut through `hasMore` was tried first
and broke `mq-proxy-web`'s "Load more" — on any queue over 400 messages
it stayed visible forever, every click returning an empty page (the
cursor is found by browsing, so the next page can't get past the window
either).

### `send-message` (POST)

Structured body DTO: `targetQueue`, `jmsType`, `headers` (map), `groupId`,
`body`, `correlationId` — not an untyped JSON blob. `jmsType` and `body`
are required; `headers`/`groupId`/`correlationId` are optional (`null`/
absent means "don't set this" on the outgoing message). A `headers` entry
named after one of the three fields governed by their own dedicated
argument (`JMSType`, `JMSCorrelationID`, `JMSXGroupID`) is dropped by
`BrokerService.sendMessage` rather than applied — the dedicated field
always wins, headers are strictly additive.

### `delete-messages` / `move-messages` (POST)

JSON body is an **array** of the request object (`openapi.yaml`'s schema
is `type: array` of `DeleteMessagesRequest`/`MoveMessagesRequest` — the
shape exists for bulk-operation parity, but the TUI always sends exactly
one element), each nesting the filter naturally (Kotlin data class
serialization):
`[{ sourceQueue, filter: { jmsType, messageId, fromDate, toDate, maxCount } }]`
(plus a target queue for move). `filter.maxCount` stays **optional** here —
unset means "match everything" (purge / move-all), a deliberately
preserved, tested behavior, unlike `list-messages`.

**Nothing in the contract promises that one call acts on every matching
message.** `mq-proxy` consumes until a receive times out, so an
unfiltered delete/move empties the queue in one call; a server that
browses first and then consumes each browsed message stops at the
browse window (~400) per call. Clients repeat until a call acts on 0
messages (see the Go client below). With a filter, `mq-proxy`'s
selector consumer only reaches matches among the messages the broker
has paged in (`maxPageSize`, default 200) — see spec/09's "Broker limits
on filtered purge/move-all".

### Errors

`BindException` (constructor-binding/query-binding failures, e.g. a
missing required field) and `IllegalArgumentException` (validation
failures, e.g. missing/non-positive `filter.maxCount` on `list-messages`)
both map to HTTP 400 via `GlobalExceptionHandler`.

## The Go client (`tui/internal/queue/proxy/proxy.go`)

- Every request it sends, including each warm-up retry, is logged as one
  line with the URL path as the operation (spec/06, "Queue backend
  requests").
- Sends `list-messages` filter fields **only in nested `filter.*` form** —
  no flat/legacy duplicate params.
- Maps the real `jmsType` response field to `queue.Message.JMSType`,
  falling back to body-presence inference (`"text"` when a body is
  present, `"other"` otherwise) only when the server doesn't supply one —
  mirroring the pattern the Jolokia backend uses for its own `jMSType`
  header (spec/08), so both backends filter on the same kind of
  value. An inferred type sets `queue.Message.JMSTypeInferred` (spec/08,
  spec/22).
- Sends `returnBody=true` always when browsing (the message browser needs
  the body for its preview column — spec/08).
- `BrowseMessages` returns `hasMore || truncated` as its bool
  (`queue.Backend.BrowseMessages` is `([]Message, bool, error)`) — the
  TUI only needs "the queue holds more than shown", not which; the
  message browser's title and the JMS Type scans use it (spec/08).
- **Only entries with a message ID count.** `delete-messages`/
  `move-messages` responses are read as the IDs they report; an entry
  without a `messageId` isn't evidence that anything was removed (a
  server that browses before consuming can report a browsed message whose
  consume came back empty). So `RemoveMessage`/`MoveMessage` report "not
  found" for a response holding only an ID-less entry.
- **Bulk delete/move repeat until nothing is left.** `PurgeQueue`,
  `MoveAllMessages`, and `DeleteMessages`/`MoveMessages` **without a
  `MaxCount`** send `delete-messages`/`move-messages` in rounds (`inRounds`)
  until a round reports no entries — so they're complete against a server
  that caps per call. With a `MaxCount` (incl. `RemoveMessage`/
  `MoveMessage`'s `MaxCount: 1`) it's a single request. Guards:
  - **No progress ends it.** The loop remembers every ID reported during
    the operation and counts only new ones. A round that reports entries
    but nothing new — only IDs already seen, or only ID-less entries —
    ends it with an "incomplete" error (`errBulkNoProgress`): `the server
    kept reporting <k> message(s) it didn't remove (<n> removed)`. A
    really removed message can't be reported again (mq-proxy consumes
    each once; a moved message gets a new ID in the target), so this
    catches a server that keeps reporting messages it can't consume —
    e.g. ones a live consumer holds — within one round of the last real
    progress, instead of repeating until the cap with a consume timeout
    per stuck message (found by reading such a server's code: ~3 hours
    for one stuck message). The IDs in that final round were counted
    earlier but are evidently still there, so they come back off the
    total. Chosen over re-reading the queue's pending count per round:
    no extra request, and it doesn't misfire while producers keep
    adding messages.
  - At most `maxBulkRounds` (1,000) rounds — the last resort, returning
    "incomplete" (`errBulkIncomplete`) with the count so far.
  - The context is checked between rounds; a failed round ends the loop
    with the count so far alongside the error. A cancelled context (Esc
    in the progress modal, spec/24) ends it the same way, with
    `context.Canceled`, but never aborts a round in flight: every POST
    (`postJSON`, i.e. every mutation) runs with
    `context.WithoutCancel`. Reads (GETs) keep the caller's context, so
    Esc cancels a load at once.

  Against `mq-proxy` an unfiltered purge is one deleting round plus one
  returning 0 — which costs one more receive timeout (~2 s, see
  Gotchas); its transacted move-all can take several rounds (seen live:
  1,200 messages in 4 requests), which the loop completes.
- `PurgeQueue` returns the number deleted (`(int, error)`, spec/09).
- `SendMessage` populates `send-message`'s `jmsType`/`headers`/`groupId`/
  `correlationId` fields directly from `queue.SendMessageRequest` (spec/09) —
  a 1:1 passthrough, no client-side precedence logic needed since
  `BrokerService.sendMessage` already handles the reserved-key rule above.
- **GET requests retry exactly once** on a pure transport-level failure
  (`http.Client.Do` returns a non-nil error — connection refused, DNS
  failure, timeout waiting for headers — meaning no response was ever
  received). This covers `List` and `BrowseMessages`. **POST requests
  (send/delete/move) never retry** — a POST that times out waiting for a
  response may already have been applied server-side, so blind retry risks
  double-applying it; a GET is idempotent and side-effect-free, so
  replaying it is always safe. Exactly one retry, no backoff; a second
  failure surfaces as a real error.
- **Client-side default `maxCount`**: the TUI's message browser
  (`internal/app/messages.go` or its post-refactor location — see
  spec/03) applies a default of **500** whenever the user hasn't set
  one via the filter form, for both the proxy and Jolokia backends — so the
  proxy backend's hard requirement is always satisfied transparently, and
  the Jolokia backend (which still fetches unbounded from JMX, then
  filters client-side) at least bounds what's kept/rendered. The table
  title always shows the effective `max=N` in use, even when the user
  never touched the filter form, so messages don't silently go missing
  with no visible explanation. `delete-messages`/`move-messages` are not
  given a client-side default — unset stays unset (purge/move-all).

## Verification tooling

`tui/scripts/verify-proxy-api.sh` (`task verify:proxy-api`) exercises the
full contract above directly against a live backend (mq-proxy or the
reference API — both speak the same nested-filter shape) without going
through the TUI: sends 10 mixed-type messages, verifies message-detail
fields, verifies filtering by JMS type and `maxCount`, deletes one message,
bulk-deletes, moves one, bulk-moves, purges both queues involved. Takes
`<base-url> <username> <password> <queue> [target-queue]`; destructive by
design (ends by purging), documented via usage text rather than an
interactive confirm prompt, matching this repo's other disposable-queue
tooling. Manual/on-demand only — not wired into CI (needs a live broker).

## Notable gotchas worth preserving

- **`consumer.receiveNoWait()` is unreliable for delete/move.** ActiveMQ
  dispatches matching messages to a freshly-created consumer's prefetch
  buffer *asynchronously*; `receiveNoWait()` gives up immediately if that
  dispatch hasn't landed yet, even against messages that have existed on
  the queue for seconds. `BrokerService.deleteMessages`/`moveMessages`
  therefore use `consumer.receive(2000)` (a real timeout wait), not
  `receiveNoWait()`. The trade-off: the final loop iteration of a
  delete/move/purge always waits up to that timeout before concluding "no
  more matches" — accepted, matches this file's existing
  `fetchStats`/`consumer.receive(3_000)` precedent.
- **How far a filtered browse gets varies.** With a selector, the broker
  sometimes scans slightly past `maxBrowsePageSize` before stopping: a
  `jmsType=TypeB` browse over 800 × TypeA then 800 × TypeB returned 0
  in one run and 48 (stable across repeats) in another, depending on
  what the broker had paged in. Likewise a filtered delete reached 0 in
  one run and 112 in another. Don't test for exact counts there —
  `truncated`/the TUI's warning are what's reliable.
- **Spring nested query-param binding only activates when the outer bound
  object has a required field.** A bare `@ModelAttribute filter:
  QueueMessageFilter` parameter (all-optional fields) silently binds
  nothing on nested paths — it does NOT error, it just returns everything
  unfiltered. The fix that works: wrap it in an outer object that has at
  least one required field (`ListMessagesQuery.sourceQueue`), which forces
  constructor-binding for the whole tree including the nested `filter.*`
  fields. Relevant if this contract is ever reimplemented in a different
  Spring version or a different framework.
- **springdoc's generated schema for `list-messages` shows an opaque
  wrapper-object reference**, not the individual query fields, once nested
  binding is used — a known, accepted discoverability trade-off.
  `mq-proxy/requests.http` and `openapi.yaml` are the practical reference
  for the real query shape.
- **`mq-proxy` and the reference API both silently ignore unrecognized
  extra query params** (ordinary Spring `@RequestParam`/`@ModelAttribute`
  behavior, no strict validation) — this is why a since-removed
  dual-send (flat + nested) workaround was safe to use as a transitional
  compatibility shim, and why it was safe to remove once `mq-proxy`
  settled on nested-only.
- **Header/property value typing is a known, accepted divergence**:
  `mq-proxy` stringifies all JMS headers/properties; the reference API
  preserves real types. The `tui` client already tolerates this — not
  something to "fix."
- **`list-messages`'s response has no `correlationId` field at all** —
  unlike `send-message`'s response, which does. `queue.Message.CorrelationID`
  is therefore never populated for a message browsed via the proxy
  backend (`toQueueMessage`, `proxy.go`), and the TUI's message detail
  view (spec/08), whose header-field lookup table is shaped around
  Jolokia's own raw-response key names anyway, shows nothing useful for
  `JMSCorrelationID`/`JMSDeliveryMode`/etc. on any proxy-backend message
  regardless. `PropertiesText` still surfaces `JMSXGroupID` and custom
  headers correctly (both travel as `headers` on the wire), so a sent
  Correlation ID is not lost server-side — it's just not visible again
  through this UI once sent, a pre-existing gap found live while
  verifying spec-wip/fe-send-message-metadata, not fixed as part of it.

## Out of scope (deliberate)

- **The Go client above does not send `filter.afterMessageId`** — it
  reads `hasMore`/`truncated` to say when a browse is incomplete, but doesn't page:
  the server finds the cursor by browsing, which stops at the same
  browse window, so paging can't get past it. `filter.afterMessageId`
  exists for `mq-proxy-web`'s "Load more" (spec/21).
- Getting past the broker's browse/paging windows (`maxBrowsePageSize`,
  `maxPageSize`) — not possible over JMS without side effects (spec/09).
- No configurable TUI-side default `maxCount` (fixed at 500).
- No retry for POST requests, no backoff/retry-count configuration for
  GET retries, no proactive connection warm-up on activation.
- The Jolokia backend's own JMX round-trip is unaffected by any of this —
  it still fetches everything and filters/caps client-side.
