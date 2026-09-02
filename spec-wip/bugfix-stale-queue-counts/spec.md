# Bugfix: queue list shows stale counts after message-level move/delete

Date: 2026-09-02

## Problem

`mq-proxy-web`'s queue list (`list-queues`: pending/consumer/enqueue/
dequeue/producer counts, spec/21) only refreshes when `loadQueues()`
runs. Today that happens on connect, on the manual "Refresh" button, and
already correctly after the two queue-list-level drain actions (Purge,
Move all…) — both call `loadQueues` on success.

It does **not** happen after any of the four message-level move/delete
actions, reached by opening a queue and working with its message list:

- Delete selected (bulk, `deleteSelectedMessagesBtn`)
- Move selected… (bulk, `moveSelectedMessagesBtn`)
- Delete (single, message detail view's `deleteMessageBtn`)
- Move… (single, message detail view's `moveMessageBtn`)

Each of these already calls `loadMessages()` afterward, so the message
list itself is correct — but `state.queues` (what the queue list renders
from) goes stale. Navigating back to the queue list shows outdated
pending/enqueue/dequeue counts until the user manually clicks "Refresh".

## Fix

After each of the four message-level handlers above successfully
completes its `delete-messages`/`move-messages` call, also refresh
`state.queues` in the background (`loadQueues()`), the same way Purge
and Move all… already do — so by the time the user navigates back to
the queue list, it's already current. No visible loading state is
needed for this background refresh since the queue list isn't the
active view when it happens.

## Scope

- `mq-proxy-web/app.js`: the four handlers listed above gain a
  `loadQueues()` call alongside their existing `loadMessages()`/
  navigation-back call.

## Out of scope

- No change to the two queue-list-level actions (Purge, Move all…) —
  already correct.
- No polling/auto-refresh while idly viewing the queue list — this is
  only about invalidating the cached counts after an action that's
  known to have changed them.
- No optimistic/local count adjustment — a real `loadQueues()` refetch,
  consistent with the existing Purge/Move all… pattern.
