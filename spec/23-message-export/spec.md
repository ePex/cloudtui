# Message export

_Condensed from spec-wip/fe-message-export. See that PR for the
incremental history and the reasoning behind each decision._

## Purpose

Save a queue's messages to a file, to read them offline, `grep`/`jq`
them, or attach them to a ticket. Works from both the TUI and the AMQ
web console (spec/21). Without it, the only way to get at message
contents is one message at a time in the detail view (copy to
clipboard, spec/08).

Exporting is **non-destructive**: it only browses, and the messages
stay on the queue. So the export is subject to the broker's browse
limit (`maxBrowsePageSize`, 400 by default — spec/09, spec/11). On a
bigger queue it holds the first ~400 messages and says so.

## Behavior / user flow

### File format: NDJSON

One JSON object per line, one line per message, UTF-8, `\n` line
endings on every platform. Fields, in this order:

| Field | Value |
|---|---|
| `queue` | the queue name |
| `messageId` | the JMS message ID |
| `jmsType` | the real `JMSType` header; omitted when the message has none. A TUI-inferred "text"/"bytes" type (spec/08) is never written as if it were real. |
| `correlationId` | `JMSCorrelationID`; omitted when unset |
| `timestamp` | RFC 3339 in UTC with milliseconds (`2026-10-10T13:30:00.123Z`); omitted when unknown |
| `headers` | the message's other headers and properties as an object, as the detail view shows them (JMS headers plus decoded properties; the correlation ID is not repeated here). Always present, `{}` when empty. |
| `body` | the **full** body text, not the list's trimmed preview; `null` for a message without a text body |

Both surfaces write the same fields, so their files are
interchangeable. HTML characters (`<`, `>`, `&`) are written as-is, not
escaped. Default file name: `<queue>-<UTC timestamp>.ndjson`, e.g.
`orders-20261010T153000Z.ndjson`. Characters that aren't allowed in
file names on some OS (`/ \ : * ? " < > |`) are replaced with `_`.

### Export cap

Both surfaces stop at **10,000 messages** per export
(`export.MaxMessages`, `EXPORT_CAP`), so a broker configured with a
large browse limit can't make the TUI or the browser build an unbounded
file in memory.

### TUI

- **`e` in the queue list** (spec/07) exports the selected queue: a
  fresh browse of all its messages, up to the cap, with no filter.
- **`e` in the message list** (spec/08) exports what the user is looking
  at: the **marked** messages if any are marked, otherwise every message
  currently shown (server-side filter and quick search applied). This
  makes no new request, because the messages are already loaded. An
  empty list shows `No messages to export` in the status line.
- **Where to:** a prompt titled `Export "<queue>"` asks for the file
  path, prefilled with `<Downloads>/<default file name>`. That is
  `~/Downloads` when it exists, otherwise the home directory, resolved
  per OS. Esc cancels. An empty path, a folder that doesn't exist, or a
  path that is a folder is rejected in the prompt.
- **No silent overwrite:** if the file exists, a confirm dialog asks
  `Overwrite the existing file <name>?` (No is the default). It names
  only the file, because a long path would push the question out of the
  three-row dialog. No returns to the path prompt to pick another name.
- **Result** in the status line: `Exported <n> message(s) to <path>`.
  When the export can't be everything, the same line says so, with the
  note **before** the path (which can run off the status line):
  - `Exported 400 of 1600 message(s) (the broker's browse limit) to
    <path>`, when the queue's pending count is known and higher.
  - `Exported 400 message(s), more on the queue (the broker's browse
    limit), to <path>`, when the backend reports more but no count is
    known.
  - From the message list, the reason is `only what the list shows`
    instead, since it exports the loaded messages and nothing more.
    Marked messages are always exported without a note.
- **Errors** (the browse failed, the file can't be written) show in red
  as `Export failed: …`. Nothing half-written is left behind.
- Works with both backends (Jolokia and proxy). Over Jolokia, `headers`
  carries whatever JMS headers Jolokia's browse returns.

### Web console

- An **"Export"** button in the message list's toolbar exports the
  **selected** messages if any are selected. Otherwise it exports
  **everything the current JMS Type filter matches**: it fetches all
  pages, not just the loaded ones, following the `afterMessageId`
  cursor up to the cap, with the Max count field as the page size.
- The browser downloads the file under the default name (a `Blob` and an
  `<a download>` link; no server round trip beyond the browse).
- **Result** in an info line under the toolbar (`Exporting…` while it
  runs):
  - `Exported <n> message(s)`.
  - `Exported 400 of 1600 message(s) (the broker's browse limit)` when
    the last page reports `truncated` (spec/11) and the total is known.
    The total is the queue's message count from the queue list, used
    only when nothing is selected and no JMS Type filter is set.
  - `Exported 400 message(s), more on the queue (the broker's browse
    limit)` when `truncated` but no total is known.
  - `… — stopped at the export cap of 10000` when the cap was reached.
- Errors show in the view's usual error line. The info line is hidden
  again on the next load.

## Data & config

No configuration. Files go wherever the user points the TUI prompt, or
to the browser's download folder. The TUI writes them with mode `0600`,
since message bodies can hold business data.

## Implementation notes

- **`tui/internal/export/`** — `Record`, `NewRecord(queue, msg)`,
  `WriteFile(path, queue, msgs)`, `DefaultFileName(queue, now)`,
  `DefaultDir()`, `MaxMessages`. `WriteFile` writes to a temp file in the
  target folder and renames it into place, removing the temp file on
  any error, so a failed export never leaves a partial file. It uses
  `json.Encoder` with `SetEscapeHTML(false)`.
- **`tui/internal/queue/fields.go`** — `HeaderFields()` (the JMS header
  labels and their raw-field keys), `Message.Body()` and
  `DecodePropertyValue`. The detail view (spec/08) and the export share
  these, so `headers`/`body` in a file always match what the detail view
  shows.
- **`tui/internal/view/export.go`** — `exportFlow`, shared by the queue
  list and message list. Each view hands it a `collect` function
  returning an `exportSet` (messages, known total, more-on-queue flag,
  reason text). The flow runs the prompt, path validation and overwrite
  confirm, then collects and writes in a goroutine and reports in the
  status line. `collectQueue` browses with `MaxCount: export.MaxMessages`
  and takes the total from the queue's pending count.
- The `TextPrompt` runs `onSubmit` before closing, then `onClose`. The
  flow only records the path in `onSubmit` and continues from `onClose`,
  so the overwrite confirm or a reopened prompt isn't closed again right
  after it opens.
- Adding `e` to the message list would have made 13 shortcut-panel
  entries, over `ui.ShortcutPanelRows` (12). Mark all and clear marks
  were merged into one `a/n mark all/none` entry to make room.
- **Web: `mq-proxy-web/src/export.js`** — `toExportRecord`, `toNdjson`,
  `exportFileName`, `fetchAllForExport`, `exportSummary` (all unit
  tested) and `downloadNdjson` (DOM only, not unit tested).
  `apiCall` exposes the response's `truncated` on the returned array,
  next to `hasMore`.

## Out of scope (deliberate)

- Export and remove (complete, destructive) — on the roadmap
  (`BACKLOG.md`).
- Importing or re-sending an export — on the roadmap.
- Other formats (JSON array, CSV, snippet files), compression.
- Binary bodies: messages without a text body are exported with
  `body: null`; no base64 of bytes messages.
- Exporting several queues at once.
