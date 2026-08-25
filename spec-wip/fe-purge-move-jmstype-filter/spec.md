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
2. The field has autocomplete, styled like the message filter overlay's
   JMS Type field (spec/08). **Revised after a second round of live
   feedback** from the originally-planned "opt-in scan only" design (see
   `tasks.md`'s implementation record for the full account): `Show()`
   now automatically runs a scan the moment the prompt opens, capped at
   `jmsTypeAutoScanCount` (500) — real type names populate the drop-down
   without any action, which is what a user opening this prompt actually
   expects to see. The always-present "↻ Scan up to 5,000 messages for
   JMS types" entry remains as an opt-in way to widen the search further
   (a bigger cap) if the automatic pass didn't surface the type wanted —
   this is the closest this prompt gets to `MessageFilter`'s free/scan
   two-tier shape (spec/08), except tier 1 here still costs a network
   call (unlike `MessageFilter`, the Queues view has no already-loaded
   messages to draw a genuinely free tier from).
3. Leaving the field blank and pressing Enter proceeds exactly as today
   (no filter) — this is the common case and must stay exactly as fast
   and robust as it already is (see "Preserving the existing unfiltered
   path" below), **regardless of whether the automatic scan from step 2
   has finished yet** — a user who just wants to skip filtering should
   never have to wait on it. **Found live (see `tasks.md`'s
   implementation record for the full account, across two separate
   rounds of feedback):** first, before the automatic-scan design
   existed, tview's `InputField` intercepted Enter to accept whatever
   was highlighted in an open drop-down (unavoidably the scan-trigger
   sentinel itself, the only entry on an untouched field) before the
   field's own "I'm done" handler ever ran — fixed with
   `field.SetInputCapture` bypassing that for a genuinely empty field.
   Second, once the automatic scan was added, that fix alone wasn't
   enough on its own to reason about safely: `Show()` now sets an
   in-flight "scanning" flag *synchronously*, before it even returns —
   so continuing to gate submission on "is any scan in flight" (as
   originally implemented for the opt-in-only design) would have made
   blank+Enter block for the *entire* automatic scan every single time
   the prompt opened. The fix's real condition is narrower: refuse only
   when the field is literally showing the scan-trigger sentinel's own
   text (meaning the *opt-in* wider scan was just triggered and hasn't
   cleared it yet) — the automatic scan never puts that text in the
   field at all, so it never blocks this path.
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
  when a filter is actually entered; the prompt's autocomplete
  (automatic scan on open, plus the opt-in wider scan).
- Out of scope: the native-selector optimization above; any filter
  field besides JMS Type (From/To date, Max Count are not exposed here
  — this mirrors the message filter overlay's own JMS-Type-only
  autocomplete, not a general bulk-filter builder); a preview of how many
  messages would be affected before confirming; changing `MessageFilter`
  (spec/08) itself; multi-queue bulk purge (already out of scope per
  spec/09).

## Manual verification

Unit-testable for the prompt's suggestion/routing logic (suggestions
before/after a scan completes; blank input calls the unfiltered path
regardless of scan state; a filled-in type calls the filtered path with
the right arguments) via a fake backend and synchronization around the
now-always-present automatic-scan goroutine Show() starts (`-race`
matters here — found a real, `-race`-flagged synchronization bug of our
own in the test suite while updating it for the automatic-scan design;
see `tasks.md`). Given this touches real destructive queue operations,
use the `verify-live` skill against a real broker (both backends):
purge/move-all with no filter still behaves exactly as before;
purge/move-all with a JMS Type filter only affects matching messages,
leaving others in place (and, for move, correctly arriving at the target
queue); the prompt shows real type names immediately on open, without
requiring the user to already know to select the scan-trigger entry.

**Outcome**: two more rounds of live feedback happened at this step,
after this feature's own earlier `verify-live` pass had already found
and fixed one bug (the overlay-clipping issue) — see `tasks.md`'s
implementation record for the full account of both. Both are the kind of
UI-correctness/UX-clarity issue this section exists to catch that unit
tests alone can't.
