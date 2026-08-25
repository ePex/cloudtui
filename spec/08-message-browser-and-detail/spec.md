# Message browser and detail view

_Condensed from spec/07, spec/08, spec/12, spec/24, spec/46 — see those folders for the incremental history._

## Purpose

Let the user inspect the messages sitting in a queue: a list view for
scanning many messages at once, and a detail view for reading one message's
full headers and body. Built on top of the queue list (spec/07);
opened by pressing Enter on a queue row.

## Behavior / user flow

### Messages view

1. User moves the cursor to a queue row in the Queues view and presses
   **Enter** — the Messages view opens, titled with the queue name, and
   loads messages asynchronously.
2. Table columns: a narrow **mark** column (blank, or `✓` when marked),
   **ID**, **Type**, **Corr.ID**, **Timestamp**, **Preview** (first
   2000 chars of the body — plenty for the vast majority of messages;
   the full body is what's already fetched, so this bound is a display/
   memory cap, not a network one). Column widths aren't equal:
   **ID**/**Corr.ID** are capped at 20 characters (`…` if longer — full
   values are only ever needed in the Message Detail view, not the
   list) and get no extra width on a wider terminal; **Preview** gets
   by far the largest share of any extra space (found live, CR 92: with
   every column claiming equal weight, **Preview** — the one column
   actually worth reading in this list — was consistently the most
   cramped, especially as the terminal widened).
3. **Escape**/**Backspace** returns to the Queues view. **r** refreshes.
4. **Enter** on a message row opens the Message Detail view for that row.
5. **w** toggles word-wrap on the Preview column, per-session (not
   persisted), off by default (spec-wip/92): when on, a preview
   that doesn't fit on one line word-wraps into non-selectable
   continuation rows directly below the message's row, up to
   `maxWrapLines` (50) — a preview whose wrapping would need more than
   that ends with a `"… N more line(s)"` indicator row rather than
   silently truncating with no sign anything was cut, protecting the
   list from one very long preview burying every other message.
   Wrapping preserves the preview's own line breaks — a multi-line body
   (e.g. formatted XML/JSON, a stack trace pasted into a text message)
   wraps line-by-line rather than being flattened into one re-flowed
   paragraph, so it still reads as its original structure. `tview.Table`'s
   own up/down navigation already skips non-selectable rows (the same
   mechanism the header row uses), so `j`/`k`/arrow navigation, marks,
   and `Enter` all keep landing on the right message with no special
   handling. Unlike a genuine reload, toggling wrap does **not** clear
   marks or reset scroll position — it only re-renders the current
   rows, since a purely cosmetic toggle has no logical reason to
   invalidate either.

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
first) — see spec/09 for the actions themselves. Only a genuinely
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
  the queue list's `/` filter (spec/07).
- **Filter form** (`f`) — a `tview.Form` overlay (JMS Type / From / To / Max
  Count fields, Apply/Clear/Cancel) that builds a `queue.MessageFilter` and
  re-fetches via `BrowseMessages` — the actual server-side-pushed-down
  filter. This is the one that bounds how much is fetched for large
  backlogs.

Both hotkeys appear in `MessagesView.Shortcuts()`/the context panel.

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
  body). See spec/11 for the exact wire shape.

## Implementation notes

- `tui/internal/view/messages.go` — `MessagesView`: table, marking state
  (`marked map[string]bool` keyed by message ID), the five/seven
  keybindings above, bulk delete/move. Originally lived at
  `internal/app/messages.go`; moved into `internal/view` as part of the
  later package split (see spec/03-architecture-and-package-layout).
- `tui/internal/view/message_detail.go` — `MessageDetailView`, registered
  as page `"message-detail"` (not in `a.views`). Same package-split move
  as above.
- `internal/queue/jolokia/` — `BrowseMessages` implementation and
  `filter.go`'s `filterMessages` helper. `previewMaxLen` (2000) is the
  jolokia-package-local constant capping `Preview`'s length — the full
  body is already in memory by the time it's applied, so this is a
  display/memory bound, not a network one.
- `internal/queue/proxy/` — `BrowseMessages` implementation (see
  spec/11); its own identical `previewMaxLen` constant (same
  reasoning as the Jolokia backend's).
- The `w` wrap toggle's helpers (`wrapText`, `wrapMultilineText`,
  `setContinuationRow`, `dynamicWrapWidth`, `maxWrapLines`) live in
  `tui/internal/view/wraptext.go`, shared with the CloudWatch Logs
  search and Datadog Logs views (spec/17, spec/18) — not specific to
  this view. `wrapMultilineText` is what's actually called when
  wrapping: it splits on the preview's own line breaks first, then
  word-wraps each of those lines independently, capping the total at
  `maxWrapLines`. The wrap width itself comes from `dynamicWrapWidth`,
  computed from the table's actual current rendered width
  (`table.GetInnerRect()`) minus `messagesOtherColumnsWidth` (an
  estimate of every other column's width, from their `MaxWidth` caps) —
  not a fixed constant. Two fixed-width constants were tried first (80,
  then 70) and both eventually proved wrong live: a fixed width can
  still exceed the *real* remaining space once every other column's
  actual width is subtracted (this is why `messageColumns`' `TYPE` also
  got a `MaxWidth` safety cap — otherwise it has no bound at all), and
  when that happens `tview` silently re-clips individual wrapped lines
  with its own `…` on top of the intentional line breaks — a confusing
  double-truncation. This can only be as accurate as `GetInnerRect()` is
  at the moment `renderRows()` calls it (needs the table laid out by its
  parent at least once — true in practice by the time a user can press
  `w` at all), and goes stale relative to a manual terminal resize until
  the next reload/toggle — the same accepted trade-off a fixed width
  already had, just tracking the real column far more often now instead
  of never.
- Per-column `Expansion`/`MaxWidth` values are defined once, in
  `messageColumns` (`messages.go`) — used by **both** the header row and
  every data row, rather than each setting its own. Found live: with
  every column (including the header) independently claiming
  `Expansion(1)`, `tview.Table` computes a column's effective expansion
  as the max across every row in it, including the header — so the
  header's blanket value silently overrode data cells' own (lower, or
  unset/0) intent. Most visibly, the mark column — which never wants
  any extra width — grew on resize anyway, since the header row's `""`
  label cell was still claiming `Expansion(1)`. The equivalent tables
  live in `logSearchColumns` (`logsearch.go`) and `datadogLogsColumns`
  (`datadoglogs.go`).

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
