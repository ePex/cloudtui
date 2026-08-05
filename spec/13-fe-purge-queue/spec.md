# Spec — FE 13: Purge queue

Date: 2026-08-06

## What

- From the **Queues list**, the user can purge all messages from a selected
  queue with `p`. A confirmation dialog is shown before any messages are deleted.
- From the **Messages list**, `p` purges the current queue (same flow).
- From the **Message detail view**, `d` removes only the single message being
  viewed, then returns to the messages list.

## Why

Purging a queue is a common operational task (clearing dead-letter queues,
resetting test environments). Having it directly in the TUI avoids needing
a separate web console.

## Approach

Because `purgeQueue()` is not available on all ActiveMQ deployments (some
return Jolokia 400 "No operation purgeQueue found"), the implementation
browses all messages and removes them one by one via
`removeMessage(java.lang.String)`, which works universally. This is fast
enough in practice for the queue sizes typically managed through this tool.

## User flow

1. User highlights a queue row in the Queues list and presses `p`.
2. A confirmation dialog appears (`" Confirm "` title) with the question
   `Purge "<queue name>"? All messages will be deleted.` and two choices:
   **No** (default focus — prevents accidental deletion) and **Yes**.
3. Selecting **Yes** issues the delete operations. The dialog closes and
   the queue list refreshes automatically.
4. Selecting **No** (or pressing Esc) dismisses the dialog without changes.

The same `p` hotkey is also available in the **Messages list** view for the
currently open queue. In the **Message detail view**, `d` removes only the
single viewed message.

## Out of scope

- Progress indicator for large queues.
- Undo.
- Bulk-purge of multiple queues at once.
