# Send message: JMS Type, headers, group ID, correlation ID

Date: 2026-09-18

## What

The "Send message" overlay (spec/09) currently sends body text only —
every message is created with a hardcoded/empty JMS Type, no correlation
ID, no group ID, and no custom properties. This extends it to let the
user set, when composing a new message:

- **JMS Type** — the JMS `JMSType` header (an application-defined string,
  e.g. `"order.created"`), not the message's body class (always a JMS
  `TextMessage` in this app, unchanged).
- **Correlation ID** — the JMS `JMSCorrelationID` header.
- **Group ID** — the ActiveMQ `JMSXGroupID` property (message grouping /
  ordered consumption within a group).
- **Headers** — arbitrary custom string properties, as key/value pairs.

## Why

Both backends' wire protocols already carry all four fields end-to-end:

- **mq-proxy**: `SendMessageRequest` (Kotlin) has `jmsType`, `headers`,
  `groupId`, `correlationId` and `BrokerService.sendMessage` already
  applies all of them to the outgoing JMS message. The Go client's
  `sendMessageRequest` struct already has matching fields — they're just
  never populated.
- **Jolokia**: the JMX operation already in use,
  `sendTextMessage(java.util.Map,java.lang.String,java.lang.String,java.lang.String)`,
  takes a headers `Map` as its first argument — ActiveMQ's own vehicle for
  setting `JMSType`/`JMSCorrelationID`/`JMSXGroupID`/custom properties on
  send. Today that map is always passed empty.

So the capability exists end-to-end already; only the TUI's `Backend`
interface, both backend implementations' argument-building, and the send
dialog's UI need to catch up. This also removes an asymmetry with the
message detail view (spec/08), which already displays JMS Type,
Correlation ID, and Group ID for existing messages — but offered no way
to set them when composing a new one.

## Scope

- Widen the compose flow so JMS Type, Correlation ID, Group ID, and an
  arbitrary set of custom headers (key/value pairs) can be set before
  sending, on both the Jolokia and mq-proxy backends.
- Field requirements:
  - **JMS Type** — mandatory. The mq-proxy wire type already requires it
    (`jmsType: String`, non-nullable); Jolokia gets no free pass just
    because its own transport is looser about it.
  - **Body** — mandatory (unchanged from today).
  - **Correlation ID** — optional to type, but never sent blank: if the
    user leaves it empty, a UUID is generated automatically at send time.
    The field is pre-filled with a generated UUID when the overlay opens
    so the default is visible and editable, not a hidden side effect.
  - **Group ID** — optional; blank means no `JMSXGroupID` is set on the
    outgoing message.
  - **Headers** — optional; zero or more key/value pairs, defaults to
    none.

## Out of scope (deliberate)

- Message templates or send history (still out of scope per spec/09).
- Editing/removing headers on an *existing* message (this is compose-only;
  spec/08's detail view remains read-only).
- Any change to what's already read/displayed for existing messages
  (spec/08) — this feature only affects composing new ones.
- A picker/autocomplete for JMS Type on send, mirroring the one purge/
  move-all already have (spec/09) — could be a natural follow-up, but not
  bundled into this change unless requested.
- Validating header keys/values against JMS naming restrictions (e.g.
  reserved property name prefixes) — sent through as typed.
