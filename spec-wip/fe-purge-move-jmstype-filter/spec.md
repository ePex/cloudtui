# Feature: optional JMS Type filter on purge and move-all

Date: 2026-08-25

## Problem

Purge (`p`) and move-all (`M`) in the Queues view always act on **every**
message on the queue — spec/09 documents this as deliberate today ("No
partial moves by selector — always moves everything on the source
queue"). But `queue.Backend` already has `DeleteMessages(ctx, queueName,
filter)` and `MoveMessages(ctx, sourceQueue, targetQueue, filter)`, fully
implemented, tested, and JMSType-filter-capable on **both** backends
(mq-proxy pushes the filter down to its `list-messages`/move endpoints;
Jolokia browses and filters client-side, then acts per matched message —
see `internal/queue/jolokia/filter.go`, `internal/queue/proxy/proxy.go`).
`PurgeQueue`/`MoveAllMessages` are the only entry points wired into the
UI, and neither takes a filter. This is purely a UI gap, not a backend
one.

## Feature

Add an optional JMS Type filter step to both purge and move-all, sharing
one new small overlay.

1. Pressing `p` or `M` on a queue row first opens a new, minimal
   **JMS Type filter prompt**: a single bordered `tview.InputField`
   (not a full `tview.Form` — one field doesn't need one), title
   reflecting the action ("Purge — JMS Type (optional)" /
   "Move All — JMS Type (optional)"). Enter continues, Esc cancels the
   whole operation (same as pressing Esc today skips purge/move
   entirely).
2. The field has autocomplete, styled and behaving like the message
   filter overlay's JMS Type field (spec/08) — **but only the opt-in scan
   tier**, no free tier: unlike the Messages view, no messages for this
   queue have necessarily been browsed yet from the Queues view, so
   there's no already-loaded set to suggest from for free. The
   always-present "↻ Scan up to 5,000 messages for JMS types" entry
   browses *this* queue on demand.
3. Leaving the field blank and pressing Enter proceeds exactly as today
   (no filter) — this is the common case and must stay exactly as fast
   and robust as it already is (see "Preserving the existing unfiltered
   path" below). **Found live (see `tasks.md`'s implementation record):**
   since this prompt has no free suggestion tier, the scan-trigger
   sentinel is unavoidably the *only* autocomplete entry on a fresh,
   untouched field — and tview's `InputField` intercepts Enter to accept
   whatever's highlighted in an open drop-down before the field's own
   "I'm done" handler ever runs. Without an explicit fix
   (`field.SetInputCapture` intercepting Enter-on-empty-field to bypass
   that drop-down-accept logic entirely), a single Enter on a blank
   field would always trigger an unwanted scan instead of proceeding
   unfiltered, contradicting this exact requirement.
4. Entering/selecting a JMS Type and pressing Enter proceeds with that
   filter:
   - **Purge**: the existing confirmation dialog still appears next,
     with its question text mentioning the type (`Purge "<queue>"? All
     <type> messages will be deleted.` vs. today's `All messages will be
     deleted.`). On **Yes**, calls `DeleteMessages(ctx, queue,
     MessageFilter{JMSType: type})` instead of `PurgeQueue`.
   - **Move-all**: the existing move-picker (target queue selection)
     still appears next, unchanged. On selecting a target, calls
     `MoveMessages(ctx, source, target, MessageFilter{JMSType: type})`
     instead of `MoveAllMessages`. No new confirmation step — picking a
     target remains the confirmation, same philosophy as today's
     unfiltered move-all.
5. Status/error reporting after the action stays as it is today (status
   bar count for move, reload-and-refresh for purge) — filtering doesn't
   change what happens after the backend call returns, only which
   backend call is made and with what filter.

## Preserving the existing unfiltered path

When no JMS Type is entered, purge/move-all **keep calling
`PurgeQueue`/`MoveAllMessages`** exactly as today — not
`DeleteMessages`/`MoveMessages` with an empty filter. This matters
because Jolokia's `PurgeQueue`/`MoveAllMessages` use single native JMX
selector calls (`removeMatchingMessages("TRUE")` /
`moveMatchingMessagesTo("TRUE", target)`, with further fallback tiers for
`PurgeQueue` — see spec/09), while `DeleteMessages`/`MoveMessages` browse
every message and act on each individually — much heavier on a large
queue. The new filtered path is a genuinely new, slower-per-message
capability layered on top; the existing fast path for the common
(unfiltered) case is untouched.

## Considered, not implemented: native selector-based filtered purge/move for Jolokia

Jolokia's `removeMatchingMessages`/`moveMatchingMessagesTo` JMX
operations already accept an arbitrary JMS selector string, not just the
hardcoded `"TRUE"` used today — meaning a *filtered* purge/move could
also use a single native call (e.g. selector `JMSType = '<type>'`)
instead of browsing and acting per message, matching the unfiltered
path's efficiency. Not pursued here: it would mean either changing
`queue.Backend`'s `PurgeQueue`/`MoveAllMessages` signatures to accept an
optional filter (a breaking interface change touching every
implementation and call site) or adding new interface methods
specifically for this, both larger changes than this feature's UI gap
warrants. `DeleteMessages`/`MoveMessages` (browse-then-act-per-message)
already exist, are already tested on both backends, and are "fast enough
in practice for the queue sizes this tool manages" (spec/09's own words
about the existing browse-and-remove purge fallback tier) — a reasonable
default until real usage shows otherwise. Worth revisiting as a
follow-up if filtered purge/move turns out to be common on large queues.

## Scope

- In scope: the JMS Type filter prompt (new, shared by both actions);
  routing purge/move-all through `DeleteMessages`/`MoveMessages` only
  when a filter is actually entered; the scan-only autocomplete tier for
  this new prompt.
- Out of scope: the native-selector optimization above; any filter
  field besides JMS Type (From/To date, Max Count are not exposed here
  — this mirrors the message filter overlay's own JMS-Type-only
  autocomplete, not a general bulk-filter builder); a preview of how many
  messages would be affected before confirming; changing `MessageFilter`
  (spec/08) itself; multi-queue bulk purge (already out of scope per
  spec/09).

## Manual verification

Unit-testable for the prompt's suggestion/routing logic (scan-only
suggestions; blank input calls the unfiltered path; a filled-in type
calls the filtered path with the right arguments) via a fake backend.
Given this touches real destructive queue operations, use the
`verify-live` skill against a real broker (both backends): purge/move-all
with no filter still behaves exactly as before; purge/move-all with a
JMS Type filter only affects matching messages, leaving others in place
(and, for move, correctly arriving at the target queue).
