# Tasks

1. [ ] `mq-proxy-web/app.js`: add a `loadQueues()` call to the success
   path of all four message-level move/delete handlers —
   `deleteSelectedMessagesBtn`, `moveSelectedMessagesBtn`,
   `deleteMessageBtn`, `moveMessageBtn` — alongside their existing
   `loadMessages()` (or `showView('messagesView'); loadMessages();`
   for the two single-message handlers). No new pure functions; no new
   `app.test.js` cases (DOM-wiring-only, see plan.md).

   **Manual verification** (`file://` or served, spec/21) against a
   live `mq-proxy` + broker:
   - Open a queue, select one or more messages, "Delete selected" (or
     single-message Delete from detail view) — confirm, then navigate
     back to the queue list without clicking "Refresh" and confirm its
     counts already reflect the deletion.
   - Same for "Move selected…" / single-message "Move…" — confirm the
     queue list's counts for *both* the source and target queue are
     current without a manual refresh.
   - Confirm Purge and Move all… (queue-list-level, already correct)
     still work as before — no regression.

2. [ ] Merge-back, done together with `fe-list-messages-pagination`'s
   own task 7 in a single commit (per the user's request to merge both
   back as one thing): update `spec/21-amq-web-console` to note that
   move/delete — at both the message level and the queue-list
   Purge/Move-all level — refresh the queue list's counts, then delete
   `spec-wip/bugfix-stale-queue-counts/` alongside
   `spec-wip/fe-list-messages-pagination/`.
