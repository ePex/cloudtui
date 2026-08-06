# Plan — FE 14: Move message to another queue

## Data layer

### `queue.Backend` — add `MoveMessage`

```go
MoveMessage(ctx context.Context, sourceQueue, messageID, targetQueue string) error
```

### `jolokia.Client.MoveMessage`

Single Jolokia exec on the source queue MBean:

```json
{
  "type":      "exec",
  "mbean":     "org.apache.activemq:type=Broker,brokerName=<b>,destinationType=Queue,destinationName=<sourceQueue>",
  "operation": "moveMessageTo(java.lang.String,java.lang.String)",
  "arguments": ["<messageID>", "<targetQueueName>"]
}
```

## Queue-picker overlay

A `tview.List` (`ShowSecondaryText(false)`) with border and title
`" Move to Queue "`, registered as page `"move-picker"` on `rootPages`
and centered: `centered(movePickerList, 52, 20)`.

`App` gains:
- `movePickerList *tview.List` field
- `movePickerVisible bool` field (guards global hotkeys, same pattern as
  `confirmVisible`)
- `showMovePicker(sourceQueue string, msg queue.Message)` — clears the
  list, adds a `"Loading…"` placeholder, shows the page, sets focus, then
  spawns a goroutine that calls `backend.List()` and calls
  `QueueUpdateDraw` to replace the placeholder with the actual queue
  names (all queues except `sourceQueue`, sorted alphabetically). Each
  item's selected-func calls `closeMovePickerr()` then runs the move in a
  goroutine.
- `closeMovePicker()` — hides the page, restores focus to the pages,
  sets `movePickerVisible = false`.

Input capture on the picker list:
- `j` → `KeyDown`, `k` → `KeyUp`
- `Esc` → `closeMovePicker()`

After a successful move:
- Switch to the messages page (`SwitchToPage("messages")`)
- Set focus to `messagesV.table`
- Repopulate context panel from `messagesV.Shortcuts()`
- Call `messagesV.load()` to reload the (now shorter) list

On error: log via `slog.Error` and display the error in the status bar;
stay on the message detail view. The Jolokia response `value` field
(boolean) is checked: `false` means the message was not found (possibly
wrong ID format) and is surfaced as an error.

Note: `browseMessages()` returns `messageId` as a JMX CompositeData object.
The canonical ID string is reconstructed as:
`<producerId.connectionId.value>:<producerId.sessionId>:<producerId.value>:<producerSequenceId>`
where `connectionId.value` already carries the `"ID:"` prefix. Note that
`brokerSequenceId` is an internal broker field and is NOT part of the JMS
message ID. This format is required by both `removeMessage` and
`moveMessageTo` to locate messages.

## `messageDetailView` changes (`message_detail.go`)

- Add `m` case to `SetInputCapture`: calls `a.showMovePicker(dv.queueName, dv.msg)`.
- `Shortcuts()` gains `{Key: "m", Description: "move"}`.

## `theme.go`

`reapplyTheme` styles `movePickerList` (selection colors via `styleList`,
background color, border/title color).

## Files touched

- `tui/internal/queue/backend.go` — add `MoveMessage` to `Backend`
- `tui/internal/queue/jolokia/jolokia.go` — implement `MoveMessage`
- `tui/internal/app/app.go` — `movePickerList`, `movePickerVisible`,
  `showMovePicker`, `closeMovePicker`; "move-picker" page on `rootPages`;
  guard `onGlobalKey` for `movePickerVisible`
- `tui/internal/app/message_detail.go` — `m` hotkey, updated `Shortcuts()`
- `tui/internal/app/theme.go` — style move picker list
- `tui/internal/app/queues_test.go` — stub `MoveMessage` on `fakeQueueBackend`

## Key decisions

- **Picker loads queues fresh on open**: ensures the list reflects the
  current broker state; avoids stale data if queues were added/removed
  since the queues view last loaded.
- **`Loading…` placeholder**: gives immediate feedback while the async
  fetch runs; replaced atomically in `QueueUpdateDraw`.
- **Current queue excluded**: moving a message to its own queue is a no-op
  and would be confusing.
- **`tview.List` not a text input**: typing queue names is error-prone;
  selecting from the live list is faster and less error-prone for the
  typical use case (small number of queues).
- **No confirm step after selecting target**: the target selection IS the
  confirmation — adding a second dialog would be unnecessarily verbose.
  Esc is always available to cancel.

## Testing

- `fakeQueueBackend.MoveMessage` stub returns `nil`.
- `TestBrowseMessagesCompositeDataMessageID`: verifies CompositeData ID reconstruction.
- `TestOpenMessageDetailSetsContextPanelShortcuts`: verifies `m`, `d`, `Esc` in context panel.
- Manual: open message detail, press `m` → picker shows "Loading…" then
  queue list; press Esc → returns to detail without moving; select a queue
  → message moved, messages list reloads.
