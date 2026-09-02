# Plan

## Approach

Add a `loadQueues()` call alongside the existing `loadMessages()` (or,
for the single-message handlers, the `showView('messagesView')` +
`loadMessages()` pair) in each of the four message-level handlers, on
the same success path that already refreshes the message list. This
mirrors the pattern `purgeQueue`/`moveAllMessages` already use for the
two queue-list-level actions.

`loadQueues()` runs unconditionally alongside `loadMessages()`, not
gated on move vs. delete or on which messages were selected — simplest
correct behavior, and consistent with how the existing queue-list-level
actions already treat purge/move-all as always affecting counts.

## Files touched

- `mq-proxy-web/app.js`:
  - `deleteSelectedMessagesBtn` handler: success callback becomes
    `function () { loadMessages(); loadQueues(); }` instead of the bare
    `loadMessages` reference.
  - `moveSelectedMessagesBtn` handler: same — `.then(loadMessages)`
    becomes `.then(function () { loadMessages(); loadQueues(); })`.
  - `deleteMessageBtn` handler: its existing success callback
    (`showView('messagesView'); loadMessages();`) gains `loadQueues();`.
  - `moveMessageBtn` handler: same addition to its existing success
    callback.

No new pure functions are introduced — `loadQueues()` already exists
and already does exactly what's needed (refetch `list-queues`, update
`state.queues`, re-render whenever the queue list is next shown).

## Testing

All four call sites are DOM event-handler wiring, not pure functions —
`app.js`'s existing test coverage is scoped to pure logic only (see
spec/21's Implementation notes: "DOM rendering/click-handling is not
[unit tested]"), so this change doesn't add new `app.test.js` cases,
consistent with that established boundary. Verification is manual,
listed in `tasks.md`.
