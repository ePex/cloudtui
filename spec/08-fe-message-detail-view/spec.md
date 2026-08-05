# FE 08 — Message detail view

Date: 2026-08-05

## What and why

The messages browser shows a summary row per message but truncates the body
and omits most JMS headers. This feature adds a detail view: pressing Enter
on a message row opens a scrollable panel showing all available metadata and
the full message body.

## User flow

1. User is in the Messages view with a queue's messages listed.
2. User presses **Enter** on a message row — the Message Details view opens,
   showing all metadata and the full body for that message.
3. The view is a scrollable `tview.TextView` with three sections:
   - **Summary** — Queue, ID, Type, Priority, Timestamp (one per line,
     label in accent color, value in text color).
   - **Headers** — all captured JMS fields rendered as sorted `Key: value`
     lines (label in accent color).
   - **Body** — full message text, pretty-printed if valid JSON (or `(binary)` for non-text messages).
4. User presses **Escape** or **Backspace** to return to the Messages view.

## Data source

`browseMessages()` already returns all needed fields in a single call. The
raw Jolokia map for each message is retained in a new `RawFields
map[string]interface{}` field on `queue.Message` so the detail view can
render any field without an additional round-trip.

Fields rendered in the Headers section (from known Jolokia keys):

| Display label    | Jolokia key       |
|------------------|-------------------|
| JMSCorrelationID | `jMSCorrelationID` |
| JMSDeliveryMode  | `jMSDeliveryMode`  |
| JMSDestination   | `jMSDestination`   |
| JMSExpiration    | `jMSExpiration`    |
| JMSRedelivered   | `jMSRedelivered`   |
| JMSReplyTo       | `jMSReplyTo`       |
| JMSXGroupID      | `groupID`          |
| JMSXGroupSeq     | `groupSequence`    |
| JMSXUserID       | `userID`           |
| Priority         | `jMSPriority`      |
| PropertiesText   | `properties`       |

## Scope

**In scope:**
- Add `RawFields map[string]interface{}` to `queue.Message`; populate it in
  `jolokia.BrowseMessages`.
- New `messageDetailView` in `internal/app/` — a bordered, scrollable
  `tview.TextView`; Esc/Backspace returns to messages view.
- Wire Enter in `messagesView` to open the detail view for the selected row.
- Register `messageDetailView` as page `"message-detail"` in the app (not in
  `a.views`); update `reapplyTheme`.
- Tests: detail view constructed with correct title; Esc shortcut present.

**Out of scope:**
- Editing or deleting the message from the detail view.
- Rendering binary/map/object message bodies.
- Pretty-printing XML body content.
