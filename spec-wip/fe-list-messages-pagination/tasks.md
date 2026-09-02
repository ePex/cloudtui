# Tasks

1. [x] `mq-proxy` DTO changes: `QueueMessageFilter.afterMessageId: String?
   = null`; `ListResponse.hasMore: Boolean = false` (shared across
   `list-queues`/`delete-messages`/`move-messages` too — unused,
   harmless default there).

2. [x] `BrokerService.browseMessages`: extract the enumeration walk into
   `doBrowse` (browser, filter, returnBody, afterMessageId) returning
   the page plus whether the cursor was found; skip messages up to and
   including a match on `afterMessageId`, collect up to `maxCount`
   after it, set `hasMore` when at least one more message remains. If
   the cursor isn't found in a full pass (stale/gone), open a second
   `QueueBrowser` and re-run with `afterMessageId = null` (silent
   restart from page one). `BrokerServiceTest`: first page unchanged;
   a mid-queue cursor returns the right remainder; a cursor equal to
   the last message returns an empty page with `hasMore=false`; a
   stale cursor falls back to page one; `hasMore` correct exactly at
   the `maxCount` boundary.

3. [x] `QueueController.listMessages` passes `hasMore` through in its
   `ListResponse`. `QueueControllerTest`: `hasMore` present in the JSON
   response. Regenerate `mq-proxy/openapi.yaml` (`task openapi:proxy`).

4. [x] `mq-proxy-web`: `buildListMessagesParams` gains an optional
   `opts.afterMessageId` nested into `filter.afterMessageId` (same
   pattern as `jmsType`/`maxCount`). New pure `appendMessages(existing,
   newPage)` (concat, no mutation). Unit tests for both.

5. [x] `mq-proxy-web`: `state.messagesHasMore`; `loadMessages()` always
   starts a fresh first page (`state.messages` replaced,
   `state.messagesHasMore` set from the response); new
   `loadMoreMessages()` fetches with `afterMessageId` = the last
   currently-rendered row's `messageId`, appends via `appendMessages`,
   re-renders, updates `state.messagesHasMore`. A "Load more" button
   below the messages table, `hidden` unless `messagesHasMore` is
   `true`. Existing row-selection state (spec/21) is preserved across
   an append since it's keyed by `messageId`, not row index.

   `apiCall` and `parseEnvelope` needed a small extension not called
   out explicitly in `plan.md`: `apiCall` previously resolved with just
   `envelope.data`, discarding `hasMore` (a sibling field, not nested
   under `data`) entirely — so `hasMore` is now attached onto the
   resolved `data` itself (harmless on any shape, array or object; a
   no-op wherever the server doesn't send it) rather than growing
   `apiCall`'s return contract for every other call site to ignore.

   Verified live end-to-end: seeded 5 messages, set max count to 2,
   confirmed "Load more" appears, selected one message, clicked "Load
   more" twice and confirmed the selection survived both appends while
   new rows arrived unselected, and confirmed the button correctly
   disappears once all 5 are loaded. Also independently confirmed via
   `curl` against the live broker: page 1 (`hasMore=true`), page 2 via
   `afterMessageId` (`hasMore=true`), and the stale-cursor fallback
   (an unknown `afterMessageId` returns all 5 from the start).

   **Testing gotcha, not a code bug**: mid-session, a stray static file
   server from an earlier task's manual testing was still squatting on
   port 8085, silently serving the *old* pre-pagination `mq-proxy-web/`
   from a different checkout — the new server never actually started,
   so every request looked "successful" but returned stale files.
   Separately, even after fixing that, the browser's own HTTP cache
   kept serving an old cached `app.js` across normal navigations
   (`python -m http.server` sends no cache-control headers, so Chrome's
   heuristic caching applies) — a cache-busting query string on
   `index.html`'s URL doesn't affect the separately-cached `app.js` it
   loads via `<script src>`; a real hard reload (which bypasses cache
   for every subresource of the navigation, not just the document) was
   what actually fixed it. Worth remembering for the next live-testing
   round: check for stray old servers first, and hard-reload (not just
   re-navigate) after any `app.js` change.

6. [x] Manual end-to-end verification against a live `mq-proxy` + broker
   (spec/13 dev tooling): seed a queue with more messages than a small
   `maxCount`, confirm "Load more" appears and appends without losing
   already-checked selections, and disappears once exhausted; delete
   the cursor message then click an already-queued "Load more" to
   confirm the stale-cursor fallback returns a sane page instead of
   erroring; confirm the JMS Type filter still composes correctly with
   pagination (each page respects the same filter).

   **JMS Type filter + pagination**: seeded 5 messages alternating
   `order-created`/`other`/`order-created`/`other`/`order-created`;
   filtering to `order-created` with max count 2 correctly returned
   only the 2 matching messages for page 1 (skipping `other` entirely
   — the JMS selector applies at the broker level, before pagination),
   and "Load more" correctly fetched the 3rd matching message as page
   2, still excluding both `other` messages, then hid itself. Confirmed
   via `curl` first, then via the actual UI.

   **Stale-cursor fallback, driven through the real UI (not just
   curl)**: loaded page 1 (max count 2, no filter) so the last rendered
   row's `messageId` became the pending "Load more" cursor, deleted
   that exact message directly via `mq-proxy`'s API (simulating a
   concurrent purge/consume racing the page), then clicked the
   already-queued "Load more". No error surfaced, matching the design
   — but since the fallback silently restarts from page one and the UI
   *appends* rather than replaces, the result includes a **visible
   duplicate of the first message and the already-deleted cursor
   message still shown** (stale, since appending never re-validates
   already-rendered rows). This is the accepted trade-off spec.md
   already named (simplicity over reconciliation — no error, but no
   attempt to dedupe/re-sync an in-flight append either) rather than a
   bug to fix here; recorded explicitly so it isn't rediscovered as a
   surprise later.

7. [ ] Merge-back: update `spec/11-mq-proxy-backend-integration` (drop
   the "no pagination/load more" line from its Out of scope, document
   `filter.afterMessageId`/`hasMore`) and `spec/21-amq-web-console`
   (document "Load more" in the message-list bullet) in place; delete
   `spec-wip/fe-list-messages-pagination/`. Mark the PR ready for
   review.
