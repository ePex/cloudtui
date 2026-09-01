# Implementation plan

## Approach

Same two-repo shape as spec/21's original CORS work: a `mq-proxy`
wire-contract change plus a `mq-proxy-web` change, one PR. The TUI's Go
client needs no code change — `json.NewDecoder(...).Decode(out)`
(`tui/internal/queue/proxy/proxy.go`) has no `DisallowUnknownFields()`,
so it silently ignores the new `hasMore` response field; confirmed by
reading the actual decode call before assuming this was safe.

## `mq-proxy` — cursor pagination

### DTO changes

- `QueueMessageFilter.kt`: add `afterMessageId: String? = null`.
- `ListResponse.kt`: add `hasMore: Boolean = false`. Shared with
  `list-queues`/`delete-messages`/`move-messages`, which all just get an
  always-`false`, unused field — simpler than forking a
  `list-messages`-specific response type for one boolean.

### `BrokerService.browseMessages`

Return type changes from `List<MessageSummary>` to a small internal
result carrying both the page and whether more exists:

```kotlin
private data class BrowseResult(val data: List<MessageSummary>, val hasMore: Boolean)
```

Walk logic (single JMS `QueueBrowser` enumeration pass):

```kotlin
private fun doBrowse(
    browser: QueueBrowser, queueName: String, filter: QueueMessageFilter,
    returnBody: Boolean, afterMessageId: String?,
): Pair<BrowseResult, Boolean> { // second value: was the cursor found?
    val enum = browser.enumeration
    var skipping = afterMessageId != null
    var foundCursor = afterMessageId == null
    val messages = mutableListOf<MessageSummary>()
    var hasMore = false
    while (enum.hasMoreElements()) {
        val msg = enum.nextElement() as? jakarta.jms.Message ?: continue
        if (skipping) {
            if (msg.jmsMessageID == afterMessageId) { skipping = false; foundCursor = true }
            continue
        }
        if (filter.maxCount != null && messages.size >= filter.maxCount) { hasMore = true; break }
        messages += msg.toSummary(queueName, returnBody)
    }
    return BrowseResult(messages, hasMore) to foundCursor
}
```

`browseMessages` calls `doBrowse` once with `filter.afterMessageId`; if
`foundCursor` comes back `false` (the cursor message is no longer on the
queue — a page racing a concurrent purge/consume), it opens a **second**
`QueueBrowser` and re-runs `doBrowse` with `afterMessageId = null`,
i.e. silently restarts from the beginning per the spec's stale-cursor
behavior. Two full passes only happen in that (rare) case; the common
case is one pass, same cost as today.

### `QueueController`

`listMessages` returns `ListResponse(data = result.data, hasMore =
result.hasMore)` instead of the current bare `data =`.

### Tests

- `BrokerServiceTest`: first-page (no cursor) still works identically;
  a cursor mid-queue returns the right remainder; a cursor equal to the
  last message returns an empty page with `hasMore=false`; a stale/
  unknown cursor falls back to page one; `hasMore` is `true`/`false`
  correctly at the boundary (exactly `maxCount` remaining vs. one more).
- `QueueControllerTest`: `hasMore` passed through in the JSON response.
- `mq-proxy/openapi.yaml` regenerated via `task openapi:proxy` after the
  DTO changes land (spec/10's documented process for an API-visible
  change).

## `mq-proxy-web` — "Load more"

### State

- `state.messagesHasMore` (bool) — from the last `list-messages`
  response.
- No separate "cursor" field needed in state: the cursor for the next
  page is always the **last currently-rendered message's `messageId`**,
  read directly off `state.messages` when "Load more" is clicked, rather
  than tracked as separate mutable state that could drift out of sync
  with what's actually rendered.

### Pure functions (`app.js`, unit tested)

- `buildListMessagesParams(sourceQueue, opts)` gains an optional
  `opts.afterMessageId`, nested into `filter.afterMessageId` — same
  pattern as the existing `jmsType`/`maxCount` options.
- `appendMessages(existing, newPage)` — simple concat, pulled out as its
  own pure function only because `loadMessages`/`loadMoreMessages` both
  need the identical "existing + new, then re-render" step and it's
  worth one shared, testable place for that.

### Behavior

- `loadMessages()` (Apply, opening a queue, or any reload after an
  action) always starts a **fresh** first page: `afterMessageId`
  omitted, `state.messages` replaced (not appended), `state.messagesHasMore`
  set from the response.
- New `loadMoreMessages()`: calls `list-messages` with the same current
  filter/maxCount plus `afterMessageId` = last row's `messageId`,
  appends the result to `state.messages` via `appendMessages`, updates
  `state.messagesHasMore`, re-renders (existing row selection, spec/21,
  is preserved since `renderMessages` already reads selection state from
  `state.selectedMessageIds` by message ID, not by row index).
- A **"Load more"** button below the messages table, `hidden` unless
  `state.messagesHasMore` is `true` after the last load — reuses the
  same `[hidden]` pattern already in place (spec/21's gotcha about
  `display`-setting elements needing an explicit `[hidden]` override
  doesn't apply here, since this button doesn't set its own `display`).

### Tests

- Unit: `buildListMessagesParams` with `afterMessageId` nests correctly;
  `appendMessages` concatenates without mutating either input array.
- Manual (no DOM test runner, matching spec/21's existing testing
  approach): seed a queue with more messages than a small `maxCount`,
  confirm "Load more" appears, appends without losing already-selected
  checkboxes, and disappears once the queue is exhausted; confirm a
  stale-cursor page (delete the cursor message, then click an
  already-queued "Load more") still returns a sane page instead of
  erroring.

## Data & config

No new config on either side. Wire contract changes only.
