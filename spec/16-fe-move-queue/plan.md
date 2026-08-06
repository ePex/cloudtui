# Plan — FE 16: Move all messages from one queue to another

## Backend

### `queue.Backend` — add `MoveAllMessages`

```go
MoveAllMessages(ctx context.Context, sourceQueue, targetQueue string) (int, error)
```

### `jolokia.Client.MoveAllMessages`

Single Jolokia exec on the source queue MBean:

```json
{
  "type":      "exec",
  "mbean":     "org.apache.activemq:type=Broker,brokerName=<b>,destinationType=Queue,destinationName=<sourceQueue>",
  "operation": "moveMatchingMessagesTo(java.lang.String,java.lang.String)",
  "arguments": ["TRUE", "<targetQueue>"]
}
```

The Jolokia response `value` is an integer (count of messages moved). Check
`status != 200` for errors. Return the integer value to the caller.

## `showMovePicker` refactor — callback-based

Currently `showMovePicker(sourceQueue string, msg queue.Message)` closes over
`msg` to know what to move. To support both single-message and whole-queue
moves, the action on selection is extracted into a caller-supplied callback.

**New signature:**
```go
func (a *App) showMovePicker(sourceQueue string, onSelect func(targetQueue string))
```

`App` gains a `movePickerOnSelect func(string)` field (set each time the picker
opens) so `fillPickerList` can reference it via the stored field rather than
needing it passed as a parameter.

`fillPickerList` signature becomes:
```go
func (a *App) fillPickerList(filter string)
```

It uses `a.movePickerOnSelect` for each item's selected func.

**Call sites updated:**
- `message_detail.go` `m` hotkey: calls `showMovePicker` with a closure that
  runs `backend.MoveMessage`, switches to messages page, reloads.
- New queues view `M` hotkey: calls `showMovePicker` with a closure that runs
  `backend.MoveAllMessages`, reloads queues, shows status bar count.

## Queues view changes (`queues.go`)

- `Shortcuts()` gains `{Key: "M", Description: "move queue"}`.
- `M` case in `table.SetInputCapture`:
  - Read selected queue name from table (skip header row / empty cell).
  - Call `a.showMovePicker(name, func(target string) { ... })`:
    - In goroutine: `count, err := a.backend.MoveAllMessages(ctx, name, target)`
    - `QueueUpdateDraw`: on error show status bar error; on success show
      `"Moved N messages from <src> to <dst>"` in status bar and call
      `qv.load()`.

## Files touched

- `tui/internal/queue/backend.go` — add `MoveAllMessages`
- `tui/internal/queue/jolokia/jolokia.go` — implement `MoveAllMessages`
- `tui/internal/queue/jolokia/jolokia_test.go` — `TestMoveAllMessages`
- `tui/internal/app/app.go` — `movePickerOnSelect` field; refactor
  `showMovePicker` and `fillPickerList` to callback-based
- `tui/internal/app/message_detail.go` — update `showMovePicker` call site
- `tui/internal/app/queues.go` — `M` hotkey, updated `Shortcuts()`
- `tui/internal/app/queues_test.go` — stub `MoveAllMessages`

## Key decisions

- **Callback over subtype**: passing `onSelect func(string)` is simpler than
  adding a mode enum or a second picker function; the picker UI is identical
  for both use cases.
- **`movePickerOnSelect` stored on App**: `fillPickerList` (called from
  `SetChangedFunc`) needs the callback without receiving it as a parameter;
  storing it on App avoids threading it through multiple call sites.
- **Selector `"TRUE"`**: matches all messages without needing to know the
  queue depth; Jolokia returns the count of moved messages.
- **No confirmation dialog**: consistent with single-message move — target
  selection is the confirmation.

## Testing

- `fakeQueueBackend.MoveAllMessages` stub returns `(0, nil)`.
- `TestMoveAllMessages`: fake server verifies the exec operation name and
  `"TRUE"` selector argument; checks the returned integer count.
- Manual: select a queue with messages, press `M`; picker appears; select
  target; status bar shows "Moved N messages from … to …"; queues list
  reloads showing updated pending counts.
