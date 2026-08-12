# CR 44 — Align mq-proxy's REST API with the reference queue-management API

Date: 2026-08-12

## What

Rework `mq-proxy`'s REST API — routing style, response envelope, and
request/response DTOs — to match the shape of an existing internal
queue-management API the user's organization already runs in production,
so that `mq-proxy`'s contract and that live system's contract converge.
`tui`'s proxy backend client is updated to speak the new shape.

This is a wire-contract change, not a UI feature: no new screens or
keybindings are added to the TUI as part of this CR (see Out of scope).

## Why

The user compared `mq-proxy`'s current API against a live internal
queue-management service with a similar purpose and found the two have
diverged in both routing conventions and capability (notably: that
service supports filtered, criteria-based bulk delete/move; `mq-proxy`
only supports single-message or whole-queue operations). The intent is to
make `mq-proxy` the reference implementation going forward — once this CR
ships, the live service will be changed to match `mq-proxy`'s (updated)
API instead of the other way around.

**No proprietary details from the live service** (its real name, its
non-queue-related endpoints, internal business terminology) are recorded
in this repo — see the "Sensitive data" note this session's cleanup
established. Only the generic, technical shape of its queue/message
management endpoints (routing style, DTO field names, an envelope
pattern) is referenced below, since that shape is ordinary REST/JMS
design, not business-specific.

## Scope

Changes to `mq-proxy` (Kotlin/Spring Boot):

1. **Routing style**: move from resource-style paths
   (`/api/queues/{name}/messages/...`) to command-style paths under
   `/api/management/command/<verb>` (e.g. `list-queues`, `list-messages`,
   `send-message`, `delete-messages`, `move-messages`).
2. **Response envelope**: wrap responses as `{ data, errors }` (or
   `{ data, error }` for single-result operations), rather than returning
   bare arrays/objects with errors only as HTTP status codes.
3. **`list-queues`**: add `producerCount` to the queue summary shape
   alongside the existing `name`/pending/consumer/enqueue/dequeue counts.
4. **`list-messages`**: support optional filtering (by JMS type, message
   ID) instead of an unfiltered full browse only; include `jmsType` in
   the returned message shape.
   **This requires a real fix, not just a DTO reshape**: `MessageSummary`/
   `MessageDetail` (`mq-proxy/src/main/kotlin/.../api/model/`) don't carry
   a `jmsType` field today, and `BrokerService`'s `toSummary()`/
   `toDetail()` extension functions (`BrokerService.kt`) never read
   `jakarta.jms.Message.jmsType` — the actual JMS `JMSType` header a
   producer may set — when building these DTOs. Add it. Without this,
   `jmsType`-based filtering would have nothing real to filter on.
5. **`send-message`**: accept a structured DTO (`targetQueue`, `jmsType`,
   `headers`, `groupId`, `body`, `correlationId`) instead of an untyped
   JSON blob — giving callers control over JMS metadata that's currently
   inexpressible.
6. **`delete-messages` / `move-messages`**: support **criteria-based bulk
   operations** — a request describing a source queue plus a filter
   (`jmsType`, `fromDate`, `toDate`, `messageId`, `maxCount`) — in
   addition to the existing single-message and whole-queue operations.
   This is the one functional (not just wire-shape) gap: today `mq-proxy`
   cannot "delete/move up to N messages matching criteria X."
7. `mq-proxy/openapi.yaml` regenerated to reflect the new surface.

Changes to `tui`:

8. `tui/internal/queue/proxy` (the Go client implementing `queue.Backend`
   against `mq-proxy`) updated to call the new endpoints/DTOs/envelope
   for every operation it already performs today. From the TUI's
   perspective, existing behavior is unchanged — this is a wire-format
   migration for the proxy backend, not a new capability surfaced to the
   user.
   **Also fixes a real bug while here**: `toQueueMessage()`
   (`tui/internal/queue/proxy/proxy.go`) currently *ignores* any JMS type
   info and always sets `JMSType` to `"text"`/`"other"` purely by whether
   `Body` is non-nil. Once mq-proxy exposes the real `jmsType` (item 4),
   the Go client must prefer it and fall back to that same body-presence
   inference only when it's empty — mirroring the pattern the Jolokia
   backend already uses correctly
   (`tui/internal/queue/jolokia/jolokia.go:307-318`: real `jMSType`
   header first, inferred `"text"`/`"bytes"`/`"other"` fallback). Without
   this, `JMSType`-based filtering (item 9) would filter on a fabricated
   value through the proxy backend while filtering on the real header
   through Jolokia — the two backends would silently disagree.
9. `queue.Backend` gains filter-based bulk delete/move methods so the new
   `mq-proxy` capability is reachable through the interface both backends
   implement. The Jolokia backend implements the same interface via
   client-side filtering (browse, filter in Go, apply `maxCount`, then
   delete/move each match) — Jolokia/JMX has no native server-side filter
   query, but this keeps both backends behaviorally consistent at the
   `queue.Backend` level. **Confirmed in scope for this CR** (both
   backends ship the filtered bulk delete/move methods together).

## Out of scope

- **No new TUI UI** (screens, keybindings, or menu entries) for
  bulk/filtered delete or move. The new `queue.Backend` methods exist and
  are unit-tested, but nothing in `internal/app`/`internal/ui` calls them
  yet — exposing bulk-filtered delete/move to the user is a candidate for
  a future `fe` spec once this contract-alignment work has landed.
- **The live service's other endpoints** (anything outside
  queue/message management) are irrelevant to `mq-proxy` and not
  referenced here at all.
- **Authentication** stays HTTP Basic on both sides — unchanged.
- **Backward compatibility** with `mq-proxy`'s current (pre-this-CR) REST
  shape is not preserved — this is a breaking change to `mq-proxy`'s API,
  acceptable per the user since `tui` is (for now) its only consumer.
