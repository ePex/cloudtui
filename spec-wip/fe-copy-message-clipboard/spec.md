# Copy a message from the detail view

Date: 2026-09-22

## Summary

Add a `c` shortcut to the Message Detail view that copies the message being viewed to the system clipboard as readable plain text. The copied record includes the queue name, message summary, all headers and properties shown by the detail view, and the full body.

## Motivation

Users investigating or sharing a broker message currently have to select and copy the detail view manually. A single shortcut makes it easy to paste the complete message context into an incident, ticket, or chat.

## Scope

- Add `c` ("copy message") to the Message Detail view and its shortcut/context hint.
- Copy plain text, without terminal color tags, containing the queue name, ID, Type, Timestamp, every header field displayed in the Headers section (including all message properties), and the full message body.
- Include the same placeholder the detail view uses for a message without a text body (`(binary)`).
- Use the existing `CopyToClipboard` host capability and report success in the status bar without including message contents in the status.
- Keep the current view and scroll position after copying.

## Out of scope

- Copying selected text or only the body.
- Adding a clipboard dependency or changing the app's existing OSC 52 clipboard behavior.
- Copying backend-only fields that the Message Detail view does not expose as headers or properties.

## Proposed clipboard format

Use a stable, readable plain-text layout, for example:

```
Queue: orders
ID: ID:broker:1:2
Type: OrderCreated
Timestamp: 2026-09-22 12:34:56

Headers:
JMSCorrelationID: corr-123
JMSDeliveryMode: 2
Priority: 4
PropertiesText:
  customerId: CUST-42

Body:
{
  "orderId": 42
}
```

Header fields with no value should follow the detail view's current representation. Property names should be sorted alphabetically. The actual message body should be included in full, not the message list's truncated preview.

## Confirmed behavior

"All headers" means every header field and message property represented on
the Message Detail page. Backend `RawFields` may also contain
implementation fields not shown there; those are excluded.
