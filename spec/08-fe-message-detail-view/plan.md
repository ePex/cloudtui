# Plan — FE 08: Message detail view

## Architecture

`messageDetailView` is added as a page `"message-detail"` in `a.pages` but
**not** in `a.views` (same pattern as `messagesView`). A dedicated
`openMessageDetail(msg queue.Message)` method on `App` handles the transition
in; Esc/Backspace in `messageDetailView` calls `a.switchTo("messages")` to
return (which re-shows the messages page without reloading).

## Data layer

### `queue.Message` — add `RawFields`

```go
type Message struct {
    ID            string
    JMSType       string
    CorrelationID string
    Timestamp     time.Time
    Preview       string
    RawFields     map[string]interface{} // full Jolokia response map
}
```

`jolokia.BrowseMessages` stores `m` (the raw decoded map) into `RawFields`
for each message after all other fields have been extracted. No additional
HTTP call is needed.

### No new Backend method

The detail view reads from `queue.Message.RawFields` directly — data is
already present from `BrowseMessages`.

## `messageDetailView` (`tui/internal/app/message_detail.go`)

A `tview.TextView` with border, title `" Message Details "`, dynamic colors
enabled, word-wrap on, and scrollable (j/k for vertical scroll).

```go
type messageDetailView struct {
    textView *tview.TextView
    app      *App
}
```

### `render(queueName string, msg queue.Message)`

Builds a color-tagged string and sets it on the TextView. Three sections:

**Summary** (label color / text color):
```
[accent]Queue:[-]     <queueName>
[accent]ID:[-]        <msg.ID>
[accent]Type:[-]      <msg.JMSType>
[accent]Priority:[-]  <raw jMSPriority>
[accent]Timestamp:[-] <msg.Timestamp formatted>
```

**Headers** (blank line, then `[accent]Headers:[-]`, then each known field):

| Label            | RawFields key     |
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

Each rendered as `[accent]<Label>:[-] <value>`. The `properties` field is
special: it is a `map[string]interface{}` where values are ActiveMQ
`ByteSequence` objects (a map with a `data` key holding `[]interface{}` of
float64 byte values). A `decodePropertyValue` helper decodes these to UTF-8
strings; each property is rendered on its own indented line sorted by key.

**Body** (blank line, then `[accent]Body:[-]`, then full text or `(binary)`).
If the body is valid JSON it is pretty-printed with 2-space indentation via
`json.Indent`.

### Input capture

- `j` → `KeyDown`, `k` → `KeyUp`
- `Esc` / `Backspace` → `a.pages.ShowPage("messages")` + update context panel (does not reload messages)

### Shortcuts

```go
func (dv *messageDetailView) Shortcuts() []ui.Shortcut {
    return []ui.Shortcut{
        {Key: "Esc", Description: "back"},
    }
}
```

## `App` changes (`app.go`, `theme.go`)

- Add `messageDetailV *messageDetailView` field to `App`.
- Construct `newMessageDetailView(a)` in `New()`; add TextView as page
  `"message-detail"`.
- Add `openMessageDetail(queueName string, msg queue.Message)` method: calls
  `messageDetailV.render(...)`, switches to page `"message-detail"`, updates
  context panel.
- Wire `messagesView.table.SetSelectedFunc` to call `a.openMessageDetail`.
- `reapplyTheme` — repaint detail TextView background, border, title.

## Files touched

- `tui/internal/queue/backend.go` — add `RawFields` to `Message`
- `tui/internal/queue/jolokia/jolokia.go` — populate `RawFields`
- `tui/internal/app/message_detail.go` — new file
- `tui/internal/app/message_detail_test.go` — new file
- `tui/internal/app/app.go` — field, method, page wiring, SetSelectedFunc
- `tui/internal/app/messages.go` — add `msgs []queue.Message` field; populate in `repaint`; wire `SetSelectedFunc`
- `tui/internal/app/theme.go` — repaint detail view

## Key decisions

- **`RawFields` over a rich struct**: avoids a large fixed struct for rarely
  used fields; any field Jolokia returns is automatically available.
- **`tview.TextView` over Table**: free-form text with color tags is simpler
  for a detail panel than a two-column table, and handles long values
  (e.g. PropertiesText) more gracefully.
- **Known fields list (not all keys)**: rendering every Jolokia key would
  include internal ActiveMQ bookkeeping fields that are meaningless to the
  user. A curated list matches what the ActiveMQ web console shows.
- **No reload on open**: data is already in `RawFields`; re-opening the
  detail for the same message is instantaneous.

## Testing

- `message_detail_test.go`: view constructs without panic; title is
  `" Message Details "`; Shortcuts() contains Esc.
- `fakeQueueBackend.BrowseMessages` already returns `nil` — `RawFields` will
  be nil; `render` must handle nil `RawFields` gracefully.
