# `list-messages` pagination ("Load more") in the AMQ web console

Date: 2026-09-02

## Purpose

Let `mq-proxy-web` (spec/21) browse a queue's messages beyond the current
`maxCount` cap. Today, `list-messages` (spec/11) fetches at most
`maxCount` messages from the start of the queue and stops — there is no
way to see what comes after that, short of raising `maxCount` and
re-fetching everything from scratch. This reverses spec/11's explicit
"no pagination/load more" decision, which was reasonable at the time
(nothing needed it yet) but is now a real gap for queues with more
messages than a sane single-page cap.

This is a `mq-proxy` wire-contract change (spec/11) plus a
`mq-proxy-web` change (spec/21) — not a `mq-proxy-web`-only feature,
since `list-messages` has no offset/cursor concept to page over today.

## Scope

- **Cursor shape: message-ID-based, not a numeric offset.** `mq-proxy`'s
  `browseMessages` walks a JMS `QueueBrowser`'s `Enumeration` — there is
  no random-access/skip API, so either scheme costs the same O(n) walk
  per page. A message-ID cursor ("give me messages after message X") is
  chosen over a numeric offset because it degrades better if the queue
  changes between page fetches: an offset silently skips or repeats
  messages when anything is added/removed/consumed before the current
  position; a message-ID cursor only misses/duplicates around the cursor
  message itself if that specific message is gone by the next fetch —
  correct everywhere else. Neither can be a fully stable snapshot across
  separate `browse()` calls (a `QueueBrowser` doesn't provide one), so
  this is about minimizing the blast radius of that inherent limitation,
  not eliminating it.
- **New request field**: `filter.afterMessageId` (nested, matching the
  existing `filter.*` fields — `jmsType`, `messageId`, `fromDate`,
  `toDate`, `maxCount`). Distinct from the existing `filter.messageId`
  (which means "return only this one message") — reusing that field for
  a different meaning would be a breaking, confusing overload.
  `afterMessageId` unset behaves exactly as today (start from the
  beginning); set, the server skips every message up to and including a
  match, then returns up to `maxCount` messages starting after it. A
  cursor message no longer present on the queue is treated as "start
  from the beginning" (silently, not an error) — a page racing a
  concurrent purge/consume is expected to happen and shouldn't surface
  as a hard failure to a non-technical user.
- **New response field**: `hasMore: Boolean` on the `list-messages`
  response, indicating whether at least one more message exists beyond
  the returned page (i.e. the browser found a message past the
  `maxCount`th one before stopping). `mq-proxy`'s `ListResponse<T>` is
  shared with `list-queues` today; adding `hasMore` there is harmless for
  that endpoint (unused, defaults `false`) rather than forking a
  message-specific response type.
- **`mq-proxy-web`**: a "Load more" button below the messages table,
  shown only when the last response's `hasMore` was `true`. Clicking it
  fetches the next page (`filter.afterMessageId` = the last row's
  message ID) and **appends** to the currently-rendered list (not a
  replace) — messages already on screen (and their checkbox selection
  state, spec/21) stay untouched. Any of the following resets back to a
  single first page: clicking Apply, opening a different queue, or a
  bulk/single delete-move action completing (all of which already
  trigger a full reload today).

## Out of scope (deliberate)

- No page-number-based navigation (jump to page 5) — "Load more" only,
  matching the cursor's forward-only nature.
- No change to the TUI (spec/07/08) — it doesn't have this problem in
  the same way (it can already re-browse with a different `maxCount`
  from its own UI, and this is explicitly an AMQ web console gap being
  closed, not a TUI one).
- No porting to the reference API (spec/11) — out of scope for this
  change, same as the CORS work was initially (spec/21); a candidate
  follow-up if/when that service needs to serve the web console for
  real, tracked separately.
- No change to `filter.maxCount`'s existing behavior (still required and
  positive on every call, per spec/11) — pagination composes with it
  (each page is still capped at `maxCount`), not a replacement for it.

## Data & config

No new config. Wire contract change only: `filter.afterMessageId`
(request), `hasMore` (response) on `list-messages`.
