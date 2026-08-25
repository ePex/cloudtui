# Queue and message actions: purge, move, send

_Condensed from spec/13, spec/14, spec/15, spec/16, spec/17 — see those folders for the incremental history._

## Purpose

Operational actions on queues and messages, so common ActiveMQ admin tasks
(clearing a DLQ, requeuing a message, draining a queue, injecting a test
message) don't require the ActiveMQ web console. Builds on the queue list
(spec/07) and message browser/detail (spec/08).

## Behavior / user flow

### Purge

- **Queues list**: `p` on a queue row purges it.
- **Messages list**: `p` purges the currently open queue (same flow).
- **Message detail view**: `d` removes only the single message being
  viewed, then returns to the messages list.

Purge shows a confirmation dialog (`" Confirm "` title, question `Purge
"<queue name>"? All messages will be deleted.`) with **No** as the default
focus (prevents accidental deletion) and **Yes** to proceed. Selecting
**Yes** closes the dialog and refreshes the queue list; **No**/Esc dismisses
without changes.

### Move single message

1. From the message detail view, `m` opens a queue-picker overlay listing
   all queues from the broker (loaded fresh at open time, current queue
   excluded).
2. Ordering is four-tier (each tier sorted alphabetically within itself),
   because DLQ requeue is the dominant real workflow:
   1. `⭐` **Preferred** — when the source queue is `dlq.*` or `imq.*`, the
      corresponding stripped-prefix queue (`dlq.foo.bar`/`imq.foo.bar` →
      `foo.bar`) is pinned first.
   2. **Regular** — everything else.
   3. `➖` **DLQ/imq** — queues themselves prefixed `dlq.`/`imq.`, shown but
      de-prioritized.
   4. `❓` **System** — queues prefixed `activemq.` or `statistics.*`,
      de-prioritized to the bottom.
   DLQ detection is a name-prefix heuristic only — no broker metadata check.
3. `/` opens an inline `tview.InputField` filter at the bottom of the
   picker; typing narrows the list live by case-insensitive substring
   match. Esc clears the filter and refocuses the list; Enter (with the
   filter active) selects the first visible item.
4. `j`/`k`/arrows navigate, Enter confirms, Esc cancels without moving.
5. On confirm, the message moves via the backend's move operation; on
   success the overlay closes, the messages list reloads.

### Move all messages (drain a queue)

- `M` (capital) in the queues view opens the same move-picker overlay
  (with the same DLQ-priority ordering and `/` search) for the selected
  queue, then moves every message on it to the chosen target in one call.
- No confirmation dialog beyond the target selection itself — picking a
  target *is* the confirmation, matching the single-message move's UX.
- On success: reload the queues list, status bar shows the count of
  messages moved. On error: status bar shows the error, queue list
  unchanged.
- No partial moves by selector — always moves everything on the source
  queue.

### Send message

- `c` in either the queues view or the messages view opens a send overlay:
  a bordered flex with a multi-line `tview.TextArea` body field and two
  actions, Submit and Cancel.
- On submit: message is sent, overlay closes, the queues list (and the
  messages list, if open) reload, status bar confirms.
- On cancel/Esc: overlay closes, focus returns to the caller.
- Text messages only — no headers, properties, JMS type selection, or
  templates/history.

## Data & config

```go
// queue.Backend interface additions
PurgeQueue(ctx context.Context, queueName string) error
DeleteMessage(ctx context.Context, queueName, messageID string) error
MoveMessage(ctx context.Context, queueName, messageID, targetQueue string) error
MoveAllMessages(ctx context.Context, sourceQueue, targetQueue string) (int, error) // returns count moved
SendMessage(ctx context.Context, queueName, body string) error
```

No config additions — send reuses the same Jolokia connection config as
every other queue operation (spec/07).

## Implementation notes

- **Purge**: `purgeQueue()` is not available on all ActiveMQ deployments
  (some return Jolokia 400 "No operation purgeQueue found"). The Jolokia
  backend's `PurgeQueue` tries, in order: `purgeQueue()` JMX operation,
  then `removeMatchingMessages("TRUE")`, then a manual browse-and-remove
  loop via `removeMessage(java.lang.String)` — the last resort works
  universally and is fast enough in practice for the queue sizes this tool
  manages.
- **Single move**: Jolokia operation `moveMessageTo(java.lang.String,
  java.lang.String)` on the source queue MBean, args `[messageID,
  targetQueueName]` — reliable across deployments.
- **Move all**: Jolokia operation `moveMatchingMessagesTo(java.lang.String,
  java.lang.String)` with selector `"TRUE"` (matches everything).
- **Send**: implemented via the Jolokia `exec` operation
  `sendTextMessage(java.util.Map,java.lang.String,java.lang.String,java.lang.String)`
  on the destination's Broker MBean, args `[{} (empty headers map), body,
  username, password]` — **not** the single-arg
  `sendTextMessage(java.lang.String)` overload, which fails with `User
  name [null]` because it never receives credentials. The
  `Map`+3-string-arg overload is what actually creates a real JMS
  `TextMessage`, fully browsable afterward, with the broker's own
  credentials enforced. No STOMP, no separate transport/port — this reuses
  the same Jolokia HTTP connection as every other queue operation.
- **Side effect**: sending via `sendTextMessage` embeds a stale
  VM-transport connection reference into the sent message. The next
  `browseMessages()` call against *that queue* then throws
  `IllegalStateException: Error while extracting clientID` for the whole
  queue, not just the sent message — this is exactly why `BrowseMessages`
  (spec/08) falls back to the simpler `browse()` JMX operation
  whenever `browseMessages()` errors, rather than surfacing the error.
  Fallback-path messages have empty IDs; a yellow status note is shown
  when this path is taken.
- The queue picker (shared by single-move and move-all) is a `tview.List`
  overlay using the same centered pattern as the confirm dialog; queue
  names are loaded via `backend.List()` in a goroutine, showing
  `"Loading…"` while in flight.
- Files: `tui/internal/dialog/movepicker.go` (see spec/03) for the picker;
  purge/move/send wiring lives alongside `QueuesView`/`MessagesView`/
  `MessageDetailView` (spec/07, 08); Jolokia implementations in
  `internal/queue/jolokia/`.

## Out of scope (deliberate)

- Progress indicators for large purges/moves; no undo.
- Bulk-purge of multiple queues at once.
- Creating a new destination queue on the fly from the picker.
- Message headers, properties, JMS type selection, templates/history on
  send — body text only.
