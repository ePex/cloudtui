# Tasks — FE 46

1. [x] Change `queue.Backend.BrowseMessages` to
   `BrowseMessages(ctx context.Context, queueName string, filter
   MessageFilter) ([]Message, error)` (`tui/internal/queue/backend.go`).
   Update every call site to compile (proxy/jolokia implementations,
   `messages.go`, and all existing tests) — behavior at this step still
   matches today exactly, since every caller passes `MessageFilter{}`.
2. [x] Proxy backend: build the `list-messages` query string from
   `filter` (reusing `toFilterDTO`'s field mapping) plus explicit
   `returnBody=true`. Add/update unit tests in `proxy_test.go` covering:
   zero-value filter produces the same query as before, each filter
   field appears correctly when set, `returnBody=true` is always sent.
3. [x] Jolokia backend: apply the existing `filterMessages` helper to
   `BrowseMessages`'s result before returning; update the three internal
   unfiltered call sites (`filter.go`'s `DeleteMessages`/`MoveMessages`,
   `jolokia.go`'s `PurgeQueue` fallback) to pass `MessageFilter{}`
   explicitly. Add/update unit tests in `jolokia_test.go` covering a
   filtered browse through both the `browseMessagesFull` and
   `browse()`-fallback paths, and confirming the three internal callers
   still fetch everything unfiltered.
4. [x] `messagesView` quick search: `allMsgs`/`quickSearch` fields,
   `repaint` split (store full set, filter into `mv.msgs`, render),
   `searchInput` wired into a new `flex` primitive (`/` opens, live
   `SetChangedFunc`, Enter/Esc/arrows close — mirroring `queuesView`).
   Update `app.go`'s `AddPage("messages", ...)` to use `.flex`. Add
   tests mirroring `TestQueuesViewFilterApplied`,
   `TestQueuesViewFilterPersistsAfterRepaint`,
   `TestQueuesViewTitleUpdatesWithFilter`, plus a test that
   marking/`targetIDs` only sees the search-filtered set.
5. [x] Message filter form: `messageFilterForm` (`tview.Form`, four
   fields + Apply/Clear/Cancel) built in `App.New()`, `"message-filter"`
   root page, `f` hotkey on the messages table, `showMessageFilter`/
   `applyMessageFilter`/`clearMessageFilter`, and the pure
   `parseMessageFilterForm` function. `messagesView.load()` passes
   `mv.filter` through to `BrowseMessages`. Table title reflects both
   the active server filter and quick search. Add table-driven tests for
   `parseMessageFilterForm` (each field valid/invalid, combinations,
   all-empty) and a test that `load()` passes the current filter.
6. [x] Update `Shortcuts()`/context panel with `/` (quick search) and
   `f` (filter) entries.

## Manual verification

Filtering is broker-facing behavior (both backends, both mechanisms) —
unit tests cover parsing/plumbing, but confirm the real interaction
before checking off tasks 4–5:

- Against the `local-mq-proxy` connection: open a queue with several
  messages of different JMS types/timestamps, open the filter form (`f`),
  set JMS Type and/or a date range, Apply — confirm only matching
  messages appear and the title shows the active filter. Clear — confirm
  everything reappears.
- Against the `default` (jolokia) connection: repeat the same filter-form
  check, confirming client-side filtering produces the same visible
  result as the proxy backend for equivalent criteria.
- In either backend: with a filter active, press `/` and type a substring
  — confirm the visible rows narrow further without a reload (no status
  bar/network activity), and that marking (`space`/`a`) only affects
  currently-visible rows.
- Confirm `r` (refresh) and leaving/returning to the view keep both the
  server filter and quick search active.
- Confirm existing mark/delete/move/purge behavior is unaffected when no
  filter is active (regression check) — `task smoke:test` covers the
  golden path already.

**Verified 2026-08-13** by driving the real TUI binary in tmux against
both backends, using disposable test queues (`fe46-verify` via
jolokia/`task test:queue:add`+`task seed:queue`; `fe46-proxy-verify` via
direct `send-message` calls to a `task dev:proxy:start`-launched
mq-proxy, so messages carried real distinct JMS types).

- **Found and fixed a real bug**: `/` initially did nothing useful —
  `onGlobalKey` (`app.go`) has an explicit per-view allowlist of filter
  inputs that bypass global hotkey handling (`a.queuesV.filterInput`,
  etc.), and `a.messagesV.searchInput` was missing from it, so the first
  keystroke after `/` (e.g. "s" in "shipped") fell through to global
  handlers ('s' → Settings). Added the missing check
  (`app.go`, next to the other filter-input bypasses). Unit tests didn't
  catch this since none of them exercise the actual global-key pipeline
  — exactly the class of bug this manual-verification step exists for.
- Quick search (jolokia): live substring narrowing confirmed (typed
  "shipped" → title `[search: shipped]`, 1/3 rows shown); clearing
  (`/`, clear text, Enter) restored all 3.
- Filter form, jolokia backend: `Max Count=1` → exactly 1 row, title
  `(filter: max=1)`; `JMS Type=order-created` → 0 rows (seeded messages
  are real JMS type "text", confirming the filter actually excludes
  non-matches, not just passing everything through); Clear restored all
  3 and reset the title both times.
- Filter form, proxy backend: `JMS Type=order-created` against 4 mq-proxy
  messages (3 `order-created`, 1 `invoice-created`) → exactly the 3
  matching messages shown, title updated; Clear restored all 4. Quick
  search "invoice" → the 1 matching row. Confirms mq-proxy's server-side
  `list-messages` filtering (CR 45) end-to-end, not just the query-string
  unit tests.
- Confirmed the `tview.Form` "remembers focus across reopen" gotcha
  documented in the `verify-live` skill applies here too (reopening the
  form after a Cancel/Clear doesn't reset focus to JMS Type) — a UX
  quirk shared with `connEditorForm`, not a regression introduced here.
- Persistence: quick search stayed active across Esc → back to queues →
  re-opening the *same* queue. Opening a *different* queue reset both
  the filter and quick search (title and rows back to unfiltered).
- Regression: mark (space, ✓ glyph), the delete confirm dialog and its
  exact wording, and the actual delete all behaved identically to
  pre-FE-46 behavior with no filter active.
- Cleanup: both test queues removed, mq-proxy stopped, `config.yaml`
  restored to its pre-verification content, tmux sessions killed, scratch
  binary removed.

**Second bug found post-verification (user report)**: the `f` shortcut
didn't appear in the context panel. Root cause: `topbar.go`'s
`shortcutPanelRows` (the top bar's fixed height, non-scrollable) was
hardcoded to 6, but `messagesView.Shortcuts()` already had 9 entries
*before* this feature (`c`/`Esc` were already silently clipping in
production) — adding `/` and `f` pushed it to 11, clipping four entries
instead of two. Fixed by hoisting `shortcutPanelRows` to a package-level
const and bumping it to 11 (`topbar.go`), updating `topbar_test.go`'s
two hardcoded height assertions to match, and adding
`TestMessagesViewShortcutsFitTopBar` (`messages_test.go`) to guard
against silent regression. Re-verified live: all 11 shortcuts, including
`<f> filter`, now render in the context panel.
