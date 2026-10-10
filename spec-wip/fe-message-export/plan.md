# Plan: export a queue's messages to a file

See `spec.md`. Three parts: a UI-free export package in the TUI, the TUI
flow in both views, and the web console. No `mq-proxy` or wire-contract
changes.

## 1. Shared message fields → `internal/queue`

The export needs the same header mapping and property decoding the
detail view uses (`messageDetailHeaderFields`, `decodePropertyValue` in
`internal/view/message_detail.go`). A new `internal/export` package
can't import `internal/view` (view will import export), so both move to
`internal/queue/fields.go`:

- `queue.HeaderFields()` — the label → raw-field-key list, unchanged.
- `queue.DecodePropertyValue(v any) string` — unchanged.
- `(Message).Body() (string, bool)` — the full body from
  `RawFields["text"]`, false when there's none.

The detail view and its clipboard text switch to them, with no
behavior change (their existing tests cover that).

## 2. `internal/export` (no UI dependency)

- `Record` — the spec's NDJSON object: `queue`, `messageId`,
  `jmsType` (omitted when `JMSTypeInferred` or empty), `correlationId`
  (omitted when empty — the proxy backend never has one, since
  `list-messages` doesn't return it, spec/11), `timestamp` (RFC 3339
  UTC), `headers`, `body` (`null` when `Body()` is false). `headers`:
  the JMS header fields that are set (detail-view labels as keys, raw
  values kept as JSON numbers/bools/strings) plus every property
  (decoded with `DecodePropertyValue`), so it matches what the detail
  view shows.
- `NewRecord(queueName string, m queue.Message) Record`.
- `WriteFile(path, queueName string, msgs []queue.Message) error` —
  writes NDJSON (`encoding/json` per record, `\n` after each) to a temp
  file in the same directory, then renames it over `path`, so a failed
  write never leaves a half-written file and never replaces an
  existing one with garbage.
- `DefaultFileName(queueName string, now time.Time) string` —
  `<queue>-<YYYYMMDDTHHMMSSZ>.ndjson`; characters invalid in file names
  on any OS (`/ \ : * ? " < > |`) are replaced with `_`.
- `DefaultDir() string` — `filepath.Join(home, "Downloads")` when that
  directory exists, else `home` (`os.UserHomeDir`); cross-platform.
- `MaxMessages = 10000` — the export cap.

Tests: record fields (real vs inferred type, missing correlation ID,
binary body → null, headers with JMS fields and decoded properties),
one line per message and valid JSON per line, atomic write (an existing
file stays intact when writing fails), file name sanitizing, default dir
with and without `Downloads` (`t.Setenv("HOME", t.TempDir())`).

## 3. TUI flow (`internal/view/export.go`)

An `exportFlow` like `purgeFlow` (spec/09), shared by both views:

1. **Path prompt** — the existing `dialog.TextPrompt`, title `Export
   "<queue>"`, prefilled with `DefaultDir()/DefaultFileName(…)`. Its
   `onSubmit` rejects an empty path or a missing parent directory with
   an error (the prompt stays open, error in the status line).
2. **Overwrite confirm** — if the file exists, `ConfirmDialog.
   ShowWithCancel`: `<path> exists. Overwrite?`, No default; No reopens
   the path prompt with the same path.
3. **Collect** (background goroutine, status `Exporting <queue>…`):
   - queue list: `BrowseMessages(queue, {MaxCount: MaxMessages})` plus
     the pending count (`pendingCount`, spec/08);
   - message list: the marked messages, or else the shown ones
     (`mv.msgs`) — already loaded, no request.
4. **Write** with `export.WriteFile`, then the status line:
   `Exported <n> message(s) to <path>`, plus ` — <n> of <total>, the
   broker's browse limit` when incomplete. Incomplete = for the queue
   list, `hasMore` or `n < pending`; for the message list (when nothing
   is marked), the last load's own incompleteness — `applyLoad` already
   computes it for the title (`browseNote`), so `MessagesView` keeps the
   load's `hasMore` and pending count alongside. A marked-only export
   is never "incomplete" — it's exactly what was marked.
5. Errors (browse failed, can't write) go to the status line in red;
   the view stays as it was.

Both views get the key **`e`** ("export") in their `Shortcuts()` and
input capture, and take the app's existing `TextPrompt` in their
constructors (`app.go` wiring). The queue list exports the selected
row's queue; with no row selected, `e` does nothing.

Tests (`export_test.go` in `view`): the flow's steps driven directly —
default path in the prompt; overwrite → confirm → No reopens the prompt;
result line with and without the incomplete note (pending known,
`hasMore` only, marked); error path; `e` in both views opens the
prompt. Writing goes to `t.TempDir()`.

## 4. Web console (`mq-proxy-web`)

- **`src/export.js`** (pure, unit-tested with `node --test` like the
  others):
  - `toExportRecord(message, queueName)` — same fields as the TUI;
    `jmsType`/`correlationId` omitted when empty; `timestamp`
    normalized to UTC RFC 3339; `body` `null` when absent.
  - `toNdjson(records)`.
  - `exportFileName(queueName, date)` — same rule as the TUI's.
  - `fetchAllForExport(conn, queueName, jmsType, cap, apiCall)` — pages
    through `list-messages` with `filter.afterMessageId` (the same
    cursor "Load more" uses) until `hasMore` is false or `cap` is
    reached; returns the messages and whether the last page said
    `truncated`.
- **`api.js`**: `parseEnvelope`/`apiCall` also carry `truncated`, next
  to `hasMore`.
- **`messages.js` + `index.html`**: an "Export" button in the message
  list toolbar. Selected messages → export those (already loaded);
  none → `fetchAllForExport` with the current JMS Type filter. Then a
  `Blob` + object URL + temporary `<a download>` click — works over
  `file://` and `http(s)://`. Result in a new inline info line
  (`messagesInfo`, next to `messagesError`): `Exported <n> message(s)`
  plus `— <n> of <total>, the broker's browse limit` when `truncated`,
  the cap was hit, or the queue's message count (from the queue list's
  `state`) is higher.
- Tests: record mapping, NDJSON shape, file name, paging (stops on
  `hasMore: false`, at the cap, reports `truncated`) with a fake
  `apiCall`.

## 5. Specs (merge-back)

- New `spec/23-message-export/spec.md` for the feature (format, both
  surfaces, cap, browse-limit honesty).
- `spec/08` (`e` in the message list), `spec/07`/`spec/09` (`e` in the
  queue list — wherever its keys are listed), `spec/21` (Export button,
  `truncated` now shown on export), `README.md` feature list.

## Trade-offs

- **Prompt first, then browse** in the queue list: the user picks the
  path before any work starts, and Esc costs nothing.
- **Export from the loaded list** in the message list rather than a
  fresh browse: "what you see is what you get", no surprise differences
  — but capped by the view's own `max=` (500 by default); the result
  line says when that left messages out.
- **Moving two helpers into `queue`** touches the detail view, but
  avoids a second copy of the header mapping drifting from the first.
- **Web console fetches all pages client-side** (no new endpoint): it
  reuses the "Load more" cursor; one request per page of the current
  `max=` size.

## Manual verification

Live via the `verify-live` skill (isolated `HOME`, local broker, both
the Jolokia and `mq-proxy` backends): export a small queue from the
queue list and the message list (marked and unmarked), check the file
with `jq`; export a 1,600-message queue → "400 of 1,600"; existing file
→ overwrite confirm, No and Yes. Web console against `mq-proxy`: Export
with and without a selection, file opens with `jq`, truncation note on
the big queue.

No new dependencies.
