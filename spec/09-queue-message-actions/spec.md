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

Both `p` entry points first open the **JMS Type filter prompt** (a single
bordered `tview.InputField`, title `Purge "<queue name>" — JMS Type
(optional)`) — see "Optional JMS Type filter" below for its shared shape
with move-all. Leaving it blank and pressing Enter proceeds exactly as
before. Entering a type narrows what's purged and changes the confirm
dialog's wording accordingly.

Purge then shows a confirmation dialog (`" Confirm "` title, question
`Purge "<queue name>"? All messages will be deleted.` — or, with a JMS
Type entered, `Purge "<queue name>"? All <type> messages will be
deleted.`) with **No** as the default focus (prevents accidental
deletion) and **Yes** to proceed. Selecting **Yes** closes the dialog and
refreshes the queue list; **No**/Esc dismisses without changes.

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

- `M` (capital) in the queues view first opens the JMS Type filter
  prompt (title `Move All "<queue name>" — JMS Type (optional)`), then
  the same move-picker overlay (with the same DLQ-priority ordering and
  `/` search) for the selected queue, then moves either every message on
  it (blank filter) or only the matching ones (a JMS Type entered) to
  the chosen target in one call.
- No confirmation dialog beyond the target selection itself — picking a
  target *is* the confirmation, matching the single-message move's UX.
  The JMS Type prompt ahead of it is not itself a confirmation either —
  it's the filter-entry step, same role it plays for purge.
- On success: reload the queues list, status bar shows the count of
  messages moved. On error: status bar shows the error, queue list
  unchanged.

### Optional JMS Type filter (purge and move-all)

Both `p` and `M` share one prompt type (`JMSTypePrompt`) for narrowing
what they act on:

- A single bordered `tview.InputField` (not a full `tview.Form` — one
  field doesn't need one). `Enter` on a blank field continues with no
  filter (identical to this feature's pre-existing behavior) —
  **immediately, regardless of whether the automatic scan below has
  finished** (see the gotcha further down for why this matters more than
  it might sound like it should); `Enter` with text entered continues
  with that as the JMS Type; `Esc` cancels the whole purge/move-all
  operation, same as `Esc` already did before this filter step existed.
- Autocomplete, styled like the message filter overlay's own JMS Type
  field (spec/08). Unlike the Messages view, the Queues view hasn't
  necessarily browsed any messages for the selected queue, so there's
  nothing already-loaded to suggest from for free — so `Show()` instead
  runs a scan **automatically, the moment the prompt opens**, capped at
  `jmsTypeAutoScanCount` (500), to populate real JMS Type suggestions
  without requiring any action. This runs regardless of whether the user
  ends up wanting a filter at all — leaving the field blank and pressing
  Enter still proceeds immediately with no filter (see below), the
  automatic scan's result is simply discarded in that case. The
  always-present "↻ Scan up to 5,000 messages for JMS types" entry
  (`jmsTypeScanCount`, shared constant with `MessageFilter`) remains as
  an **opt-in way to widen the search further** if the automatic pass
  didn't surface the type wanted — same completeness caveat as spec/08's
  own scan tier applies to both (never guaranteed exhaustive; mq-proxy
  requires a positive `maxCount` on every `list-messages` call, so a
  truly unbounded scan isn't possible on that backend either). This
  design was revised from an initially opt-in-only autocomplete after
  live feedback that a fresh prompt showing only that sentinel — with no
  indication it was an interactive entry rather than a static
  message — read as "nothing is here," not as "select this to see
  available types."
- When a JMS Type is entered, purge routes through
  `DeleteMessages(ctx, queueName, queue.MessageFilter{JMSType: type})`
  and move-all routes through `MoveMessages(ctx, sourceQueue,
  targetQueue, queue.MessageFilter{JMSType: type})` — both already
  existed on `queue.Backend`, implemented and tested on both backends
  (`internal/queue/jolokia/filter.go`, `internal/queue/proxy/proxy.go`),
  but had no UI entry point before this filter prompt. **A blank filter
  keeps using `PurgeQueue`/`MoveAllMessages`** (the pre-existing, faster,
  native-selector calls — see the gotcha below for why this distinction
  is deliberate, not an oversight).

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

// Also on queue.Backend (pre-existing — see spec/08 for MessageFilter's
// shape — but this is their first UI entry point, via the optional JMS
// Type filter above):
DeleteMessages(ctx context.Context, queueName string, filter MessageFilter) (int, error) // returns count deleted
MoveMessages(ctx context.Context, sourceQueue, targetQueue string, filter MessageFilter) (int, error) // returns count moved
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
- `tui/internal/dialog/jmstypeprompt.go` — `JMSTypePrompt`, the JMS Type
  filter prompt shared by purge and move-all. `QueuesView.doPurge`/
  `doMoveAll` (`tui/internal/view/queues.go`) are the small, directly
  unit-tested dispatch functions that branch between the native
  unfiltered calls and the filtered ones — separated out specifically so
  that branch is testable without driving the confirm dialog or
  move-picker's own async selection flow — this codebase has no unit
  tests at that layer for either (`ConfirmDialog`/`SendMessageOverlay`
  have no dedicated test files at all); bugs there have historically
  only been caught by driving the real binary against a real broker
  (spec/13), which is how this feature's own flows are verified too.
- **Why a blank JMS Type filter still calls `PurgeQueue`/
  `MoveAllMessages`, not `DeleteMessages`/`MoveMessages` with an empty
  filter.** The unfiltered calls use a single native JMX selector call
  each (`removeMatchingMessages("TRUE")` / `moveMatchingMessagesTo("TRUE",
  target)`, with `PurgeQueue`'s further fallback tiers above) —
  `DeleteMessages`/`MoveMessages` instead browse every message and act
  on each individually, much heavier on a large queue. The filtered path
  is a genuinely new, slower-per-message capability layered on top; the
  existing fast path for the common (unfiltered) case is untouched by
  this feature.
- **Considered, not implemented: a native-selector-based *filtered*
  purge/move for Jolokia.** `removeMatchingMessages`/
  `moveMatchingMessagesTo` already accept an arbitrary JMS selector
  string, not just the hardcoded `"TRUE"` used today — a filtered
  purge/move could in principle also use one native call (e.g. selector
  `JMSType = '<type>'`) instead of browse-then-act-per-message, matching
  the unfiltered path's efficiency. Not pursued: it would mean either
  changing `PurgeQueue`/`MoveAllMessages`'s signatures to accept an
  optional filter (a breaking interface change touching every
  implementation and call site) or adding new interface methods just for
  this — both bigger changes than this feature's UI gap warranted, given
  `DeleteMessages`/`MoveMessages` already existed, tested, on both
  backends. A candidate follow-up if filtered purge/move turns out to be
  common on large queues.
- **`JMSTypePrompt`'s overlay must be tall enough to leave room for the
  autocomplete drop-down below the field, or the drop-down draws on top
  of the box's own bottom border — found live.** `tview.InputField.Draw`
  positions the drop-down exactly one row below the field's own content
  row, regardless of the box's declared height. `MessageFilter`'s
  overlay (spec/08) never hits this because its box has three more form
  fields and a button row below the JMS Type field, giving the drop-down
  room "for free"; `JMSTypePrompt` is a single field with nothing else
  in the box to borrow room from, so its overlay height
  (`ui.Centered(a.jmsTypePrompt.Primitive(), 64, 12)` in `app.go`) is set
  explicitly generous — 9 spare rows below the field — to comfortably
  cover a typical suggestions list without the drop-down overlapping the
  border.
- **`tview.InputField` accepts an open autocomplete drop-down's
  highlighted entry on Enter before its own `SetDoneFunc` ever runs —
  found live, breaking `JMSTypePrompt`'s "blank field + Enter continues
  unfiltered" contract.** On an untouched field, an open drop-down's
  highlighted entry could be the scan-trigger sentinel (before the
  automatic scan below completes, or if it found nothing), and without
  intervention, the very first Enter on a blank field would accept that
  sentinel and kick off the wider scan, never reaching "continue with no
  filter" at all. Fixed with `field.SetInputCapture`: when
  `event.Key() == tcell.KeyEnter` and the field is genuinely empty, call
  the continue handler directly and swallow the event —
  `SetInputCapture` runs before `InputField`'s own `InputHandler` (per
  `tview.Box`'s own doc comment), so only that exact case is affected;
  typing or navigating into the drop-down with arrows still uses tview's
  normal accept-then-second-Enter flow, same as every other autocomplete
  field in this app.
- **`Show()`'s automatic scan sets `scanning = true` *synchronously*,
  before `Show()` even returns — so gating "continue with no filter" on
  "is any scan in flight" would block it for the entire duration of
  every automatic scan, not just the opt-in wider one.** Found live,
  once the automatic-scan design (above) was added: the field's own
  submit guard originally refused whenever `jp.scanning` was true, which
  was safe under the original opt-in-only design (a scan only started on
  deliberate user action) but became wrong once `Show()` itself started
  a scan unconditionally on every open. The guard now checks a narrower,
  actually-unsafe condition instead: whether the field is *literally*
  showing the scan-trigger sentinel's own text (only possible via the
  opt-in wider scan, which is the only path that ever writes that text
  into the field) — the automatic scan never touches the field's text at
  all while in flight, so it never blocks this path, however long it's
  still running.
- **A directly-set field text that matches the scan-trigger sentinel
  fires the field's real production `SetChangedFunc` wiring, starting a
  genuine background scan — a test-writing trap, not a production bug,
  but one that produced a real `go test -race` failure.** A test
  simulating "the sentinel was just selected" via
  `jp.field.SetText(jmsTypeScanSentinel)` inadvertently triggers the same
  scan a live keystroke would, and with the test's default fake
  `ScanJMSTypes` returning immediately (not blocked), that scan's
  goroutine could call `SetStatus` concurrently with the test's own
  subsequent assertions doing the same, unsynchronized — caught by
  `-race`, not by a plain test run. Every test in this file that starts
  a real scan (via `Show()` or by triggering the sentinel) needs an
  explicitly non-returning fake `ScanJMSTypes` unless it's actually
  waiting on that scan's completion through a channel.

## Out of scope (deliberate)

- Progress indicators for large purges/moves; no undo.
- Bulk-purge of multiple queues at once.
- Creating a new destination queue on the fly from the picker.
- Message headers, properties, JMS type selection, templates/history on
  send — body text only.
- Filtering purge/move-all by anything other than JMS Type (From/To
  date, Max Count) — mirrors `MessageFilter`'s own JMS-Type-only
  autocomplete (spec/08), not a general bulk-filter builder.
- A preview of how many messages a filter would affect before confirming.
- The native-selector optimization for filtered Jolokia purge/move
  described above.
