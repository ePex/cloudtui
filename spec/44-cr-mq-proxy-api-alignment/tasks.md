# Tasks — CR 44

1. [x] `mq-proxy`: envelope types (`ApiError`, `ListResponse<T>`,
   `ItemResponse<T>`) and DTO changes — `QueueSummary` field rename +
   `producerCount`; `MessageSummary` reshape (`sourceQueue`, `messageId`,
   `jmsType`, `body`, `timestamp`, `headers`); remove `MessageDetail`;
   add `QueueMessageFilter`, `DeleteMessagesRequest`,
   `MoveMessagesRequest`, `SendMessageRequest`, and the response DTOs.
2. [x] `mq-proxy`: `BrokerService` changes — real `jmsType` on
   `toSummary()`, `producerCount` read from the stats-plugin reply,
   selector-based `deleteMessages`/`moveMessages` with `maxCount`
   enforcement; remove `getMessage`, single-item `deleteMessage`/
   `moveMessage`, `moveAll`, and the now-dead `NotFoundException`.
   (Deviation from plan: `GlobalExceptionHandler` was kept, minus only
   the `NotFoundException` handler — its `JMSException`/generic-`Exception`
   handlers are still needed and unrelated to the single-item removal.)
3. [x] `mq-proxy`: `QueueController` — the five new
   `/api/management/command/*` routes wired to task 2's service methods;
   remove the old resource-style routes.
4. [x] `mq-proxy`: regenerate `openapi.yaml`; hand-rewrite
   `requests.http` for the new endpoints.
5. [x] `mq-proxy`: update `BrokerServiceTest.kt` and
   `QueueControllerTest.kt` for the new DTOs/routes/envelope; delete the
   test cases for removed operations. `./gradlew test` and `bootJar`
   both green.
6. [ ] `tui`: `queue.Backend` — add `MessageFilter`, `Summary.ProducerCount`,
   `DeleteMessages`/`MoveMessages` methods (existing methods' signatures
   unchanged); update `fakeQueueBackend` in
   `internal/app/queues_test.go` to satisfy the grown interface.
7. [ ] `tui`: rewrite `internal/queue/proxy` against the new `mq-proxy`
   shape — envelope unwrapping, renamed DTOs, existing `Backend` methods
   reimplemented as thin wrappers over the new filtered endpoints, new
   `DeleteMessages`/`MoveMessages`. Update `proxy_test.go`.
8. [ ] `tui`: `internal/queue/jolokia` — add the pure `filterMessages`
   helper and `DeleteMessages`/`MoveMessages` (client-side filtering over
   existing `BrowseMessages`/`RemoveMessage`/`MoveMessage`). Update
   `jolokia_test.go`.
9. [ ] Manual verification (per `plan.md`'s Testing section) — start
   `mq-proxy` from source, confirm existing TUI operations (list/browse/
   send/single delete/single move/purge) still work against both
   backends, then exercise a filtered delete/move directly against each
   backend with a mix of `jmsType`s. Record what was checked here once
   done.
