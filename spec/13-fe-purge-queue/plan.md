# Plan — FE 13: Purge queue

## Data layer

### `queue.Backend` — new methods

```go
type Backend interface {
    List(ctx context.Context) ([]Summary, error)
    BrowseMessages(ctx context.Context, queueName string) ([]Message, error)
    PurgeQueue(ctx context.Context, queueName string) error
    RemoveMessage(ctx context.Context, queueName, messageID string) error
}
```

### `jolokia.Client.PurgeQueue`

Calls `BrowseMessages` to obtain all message IDs, then issues one
`removeMessage(java.lang.String)` exec per message. `purgeQueue()` is
deliberately avoided — it is not available on all ActiveMQ deployments.

Returns on the first error; empty queues succeed immediately.

### `jolokia.Client.RemoveMessage`

Issues a single `removeMessage(java.lang.String)` exec for the given
message ID. Used from the message detail view to remove one message.

## Confirmation dialog

A `tview.Flex` (FlexRow, with border, title `" Confirm "`) centered on
screen, added as page `"confirm"` on `rootPages`. The Flex contains:

- A `tview.TextView` (2 rows, word-wrap) — the question text
- A `tview.List` (no secondary text) — `No` (item 0, default focus) and `Yes`

Using a separate TextView for the question ensures the title is always
a short fixed string (`" Confirm "`) that never clips regardless of queue
name length. `tview.List` is used instead of `tview.Modal` because Modal
focuses its last button (dangerous default).

Layout: `centered(confirmFlex, 52, 8)`.

`App` gets:
- `confirmFlex *tview.Flex`, `confirmText *tview.TextView`,
  `confirmList *tview.List`, `confirmVisible bool` fields
- `showConfirm(question string, onConfirm func())` — sets question text,
  populates list, shows page, sets focus to list
- `closeConfirm()` — hides page, restores focus

Esc on the list also calls `closeConfirm`.

## `messageDetailView` changes (`message_detail.go`)

- Add `queueName string` and `msg queue.Message` fields; populated in
  `render()` so the `d` handler always has the current message.
- Add `d` hotkey: calls `showConfirm("Delete message from <q>?", ...)`;
  on confirm, runs `backend.RemoveMessage(ctx, queueName, msg.ID)` in a
  goroutine; on success, returns to the messages list and reloads it.
- `Shortcuts()` returns `{Key: "d", Description: "delete message"}` and
  `{Key: "Esc", Description: "back"}`.

## `messagesView` changes (`messages.go`)

- Add `p` hotkey: calls `showConfirm("Purge <q>? All messages will be
  deleted.", ...)`, then `backend.PurgeQueue(...)` in a goroutine, then
  `mv.load()` on success.
- `Shortcuts()` includes `{Key: "p", Description: "purge"}`.

## `queuesView` changes (`queues.go`)

- Add `p` hotkey: reads selected row name, calls `showConfirm(...)`, then
  `backend.PurgeQueue(...)` in a goroutine, then `qv.load()` on success.
- `Shortcuts()` — all four shortcuts now fit because the top bar minimum
  height is raised to 4 rows (`shortcutPanelRows` in `topbar.go`):

```go
[]ui.Shortcut{
    {Key: "r",   Description: "refresh"},
    {Key: "p",   Description: "purge"},
    {Key: "/",   Description: "filter"},
    {Key: "o/O", Description: "sort col/dir"},
}
```

## `topbar.go`

`shortcutPanelRows = 4` constant sets the minimum top bar height, ensuring
all four queues-view shortcuts are always visible.

## `theme.go`

`reapplyTheme` styles `confirmFlex` (border/title colors), `confirmText`
(background, text color), and `confirmList` (selection colors via
`styleList`).

## Files touched

- `tui/internal/queue/backend.go` — add `PurgeQueue`, `RemoveMessage` to `Backend`
- `tui/internal/queue/jolokia/jolokia.go` — implement `PurgeQueue`, `RemoveMessage`
- `tui/internal/app/app.go` — `showConfirm`, `closeConfirm`, "confirm" page,
  `confirmFlex`/`confirmText`/`confirmList`/`confirmVisible` fields
- `tui/internal/app/queues.go` — `p` hotkey, updated `Shortcuts()`
- `tui/internal/app/messages.go` — `p` hotkey, updated `Shortcuts()`
- `tui/internal/app/message_detail.go` — `d` hotkey, `queueName`/`msg` fields,
  updated `Shortcuts()`
- `tui/internal/app/theme.go` — style confirm dialog elements
- `tui/internal/app/topbar.go` — `shortcutPanelRows = 4` minimum height
- `tui/internal/app/queues_test.go` — stubs for `PurgeQueue`, `RemoveMessage`;
  tests for `r`, `p`, `o/O` shortcut entries
- `tui/internal/app/topbar_test.go` — update height test to expect minimum 4

## Key decisions

- **`removeMessage` not `purgeQueue`**: universal ActiveMQ compatibility.
- **`RemoveMessage` on Backend interface**: needed separately from
  `PurgeQueue` for single-message deletion from the detail view.
- **`tview.Flex` + `tview.TextView` dialog**: fixed title `" Confirm "`
  never clips; question text wraps inside the body.
- **`tview.List` not `tview.Modal`**: Modal focuses last button (dangerous
  default); List starts on item 0 (No = safe default).
- **Top bar expanded to 4 rows**: `shortcutPanelRows = 4` minimum in
  `topbar.go` so all four queue shortcuts fit without combining any.
- **`d` for single delete, `p` for purge**: distinct keys with distinct
  semantics — `d` removes one message, `p` removes all.

## Testing

- `fakeQueueBackend` stubs: `PurgeQueue` and `RemoveMessage` both return nil.
- `TestQueuesViewShortcutRPresent` — `r` entry in `Shortcuts()`.
- `TestQueuesViewPurgeShortcutPresent` — `p` entry in `Shortcuts()`.
- `TestQueuesViewSortShortcutsPresent` — `o/O` entry in `Shortcuts()`.
- Manual: press `p` on a queue → confirm dialog shows with "No" focused;
  Esc and "No" both dismiss without change; "Yes" removes all messages and
  list refreshes. Press `d` in message detail → single message removed,
  returns to message list.
