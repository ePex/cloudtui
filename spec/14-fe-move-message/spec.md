# Spec — FE 14: Move message to another queue

Date: 2026-08-06

## What

From the **message detail view**, the user can move the currently viewed
message to a different queue by pressing `m`. A target queue is chosen from
an interactive list of all known queues, then the move is executed.

## Why

Moving messages between queues (e.g. reprocessing a dead-letter message by
moving it back to the original queue) is a common operational task. Having
it in the TUI avoids the need for the ActiveMQ web console.

## User flow

1. User opens a message's detail view and presses `m`.
2. A queue-picker overlay appears showing all queues from the broker
   (loaded fresh at open time). The current queue is excluded.
3. The user navigates the list with `j`/`k` or arrow keys and confirms
   with Enter.
4. The message is moved via the Jolokia `moveMessageTo` operation.
5. On success the overlay closes, the view returns to the messages list,
   and the list is reloaded.
6. Pressing Esc cancels without moving.

## Approach

- Jolokia operation: `moveMessageTo(java.lang.String,java.lang.String)`
  on the source queue MBean, with arguments `[messageID, targetQueueName]`.
  This operation works reliably across ActiveMQ deployments.
- The queue picker is a `tview.List` overlay (same centered pattern as the
  confirm dialog), populated by calling `backend.List()` at open time in a
  goroutine. While loading, the list shows a `"Loading…"` placeholder.

## Out of scope

- Moving from the messages list view (detail view only).
- Moving multiple messages at once.
- Creating a new destination queue on the fly.
- Filtering/searching within the picker list.
