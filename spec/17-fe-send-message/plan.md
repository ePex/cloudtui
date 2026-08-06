# Plan — FE 17: Send message to queue

## Backend

### `queue.Backend` — add `SendMessage`

```go
SendMessage(ctx context.Context, queueName, body string) error
```

### `jolokia.Client.SendMessage` — JMX `sendTextMessage`

`SendMessage` calls the Jolokia `exec` operation
`sendTextMessage(java.lang.String)` on the queue's MBean. This always creates a
proper JMS **TextMessage** so the body is stored as text and readable in the
detail view.

**Why not STOMP**: the STOMP adapter in this ActiveMQ 5.18.x deployment stores
all STOMP SEND frames as **BytesMessage** regardless of `content-type:text/plain`
or `transformation:jms-text` headers. `browseMessages()` returns no body for
BytesMessages, and `browse()` also returns null for STOMP BytesMessages, so the
detail view always shows "(binary)". JMX `sendTextMessage` is the only reliable
path to readable message bodies.

**Known side effect**: JMX-sent messages embed a stale VM-transport connection
reference. When `browseMessages()` iterates the queue it calls `getClientID()`
on this closed connection and throws `IllegalStateException`. This is handled by
the `browseMessagesFallback` path (see below) — the queue shows in "limited info"
mode (yellow status note, empty IDs) but bodies are fully readable via `browse()`.

### `jolokia.Client.BrowseMessages` — body backfill + `browse()` fallback

`browseMessagesFull` is the primary path. After decoding the `browseMessages()`
response, any message with no `text` field triggers a secondary `browseBodies()`
call (`browse()` operation) to attempt body backfill.

`BrowseMessages` wraps `browseMessagesFull`: on any error (e.g.
`IllegalStateException: Error while extracting clientID` thrown for
JMX-sent messages), retry with `browseMessagesFallback` (delegates to
`browseBodies`) which returns `queue.Message` values with empty IDs (body
only). If both fail, the original `browseMessages()` error is returned.
Caller shows a yellow status note when empty-ID messages are detected.

### `jolokia.Client.PurgeQueue` — three-tier chain

1. `execSimple(ctx, mbean, "purgeQueue()")` — direct, no iteration.
2. `removeMatchingMessages(ctx, mbean, "TRUE")` — store-removal path, avoids
   the `getClientID()` call that fails for JMX-originated messages.
3. Browse-and-remove loop (existing behavior, last resort).

## Send-message overlay

### Layout

A `tview.Flex` (vertical) with border and title `" Send Message — <queueName> "`:

```
sendMessageFlex  (border + title, height 14)
├── sendMessageArea   *tview.TextArea  (proportion 1)
└── sendMessageList   *tview.List     (fixed 2 rows: "Submit" + "Cancel")
```

Registered as page `"send-message"` on `rootPages` via `centered(sendMessageFlex, 70, 14)`.

### Behavior

`showSendMessage(queueName string, onClose func())`:
1. Update flex title; clear `sendMessageArea`; repopulate `sendMessageList`.
2. Show page, focus `sendMessageArea`, set `sendMessageVisible = true`.
3. Update context panel: `<Tab> actions  <Esc> cancel`.

`doSend(queueName string)`:
- Reads body, closes overlay, spawns goroutine calling `backend.SendMessage`.
- On success: show status bar confirmation; reload queues list; reload messages
  list if open on the same queue.
- On error: show error in status bar.

`closeSendMessage()`: hides page, calls stored `sendMessageOnClose`.

### Input capture

`sendMessageArea`: `Tab` → focus list; `Esc` → close.
`sendMessageList`: `j`/`k` → down/up; `Esc` → focus area.

### `onGlobalKey` guard

`confirmVisible || movePickerVisible || sendMessageVisible`

## Queues view changes (`queues.go`)

- `Shortcuts()` gains `{Key: "c", Description: "create message"}`.
- `c` case: reads selected queue name, calls `a.showSendMessage(name, onClose)`
  where `onClose` restores focus to `qv.table` and queues shortcuts.

## Messages view changes (`messages.go`)

- `Shortcuts()` gains `{Key: "c", Description: "create message"}`.
- `c` case: calls `a.showSendMessage(mv.queueName, onClose)` where `onClose`
  restores focus to `mv.table` and messages shortcuts.

## message_detail.go fix

Delete-error (`d` hotkey) shows error in status bar alongside `slog.Error`.

## Theme

`reapplyTheme` styles `sendMessageFlex` (background, border, title),
`sendMessageArea` (`SetTextStyle`, `SetLabelStyle`), and `sendMessageList`
(via `styleList`, background).

## Infrastructure

`infra/activemq.xml` — full ActiveMQ 5.18.x broker config enabling both
OpenWire (61616) and STOMP (61613) transport connectors. STOMP is not
required by the TUI (messages are sent via JMX) but is included for
completeness and manual testing via external STOMP clients.
`infra/compose.yaml` — mounts `activemq.xml` and exposes port 61613.

## Files touched

- `tui/internal/queue/backend.go` — `SendMessage`, `MoveAllMessages`
- `tui/internal/queue/jolokia/jolokia.go` — `SendMessage` (JMX), `BrowseMessages`
  fallback, `PurgeQueue` three-tier chain, helpers
- `tui/internal/queue/jolokia/jolokia_test.go` — tests for all new paths
- `tui/internal/app/app.go` — overlay fields, `showSendMessage`, `doSend`,
  `closeSendMessage`, `onGlobalKey` guard, page registration, auto-reload
- `tui/internal/app/queues.go` — `c` hotkey, updated `Shortcuts()`
- `tui/internal/app/queues_test.go` — `SendMessage` stub
- `tui/internal/app/messages.go` — `c` hotkey, updated `Shortcuts()`,
  status note for partial browse results
- `tui/internal/app/message_detail.go` — delete-error in status bar
- `tui/internal/app/theme.go` — send-message overlay styling
- `infra/activemq.xml` — new: broker config with STOMP enabled
- `infra/compose.yaml` — STOMP port mapping, activemq.xml volume mount
