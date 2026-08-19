# mq-proxy wire contract and the TUI's proxy backend

_Condensed from spec/21, spec/44, spec/45, spec/47, spec/48, spec/49, spec/50, spec/51, spec/58 — see those folders for the incremental history. The wire contract went through several corrections (spec/44 → 45 → 48 → 49 → 51); this document describes ONLY the final, current shape. Do not build against any REST route table in spec/20 or spec/21 — those are superseded._

## Purpose

`tui/internal/queue/proxy/` implements `queue.Backend` (spec-origin/07) by
calling `mq-proxy` (spec-origin/10) over REST/JSON, so the TUI can talk to
brokers that don't expose Jolokia (notably AWS Amazon MQ). Which backend a
connection uses (`jolokia` vs `proxy`) is a per-connection config field —
see spec-origin/12 (Named connections) for that config shape; this document
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
  spec-origin/10.

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
  `filter.fromDate`, `filter.toDate`, `filter.maxCount`.
- **`filter.maxCount` is required and must be a positive integer** — a
  request without it, or with `maxCount <= 0`, is rejected with 400. This
  is the one endpoint where the cap is non-optional (see Gotchas).

Each returned message includes the real JMS `jmsType` header (not
inferred).

### `send-message` (POST)

Structured body DTO: `targetQueue`, `jmsType`, `headers` (map), `groupId`,
`body`, `correlationId` — not an untyped JSON blob.

### `delete-messages` / `move-messages` (POST)

JSON body nests the filter naturally (Kotlin data class serialization):
`{ sourceQueue, filter: { jmsType, messageId, fromDate, toDate, maxCount } }`
(plus a target queue for move). `filter.maxCount` stays **optional** here —
unset means "match everything" (purge / move-all), a deliberately
preserved, tested behavior, unlike `list-messages`.

### Errors

`BindException` (constructor-binding/query-binding failures, e.g. a
missing required field) and `IllegalArgumentException` (validation
failures, e.g. missing/non-positive `filter.maxCount` on `list-messages`)
both map to HTTP 400 via `GlobalExceptionHandler`.

## The Go client (`tui/internal/queue/proxy/proxy.go`)

- Sends `list-messages` filter fields **only in nested `filter.*` form** —
  no flat/legacy duplicate params.
- Maps the real `jmsType` response field to `queue.Message.JMSType`,
  falling back to body-presence inference (`"text"` when a body is
  present, `"other"` otherwise) only when the server doesn't supply one —
  mirroring the pattern the Jolokia backend uses for its own `jMSType`
  header (spec-origin/08), so both backends filter on the same kind of
  value.
- Sends `returnBody=true` always when browsing (the message browser needs
  the body for its preview column — spec-origin/08).
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
  spec-origin/03) applies a default of **500** whenever the user hasn't set
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

## Out of scope (deliberate)

- No TUI UI (screens/keybindings) for the filtered bulk delete/move
  `queue.Backend` methods that exist for API-contract parity — they're
  implemented and unit-tested on both backends but nothing in
  `internal/app`/`internal/view` calls them; exposing bulk-filtered
  delete/move to the user would be a future feature.
- No pagination/"load more" — `maxCount` caps what's fetched; no follow-up
  affordance to fetch the next batch, and no in-UI indication that more
  messages exist beyond the cap.
- No configurable TUI-side default `maxCount` (fixed at 500).
- No retry for POST requests, no backoff/retry-count configuration for
  GET retries, no proactive connection warm-up on activation.
- The Jolokia backend's own JMX round-trip is unaffected by any of this —
  it still fetches everything and filters/caps client-side.
