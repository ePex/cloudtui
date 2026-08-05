# Plan — FE 07: Queue message browser

## Architecture

`messagesView` is added as a page in `a.pages` but is **not** in `a.views`
(so it never appears in the home dashboard, has no `:messages` command, and
`switchTo` doesn't manage it). A dedicated `openMessages(queueName)` method on
`App` handles the transition in; Esc/Backspace in `messagesView` calls
`a.switchTo("queues")` to return.

## Data layer

### `queue.Message` struct (in `backend.go`)

```go
type Message struct {
    ID            string
    JMSType       string // jMSType header, or inferred type ("text"/"bytes"/"other")
    CorrelationID string // jMSCorrelationID header
    Timestamp     time.Time
    Preview       string // first 80 chars of body text
}
```

### `queue.Backend` interface — add one method

```go
BrowseMessages(ctx context.Context, queueName string) ([]Message, error)
```

All existing implementations (`jolokia.Client`) and test fakes must implement
this method.

### Jolokia `BrowseMessages` implementation

Single `exec` POST to the Jolokia URL:

```json
{
  "type":      "exec",
  "mbean":     "org.apache.activemq:type=Broker,brokerName=<name>,destinationType=Queue,destinationName=<queue>",
  "operation": "browseMessages()"
}
```

Response `value` is a JSON array of objects. Fields used:

| JSON key     | Type  | Mapped to         |
|--------------|-------|-------------------|
| `messageId`        | object (JMX CompositeData) | `Message.ID` — reconstructed from nested fields |
| `jMSType`          | string | `Message.JMSType` — falls back to inferring `"text"`/`"bytes"`/`"other"` from `text`/`bodyLength` presence |
| `jMSCorrelationID` | string | `Message.CorrelationID` |
| `timestamp`        | float64 (JSON number) | `Message.Timestamp` (epoch ms → `time.UnixMilli`) |
| `text`             | string | `Message.Preview` (truncated to 80 chars; absent for non-text messages) |

ActiveMQ returns `messageId` as a JMX `CompositeData` object, not a plain
string. Value items are decoded as `map[string]interface{}`. The ID string is
reconstructed as `ID:<connectionId>:<producerSeq>:<brokerSeq>` from the nested
`producerId.producerSessionId.connectionId.value` field. If that path is
unavailable the format falls back to `ID:?:<producerSeq>:<brokerSeq>`.

Non-200 Jolokia status → error. Missing `text` field → Preview is `"(binary)"`.

### Tests

`jolokia_test.go` — `TestBrowseMessagesHappyPath` (httptest server returning
one message), `TestBrowseMessagesHTTPError`, `TestBrowseMessagesJolokiaError`.

## `messagesView` (`tui/internal/app/messages.go`)

Bordered `tview.Table`, 5 fixed columns: `ID`, `TYPE`, `CORR.ID`, `TIMESTAMP`, `PREVIEW`.
Header row styled identically to queuesView header (label-bg, background-fg).
Rows are selectable (future actions); sorted by timestamp descending (newest
first). Shortcuts: `r` refresh, Esc/Backspace back to queues.

```go
type messagesView struct {
    table     *tview.Table
    app       *App
    queueName string
}
```

`load()` calls `backend.BrowseMessages(ctx, queueName)` in a goroutine +
`QueueUpdateDraw`. Errors logged via `slog.Error` and shown in the table.

Title set dynamically: ` Messages — <queueName> `.

## `App` changes (`app.go`, `theme.go`)

- Add `messagesV *messagesView` field to `App`.
- Construct `newMessagesView(a)` in `New()` alongside other views; add the
  primitive as page `"messages"` in `a.pages` (but NOT to `a.views`).
- Add `openMessages(queueName string)` method: sets `messagesV.queueName`,
  updates the table title, calls `messagesV.load()`, switches page, updates
  context panel.
- `reapplyTheme` — repaint messages table background, border, title.
- `queuesView.SetSelectedFunc` calls `a.openMessages(name)` on Enter.

## Files touched

- `tui/internal/queue/backend.go` — `Message` struct, extend `Backend`
- `tui/internal/queue/jolokia/jolokia.go` — `BrowseMessages` implementation
- `tui/internal/queue/jolokia/jolokia_test.go` — new tests
- `tui/internal/app/messages.go` — new file
- `tui/internal/app/messages_test.go` — new file
- `tui/internal/app/app.go` — `messagesV` field, `openMessages`, page wiring,
  `SetSelectedFunc` on queuesView
- `tui/internal/app/theme.go` — repaint messages table
- `tui/internal/app/queues.go` — remove placeholder; `SetSelectedFunc` now
  wired from `app.go` after both views are constructed

## Key decisions

- **Not in `a.views`**: messages depend on a selected queue; they are not a
  standalone destination. Keeping them out of `a.views` avoids stale state
  (wrong queue shown) if someone navigated there without a selection.
- **`SetSelectedFunc` wired in `app.go`** (not in `newQueuesView`): at
  construction time `messagesV` doesn't exist yet. `app.go` wires the callback
  after both views are constructed.
- **Timestamp descending**: newest messages at the top matches typical
  debugging behaviour.
- **Preview truncation at 80 chars**: keeps the table readable without
  horizontal scrolling.

## Testing

- Jolokia: happy path, HTTP error, Jolokia error status.
- `messagesView`: header labels, column count, `r` shortcut present.
- Fake backend update: `fakeQueueBackend.BrowseMessages` returns nil.
