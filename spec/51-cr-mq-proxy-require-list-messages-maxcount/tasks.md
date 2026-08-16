# Tasks — CR 51

1. [x] `mq-proxy`: require `filter.maxCount` (`> 0`) in
   `QueueController.listMessages()`, add
   `IllegalArgumentException → 400` handler
   (`GlobalExceptionHandler.kt`), update/add tests in
   `QueueControllerTest.kt`.
2. [x] `mq-proxy`: regenerate `openapi.yaml`; add `filter.maxCount` to
   the two `requests.http` `list-messages` examples that don't already
   have it.
3. [x] `tui`: `defaultBrowseMaxCount` + `withDefaultMaxCount()` helper
   in `messages.go`, wired into `load()` and `updateTitle()`; new/updated
   tests in `messages_test.go`.

## Manual verification

- `mq-proxy` via `curl`: `list-messages` with no `filter.maxCount` → 400;
  `filter.maxCount=0` → 400; `filter.maxCount=5` on a queue with more
  than 5 messages → exactly 5 returned.
- Real `tui` (proxy backend): open a queue with no filter applied —
  title shows `(filter: max=500)`, at most 500 rows shown. Open the
  filter form (`f`) — Max Count field is blank, not pre-filled. Apply an
  explicit `maxCount` — title/behavior reflect that value instead of the
  default.
- Confirm `d`/`m` (delete/move, marked-set and purge paths) unaffected —
  they don't go through `list-messages`.
- `task smoke:test` golden-path regression check.

**Verified 2026-08-14**, against a local `mq-proxy` instance
(`task dev:proxy:start`, `JAVA_HOME` pointed at a local JDK 21 install)
and the existing `local-mq-proxy` TUI connection, using the shared
`orders` scratch queue (seeded with 8 messages via `task seed:queue --
orders 8`, purged back to empty afterward):

- `curl`: `list-messages?sourceQueue=orders` (no `filter.maxCount`) →
  400 with body `{"error":"filter.maxCount is required and must be > 0"}`;
  `filter.maxCount=0` → 400; `filter.maxCount=-1` → 400;
  `filter.maxCount=3` on the 8-message queue → exactly 3 returned;
  `filter.maxCount=500` → all 8 returned.
- Real `tui` (`local-mq-proxy`, driven via tmux): opened `orders` with no
  filter set — title read `Messages — orders (filter: max=500)`, all 8
  messages shown. Opened the filter form (`f`) — Max Count field was
  blank. Typed `3`, Apply — title changed to `(filter: max=3)`, exactly 3
  rows shown. Reopened the form and hit Clear — title reverted to
  `(filter: max=500)`, all 8 rows shown again.
- Did not re-check `d`/`m`/purge by hand this round — unchanged code
  paths, not exercised by this CR, and already covered by `task
  smoke:test`/existing unit tests.
- `go build ./...`, `go vet ./...`, `go test ./...`, and `gofmt -l .`
  (clean) all pass on `tui`; `./gradlew test --tests
  "...QueueControllerTest"` passes on `mq-proxy`.
- Cleanup: `orders` purged back to 0 pending messages (matching its
  state before this session), `task dev:proxy:stop`, tmux session
  killed, scratch binary removed.
