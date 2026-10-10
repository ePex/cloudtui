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
- **Messages list**: `p` purges the currently open queue — the exact same
  flow, through the same code (`purgeFlow`, see Implementation notes), so
  the two entry points can't drift apart.
- **Message detail view**: `d` removes only the single message being
  viewed, then returns to the messages list.

Both `p` entry points first open the **JMS Type filter prompt** (a single
bordered `tview.InputField`, title `Purge "<queue name>" — JMS Type
(optional)`) — see "Optional JMS Type filter" below for its shared shape
with move-all. Leaving it blank and pressing Enter purges every message.
Entering a type narrows what's purged and changes the confirm dialog's
wording accordingly.

Purge then shows a confirmation dialog (`" Confirm "` title, question
`Purge "<queue name>"? All messages will be deleted.` — or, with a JMS
Type entered, `Purge "<queue name>"? All <type> messages will be
deleted.`) with **No** as the default focus (prevents accidental
deletion) and **Yes** to proceed. **No**/Esc dismisses without changes.

On **Yes**, the purge runs in the background. On success the status bar
shows how many messages were deleted — `Purged <n> message(s) from
"<queue name>"`, or `Purged <n> <type> message(s) from "<queue name>"`
with a JMS Type — and the view it was started from (queue list or
messages list) reloads. A purge with a JMS Type may add a warning that
matching messages further back weren't reached (see "Broker limits on
filtered purge/move-all" below). On error, the view shows the error
instead.

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
  messages moved (`Moved <n> message(s) from "<src>" to "<dst>"`) —
  plus, with a JMS Type, the same possible warning as purge (see "Broker
  limits on filtered purge/move-all" below). On error: status bar shows
  the error, queue list unchanged.

### Broker limits on filtered purge/move-all

ActiveMQ only matches a consumer with a JMS selector — which is what a
purge or move-all with a JMS Type uses on the proxy backend — against the
messages it has paged into memory from the head of the queue (broker
policy `maxPageSize`, default 200). Matching messages further back than
that are never handed out: with 800 × TypeA followed by 800 × TypeB,
purging `TypeB` deletes 0. How many it reaches varies with how much of
the queue the broker happens to have paged in. (The Jolokia backend's
filtered path browses first and acts per message, so it's bounded by the
broker's browse limit, `maxBrowsePageSize`, default 400, instead — the
same kind of limit, and the same warning applies.) JMS offers no
side-effect-free way around this (see Implementation notes), and Amazon
MQ exposes no JMX, so the TUI makes the limit visible instead:

- After a successful purge or move-all **with a JMS Type**, the TUI reads
  the queue's pending count. If more than 200 messages remain
  (`filteredWindowWarnAt`, ActiveMQ's default `maxPageSize`), the result
  line gets the suffix `— <m> message(s) remain; matching messages
  further back may not have been reached`, e.g. `Purged 0 TypeB
  message(s) from "orders" — 1600 message(s) remain; matching messages
  further back may not have been reached`.
- "May": the TUI can't tell whether the remaining messages include
  matches. The threshold is a heuristic — on a broker configured with a
  larger window the warning shows more often than needed, with a smaller
  one it can miss.
- Unfiltered purge/move-all never warn: an unfiltered consumer reaches
  every message.
- If the pending count can't be read, there's no warning (logged); the
  operation itself still succeeded.
- **Remedy** for a queue that really needs filtered operations beyond
  the window: raise the broker's `maxPageSize` policy for that
  destination. Neither the TUI nor mq-proxy changes broker
  configuration.

### Optional JMS Type filter (purge and move-all)

Both `p` and `M` share one prompt type (`JMSTypePrompt`) for narrowing
what they act on:

- A single bordered `tview.InputField` (not a full `tview.Form` — one
  field doesn't need one). Keys:
  - `Enter` continues with **exactly what's typed** — blank means no
    filter — **immediately, regardless of whether the automatic scan
    below has finished**.
  - `Tab` keeps the typed text in the field.
  - Neither ever starts a scan or picks a suggestion on its own: a
    suggestion is only accepted after the user moves the highlight onto
    it with the arrow keys (or clicks it). Then `Enter`/`Tab` put that
    type in the field (a second `Enter` continues), or, on the "↻ Scan
    up to …" entry, start the wider scan. See "JMS Type autocomplete"
    in Implementation notes.
  - `Esc` cancels the whole purge/move-all operation.
- Text typed while a scan runs stays; the scan's types become
  suggestions for it.
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
  truly unbounded scan isn't possible on that backend either). Either
  scan stops at the broker's browse limit (400 by default) however much
  it asks for; when the queue holds more, the status line says so —
  `Scanned the first <n> messages — the queue holds more; types further
  back aren't listed` (spec/08). This
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
  a bordered `tview.Form` with fields for JMS Type, Correlation ID, Group
  ID, Headers, and Body, plus Submit/Cancel/**Load snippet…** buttons.
- **Load snippet…** opens the snippet picker. Picking a snippet fills in
  JMS Type and Body only, asking first if either already holds text; see
  spec/22.
- **JMS Type** and **Body** are mandatory; submitting with either blank
  shows a status-bar error and keeps the overlay open (mirroring
  `MessageFilter.apply`'s pattern) rather than sending.
- **Correlation ID** is pre-filled with a freshly generated RFC 4122 v4
  UUID every time the overlay opens — visible and editable, not a hidden
  default. Clearing it and submitting anyway regenerates a fresh UUID
  right before sending rather than sending it blank.
- **Group ID** is optional and blank by default — unset means no
  `JMSXGroupID` is set on the outgoing message.
- **Headers** is an optional, free-form `key: value`-per-line text area
  for custom message properties. Blank lines are skipped; a line with no
  `:` separator (or an empty key) is a validation error, same treatment
  as a blank JMS Type/Body. **A header entry named `JMSType`,
  `JMSCorrelationID`, or `JMSXGroupID` is dropped, never applied** — those
  three already have their own dedicated field, and a header can only add
  to the message, never override what the dedicated fields set.
- On submit: message is sent, overlay closes, the queues list (and the
  messages list, if open) reload, status bar confirms.
- On cancel/Esc: overlay closes, focus returns to the caller.

## Data & config

```go
// queue.Backend interface additions
PurgeQueue(ctx context.Context, queueName string) (int, error) // returns count deleted (on error: deleted so far)
DeleteMessage(ctx context.Context, queueName, messageID string) error
MoveMessage(ctx context.Context, queueName, messageID, targetQueue string) error
MoveAllMessages(ctx context.Context, sourceQueue, targetQueue string) (int, error) // returns count moved
SendMessage(ctx context.Context, queueName string, req SendMessageRequest) error

// SendMessageRequest: JMSType and Body are required; CorrelationID,
// GroupID, and Headers are optional — a zero value means "don't set
// this" on the outgoing message.
type SendMessageRequest struct {
	JMSType       string
	Body          string
	CorrelationID string
	GroupID       string
	Headers       map[string]string
}

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
- **Purge count per Jolokia tier**: `purgeQueue()` reports nothing, so
  the queue's `QueueSize` attribute is read just before it; a message
  arriving in between makes the count off by that many. If `QueueSize`
  can't be read, tier 1 is skipped so the count is never made up.
  `removeMatchingMessages` returns the number it removed (Jolokia
  `value`); browse-and-remove counts successful removals and, on a
  failed one, returns the count so far with the error. The proxy
  backend's count is the number of distinct message IDs its rounds
  reported, minus any it kept reporting at the end (spec/11). A proxy
  purge or move-all that stops for lack of progress shows the
  "incomplete" error (with both numbers) instead of the result line.
- **Single move**: Jolokia operation `moveMessageTo(java.lang.String,
  java.lang.String)` on the source queue MBean, args `[messageID,
  targetQueueName]` — reliable across deployments.
- **Move all**: Jolokia operation `moveMatchingMessagesTo(java.lang.String,
  java.lang.String)` with selector `"TRUE"` (matches everything).
- **Send**: implemented via the Jolokia `exec` operation
  `sendTextMessage(java.util.Map,java.lang.String,java.lang.String,java.lang.String)`
  on the destination's Broker MBean, args `[headers, body, username,
  password]` — **not** the single-arg `sendTextMessage(java.lang.String)`
  overload, which fails with `User name [null]` because it never receives
  credentials. The `Map`+3-string-arg overload is what actually creates a
  real JMS `TextMessage`, fully browsable afterward, with the broker's own
  credentials enforced. No STOMP, no separate transport/port — this reuses
  the same Jolokia HTTP connection as every other queue operation.
- **The headers `Map` argument is where JMS Type/Correlation ID/Group
  ID/custom headers all get set** (confirmed live against a real broker,
  not just inferred from ActiveMQ's public API): `headers["JMSType"]` is
  always set from `SendMessageRequest.JMSType`; `"JMSCorrelationID"`/
  `"JMSXGroupID"` are set only when `CorrelationID`/`GroupID` are
  non-empty; `SendMessageRequest.Headers`' custom entries are copied in
  last, **except** the three reserved key names above, which are dropped
  rather than applied — the dedicated field always wins, a header can
  only add to the message, never override what a dedicated field set
  (`sendMessageHeaders`, `internal/queue/jolokia/mutate.go`). mq-proxy's
  `BrokerService.sendMessage` (spec/11) enforces the same rule, so the two
  backends behave identically on a colliding header name.
- **Side effect**: sending via `sendTextMessage` embeds a stale
  VM-transport connection reference into the sent message. The next
  `browseMessages()` call against *that queue* then throws
  `IllegalStateException: Error while extracting clientID` for the whole
  queue, not just the sent message — this is exactly why `BrowseMessages`
  (spec/08) falls back to the simpler `browse()` JMX operation
  whenever `browseMessages()` errors, rather than surfacing the error.
  On ActiveMQ 5.18+ `browse()` returns full message objects with a real
  `JMSMessageID`, so delete and move keep working on the fallback path.
  Only older brokers' plain-string `browse()` results carry no ID. For
  those, the messages list shows a yellow "limited message info" note,
  and individual move/delete/mark are unavailable (spec/08).
- **Remove and move report "not found" in the result, not the status**:
  `removeMessage` and `moveMessageTo` return a JSON boolean in Jolokia's
  `value`, with HTTP/Jolokia status still 200 when the message wasn't
  found. The backend treats `value == false` as an error ("message not
  found in queue"), rather than as success.
- The queue picker (shared by single-move and move-all) is a `tview.List`
  overlay using the same centered pattern as the confirm dialog; queue
  names are loaded via `backend.List()` in a goroutine, showing
  `"Loading…"` while in flight.
- Files: `tui/internal/dialog/movepicker.go` (see spec/03) for the picker;
  purge/move/send wiring lives alongside `QueuesView`/`MessagesView`/
  `MessageDetailView` (spec/07, 08); Jolokia implementations in
  `internal/queue/jolokia/`.
- `tui/internal/dialog/jmstypeprompt.go` — `JMSTypePrompt`, the JMS Type
  filter prompt shared by purge and move-all.
- `tui/internal/view/purge.go` — `purgeFlow`, the purge action both
  `QueuesView` and `MessagesView` call for `p`: JMS Type prompt →
  confirm → backend call → status line. Each view passes its own
  callbacks for closing the prompt (restore focus and shortcut hints),
  success (reload) and error (show it). A purge inside an open queue
  used to skip the prompt and always purge everything, while the queue
  list's `p` asked for a type; one shared flow keeps them identical.
  `purgeFlow.finish` (the result handling) is split out of the
  background goroutine so it's unit-tested directly; `filteredRemainderWarning`
  and `pendingCount` (also used by move-all and the message browser's
  title, spec/08) live there too.
- `purge`/`doMoveAll` are the small, directly unit-tested dispatch
  functions that branch between the native unfiltered calls and the
  filtered ones — separated out specifically so that branch is testable
  without driving the confirm dialog or move-picker's own async
  selection flow. The flows themselves are verified by driving the real
  binary against a real broker (spec/13).
- **Why filtered purge/move-all don't work around the broker's limits**
  (verified live): browsing sees only the first `maxBrowsePageSize`
  (400) messages, filtered or not; consuming everything and
  acknowledging only matches (ActiveMQ's individual-acknowledge mode)
  stalled after 800 of 1,600 messages and handed every other message
  back flagged as redelivered and out of order; moving everything to a
  temporary queue and moving the rest back would give every kept
  message a new ID and timestamp. All change or risk real data, so none
  is attempted; the limit is reported instead (see "Broker limits on
  filtered purge/move-all").
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
- **JMS Type autocomplete** (`tui/internal/dialog/jmstypeautocomplete.go`,
  shared with the message filter's JMS Type field, spec/08). Two tview
  v0.42 behaviors made the plain wiring lose what the user typed:
  - While the drop-down is open, `InputField` treats **Tab exactly like
    Enter**: "accept the highlighted entry", before the field's own
    Enter/Tab handling runs.
  - When it rebuilds the drop-down it **keeps whichever entry was
    highlighted before**. The scan entry is the only entry on the very
    first build, so it stayed highlighted until the user used the arrow
    keys.

  So Enter or Tab on an empty field, an unlisted type, or even a
  matching prefix started a 5,000-message scan and lost the text. The
  shared `jmsTypeAutocomplete` fixes it:
  - It tracks `chosen`: the arrow keys moved the highlight since the
    last text change (tview's `AutocompletedNavigate`). Its
    `SetAutocompletedFunc` callback leaves the text alone on navigation,
    so tview doesn't rebuild the drop-down and it stays open (tview
    only rebuilds when a key changed the text).
  - Its input capture, on Enter/Tab without `chosen`, closes the
    drop-down — tview has no public "close", but `Autocomplete()` with
    no entries removes it — and lets the key through to its normal job.
  - With `chosen`, the callback puts a real type in the field (tview
    leaves setting the text to the callback) or calls the dialog's
    `onScan` for the scan entry.
  - Up/Down with a **single** suggestion count as choosing it: tview
    can't move the highlight in a one-entry list, so it reports no
    navigation — and that's typically the scan entry alone, when there
    are no types to suggest yet (found live).
  - The scan entry's text never goes into the field. That retired three
    workarounds: an Enter-on-empty input capture, clearing the field
    after every scan (which wiped text typed during the automatic scan),
    and refusing to continue while the field held the scan entry.
- **`startScan` reads the queue name on the UI goroutine**, before
  starting the scan's goroutine. It used to read `jp.queueName` inside
  the goroutine, racing with a reopened prompt's `Show` writing it
  (caught by `go test -race`).
- **Tests that let a scan start need a non-returning fake
  `ScanJMSTypes`** (`blockForever`) unless they wait for completion
  through a channel: the test host runs `QueueUpdateDraw` inline, so a
  scan that completes runs its result handler on the scan's goroutine,
  concurrently with the test. A theming test missed this and failed
  under `-race`; CI now runs the TUI tests with `-race` on Linux
  (spec/02). Key behavior is tested by sending real key events through
  the field's input handling (`press`/`navigateTo` in
  `jmstypeautocomplete_test.go`), input capture included.

## Out of scope (deliberate)

- Progress indicators for large purges/moves; no undo.
- Reaching matching messages beyond the broker's paging/browse limits
  for filtered purge/move-all (see "Broker limits on filtered
  purge/move-all"), and changing broker configuration.
- Bulk-purge of multiple queues at once.
- Creating a new destination queue on the fly from the picker.
- Message templates or send history.
- A JMS Type autocomplete/picker on send, mirroring purge/move-all's
  `JMSTypePrompt` above — a plain text field only.
- Validating header keys/values against JMS naming restrictions beyond
  the three reserved keys handled above — anything else is sent through
  as typed.
- Editing headers/properties on an *existing* message — this is
  compose-only; the message detail view (spec/08) remains read-only.
- Filtering purge/move-all by anything other than JMS Type (From/To
  date, Max Count) — mirrors `MessageFilter`'s own JMS-Type-only
  autocomplete (spec/08), not a general bulk-filter builder.
- A preview of how many messages a filter would affect before confirming.
- The native-selector optimization for filtered Jolokia purge/move
  described above.
