# FE 07 — Queue message browser

Date: 2026-08-05

## What and why

The queue list shows how many messages are pending in each queue but gives no
way to inspect them. This feature adds a message browser: pressing Enter on a
selected queue row opens a detail view listing the messages currently in that
queue.

## User flow

1. User navigates to the Queues view and moves the cursor to a queue.
2. User presses **Enter** — the Messages view opens, titled with the queue name,
   and begins loading messages asynchronously.
3. The Messages view shows a table with one row per message:
   **ID**, **Type**, **Corr.ID**, **Timestamp**, and **Preview** (first ~80 chars of the body).
4. User presses **Escape** or **Backspace** to return to the Queues view.
5. **r** refreshes the message list.

## Data source

ActiveMQ exposes messages via the Jolokia `exec` operation `browseMessages()`
on the queue MBean. Each returned object includes (at minimum):

- `messageId` — unique message identifier
- `jMSType` — JMS type header (application-set); inferred from body shape if absent
- `jMSCorrelationID` — correlation ID header
- `timestamp` — epoch millis
- `text` — body text (for `TextMessage`; may be absent for other types)

The `broweMessages()` Jolokia operation is known-good on this broker
(confirmed in project memory).

## Scope

**In scope:**
- New `queue.Message` struct (`ID`, `JMSType`, `CorrelationID`, `Timestamp`, `Preview string`).
- New `BrowseMessages(ctx, queueName string) ([]Message, error)` method on
  `queue.Backend` interface.
- Jolokia implementation of `BrowseMessages` using the `exec` operation.
- New `messagesView` in `internal/app/` — bordered table with ID / Type /
  Corr.ID / Timestamp / Preview columns, `r` to refresh, Escape/Backspace to
  return to queues.
- Wire Enter in `queuesView` to open `messagesView` for the selected queue.
- Register `messagesView` in the app and update `reapplyTheme`.
- Tests for the new Jolokia method and the messages view.

**Out of scope:**
- Deleting or moving individual messages (a separate feature).
- Filtering messages.
- Displaying non-text message bodies (binary, map, object).
- Pagination (load all messages returned by `browseMessages()`).
