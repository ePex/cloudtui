# CR 49 — mq-proxy's list-messages binds a nested filter object

Date: 2026-08-13

## What

Change `mq-proxy`'s `GET /list-messages` from five flat query params
(`jmsType`, `messageId`, `fromDate`, `toDate`, `maxCount`) to a nested
`filter.*` shape (`filter.jmsType`, `filter.maxCount`, etc.), matching
both the reference queue-management API's binding convention and
`mq-proxy`'s own `delete-messages`/`move-messages` endpoints (whose JSON
bodies already nest `filter: {...}`, since that's how a Kotlin data
class with a `filter` field serializes — `list-messages` was the one
inconsistent case, a GET query string, which doesn't auto-nest the same
way). `sourceQueue` and `returnBody` stay top-level, matching the
reference API.

This obsoletes bugfix 48's client-side workaround (`tui`'s proxy client
sending each filter field under both a flat and a `filter.`-prefixed
name) — once `mq-proxy` only recognizes nested, the flat half of that
dual-send becomes dead weight and is removed.

## Why

Discussed live: `list-messages`' flat shape was never a deliberate
design choice, just an artifact of Kotlin's default JSON/query nesting
behavior differing between POST (JSON body, naturally nests) and GET
(query string, needs explicit nested binding to nest) — see the
conversation that led to bugfix 48. Decided to fix it at the source
(`mq-proxy`) rather than keep carrying a client-side dual-send
workaround indefinitely.

**Prototyped and confirmed live** before committing to this approach:

- `@ModelAttribute` binding to a wrapper data class
  (`ListMessagesQuery(sourceQueue, returnBody, filter: QueueMessageFilter)`)
  correctly binds `filter.jmsType`/`filter.maxCount`/etc. via Spring's
  constructor-binding for nested Kotlin data classes — confirmed against
  a real broker (`filter.maxCount=1` correctly capped a 10-message
  queue to 1; a flat `maxCount=1` is now correctly *ignored*).
- **A bare `@ModelAttribute filter: QueueMessageFilter` parameter (not
  wrapped) does NOT do the same nested binding** — it silently binds
  nothing, returning every message unfiltered. This is a real Spring
  behavior gap worth documenting: constructor-binding for nested objects
  only kicks in when the *outer* bound object also needs constructor
  binding (i.e. has a required field), not for a bare all-optional
  data class parameter. The wrapper class works because `sourceQueue`
  is a required field, forcing constructor binding for the whole tree.
- **Found and fixed a regression along the way**: with the wrapper
  approach, a request missing `sourceQueue` failed with a raw HTTP 500
  (a Kotlin null-check exception falling through to the generic
  exception handler) instead of the 400 a plain `@RequestParam` would
  give. Added a `BindException` handler to `GlobalExceptionHandler`
  mapping constructor-binding failures to 400, confirmed live.
- **Accepted trade-off**: springdoc's generated OpenAPI schema for
  `list-messages` now shows the query parameter as a single opaque
  `ListMessagesQuery` object reference, not the five individual fields
  it used to list — the same discoverability quirk this session
  originally found and had to work around in the *reference* API's own
  docs. `openapi.yaml` and `requests.http`'s examples are the practical
  reference for this endpoint's real query shape going forward.

## Scope

`mq-proxy` (Kotlin):

1. `QueueController.kt`: replace the five flat `@RequestParam`s with
   `@ModelAttribute query: ListMessagesQuery` (new data class,
   `sourceQueue: String`, `returnBody: Boolean? = null`,
   `filter: QueueMessageFilter = QueueMessageFilter()`).
2. `GlobalExceptionHandler.kt`: add a `BindException → 400` handler.
3. `QueueControllerTest.kt`: update the existing filter-passthrough test
   to use `filter.jmsType`/`filter.messageId` query params; add a test
   confirming a missing `sourceQueue` returns 400.
4. `openapi.yaml` regenerated (`task openapi:proxy`).
5. `requests.http`: update the filtered `list-messages` examples to the
   nested query shape.

`tui` (Go):

6. `tui/internal/queue/proxy/proxy.go`'s `browseQuery`: remove the flat
   half of bugfix 48's dual-send — send only `filter.<name>` for each
   set field. `sourceQueue`/`returnBody` unaffected.
7. `proxy_test.go`: update `TestBrowseMessagesFilterQuery` (and the
   zero-value-filter assertion in `TestBrowseMessages`) to expect only
   the nested form.

## Out of scope

- **`delete-messages`/`move-messages`** — already nest `filter` in their
  JSON bodies, unaffected by this CR.
- **The reference API itself** — not ours to change; this CR only makes
  `mq-proxy` consistently nested, which happens to also make it match
  the reference API's `list-messages` shape (previously it only matched
  on the POST endpoints).
- **No further springdoc/OpenAPI tooling changes** to work around the
  opaque-schema trade-off — accepted as-is, per `requests.http` already
  being the practical reference for concrete query shapes.
