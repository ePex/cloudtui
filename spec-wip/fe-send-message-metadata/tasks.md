# Tasks

1. [ ] **Widen the `Backend.SendMessage` interface and wire up mq-proxy +
   plumbing.** Add `queue.SendMessageRequest` and change
   `Backend.SendMessage`'s signature (`internal/queue/backend.go`).
   Update `proxy.Client.SendMessage` (`internal/queue/proxy/proxy.go`) to
   populate `sendMessageRequest`'s existing `Headers`/`GroupID`/
   `CorrelationID` fields from the new request (straightforward 1:1
   passthrough — no precedence logic needed here, mq-proxy's own
   filtering is task 3). Update the `secretbackend` passthrough decorator
   (`internal/queue/secretbackend/secretbackend.go`), the seed tool
   (`internal/seed/seed.go`, hardcoding `JMSType: "text"` as its only
   set field, matching today's implicit default) and its test, and every
   fake backend (`internal/dialog/dialogtest_test.go`,
   `internal/app/host_test.go`, `internal/view/queues_test.go`) to the
   new signature. Jolokia's `SendMessage` (`internal/queue/jolokia/mutate.go`)
   gets its call signature updated too, but keeps building an empty
   headers map for now (unchanged behavior) — its real field-mapping
   logic is task 2, kept separate so this task is pure plumbing.
   Update `proxy_test.go`/`secretbackend_test.go` for the new fields.
   Everything compiles, existing tests pass, mq-proxy backend sending is
   now feature-complete end to end (Jolokia isn't yet).

2. [ ] **Jolokia: build the headers map with the reserved-key precedence
   rule.** In `internal/queue/jolokia/mutate.go`, replace the hardcoded
   empty map with one built from `req.Headers` (reserved keys `JMSType`/
   `JMSCorrelationID`/`JMSXGroupID` dropped) plus the three dedicated
   fields set afterward, per `plan.md`. Unit tests in `mutate_test.go`:
   `JMSType` always present, `JMSCorrelationID`/`JMSXGroupID` only when
   non-empty, non-reserved custom headers pass through unchanged, and a
   custom header named after a reserved key is dropped rather than
   applied.

3. [ ] **mq-proxy: fix `BrokerService.sendMessage`'s header precedence.**
   In `mq-proxy/src/main/kotlin/.../service/BrokerService.kt`, filter the
   same three reserved keys out of `request.headers` before applying them
   (currently applied last, so a colliding header wins today — the fix
   this feature's contract requires, per `plan.md`). Update
   `BrokerServiceTest.kt`'s existing `sendMessage sets jmsType,
   correlationId, groupId, and custom headers` test if the change affects
   its mock expectations, and add a new case asserting a `headers` entry
   named `JMSXGroupID` (or `JMSType`/`JMSCorrelationID`) never reaches
   `setStringProperty` with that key — only the dedicated-field call does.

4. [ ] **Rebuild the send overlay UI.** In `internal/dialog/sendmessage.go`,
   replace the `TextArea`+`tview.List` with a `tview.Form` (JMS Type,
   Correlation ID pre-filled via a new `newCorrelationID()` UUID
   generator, Group ID, a `key: value`-per-line Headers `TextArea`, the
   existing Body `TextArea` added via `Form.AddFormItem`, Submit/Cancel
   buttons), per `plan.md`. Submit validation: blank JMS Type or Body
   blocks sending with a status-bar error and keeps the overlay open
   (mirroring `MessageFilter.apply`'s pattern); a malformed Headers line
   (no `:`) does the same; a blank Correlation ID at submit time is
   regenerated rather than sent blank. `ApplyPalette` updated for the new
   form fields. Unit tests for the pure helpers: `newCorrelationID`
   (UUID v4 shape, no two calls equal), the headers-line parser (valid
   lines, blank lines skipped, malformed line errors), and the
   submit-validation rule — the same directly-testable-helper pattern
   `QueuesView.doPurge`/`doMoveAll` and `MessageFilter.handleScanResult`
   already use, since the overlay's tview wiring itself stays covered by
   live verification (task 5), not unit tests, matching spec/09's existing
   note that `SendMessageOverlay` has no dedicated tview-level test file.

5. [ ] **Live-verify against both backends** (`verify-live` skill) and
   record what was checked here:
   - [ ] Jolokia: send with JMS Type, Correlation ID, Group ID, and a
     custom header set; open the message in the detail view (spec/08)
     and confirm JMS Type/Correlation ID show the entered values, and the
     raw fields show the group ID and custom header as real message
     properties — the step that confirms the ActiveMQ `sendTextMessage`
     `Map` argument behaves the way `plan.md` inferred (not previously
     verified in this repo).
   - [ ] mq-proxy (`task dev:proxy:start`): repeat the same send-and-verify.
   - [ ] Both backends: send with only JMS Type + Body (everything else
     blank) — confirm it still sends, Correlation ID shows the
     auto-generated UUID, and no Group ID/custom properties appear.
   - [ ] Both backends: send with a custom header literally named
     `JMSXGroupID` set to a different value than the Group ID field —
     confirm the message's real group comes from the Group ID field, not
     the header, proving the "dedicated field always wins" contract holds
     against a real broker, not just mocked unit tests.
   - [ ] Attempt to submit with a blank JMS Type, and with a malformed
     Headers line — confirm the overlay stays open with a status-bar
     error each time.
