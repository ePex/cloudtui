# FE 46 — Message browser: server-side browse filtering

Date: 2026-08-13

## What

Let the user narrow what `messagesView` browses/displays by JMS type and
date range, and cap how many messages are fetched (`maxCount`) — using
the same `queue.MessageFilter` type `queue.Backend` already exposes for
bulk delete/move (CR 44), now that `mq-proxy`'s `list-messages` endpoint
can push `jmsType`/`fromDate`/`toDate`/`maxCount` down to a JMS selector
(CR 45) instead of only unfiltered browsing.

## Why

Today `messagesView.load()` calls `BrowseMessages(ctx, queueName)` with
no filter — every message on the queue is fetched and rendered, however
many there are. For queues with large backlogs this is slow and hard to
navigate manually. `queue.Backend` already carries a `MessageFilter`
type and `DeleteMessages`/`MoveMessages` already use it (CR 44), but
nothing lets the user apply that same filter to *browsing* — the one
place a filter would help the most, since it's the view the user actually
scrolls through by hand.

**Checked before writing this spec** (see conversation): compared
`mq-proxy`'s own `openapi.yaml` (CR 45, flat query params:
`sourceQueue`, `jmsType`, `messageId`, `fromDate`, `toDate`, `maxCount`,
`returnBody`) against the live reference API's actual query binding
(probed directly, since its OpenAPI doc collapses the query DTO opaquely)
— the reference API nests filter fields as `filter.jmsType` etc. and
defaults `returnBody` to `false`. This TUI work builds against
`mq-proxy`'s own (flat, already-shipped) shape, not the reference's
nesting — per CR 44, `mq-proxy` is the reference implementation going
forward, so this divergence is intentional and not something this spec
changes.

## Scope

1. **`queue.Backend.BrowseMessages` gains a filter parameter**:
   `BrowseMessages(ctx context.Context, queueName string, filter
   MessageFilter) ([]Message, error)` — a breaking signature change to
   the interface. `messagesView.load()` is the only caller in `tui`
   today; it's updated to pass the view's active filter (zero-value
   `MessageFilter{}` when no filter is set, matching today's "browse
   everything" behavior exactly).
2. **Proxy backend** (`tui/internal/queue/proxy`): `BrowseMessages` calls
   `list-messages` with `jmsType`/`messageId`/`fromDate`/`toDate`/
   `maxCount` query params built from the filter, and `returnBody=true`
   always (the message browser needs the body for its preview column —
   see `tui/internal/app/messages.go` repaint/preview logic).
3. **Jolokia backend** (`tui/internal/queue/jolokia`): `BrowseMessages`
   browses everything (as today — JMX has no selector-based browse) and
   then applies the existing `filterMessages` helper
   (`tui/internal/queue/jolokia/filter.go`), the same function
   `DeleteMessages`/`MoveMessages` already use. No new filtering logic;
   just a new call site for existing, tested code.
4. **Two complementary filter mechanisms in `messagesView`**, since a
   single free-text input doesn't fit `MessageFilter`'s four fields well,
   but losing FE 09's fast, no-round-trip narrowing would be a
   regression:
   - **Quick search** (`/`, unchanged trigger from the original plan):
     a live, client-side substring filter over the *currently loaded*
     rows — the exact mechanic and precedent FE 09 established for the
     queue list, just applied to messages (matching JMS type and/or
     preview text). No network/JMX round trip; narrows what's already
     on screen.
   - **Filter form** (a new hotkey, e.g. `f`): a small `tview.Form`
     overlay — following the existing multi-field-form precedent in this
     codebase (`connEditorForm`, `app.go`) rather than a modal built from
     scratch — with JMS Type / From / To / Max Count fields and
     Apply/Clear/Cancel buttons. Submitting builds a `queue.MessageFilter`
     and re-fetches via `BrowseMessages`, i.e. this is the actual
     server-side-pushed-down filtering CR 45 exists for.
   The two compose: the form narrows what's fetched from the backend;
   quick search then narrows what's displayed from that already-filtered
   set. Both persist across reloads/refresh and are reflected in the
   table title.
5. Shortcut entries added to `messagesView.Shortcuts()`/the context panel
   for both new hotkeys.

## Out of scope

- **No UI for `MessageFilter.MessageID`** — filtering the browse by one
  exact message ID has no real interactive use case; that's what `/`
  substring-filtering the *rendered* rows would be for, which this spec
  doesn't add either (see next point).
- **`d`/`m` do not gain a "act on every filter match, not just
  marked/cursor" mode.** `queue.Backend.DeleteMessages`/`MoveMessages`
  (the single-server-call filtered bulk operations from CR 44) remain
  unwired to any UI — reusing them here would silently change what `d`/
  `m` do when nothing is marked (today: acts on the cursor row only),
  which needs its own deliberate design (confirmation copy, undo-safety
  for a potentially large blast radius) rather than being folded into a
  browse-filtering feature. Still a candidate for a future `fe`/`cr`.
- **No pagination / "load more."** `maxCount` caps what's fetched; there
  is no follow-up affordance to fetch the next batch.
- **No change to `mq-proxy` or the reference API's query-parameter
  shape** — this spec only adds a `tui`-side consumer of what CR 45
  already shipped.
