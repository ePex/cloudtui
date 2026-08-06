# Spec — FE 16: Move all messages from one queue to another

Date: 2026-08-06

## What and why

FE 14 added the ability to move a single message to another queue. A common
operational task is to drain an entire queue (e.g. a DLQ) into another queue
in one action. This feature adds a "move queue" operation: from the queues
view the user selects a source queue and chooses a target queue; all messages
are moved atomically via a single Jolokia call.

## Scope

**In scope:**
- New `MoveAllMessages(ctx context.Context, sourceQueue, targetQueue string) (int, error)`
  method on `queue.Backend` and `jolokia.Client`, implemented via the
  `moveMatchingMessagesTo(java.lang.String,java.lang.String)` Jolokia exec
  operation with selector `"TRUE"` (matches all messages). Returns the number
  of messages moved as reported by Jolokia.
- `M` hotkey (capital M) in the queues view to open the existing move-picker
  overlay for the selected queue.
- After a successful move: reload the queues list and show a status bar
  confirmation with the count of messages moved.
- On error: display the error in the status bar; queues list unchanged.
- Unit test: stub `MoveAllMessages` on `fakeQueueBackend`.
- Unit test: `TestMoveAllMessagesJolokia` — verifies the correct Jolokia
  request and response parsing (including the returned integer count).

**Out of scope:**
- Per-message progress indication (the Jolokia call is atomic).
- Moving a subset of messages by selector (always moves all).
- Confirmation dialog before moving (the target selection is the confirmation,
  consistent with the single-message move).
