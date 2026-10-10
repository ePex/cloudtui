# Feature: export a queue's messages to a file

Date: 2026-10-10

## What and why

A way to save a queue's messages to a file — to look at them offline,
grep/`jq` them, or attach them to a ticket — from both the TUI and the
AMQ web console (spec/21). Today the only way to get at message
contents is one message at a time in the detail view (copy to
clipboard, spec/08).

**Non-destructive:** exporting only browses; the messages stay on the
queue. That means the export is subject to the broker's browse limit
(`maxBrowsePageSize`, 400 by default — spec/09, spec/11): on a bigger
queue it contains the first ~400 messages and says so. A complete,
destructive "export and remove" is a possible follow-up, not part of
this feature.

## File format: NDJSON

One JSON object per line, one line per message, UTF-8, `\n` line
endings on every platform. Each object:

| Field | Value |
|---|---|
| `queue` | the queue name |
| `messageId` | the JMS message ID |
| `jmsType` | the real `JMSType` header; omitted when the message has none (a TUI-inferred "text"/"bytes" type is never written as if it were real — spec/08) |
| `correlationId` | `JMSCorrelationID`; omitted when unset |
| `timestamp` | RFC 3339, UTC |
| `headers` | the message's other headers/properties as an object (as the detail view shows them) |
| `body` | the **full** body text — not the list view's trimmed preview; `null` for a message without a text body |

The same field set from both surfaces, so files are interchangeable.
Default file name: `<queue>-<UTC timestamp>.ndjson`, e.g.
`orders-20261010T153000Z.ndjson`.

## TUI

- **`e` in the queue list** exports the selected queue: a fresh browse
  of all its messages, up to the export cap (below), no filter.
- **`e` in the message list** exports what the user is looking at: the
  **marked** messages if any are marked, otherwise every message
  currently shown (server-side filter and quick search applied). No new
  request — the messages are already loaded.
- **Where to:** a prompt asks for the file path, prefilled with
  `<Downloads>/<default file name>` (`~/Downloads` when it exists,
  otherwise the home directory; resolved per OS). Esc cancels. An
  existing file is never overwritten silently: a confirm dialog asks
  `<path> exists. Overwrite?` (No is the default); No returns to the
  path prompt to pick another name.
- **Result** in the status line: `Exported <n> message(s) to <path>`.
  When the export can't be complete, it says so in the same line —
  `Exported 400 of 1,600 message(s) to <path> — the broker's browse
  limit`, using the pending count / `hasMore` the way the message
  browser's title does (spec/08). Errors (can't write, browse failed)
  show in the status line; nothing half-written is left behind.
- Works with both backends (Jolokia and proxy).

## Web console

- **"Export" button** in the message list's toolbar: exports the
  **selected** messages if any are selected, otherwise **everything the
  current filter matches** — fetching all pages (not just the loaded
  ones) up to the export cap.
- The browser downloads the file under the default name; works when the
  page is opened via `file://` too.
- **Result** shown inline like the console's other messages:
  `Exported <n> message(s)`, plus the same "of <total> — the broker's
  browse limit" note when the response reports `truncated` (spec/11) or
  the queue's message count is higher.
- The console only talks to `mq-proxy`-compatible servers (spec/21), so
  this is the only backend it needs.

## Export cap

Both surfaces stop at **10,000 messages** per export, so a broker
configured with a large browse limit can't make the TUI or the browser
hold an unbounded file in memory. Reaching the cap is reported like the
browse limit ("of <total>").

## Out of scope

- Export-and-remove (complete, destructive) — possible follow-up.
- Importing/re-sending an export.
- Other formats (JSON array, CSV, snippet files), compression.
- Binary bodies: messages without a text body are exported with
  `body: null`; no base64 of bytes messages.
- Exporting several queues at once.
