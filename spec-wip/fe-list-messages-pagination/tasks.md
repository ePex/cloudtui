# Tasks

1. [x] `mq-proxy` DTO changes: `QueueMessageFilter.afterMessageId: String?
   = null`; `ListResponse.hasMore: Boolean = false` (shared across
   `list-queues`/`delete-messages`/`move-messages` too — unused,
   harmless default there).

2. [ ] `BrokerService.browseMessages`: extract the enumeration walk into
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

3. [ ] `QueueController.listMessages` passes `hasMore` through in its
   `ListResponse`. `QueueControllerTest`: `hasMore` present in the JSON
   response. Regenerate `mq-proxy/openapi.yaml` (`task openapi:proxy`).

4. [ ] `mq-proxy-web`: `buildListMessagesParams` gains an optional
   `opts.afterMessageId` nested into `filter.afterMessageId` (same
   pattern as `jmsType`/`maxCount`). New pure `appendMessages(existing,
   newPage)` (concat, no mutation). Unit tests for both.

5. [ ] `mq-proxy-web`: `state.messagesHasMore`; `loadMessages()` always
   starts a fresh first page (`state.messages` replaced,
   `state.messagesHasMore` set from the response); new
   `loadMoreMessages()` fetches with `afterMessageId` = the last
   currently-rendered row's `messageId`, appends via `appendMessages`,
   re-renders, updates `state.messagesHasMore`. A "Load more" button
   below the messages table, `hidden` unless `messagesHasMore` is
   `true`. Existing row-selection state (spec/21) is preserved across
   an append since it's keyed by `messageId`, not row index.

6. [ ] Manual end-to-end verification against a live `mq-proxy` + broker
   (spec/13 dev tooling): seed a queue with more messages than a small
   `maxCount`, confirm "Load more" appears and appends without losing
   already-checked selections, and disappears once exhausted; delete
   the cursor message then click an already-queued "Load more" to
   confirm the stale-cursor fallback returns a sane page instead of
   erroring; confirm the JMS Type filter still composes correctly with
   pagination (each page respects the same filter).

7. [ ] Merge-back: update `spec/11-mq-proxy-backend-integration` (drop
   the "no pagination/load more" line from its Out of scope, document
   `filter.afterMessageId`/`hasMore`) and `spec/21-amq-web-console`
   (document "Load more" in the message-list bullet) in place; delete
   `spec-wip/fe-list-messages-pagination/`. Mark the PR ready for
   review.
