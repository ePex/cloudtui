# CR 45 — Extend mq-proxy's `list-messages` filtering to match the reference API

Date: 2026-08-13

## What

Extend `mq-proxy`'s `/api/management/command/list-messages` endpoint to
accept the same filter criteria (`fromDate`, `toDate`, `maxCount`) that
`delete-messages`/`move-messages` already accept, plus an optional
`returnBody` flag to skip payload transfer when only metadata is needed.
Today `list-messages` only accepts `jmsType` and `messageId` — a subset
of the shared `QueueMessageFilter` type.

## Why

Re-checked `mq-proxy`'s API against the reference queue-management
API's OpenAPI doc (CR 44 follow-up) and found `list-messages` lags
behind the parity CR 44 established for `delete-messages`/
`move-messages`: the reference API's list endpoint takes a full filter
(`jmsType`, `messageId`, `fromDate`, `toDate`, `maxCount`) plus a
`returnBody` toggle; ours only takes `jmsType`/`messageId`.

This is mostly a missing-plumbing gap, not missing capability:
`QueueController.listMessages()` only extracts `sourceQueue`, `jmsType`,
and `messageId` from the query string, but `BrokerService.browseMessages`
already builds its JMS selector from `fromDate`/`toDate` via
`QueueMessageFilter.toSelector()` — those two fields already work
end-to-end for `deleteMessages`/`moveMessages` and would work for
`browseMessages` too if the controller passed them through. `maxCount`
is the one field that needs real logic added: `browseMessages` has no
cap loop today (unlike `deleteMessages`/`moveMessages`, which loop
`while (filter.maxCount == null || <consumed>.size < filter.maxCount)`).
`returnBody` doesn't exist anywhere yet.

**No proprietary details from the reference service** are recorded here
(see the "Sensitive data" precedent from CR 44) — only the generic shape
of its `list-messages` query surface (field names or a filtering/paging
convention) is referenced, since that shape is ordinary REST design, not
business-specific.

## Scope

Changes to `mq-proxy` (Kotlin/Spring Boot):

1. `QueueController.listMessages()` (`api/QueueController.kt`): accept
   `fromDate`, `toDate`, `maxCount` as additional optional query params,
   alongside the existing `sourceQueue`/`jmsType`/`messageId`, and pass
   them into the `QueueMessageFilter` built for `browseMessages`.
2. `BrokerService.browseMessages()`: add a `maxCount` cap on the browse
   loop, matching the `while (filter.maxCount == null || ...)` pattern
   already used in `deleteMessages`/`moveMessages`.
3. Add an optional `returnBody: Boolean?` query param (default
   `false`/omit-body, matching the reference API's semantics) that, when
   not `true`, skips populating `MessageSummary.body` — avoiding payload
   transfer for bulk/metadata-only listing. When absent, listing behaves
   exactly as today (body included), so this is additive, not breaking.
4. `mq-proxy/openapi.yaml` regenerated to reflect the new query surface.

## Out of scope

- **No `tui` changes.** `queue.Backend.BrowseMessages(ctx, queueName)`
  takes no filter today, and nothing in `tui` needs filtered/paged
  browsing yet — this CR only closes the server-side capability gap so
  `mq-proxy`'s surface matches the reference API. Wiring a filtered
  `BrowseMessages` (or a `returnBody`-aware variant) into `queue.Backend`
  and the Go proxy client is a candidate for a future `fe`/`cr` once a
  concrete TUI use case (e.g. paged/large-queue browsing) needs it.
- **No change to `deleteMessages`/`moveMessages`** — their filtering
  already matches the reference API per CR 44.
- **No change to header/property value typing** (`mq-proxy` stringifies
  JMS headers; the reference API preserves real types) — this is a
  known, deliberate divergence the `tui` client already tolerates
  (commit `ccb520b`), not something this CR addresses.
