# Tasks

1. [x] `mq-proxy-web/app.js`: add a `loadQueues()` call to the success
   path of all four message-level move/delete handlers —
   `deleteSelectedMessagesBtn`, `moveSelectedMessagesBtn`,
   `deleteMessageBtn`, `moveMessageBtn` — alongside their existing
   `loadMessages()` (or `showView('messagesView'); loadMessages();`
   for the two single-message handlers). No new pure functions; no new
   `app.test.js` cases (DOM-wiring-only, see plan.md).

   `mq-proxy-web`'s unit suite still passes in full (43/43) — this
   change touches only DOM-wired click handlers, none of the tested
   pure functions.

   **Manual verification** (`file://`, spec/21) against a live
   `mq-proxy` + broker: user confirmed working — the queue list's
   counts are current after message-level delete/move without needing
   the manual "Refresh" button.

2. [ ] Merge-back, done together with `fe-list-messages-pagination`'s
   own task 7 in a single commit (per the user's request to merge both
   back as one thing): update `spec/21-amq-web-console` to note that
   move/delete — at both the message level and the queue-list
   Purge/Move-all level — refresh the queue list's counts, then delete
   `spec-wip/bugfix-stale-queue-counts/` alongside
   `spec-wip/fe-list-messages-pagination/`.
