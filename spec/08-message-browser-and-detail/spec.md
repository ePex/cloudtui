# Message browser and detail view

_Condensed from spec/07, spec/08, spec/12, spec/24, spec/46 — see those folders for the incremental history._

## Purpose

Let the user inspect the messages sitting in a queue: a list view for
scanning many messages at once, and a detail view for reading one message's
full headers and body. Built on top of the queue list (spec-origin/07);
opened by pressing Enter on a queue row.

## Behavior / user flow

### Messages view

1. User moves the cursor to a queue row in the Queues view and presses
   **Enter** — the Messages view opens, titled with the queue name, and
   loads messages asynchronously.
2. Table columns: a narrow **mark** column (blank, or `✓` when marked),
   **ID**, **Type**, **Corr.ID**, **Timestamp**, **Preview** (first ~80
   chars of the body).
3. **Escape**/**Backspace** returns to the Queues view. **r** refreshes.
4. **Enter** on a message row opens the Message Detail view for that row.
5. **W** toggles wrap-around navigation for this session only (off by
   default, not persisted): when on, moving down from the last row jumps
   to the first, and up from the first jumps to the last (spec-origin/91,
   `ui.TableWrap` — shared by every list view in the app, not specific to
   this one). Independent of the Queues view's own wrap toggle.

Multi-select, independent of the table cursor:

- **space** — toggle the mark on the row under the cursor, then advance the
  cursor (so repeated presses mark a run of rows quickly).
- **a** — mark every message that has an ID (some limited-info Jolokia
  responses lack one and can't be marked, same restriction single
  delete/move already have).
- **n** — clear all marks.
- Marks do not survive a reload/refresh — `repaint()` always clears them,
  since a refreshed list may reorder or drop rows and a mark keyed by an ID
  that no longer matches its row would be confusing.
- Marker glyph is `✓`/`" "`, not `"[x]"`/`"[ ]"` — `tview.Table` interprets
  any `[...]` in cell text as a color/region tag with no per-cell opt-out,
  so bracketed glyphs are silently swallowed.

Delete (`d`) and move (`m`) act on the marked set when one exists, and fall
back to the single message under the cursor when nothing is marked (so
`d`/`m` double as a single-item shortcut without requiring an explicit mark
first) — see spec-origin/09 for the actions themselves. Only a genuinely
empty target (empty list, or a cursor row with no ID) is a no-op. Bulk
delete/move run in a goroutine (unlike the single-item path, which calls the
backend synchronously) so a large batch doesn't visibly freeze the UI, and
tolerate partial failure — one bad message doesn't abort the rest of the
batch; the status bar reports how many of the batch actually succeeded.

### Message Detail view

1. **Enter** on a message row opens a scrollable `tview.TextView` with three
   sections:
   - **Summary** — Queue, ID, Type, Priority, Timestamp (label in accent
     color, value in text color).
   - **Headers** — all captured JMS fields as sorted `Key: value` lines.
   - **Body** — full message text, pretty-printed if valid JSON, or
     `(binary)` for non-text messages.
2. **Escape**/**Backspace** returns to the Messages view via
   `pages.SwitchToPage("messages")` — not `pages.ShowPage("messages")`.
   `ShowPage` only makes the target visible without hiding others; because
   "message-detail" is added to the pages stack *after* "messages" it would
   sit on top in z-order and stay visible, so a first Esc would move focus
   but not the visible page, and a second Esc would fall through to the
   messages table's own Esc handler and jump all the way to the queue list.
   `SwitchToPage` hides every other page before showing the target, which is
   the pattern used everywhere else in the app and the one to follow for any
   new overlay/page.

### Server-side browse filtering

Two complementary, composable filter mechanisms (both persist across
reloads and are reflected in the table title):

- **Quick search** (`/`) — a live, client-side substring filter over the
  *currently loaded* rows (matches JMS type and/or preview text). No
  network round trip; narrows what's already on screen. Same mechanic as
  the queue list's `/` filter (spec-origin/07).
- **Filter form** (`f`) — a `tview.Form` overlay (JMS Type / From / To / Max
  Count fields, Apply/Clear/Cancel) that builds a `queue.MessageFilter` and
  re-fetches via `BrowseMessages` — the actual server-side-pushed-down
  filter. This is the one that bounds how much is fetched for large
  backlogs.

Both hotkeys appear in `messagesView.Shortcuts()`/the context panel.

## Data & config

```go
// queue package
type Message struct {
    ID             string
    JMSType        string
    CorrelationID  string
    Timestamp      int64 // epoch millis
    Preview        string
    RawFields      map[string]interface{} // full Jolokia map, for the detail view
}

type MessageFilter struct {
    JMSType   string
    MessageID string // no UI for this today — see below
    FromDate  time.Time
    ToDate    time.Time
    MaxCount  int
}

// Backend interface
BrowseMessages(ctx context.Context, queueName string, filter MessageFilter) ([]Message, error)
```

Detail view header labels, mapped from Jolokia's `browseMessages()` keys:

| Display label    | Jolokia key       |
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

Backend-specific filter behavior:

- **Jolokia backend**: `browseMessages()` has no server-side selector, so it
  always browses everything and then applies the filter client-side via the
  shared `filterMessages` helper — the same function the bulk delete/move
  operations use.
- **Proxy backend**: pushes `jmsType`/`messageId`/`fromDate`/`toDate`/
  `maxCount` down to `mq-proxy`'s `list-messages` endpoint as real query
  params, and always sends `returnBody=true` (the preview column needs the
  body). See spec-origin/11 for the exact wire shape.

## Implementation notes

- `internal/app/messages.go` — `messagesView`: table, marking state
  (`marked map[string]bool` keyed by message ID), the five/seven
  keybindings above, bulk delete/move.
- `internal/app/message_detail.go` — `messageDetailView`, registered as
  page `"message-detail"` (not in `a.views`).
- `internal/queue/jolokia/` — `BrowseMessages` implementation and
  `filter.go`'s `filterMessages` helper.
- The `W` wrap toggle is `ui.TableWrap` (`tui/internal/ui/tablewrap.go`) —
  see spec/07-activemq-queue-list for the shared helper's details.
- `internal/queue/proxy/` — `BrowseMessages` implementation (see
  spec-origin/11).

## Out of scope (deliberate)

- Pagination / "load more" — `MaxCount` caps what's fetched; there is no
  follow-up affordance to fetch the next batch.
- No UI for `MessageFilter.MessageID` — no real interactive use case; `/`
  substring-filtering the rendered rows covers the practical need.
- `d`/`m` do not gain an "act on every filter match" mode — they act on the
  marked set or the cursor row only, never on the full filtered result set.
- Multi-select on any other table in the app (queue list, connection
  manager, move picker).
- A "select range" gesture — only per-row toggle (`space`) and mark-all
  (`a`).
- Rendering binary/map/object message bodies, or pretty-printing XML.
