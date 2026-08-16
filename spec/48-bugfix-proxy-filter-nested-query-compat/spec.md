# Bugfix 48 — proxy backend's filter form is silently ignored by the reference API

Date: 2026-08-13

## What

`tui`'s proxy backend client (`browseQuery`, `tui/internal/queue/proxy/proxy.go`,
added in FE 46) sends filter fields — `jmsType`, `messageId`, `fromDate`,
`toDate`, `maxCount` — as flat top-level query params on `list-messages`.
This matches `mq-proxy`'s own (committed, CR 45) query shape, but the
reference queue-management API expects those same fields **nested**
under `filter.` (`filter.jmsType`, `filter.maxCount`, etc.) — a
divergence already documented in FE 46's spec as intentional, since
`mq-proxy` is meant to become the reference implementation going
forward (CR 44).

In practice, the user hit this immediately: connected to the reference
API (`local-other-proxy`), set Max Count to 1 in the filter form, and got
every message back — the flat `maxCount` param is silently unbound by
the reference API's nested-object query binding.

## Why

The reference service isn't ours to change, and hasn't been migrated to
`mq-proxy`'s shape yet. Rather than leave filtering non-functional
against it (the "known limitation" option), send both shapes.

**Confirmed live, both directions, no errors**:
- Reference API: flat `maxCount=1` alone → all messages (ignored).
  Nested `filter.maxCount=1` alone → correctly limited to 1. Both sent
  together (`maxCount=1&filter.maxCount=1`) → correctly limited to 1
  (nested wins, flat extra param is silently ignored — ordinary Spring
  `@RequestParam`/`@ModelAttribute` behavior, no strict param
  validation).
- `mq-proxy`'s own `list-messages` (`QueueController.kt`) binds named
  `@RequestParam`s individually; an unrecognized extra `filter.maxCount`
  param is likewise silently ignored, not an error.

So sending both shapes in one request works against either backend
without needing to detect which one is on the other end.

## Scope

`tui/internal/queue/proxy/proxy.go`, `browseQuery`: for each of the five
filter fields (`jmsType`, `messageId`, `fromDate`, `toDate`, `maxCount`),
add the corresponding `filter.<field>` query param alongside the
existing flat one, whenever that field is set. `sourceQueue` and
`returnBody` are unaffected — both APIs treat them as top-level already
(confirmed in FE 46's investigation).

## Out of scope

- **`delete-messages`/`move-messages`** (POST, JSON body) are unaffected
  — they're not part of this bug (the reference API's filter DTO there
  is nested *inside the JSON body*, i.e. `{"filter": {...}}`, which
  `mq-proxy`'s own `DeleteMessagesRequest`/`MoveMessagesRequest` already
  match structurally — CR 44 confirmed this alignment, this bugfix only
  touches the GET query-string case).
- **No change to `mq-proxy` or the reference API themselves.**
