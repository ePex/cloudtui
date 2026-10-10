// Message export (spec-wip fe-message-export): NDJSON, one object per
// message, the same format the TUI's export writes. Everything here is
// pure except downloadNdjson, so it's unit-tested with `node --test`.
import { buildListMessagesParams } from './api.js';

// The most one export fetches, so a broker configured with a large browse
// limit can't make the browser hold an unbounded file — matches the TUI's
// export.MaxMessages.
export var EXPORT_CAP = 10000;

// One message from list-messages → one export record. Same fields, order
// and omissions as the TUI: jmsType/correlationId omitted when empty (the
// API's list-messages has no correlationId at all — spec/11), timestamp
// as UTC RFC 3339 with milliseconds, omitted when unparseable; headers
// always an object; body null without one.
export function toExportRecord(message, queueName) {
  var record = { queue: queueName, messageId: message.messageId };
  if (message.jmsType) record.jmsType = message.jmsType;
  if (message.correlationId) record.correlationId = message.correlationId;
  var ts = message.timestamp ? new Date(message.timestamp) : null;
  if (ts && !isNaN(ts.getTime())) record.timestamp = ts.toISOString();
  record.headers = message.headers || {};
  record.body = typeof message.body === 'string' ? message.body : null;
  return record;
}

// One JSON object per line, "\n" after each.
export function toNdjson(records) {
  return records.map(function (r) { return JSON.stringify(r) + '\n'; }).join('');
}

// "<queue>-<UTC timestamp>.ndjson", e.g. "orders-20261010T153000Z.ndjson";
// characters invalid in file names on any OS become "_" — same rule as the
// TUI's export.DefaultFileName.
export function exportFileName(queueName, date) {
  var safe = queueName.replace(/[\/\\:*?"<>|]/g, '_');
  var stamp = date.toISOString().replace(/[-:]/g, '').replace(/\.\d{3}Z$/, 'Z');
  return safe + '-' + stamp + '.ndjson';
}

// Pages through list-messages for an export — the same afterMessageId
// cursor "Load more" uses — until the server reports no next page or cap
// messages are in. apiCallFn is apiCall (passed in so tests can fake it).
// Resolves to { messages, truncated, capped }: truncated when the last
// page said the broker cut the browse short (spec/11), capped when the
// cap stopped it with more still there.
export function fetchAllForExport(apiCallFn, conn, queueName, jmsType, pageSize, cap) {
  var messages = [];
  function next(afterMessageId) {
    var maxCount = Math.min(pageSize, cap - messages.length);
    return apiCallFn(conn, 'list-messages', {
      query: buildListMessagesParams(queueName, { jmsType: jmsType, maxCount: maxCount, afterMessageId: afterMessageId }),
    }).then(function (page) {
      var batch = page || [];
      messages = messages.concat(batch);
      var hasMore = !!(page && page.hasMore);
      if (hasMore && messages.length < cap && batch.length > 0) {
        return next(batch[batch.length - 1].messageId);
      }
      return { messages: messages, truncated: !!(page && page.truncated), capped: hasMore && messages.length >= cap };
    });
  }
  return next(undefined);
}

// The inline result line: "Exported <n> message(s)", with the reason when
// it can't be everything — total is the queue's message count when known
// (0 otherwise).
export function exportSummary(n, total, truncated, capped) {
  var limit = " (the broker's browse limit)";
  if (capped) return 'Exported ' + n + ' message(s) — stopped at the export cap of ' + EXPORT_CAP;
  if (total > n) return 'Exported ' + n + ' of ' + total + ' message(s)' + limit;
  if (truncated) return 'Exported ' + n + ' message(s), more on the queue' + limit;
  return 'Exported ' + n + ' message(s)';
}

// Hands text to the browser as a file download — a Blob behind a
// temporary object URL and <a download>, which works whether the page was
// opened via file:// or served over http(s)://.
export function downloadNdjson(fileName, text) {
  var url = URL.createObjectURL(new Blob([text], { type: 'application/x-ndjson' }));
  var a = document.createElement('a');
  a.href = url;
  a.download = fileName;
  document.body.appendChild(a);
  a.click();
  a.remove();
  setTimeout(function () { URL.revokeObjectURL(url); }, 0);
}
