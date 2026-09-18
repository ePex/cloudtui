# Implementation plan

## Approach

Widen `queue.Backend.SendMessage` from `(ctx, queueName, body string)` to
take a new `queue.SendMessageRequest` struct carrying all four fields,
mirroring the existing `MessageFilter` convention (a struct for the
"several optional fields" case, queue name kept as its own positional
argument to match `DeleteMessages`/`MoveMessages`). Both concrete
backends, the `secretbackend` passthrough decorator, the dev-only seed
tool, and every test fake are updated to the new signature. The send
overlay becomes a `tview.Form` (replacing today's bare `TextArea` +
`tview.List`) with fields for JMS Type, Correlation ID, Group ID, and
Headers, reusing `MessageFilter`'s established form/validation/status-bar
pattern rather than inventing a new one.

## `queue.Backend` (`internal/queue/backend.go`)

```go
// SendMessageRequest carries the fields for composing a new message.
// JMSType and Body are required; CorrelationID, GroupID, and Headers are
// optional — a zero value means "don't set this" on the outgoing message.
type SendMessageRequest struct {
	JMSType       string
	Body          string
	CorrelationID string
	GroupID       string
	Headers       map[string]string
}
```

```go
SendMessage(ctx context.Context, queueName string, req SendMessageRequest) error
```

No change to `Message`/`MessageFilter` — this is compose-only, per spec.

## Jolokia (`internal/queue/jolokia/mutate.go`)

Build the headers `Map` argument instead of passing `map[string]string{}`.
Custom `Headers` entries are appended, never allowed to override a
reserved key — any entry named `JMSType`, `JMSCorrelationID`, or
`JMSXGroupID` is dropped before the map is built, then the dedicated
fields are set:

```go
reserved := map[string]bool{"JMSType": true, "JMSCorrelationID": true, "JMSXGroupID": true}
headers := map[string]string{}
for k, v := range req.Headers {
	if reserved[k] {
		continue
	}
	headers[k] = v
}
headers["JMSType"] = req.JMSType
if req.CorrelationID != "" {
	headers["JMSCorrelationID"] = req.CorrelationID
}
if req.GroupID != "" {
	headers["JMSXGroupID"] = req.GroupID
}
```

**Design decision: the three dedicated fields always win; a custom header
using one of their reserved names is silently dropped rather than
applied.** Dropping (not just ordering-so-it-loses) also sidesteps
provider-specific risk: `JMSType` in particular is a real reserved JMS
header name, and calling `setStringProperty("JMSType", ...)` on a stricter
JMS provider than ActiveMQ can throw rather than just being a harmless
no-op — never issuing that call at all is the safer contract.

The `sendTextMessage(java.util.Map,...)` operation's `Map` argument is
ActiveMQ's own vehicle for `JMSType`/`JMSCorrelationID`/reply-to-style
headers (special-cased) plus arbitrary properties (everything else,
including `JMSXGroupID`, via `setStringProperty`) — this is inferred from
ActiveMQ's known `BrokerView` implementation, not something vendored or
previously verified in this repo, so it's re-confirmed live in the
testing section below before being relied on.

## mq-proxy client (`internal/queue/proxy/proxy.go`)

`sendMessageRequest` (the wire struct) already has all four fields
(`JMSType`, `Headers`, `GroupID`, `CorrelationID`) — `SendMessage` just
needs to populate them from the new `queue.SendMessageRequest` instead of
hardcoding `JMSType: "text"` and leaving the rest zero-valued:

```go
func (c *Client) SendMessage(ctx context.Context, queueName string, req queue.SendMessageRequest) error {
	wireReq := sendMessageRequest{
		TargetQueue:   queueName,
		JMSType:       req.JMSType,
		Headers:       req.Headers,
		GroupID:       req.GroupID,
		Body:          req.Body,
		CorrelationID: req.CorrelationID,
	}
	...
}
```

## mq-proxy server (`mq-proxy/src/main/kotlin/.../service/BrokerService.kt`)

The wire contract and DTO need no change — `SendMessageRequest` (Kotlin)
already has all four fields. But `sendMessage`'s current field order
(`BrokerService.kt:154-157`) applies `request.headers` *last*, after
`jmsType`/`correlationId`/`groupId` — so a colliding custom header
currently wins there today, the opposite of what we want. To keep the
"headers append, dedicated fields always win" contract true end-to-end
(not just from the Jolokia side), this needs the matching fix: filter the
three reserved keys out of `headers` before applying them, same rule as
the Go client above:

```kotlin
val reservedHeaderKeys = setOf("JMSType", "JMSCorrelationID", "JMSXGroupID")
val message = session.createTextMessage(request.body)
message.jmsType = request.jmsType
request.correlationId?.let { message.jmsCorrelationID = it }
request.groupId?.let { message.setStringProperty("JMSXGroupID", it) }
request.headers
	?.filterKeys { it !in reservedHeaderKeys }
	?.forEach { (key, value) -> message.setStringProperty(key, value) }
```

This is a small, narrowly-scoped correctness fix directly required by
this feature's own contract — not a drive-by change to an unrelated part
of `mq-proxy`. It touches a different module/toolchain (Kotlin/Gradle,
not `tui/`'s Go), called out explicitly since it's a bigger scope jump
than "just the TUI catching up to what the backends already do."

## `secretbackend` passthrough (`internal/queue/secretbackend/secretbackend.go`)

Trivial signature update — `SendMessage` here just forwards to
`cur.SendMessage(ctx, queueName, req)`, no new logic.

## Dev-only seed tool (`internal/seed/seed.go`, used by `cmd/seedqueue`)

`seed.Sender`'s narrow interface and `Run`'s call site are updated to the
new signature. The seeded sample messages get `JMSType: "text"` (matching
today's previous default, now made explicit) and nothing else —
Correlation ID/Group ID/Headers stay unset. Auto-generating a Correlation
ID is a UI default for the interactive compose flow (see below), not a
backend-level guarantee, so this dev tool is unaffected either way; it's
out of this feature's scope to change what the seed tool sends beyond
what's needed to compile against the new interface.

## Send overlay (`internal/dialog/sendmessage.go`)

Rebuilt on `tview.Form` (matching `MessageFilter`'s pattern), replacing
the current `TextArea` + `tview.List`:

1. **JMS Type** — `tview.InputField`. Mandatory.
2. **Correlation ID** — `tview.InputField`. Pre-filled with a generated
   UUID every time `Show()` opens the overlay (visible, editable, not a
   hidden side effect — per spec). If it's somehow empty at submit time
   (the user cleared it), a fresh UUID is generated right before sending
   rather than sending it blank.
3. **Group ID** — `tview.InputField`. Optional, blank by default.
4. **Headers** — a `tview.TextArea` labeled to indicate `key: value` per
   line; optional, blank by default. Parsed on submit into
   `map[string]string`, skipping blank lines; a line with no `:`
   separator is a validation error (status bar, form stays open),
   consistent with `MessageFilter.apply()`'s existing parse-error
   handling. See "Alternatives considered" for why this shape over a
   repeating add-row widget.
5. **Body** — the existing `tview.TextArea`, added to the form via
   `Form.AddFormItem` (confirmed `TextArea` satisfies tview's
   `FormItem` interface in v0.42.0 — `SetDisabled`/`SetFinishedFunc`/
   `SetFormAttributes` all return `FormItem`). Mandatory (unchanged
   behavior, now explicitly validated instead of implicitly allowed to
   be empty).
6. **Submit / Cancel** — `Form.AddButton`, same as `MessageFilter`.

Validation on submit (mirroring `MessageFilter.apply`'s
parse-then-report-via-status-bar shape): blank JMS Type or blank Body
blocks submission with a status-bar error and keeps the overlay open;
a malformed Headers line does the same.

A small pure function generates the default Correlation ID:

```go
// newCorrelationID returns a random RFC 4122 v4 UUID string.
func newCorrelationID() string
```

Implemented with `crypto/rand` (16 random bytes, version/variant bits
set per RFC 4122, hex-formatted with dashes) — no new dependency, per
`tui/CLAUDE.md`'s "justify every new dependency" rule; this is a dozen
lines against the standard library.

## Fakes / call sites needing a signature update

Everywhere implementing or calling the old
`SendMessage(ctx, queueName, body string)`:
- `internal/dialog/dialogtest_test.go:60` (`fakeQueueBackend`)
- `internal/app/host_test.go:189` (`fakeBrowseBackend`)
- `internal/view/queues_test.go:69` (`fakeQueueBackend`)
- `internal/queue/proxy/proxy_test.go`, `internal/queue/jolokia/mutate_test.go`,
  `internal/queue/secretbackend/secretbackend_test.go` — updated to
  exercise the new fields, not just adjusted to compile.
- `internal/seed/seed.go` + `internal/seed/seed_test.go`.
- `internal/dialog/sendmessage.go`'s own call to
  `host.Backend().SendMessage(...)`.

No changes needed in `internal/view/queues.go`/`messages.go`/
`internal/app/app.go` themselves — they only wire up `*SendMessageOverlay`
by reference, never call `SendMessage` directly.

## Testing

- **Jolokia** (`mutate_test.go`): assert the `sendTextMessage` call's
  headers-map argument contains `JMSType` always, `JMSCorrelationID`/
  `JMSXGroupID` only when non-empty, non-reserved custom headers pass
  through, and — the precedence decision above — a custom header named
  `JMSType`/`JMSCorrelationID`/`JMSXGroupID` is dropped, not applied, so
  the dedicated field's value is the only one that ends up in the map.
- **Proxy** (`proxy_test.go`): assert the posted JSON body carries
  `jmsType`, `headers`, `groupId`, `correlationId` from the request.
- **mq-proxy** (`BrokerServiceTest.kt`): update the existing `sendMessage
  sets jmsType, correlationId, groupId, and custom headers` test if the
  reorder affects mock call expectations, and add a new case — a
  `headers` entry named `JMSXGroupID` (or `JMSType`/`JMSCorrelationID`)
  must not result in `setStringProperty` being called with that reserved
  key, only the dedicated-field call (`textMsg.jmsType =`, etc.) fires.
- **`sendmessage.go`**: unit tests for the pure functions —
  `newCorrelationID` (format: matches UUID v4 shape, no two calls equal),
  the headers-textarea parser (valid lines, blank lines skipped, a
  malformed line errors), and the submit-validation rule (blank JMS
  Type/Body rejected). These are the same kind of directly-tested,
  side-effect-free helper this codebase already carves out elsewhere
  (`QueuesView.doPurge`/`doMoveAll`, `MessageFilter.handleScanResult`) —
  the overlay's actual tview wiring stays covered by live verification,
  same as today (`SendMessageOverlay` has no dedicated tview-level tests
  currently, per spec/09).
- **Fakes**: update signatures in the three fake-backend files above;
  `fakeQueueBackend`/`fakeBrowseBackend` capture the passed
  `SendMessageRequest` where a test needs to assert on it (queues_test.go
  already has call sites that will want this for a "fields reach the
  backend" assertion).
- **Live verification** (`verify-live` skill), against both backends:
  1. Send a message with JMS Type, Correlation ID, Group ID, and a custom
     header set, on the Jolokia backend. Open it in the detail view (spec/08)
     and confirm JMS Type/Correlation ID show the entered values, and
     check the raw fields for the group ID and custom header actually
     landing as message properties — this is the step that confirms the
     ActiveMQ `Map` header behavior inferred above actually holds.
  2. Repeat against mq-proxy (`task dev:proxy:start`).
  3. Send with only JMS Type + Body (everything else left blank) on both
     backends — confirm it still sends, Correlation ID shows the
     auto-generated UUID, and no Group ID/custom properties appear.
  4. Send with a custom header literally named `JMSXGroupID` set to a
     different value than the Group ID field, on both backends — confirm
     the message's actual group comes from the Group ID field, not the
     header, proving the "dedicated field always wins" contract holds
     for real, not just in unit tests with mocked JMS objects.
  5. Attempt to submit with a blank JMS Type, and with a malformed
     Headers line — confirm the overlay stays open with a status-bar
     error each time.

## Alternatives considered

- **Repeating add/remove-row widget for Headers**, instead of a
  `key: value`-per-line `TextArea`. Rejected as disproportionate UI
  complexity for this feature — no other dialog in this codebase builds
  a dynamic repeating-row widget, and the multi-line text convention is
  already how this form-based dialog handles its other free-text field
  (Body). Can revisit if the flat-text UX proves awkward in practice.
- **Generating the Correlation ID inside each backend** (so any caller,
  UI or otherwise, always gets a non-blank one) instead of only in the
  overlay. Rejected: it would duplicate the same generation logic in two
  backend implementations for a guarantee that's really a UX default for
  the interactive compose flow, not a backend invariant — the mq-proxy
  and Jolokia wire formats both already treat a blank Correlation ID as
  "don't set it," which is the correct, simpler backend behavior to keep
  uniform. The seed tool (the only other caller) doesn't need this
  guarantee.
- **A JMS Type autocomplete/picker on send**, mirroring purge/move-all's
  `JMSTypePrompt` (spec/09). Left out per spec's "Out of scope" — flagged
  there as a natural follow-up, not bundled in here.
- **Letting a colliding custom header override the dedicated field**
  (matching mq-proxy's original, unfixed field order). Rejected per
  explicit feedback: headers are additive/appended, never a way to
  override JMS Type/Correlation ID/Group ID that already have their own
  dedicated inputs — silently dropping the reserved-name header entry
  is more predictable than either "last write wins" or a submit-time
  validation error over something this minor.

## No new dependencies

UUID generation is implemented against `crypto/rand`, not a library.
