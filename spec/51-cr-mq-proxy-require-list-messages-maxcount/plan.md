# Plan — CR 51

## Approach

1. **`QueueController.kt`**: in `listMessages()`, before calling
   `brokerService.browseMessages(...)`, validate:
   ```kotlin
   val maxCount = query.filter.maxCount
   require(maxCount != null && maxCount > 0) { "filter.maxCount is required and must be > 0" }
   ```
   (`require()` throws `IllegalArgumentException` with that message.) The
   `ListMessagesQuery`/`QueueMessageFilter` types themselves are
   unchanged — `maxCount` stays `Int?` on the shared DTO since
   `DeleteMessagesRequest`/`MoveMessagesRequest` reuse it unvalidated.
2. **`GlobalExceptionHandler.kt`**: add, above `handleGeneric`
   (matching the file's existing most-specific-first layout):
   ```kotlin
   @ExceptionHandler(IllegalArgumentException::class)
   @ResponseStatus(HttpStatus.BAD_REQUEST)
   fun handleIllegalArgument(ex: IllegalArgumentException): Map<String, String> =
       mapOf("error" to (ex.message ?: "Invalid request"))
   ```
3. **`QueueControllerTest.kt`**:
   - `listMessages returns 200 with message list` and
     `listMessages passes jmsType and messageId filters through`: add
     `&filter.maxCount=<N>` to the request URL and `maxCount = <N>` to
     the mocked `QueueMessageFilter(...)`.
   - New test `listMessages returns 400 when maxCount is missing`
     (request with `sourceQueue` only, no `filter.maxCount`).
   - New test `listMessages returns 400 when maxCount is not positive`
     (`filter.maxCount=0`).
4. **`openapi.yaml`**: `task openapi:proxy`.
5. **`requests.http`**: add `filter.maxCount=<N>` to the two
   `list-messages` examples that don't already have it (bare browse,
   jmsType-filtered browse); the third example (date-range) already
   includes `filter.maxCount=10`, unchanged.
6. **`tui/internal/app/messages.go`**:
   - Add `const defaultBrowseMaxCount = 500` near the top of the file.
   - Add `func withDefaultMaxCount(f queue.MessageFilter) queue.MessageFilter`:
     returns `f` unchanged if `f.MaxCount > 0`, otherwise returns a copy
     with `MaxCount: defaultBrowseMaxCount`.
   - `load()`: `filter := withDefaultMaxCount(mv.filter)` (replacing the
     current direct `filter := mv.filter`) before the `BrowseMessages`
     call.
   - `describeMessageFilter`: call sites in `updateTitle()` change from
     `describeMessageFilter(mv.filter)` to
     `describeMessageFilter(withDefaultMaxCount(mv.filter))`, so the
     title always reflects the effective cap. `describeMessageFilter`
     itself is unchanged (it already renders `max=N` whenever `N > 0`).
   - `mv.filter` is never assigned a defaulted value — `showMessageFilter`
     (pre-fills the form from raw `mv.filter.MaxCount`) and
     `parseMessageFilterForm`/`clearMessageFilter` are all unchanged.
7. **`tui/internal/app/messages_test.go`**:
   - New table-driven `TestWithDefaultMaxCount`: `0` → default, negative
     → default (defensive; `parseMessageFilterForm` already rejects
     negative input, but the helper shouldn't assume that), positive →
     unchanged.
   - Update `TestMessagesViewTitleUpdatesWithFilterAndSearch`'s first
     assertion (no filter set) from expecting
     `" Messages — orders "` to `" Messages — orders (filter: max=500) "`
     — this is the intended user-visible behavior change from this CR.

## Files touched

- `mq-proxy/src/main/kotlin/com/github/epex/mqproxy/api/QueueController.kt`
- `mq-proxy/src/main/kotlin/com/github/epex/mqproxy/api/GlobalExceptionHandler.kt`
- `mq-proxy/src/test/kotlin/com/github/epex/mqproxy/api/QueueControllerTest.kt`
- `mq-proxy/openapi.yaml` (regenerated)
- `mq-proxy/requests.http`
- `tui/internal/app/messages.go`
- `tui/internal/app/messages_test.go`
- `spec/51-cr-mq-proxy-require-list-messages-maxcount/tasks.md` (next gate)

## Key decisions

- **`IllegalArgumentException` + new handler, not `ResponseStatusException`
  directly.** `GlobalExceptionHandler` already has a catch-all
  `@ExceptionHandler(Exception::class)` returning 500 — since
  `ResponseStatusException` is-a `Exception`, throwing one directly would
  get caught by `handleGeneric` and turned into a 500, silently losing
  the intended 400. A dedicated handler (mirroring the `BindException`
  precedent from CR 49) is the pattern this file already uses for
  "specific validation failure → specific status."
- **Validation lives in the controller, not the shared DTO.** Making
  `QueueMessageFilter.maxCount` non-nullable would force it on
  `DeleteMessagesRequest`/`MoveMessagesRequest` too (compile-time, same
  type) — which the spec explicitly keeps optional. A `require()` check
  local to `listMessages()` is the minimal way to make it required for
  exactly one endpoint.
- **Default applied at the two use sites (`load()`, title), not written
  back into `mv.filter`.** Keeps "what the user actually set in the
  filter form" and "what's effectively sent/shown" distinguishable in
  the data model, even though nothing currently reads that distinction —
  it's what lets `showMessageFilter()`'s pre-fill logic (blank when
  unset) keep working unchanged. Considered seeding `mv.filter` with the
  default at view-open/clear time instead; rejected because it would
  make "user explicitly typed 500" and "user never touched the form"
  indistinguishable in the one place (`mv.filter`) that already
  distinguishes them today.
- **No unit test for `load()` itself.** It dispatches through a goroutine
  and `tview.QueueUpdateDraw`, and no existing test in this file exercises
  it directly (`fakeQueueBackend.BrowseMessages` in `queues_test.go`
  ignores its filter argument entirely) — consistent with the existing
  pattern, coverage here comes from unit-testing the pure
  `withDefaultMaxCount` helper plus manual verification below, not from
  a new async test harness.

## Manual verification

Per `tui/CLAUDE.md`, this touches queue/message-browsing behavior, so
verify against a real broker via the `verify-live` skill in addition to
unit tests:

- `task dev:proxy:start`. `curl` `list-messages` with no `filter.maxCount`
  → 400; with `filter.maxCount=0` → 400; with `filter.maxCount=5` on a
  queue with more than 5 messages → exactly 5 returned.
- Drive the real `tui` (proxy backend): open a queue with no filter
  applied — title shows `(filter: max=500)`, table shows at most 500
  rows. Open the filter form (`f`) — Max Count field is blank (not
  pre-filled with 500). Apply an explicit `maxCount` — title/behavior
  reflect the explicit value, not the default.
- Confirm `d`/`m` (delete/move, including the marked-set and
  purge-via-`PurgeQueue` paths) are unaffected — these don't go through
  `list-messages`' new validation.
- `task smoke:test` for a golden-path regression check.
