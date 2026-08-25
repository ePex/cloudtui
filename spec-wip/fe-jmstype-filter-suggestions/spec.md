# Feature: JMS Type autocomplete in the message filter overlay

Date: 2026-08-25

## Problem

The message filter overlay (`internal/dialog/messagefilter.go`, FE 46 —
opened with `f` from the Messages view) has a "JMS Type" field that's a
plain free-text `tview.InputField` with no suggestions. A user filtering
messages on a queue has to already know (or go check, by browsing
unfiltered first) the exact JMS type string used on that queue — there's
no way to see "what types actually exist here" from within the dialog.

## Feature

Add autocomplete to the "JMS Type" field, with two tiers:

1. **Default (free, no extra network call): suggest distinct `JMSType`
   values seen among the messages currently loaded** in the Messages view
   for that queue (`MessagesView.allMsgs` — the full, pre-quick-search set
   from the last load, capped at `defaultBrowseMaxCount` = 500 unless the
   user has set a larger explicit `MaxCount`). This is inherently a
   best-effort sample, not guaranteed to include every type on the queue
   — see "Known limitation" below.
2. **Opt-in: a special, always-present, visually distinct suggestion
   entry — "Scan up to 5,000 messages for JMS types" (exact wording
   decided during implementation) — that triggers a one-off, larger
   browse purely to widen the suggestion sample.** Selecting it does
   *not* insert text into the field or submit the form: it clears back to
   the current text, shows a loading status
   (`mf.host.SetStatus("Scanning for JMS types...")`), and kicks off an
   async `BrowseMessages` call with **no `JMSType` filter** (we're
   discovering types, not narrowing by one) and `MaxCount: 5000`. The
   scan's result populates a separate, dialog-local "expanded types" set
   — merged into future suggestion lookups for as long as the dialog
   stays open — **without touching `MessagesView.allMsgs` or the visible
   message table**: this is a suggestions-only fetch, not a change to
   what's displayed. Available on both backends: Jolokia can fetch
   unbounded from JMX and would cap client-side anyway; mq-proxy's
   `list-messages` endpoint requires a positive `maxCount` on every call
   regardless of whether other filter fields (like `jmsType`) are also
   set (checked against `internal/queue/proxy/proxy.go`'s `browseQuery`
   and spec/11) — so a fixed, honest, bounded number is required either
   way, not just a proxy-backend workaround.
- Typing filters suggestions by prefix, same interaction model as the
  `:` command prompt's autocomplete (`↑`/`↓` navigate and live-update the
  field, `Enter`/`Tab` accepts the highlighted entry without submitting
  the form, a second `Enter` on the field submits/moves on) — this is
  `tview.InputField`'s own built-in behavior, already used elsewhere in
  this app.
- Styled via `ui.StyleInputFieldAutocomplete` (same accent-tinted panel
  background as the command prompt, fixed just prior in this session) so
  it's readable rather than reusing tview's unthemed defaults.
- Empty/blank `JMSType` values among loaded messages are not suggested
  (an empty entry would be indistinguishable from "no suggestion" and
  isn't a meaningful thing to filter on).
- No behavior change to filtering/parsing itself
  (`ui.ParseMessageFilterForm`) — this only affects what's suggested
  while typing, not what's accepted.

## Known limitation (by design, communicated honestly)

Suggestions — even after an opt-in scan — are a bounded sample, never a
guaranteed-complete list of every JMS type on the queue: mq-proxy cannot
be asked for an unbounded browse at all (hard 400 without a positive
`maxCount`), and even Jolokia's unbounded JMX fetch would be a real
latency/memory risk on a very large queue, so it isn't offered as a
"truly get everything" escape hatch either. The wording used in the app
("Scan up to N messages", not "load all types") must not imply
completeness it can't deliver.

## Out of scope

- A true "every JMS type on this queue, guaranteed" capability — not
  achievable within the existing `Backend` interface/mq-proxy API
  without a new, dedicated aggregate endpoint, which is a bigger, separate
  feature.
- Autocomplete on the filter overlay's other fields (From/To date,
  Max Count) — free-form/numeric fields where a "distinct values seen"
  suggestion doesn't make sense the way it does for a type string.
- Any change to `queue.Message`/`queue.MessageFilter`'s shape.

## Manual verification

Unit-testable for the suggestion-computation logic (distinct, non-empty,
prefix-filtered, merge of loaded-set + scanned-set) via a fake message
set/fake backend. The autocomplete *styling* itself can't be asserted
without rendering (same limitation as the command prompt's autocomplete
— see this session's earlier fix). Given this touches real broker
behavior (an extra `BrowseMessages` call with a large `MaxCount`), use
the `verify-live` skill against a real broker (both backends, since the
mq-proxy `maxCount`-required constraint is backend-specific): opening the
filter after browsing a queue with mixed JMS types shows the ones
already loaded; typing narrows the list; selecting the scan entry shows
a loading status, then widens the suggestions without changing the
underlying message table; accepting a suggestion and applying actually
filters correctly.
