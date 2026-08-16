# Plan — FE 46

## Approach

Three layers, bottom-up: interface change → both backend implementations
→ UI. No new types beyond what's needed to parse the filter form's
fields; everything else (`queue.MessageFilter`, the proxy's
`toFilterDTO`, the Jolokia `filterMessages` helper) already exists from
CR 44.

### 1. `queue.Backend` interface

`tui/internal/queue/backend.go`: change
`BrowseMessages(ctx, queueName) ([]Message, error)` to
`BrowseMessages(ctx, queueName, filter MessageFilter) ([]Message, error)`.
A zero-value `MessageFilter{}` must behave exactly like today's
unfiltered browse in both backends.

### 2. Proxy backend (`tui/internal/queue/proxy/proxy.go`)

`BrowseMessages` (line 166) builds its query string from `filter` the
same way `toFilterDTO` (line 95) already maps a `queue.MessageFilter`
for the POST-body backends — reuse `toFilterDTO` to get the wire-shape
values, then append them as query params instead of a JSON body:
`jmsType`, `messageId`, `fromDate`, `toDate` (RFC3339, same formatting
`toFilterDTO` already does), `maxCount` (only when set). Always add
`returnBody=true` explicitly — the message browser's preview column
needs the body, and CR 45 made `mq-proxy`'s default `true` too, but
being explicit here means this keeps working even if that default ever
changes. `toQueueMessage` (line 276) is untouched.

Internal call sites that already do their own filtering
(`DeleteMessages`/`MoveMessages` don't call `BrowseMessages` at all —
they're server-side filtered POSTs, not affected) — proxy has no
unfiltered internal callers to update.

### 3. Jolokia backend (`tui/internal/queue/jolokia`)

`BrowseMessages` (`jolokia.go:213`) takes the new `filter` param, keeps
its existing full/fallback logic exactly as-is, and applies the
already-tested `filterMessages` helper (`filter.go:14`) to the combined
result right before returning — the exact same function
`DeleteMessages`/`MoveMessages` already use, so no new filtering logic
is written, just a new call site.

Internal unfiltered callers get `queue.MessageFilter{}` explicitly,
preserving today's behavior:
- `filter.go:40` (`DeleteMessages`) and `filter.go:56` (`MoveMessages`)
  already run their own `filterMessages` pass after browsing — passing
  `MessageFilter{}` to the inner `BrowseMessages` call keeps them
  fetching everything before filtering, unchanged.
- `jolokia.go:624` (`PurgeQueue`'s tier-3 browse-and-remove fallback)
  wants every message — `MessageFilter{}`.

### 4. UI (`tui/internal/app/messages.go`)

Two independent, composable mechanisms — a client-side quick search
(cheap, live) and a server-side filter form (round-trip, deliberate
submit). Quick search narrows what's *displayed* from `allMsgs`; the
form narrows what's *fetched* in the first place.

**Structure**: `messagesView` currently exposes only `table` as its
primitive (`app.go:319`: `a.pages.AddPage("messages", a.messagesV.table,
...)`). Restructure like `queuesView` (`queues.go:59-69`): add `flex
*tview.Flex` and `searchInput *tview.InputField` fields, build
`flex = Flex(table, searchInput)` in `newMessagesView`, and change
`app.go:319` to add `a.messagesV.flex` instead of `.table`. Border and
title stay on `table` (`openMessages`, `app.go:1092`, keeps calling
`a.messagesV.table.SetTitle(...)`).

**4a. Quick search (`/`)** — mirrors `queuesView` exactly
(`queues.go:59-108`, `220-248`):
- `messagesView` gains `allMsgs []queue.Message` (full set from the last
  `load()`, pre-search) and `quickSearch string` (active search text).
- `repaint(msgs)` (`messages.go:156`) is renamed in behavior, not just
  signature: it stores `msgs` into `allMsgs`, filters into `mv.msgs` by
  `quickSearch` (case-insensitive substring match against `JMSType` and
  `Preview`), then renders `mv.msgs` as today. Mark/target logic
  (`markedIDs`, `targetIDs`, etc.) keeps operating on `mv.msgs` — i.e.
  the currently-*displayed* (search-filtered) set, same as how
  `queuesView`'s sort/selection already only sees filtered rows.
- `searchInput.SetChangedFunc` calls `applyQuickSearch(text)` on every
  keystroke (live, client-side, no reload) — sets `quickSearch` and
  calls `mv.repaint(mv.allMsgs)`.
- `/` opens `searchInput` (prefilled with the current `quickSearch`),
  `Enter`/`Esc`/arrow-keys close it and return focus to `table` — same
  `SetDoneFunc`/`SetInputCapture` pattern as `queues.go:75-89`.

**4b. Filter form (`f`)** — a `tview.Form` overlay, following the
`connEditorForm` precedent (`app.go:455-479`, `connections.go:53-88`)
rather than inventing new modal machinery:
- New `App` fields: `messageFilterForm *tview.Form`,
  `messageFilterVisible bool`, built once in `New()` next to
  `connEditorForm`: `AddInputField("JMS Type", ...)`,
  `AddInputField("From (RFC3339 or YYYY-MM-DD)", ...)`,
  `AddInputField("To (RFC3339 or YYYY-MM-DD)", ...)`,
  `AddInputField("Max Count", ...)`, `AddButton("Apply", ...)`,
  `AddButton("Clear", ...)`, `AddButton("Cancel", ...)`. Root page
  `"message-filter"`, shown centered (`centered(...)`, `app.go:41`)
  like `conn-editor`. `Esc` cancels, same `SetInputCapture` pattern as
  `connEditorForm` (`app.go:470-476`).
- `showMessageFilter()` (new, in `messages.go` or `app.go` next to
  `showConnEditor`): prefills the four fields from `messagesView.filter`
  (the currently-applied `queue.MessageFilter`), shows the page, focuses
  the form.
- `applyMessageFilter()` reads the four field strings and delegates to a
  new pure function `parseMessageFilterForm(jmsType, from, to, maxCount
  string) (queue.MessageFilter, error)` (in `messages.go`, next to the
  other filter logic, so it's unit-testable without touching the Form
  widget) — parses From/To trying `time.RFC3339` then `"2006-01-02"`
  (date-only → UTC midnight, matching how `toFilterDTO` already
  normalizes to UTC) and Max Count as a non-negative int. On a parse
  error: status bar shows it (`[red]...[-]`, same style as
  `saveConnEditor`'s validation, `connections.go:113-121`), form stays
  open. On success: sets `messagesView.filter`, hides the page, calls
  `mv.load()`.
- `clearMessageFilter()`: resets all four fields, sets
  `messagesView.filter = queue.MessageFilter{}`, closes, reloads.
- `f` on the messages table opens the form (mirroring how `p`/`c`
  already open other overlays from `table.SetInputCapture`,
  `messages.go:61-117`).

**Title**: reflects both active mechanisms, e.g.
`" Messages — orders (filter: type=order-created) [search: foo] "`
(server filter and quick search shown independently; either segment
omitted when empty — same "don't show `[]`/`()` when inactive" rule
`updateTitle` already follows, `queues.go:241-248`).

**Persistence across reloads**: `load()` (`messages.go:138`) passes
`mv.filter` to `BrowseMessages`, so `r` (refresh) and returning to this
view keep the server-side filter; `quickSearch` already persists since
it's only ever cleared explicitly (typing empty text), matching FE 09.

**Shortcuts**: add `{Key: "/", Description: "quick search"}` and
`{Key: "f", Description: "filter"}` to `Shortcuts()` (`messages.go:36`).

## Files touched

- `tui/internal/queue/backend.go` — interface signature.
- `tui/internal/queue/proxy/proxy.go` + `proxy_test.go` — query-building,
  tests for filtered/unfiltered browse (mirroring the existing
  `DeleteMessages`/`MoveMessages` request-shape tests).
- `tui/internal/queue/jolokia/jolokia.go` + `jolokia_test.go` — filter
  param + internal call-site updates, tests for filtered browse (both
  the `browseMessagesFull` and `browse()`-fallback paths) and confirming
  unfiltered internal callers still fetch everything.
- `tui/internal/app/messages.go` + `messages_test.go` — quick search
  (`allMsgs`/`quickSearch`/`repaint` split, applyQuickSearch), title
  logic, and the `queue.MessageFilter` plumbing (`filter` field,
  `load()` passing it through). Tests: quick-search filters/persists
  (mirroring `TestQueuesViewFilterApplied`,
  `TestQueuesViewFilterPersistsAfterRepaint`,
  `TestQueuesViewTitleUpdatesWithFilter`), mark/target operate on the
  search-filtered set only.
- `tui/internal/app/app.go` + a new `messagefilter.go` (or alongside
  `connections.go`) — `messageFilterForm` construction (`New()`),
  `showMessageFilter`/`applyMessageFilter`/`clearMessageFilter`, the
  `"message-filter"` root page, and `AddPage("messages", ...)` target
  changing to `a.messagesV.flex`. Tests: field parsing (From/To/Max
  Count valid values and each error case) as a table-driven pure-function
  test, following `saveConnEditor`'s validation-error pattern
  (`connections.go:113-121`) for style.
- `spec/46-fe-message-browser-filters/tasks.md` (next gate).

## Key decisions

- **Two mechanisms, not one.** A single input can't represent
  `MessageFilter`'s four fields cleanly, but collapsing FE 09's fast,
  no-round-trip substring narrowing into a form-only design would be a
  regression — the form requires a submit round trip even for "just show
  me messages with 'foo' in the preview." Quick search (`/`, live,
  client-side) and the filter form (`f`, explicit submit, server-side)
  solve different problems and compose rather than compete.
- **Filter form applies on explicit submit, not live-as-you-type.**
  Unlike quick search, submitting the form triggers a real fetch (JMX
  browse-and-filter, or an mq-proxy HTTP call) — wiring that to
  `SetChangedFunc` the way `queues.go`'s single filter input does would
  spam the backend/broker on every keystroke. This is the one deliberate
  UX difference between the two mechanisms.
- **A `tview.Form` (matching `connEditorForm`), not a new modal style.**
  This codebase already has a multi-field-form pattern
  (`app.go:455-479`, `connections.go`) — reusing it keeps the filter form
  visually and behaviorally consistent with the connection editor instead
  of introducing a second way to build a form overlay.
- **`returnBody=true` always, from the proxy backend.** The message
  browser's preview column needs body text; there's no user-facing
  "hide bodies to browse faster" toggle in this spec (see Out of scope
  in `spec.md` — no pagination either, so trimming body payload alone
  wouldn't solve large-queue slowness the way `maxCount` does).
- **No change to `DeleteMessages`/`MoveMessages` UI wiring** — confirmed
  out of scope in `spec.md`; `d`/`m` keep acting on marked/cursor rows
  only, now just possibly a filtered subset since that's what's loaded.
