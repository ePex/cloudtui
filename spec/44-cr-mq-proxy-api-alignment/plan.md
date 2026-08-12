# Plan — CR 44

## Sequencing

1. `mq-proxy` (Kotlin): new DTOs/envelope, new `/api/management/command/*`
   routes, `BrokerService` changes (jmsType, selector-based filtered
   delete/move, producerCount), removal of the endpoints/code the new
   surface makes dead, `openapi.yaml` + `requests.http` regenerated.
2. `tui` (Go): `queue.Backend` gains `MessageFilter` + two new methods;
   `internal/queue/proxy` rewritten against the new wire shape (existing
   `Backend` methods keep their signatures, reimplemented under the hood);
   `internal/queue/jolokia` gets the two new methods via client-side
   filtering over its existing primitives.
3. Update the `fakeQueueBackend` test double in
   `tui/internal/app/queues_test.go` (interface grew two methods).

`mq-proxy` first because `tui`'s new client code and tests are written
against its (updated) contract.

## mq-proxy (Kotlin)

### New package: `api/model/envelope`

```kotlin
data class ApiError(val code: String, val message: String)
data class ListResponse<T>(val data: List<T>, val errors: List<ApiError> = emptyList())
data class ItemResponse<T>(val data: T?, val error: ApiError? = null)
```

`errors`/`error` are populated only for **partial failures inside a
batch** (see `delete-messages`/`move-messages` below) — a fully failed
request (broker unreachable, etc.) still surfaces as HTTP 500, unchanged
from today. This keeps error handling grounded in what's actually needed
rather than inventing a second error-reporting channel for whole-request
failures nothing currently produces.

### DTO changes (`api/model/`)

- `QueueSummary`: rename `pendingCount`→`messageCount`,
  `enqueueCount`→`enqueuedCount`, `dequeueCount`→`dequeuedCount` (aligning
  field names with the reference shape); add `producerCount: Long`.
- `MessageSummary`: becomes `sourceQueue: String, messageId: String,
  jmsType: String, body: String?, timestamp: String, headers: Map<String,
  String>?` (renamed from `id`/`properties`, added `sourceQueue`/
  `jmsType`).
- `MessageDetail`: **removed**. Nothing in `tui` calls `GET
  /api/queues/{name}/messages/{id}` today (checked: `queue.Backend` has
  no `GetMessage` method — the message-detail view in `tui` already works
  from data returned by browse, not a fresh per-message fetch), and the
  reference API has no equivalent endpoint either. Dropping it removes
  dead code (`MessageDetail.kt`, `BrokerService.getMessage`,
  `QueueController.getMessage`, and their tests) rather than carrying it
  forward unused.
- New filter DTOs:
  ```kotlin
  data class QueueMessageFilter(
      val jmsType: String? = null,
      val fromDate: String? = null,   // ISO-8601 instant
      val toDate: String? = null,     // ISO-8601 instant
      val messageId: String? = null,
      val maxCount: Int? = null,
  )
  data class DeleteMessagesRequest(val sourceQueue: String, val filter: QueueMessageFilter)
  data class MoveMessagesRequest(val sourceQueue: String, val targetQueue: String, val filter: QueueMessageFilter)
  data class SendMessageRequest(
      val targetQueue: String,
      val jmsType: String,
      val headers: Map<String, String>? = null,
      val groupId: String? = null,
      val body: String,
      val correlationId: String? = null,
  )
  data class SendMessageResponseDto(val messageId: String)
  data class DeletedMessageDto(val messageId: String)
  data class MovedMessageDto(val messageId: String)
  ```

### `QueueController.kt` — new routes, all under `/api/management/command`

| Method | Path | Request | Response |
|---|---|---|---|
| GET | `list-queues` | — | `ListResponse<QueueSummary>` |
| GET | `list-messages` | query params `sourceQueue` (required), `jmsType`, `messageId` (optional) | `ListResponse<MessageSummary>` |
| POST | `send-message` | `SendMessageRequest` | `ItemResponse<SendMessageResponseDto>` |
| POST | `delete-messages` | `List<DeleteMessagesRequest>` | `ListResponse<DeletedMessageDto>` |
| POST | `move-messages` | `List<MoveMessagesRequest>` | `ListResponse<MovedMessageDto>` |

`list-messages` takes individual query params rather than a single JSON
blob — simpler to bind with `@RequestParam` and equivalent in
capability; the reference API's exact query encoding for that endpoint
wasn't fully visible from its OpenAPI doc and isn't worth reverse
engineering byte-for-byte when the goal (per the CR's "why") is a
shape *our* side defines going forward.

`delete-messages`/`move-messages` take a **list** of requests (matching
the reference shape) so one call can act across multiple source queues;
`tui`'s client always sends a single-element list for the operations it
performs today (see below).

### `BrokerService.kt` changes

- `toSummary()`/`toDetail()` → `toSummary()` only now, add `jmsType =
  this.jmsType ?: ""` (real `jakarta.jms.Message.jmsType`, the JMS
  `JMSType` header — not inferred).
- `fetchStats`: add `"producerCount" to runCatching { reply.getLong("producerCount") }.getOrDefault(-1L)` alongside the existing four fields, using the same plugin-reply MapMessage (ActiveMQ's `StatisticsBrokerPlugin` reply already includes `producerCount`; falls back to `-1` exactly like the existing fields do when the plugin isn't enabled).
- New `deleteMessages(sourceQueue: String, filter: QueueMessageFilter): List<DeletedMessageDto>`:
  builds a JMS selector from the filter (see below), opens a
  non-transacted `AUTO_ACKNOWLEDGE` consumer with that selector, and
  loops `receiveNoWait()` — collecting each consumed message's ID —
  until either it returns `null` or `filter.maxCount` matches have been
  consumed (unset `maxCount` = unlimited, i.e. today's purge behavior
  when the filter is otherwise empty).
- New `moveMessages(sourceQueue: String, targetQueue: String, filter: QueueMessageFilter): List<MovedMessageDto>`:
  same selector + `maxCount` loop, but in a `SESSION_TRANSACTED` session
  that also creates a producer on `targetQueue` and forwards each
  consumed message before committing — mirrors the existing `moveAll`
  pattern.
- **Removed**: `getMessage`, `moveMessage(single)`, `deleteMessage(single)`,
  `moveAll` — all three single/whole-queue operations are subsumed by
  `deleteMessages`/`moveMessages` with an appropriately-shaped filter
  (single message: `filter.messageId` set, `maxCount = 1`; whole queue:
  empty filter, `maxCount` unset).
- `jmsIdSelector` helper generalizes into `QueueMessageFilter.toSelector()`:
  ```kotlin
  private fun QueueMessageFilter.toSelector(): String? {
      val clauses = mutableListOf<String>()
      jmsType?.let { clauses += "JMSType = '${it.replace("'", "''")}'" }
      messageId?.let { clauses += "JMSMessageID = '${it.replace("'", "''")}'" }
      fromDate?.let { clauses += "JMSTimestamp >= ${Instant.parse(it).toEpochMilli()}" }
      toDate?.let { clauses += "JMSTimestamp <= ${Instant.parse(it).toEpochMilli()}" }
      return clauses.takeIf { it.isNotEmpty() }?.joinToString(" AND ")
  }
  ```
  `null` selector (no filter fields set) means "match everything" —
  `session.createConsumer(queue, null)` is valid and behaves like the
  existing unfiltered `purgeQueue`/`moveAll` consumers.
- `NotFoundException` becomes dead once the three single-item methods
  that threw it are removed (nothing else throws it) — delete it.
  "Not found" is no longer a distinct error case: a filter matching zero
  messages just returns an empty `data` list, which is the correct
  semantics for a *criteria*-based operation (there's no single "the"
  message that must exist). `GlobalExceptionHandler.kt` itself is
  **kept** — only its `NotFoundException` handler is removed; its
  `JMSException`/generic-`Exception` handlers still translate broker
  failures into the existing structured error responses and aren't tied
  to the single-item lookups being removed.

### Docs

- `mq-proxy/openapi.yaml`: regenerate from the running app (springdoc
  already wired per `spec/38-fe-proxy-openapi-export`) — don't hand-edit.
- `mq-proxy/requests.http`: rewritten by hand to the five new endpoints,
  same style as today (variables, comments per section).

### Tests (Kotlin)

- `BrokerServiceTest.kt`: replace single-operation test cases with
  filter-based ones (empty filter = all, `messageId` filter = one,
  `jmsType` filter, `maxCount` capping partway through a queue,
  `fromDate`/`toDate` range); add a `jmsType`-is-populated assertion to
  the `toSummary()` coverage; delete `getMessage`/`deleteMessage`/
  `moveMessage`/`moveAll`-specific cases.
- `QueueControllerTest.kt`: replace route-specific tests with the five
  new routes, asserting the `{data, errors}` (or `{data, error}`)
  envelope shape on both success and empty-match responses.

## tui (Go)

### `internal/queue/backend.go`

```go
// MessageFilter selects which messages a bulk delete/move operation
// applies to. A zero-value field means "don't filter on this."
type MessageFilter struct {
	JMSType   string
	MessageID string
	FromDate  time.Time
	ToDate    time.Time
	MaxCount  int // 0 = unlimited
}

// Summary gains:
	ProducerCount int64

// Backend gains, alongside the existing (unchanged-signature) methods:
	DeleteMessages(ctx context.Context, queueName string, filter MessageFilter) (int, error)
	MoveMessages(ctx context.Context, sourceQueue, targetQueue string, filter MessageFilter) (int, error)
```

Existing methods (`PurgeQueue`, `RemoveMessage`, `MoveMessage`,
`MoveAllMessages`) keep their current signatures — nothing in
`internal/app`/`internal/ui` changes, per the spec's "no new TUI UI."
They're expressible as the new primitives (a `PurgeQueue` is a
`DeleteMessages` with a zero-value filter; `RemoveMessage` is a
`DeleteMessages` with `MessageID` + `MaxCount: 1`) but are **kept as
separate interface methods** rather than rewritten as call-sites that
build a `MessageFilter` — smaller diff in `internal/app`, and the
single/whole-queue cases are common enough to deserve their own names.

### `internal/queue/proxy/proxy.go` — rewritten against the new shape

- Wire envelope types:
  ```go
  type apiError struct {
      Code    string `json:"code"`
      Message string `json:"message"`
  }
  type listEnvelope[T any] struct {
      Data   []T        `json:"data"`
      Errors []apiError `json:"errors"`
  }
  type itemEnvelope[T any] struct {
      Data  *T        `json:"data"`
      Error *apiError `json:"error"`
  }
  ```
  (Go 1.26 per `go.mod` — generics are available.)
- `doRequest`/`getJSON` unwrap the envelope: a populated `Errors`/`Error`
  on an otherwise-200 response surfaces as a Go `error` built from the
  first entry's `code`/`message`.
- `proxyQueue` fields renamed to match: `messageCount`, `consumerCount`,
  `enqueuedCount`, `dequeuedCount`, `producerCount`; `List` maps these
  onto `queue.Summary{PendingCount, ConsumerCount, EnqueueCount,
  DequeueCount, ProducerCount}` (internal names deliberately unchanged —
  only fixing the wire mapping, not renaming `queue.Summary`'s existing
  fields, which are shared with the Jolokia backend and the UI).
- `proxyMessage` fields become `sourceQueue`, `messageId`, `jmsType`,
  `body`, `timestamp`, `headers`; `toQueueMessage` uses the real
  `jmsType` when non-empty, falling back to today's body-presence
  inference (`"text"`/`"other"`) only when it's empty — mirrors
  `jolokia.go:307-318`.
- `List`/`BrowseMessages` call `GET .../list-queues` /
  `GET .../list-messages?sourceQueue=...`.
- `SendMessage(ctx, queueName, body string)` keeps its existing
  signature (nothing in `tui` sets `jmsType`/`groupId`/`correlationId`
  today) — posts `SendMessageRequest{TargetQueue: queueName, JMSType:
  "text", Body: body}` to `send-message`. `JMSType: "text"` matches the
  implicit behavior the old `sendMessage` had (it always created a
  `TextMessage`).
- `PurgeQueue`, `RemoveMessage`, `MoveMessage`, `MoveAllMessages` become
  thin wrappers that call the new `DeleteMessages`/`MoveMessages` with
  the appropriate filter, as described above.
- `DeleteMessages`/`MoveMessages` (new) POST a single-element
  `[]DeleteMessagesRequest`/`[]MoveMessagesRequest` and sum the returned
  `data` length across the (always one) result entries.

### `internal/queue/jolokia/jolokia.go` — two new methods, no changes to existing ones

```go
func (c *Client) DeleteMessages(ctx context.Context, queueName string, filter queue.MessageFilter) (int, error) {
	msgs, err := c.BrowseMessages(ctx, queueName)
	...
	matches := filterMessages(msgs, filter)
	for _, m := range matches {
		if err := c.RemoveMessage(ctx, queueName, m.ID); err != nil { ... }
	}
	return len(matches), nil
}
```
`MoveMessages` mirrors this using the existing `MoveMessage` per match.
`filterMessages([]queue.Message, queue.MessageFilter) []queue.Message` is
a pure function (JMSType exact match, MessageID exact match, Timestamp
within `[FromDate, ToDate]` when set, truncated to `MaxCount`) — unit
tested directly without a broker, same pattern as `buildStageStatuses`
etc. elsewhere in this codebase.

This is real client-side filtering (browse everything, filter in Go),
exactly as scoped — Jolokia/JMX has no selector-based browse.

### `internal/app/queues_test.go`

`fakeQueueBackend` gets two more no-op methods
(`DeleteMessages`/`MoveMessages`) so it keeps satisfying `queue.Backend`.

### New/updated tests

- `tui/internal/queue/proxy/proxy_test.go`: rewritten fixtures for the
  new endpoints/envelope; a case asserting `jmsType` passthrough and the
  fallback-to-inference-when-empty behavior; error-envelope decoding.
- `tui/internal/queue/jolokia/jolokia_test.go`: new table-driven tests
  for `filterMessages` (each filter field alone, combined, `MaxCount`
  truncation, empty filter = everything) and for `DeleteMessages`/
  `MoveMessages` wiring (using the existing fake HTTP transport pattern
  already in that file).

## Manual verification (per `tui/CLAUDE.md` / the `verify-live` skill)

Since this changes both the proxy backend's wire protocol and the
Jolokia backend's message operations:

1. `task dev:proxy:start` (rebuilds `mq-proxy` from source), point `tui`
   at it, confirm queue list / browse / send / single delete / single
   move / purge all still work through the TUI exactly as before.
2. Seed a queue with a mix of `jmsType`s (or rely on the existing
   `task seed:queue` sample shape plus one manually-sent message with a
   distinct `jmsType`) and confirm a filtered delete/move (driven
   directly, not through UI — there's no UI for it yet) removes/moves
   only the matching subset, against **both** backends.
3. Re-run `task smoke:test` against the proxy backend to catch any
   regression in the paths it already covers.

## Risk / rollout note

This breaks `mq-proxy`'s API for any consumer other than `tui` — per the
spec, acceptable since `tui` is its only consumer today. `openapi.yaml`
and `requests.http` are updated in the same commit so they never
describe a stale contract.
