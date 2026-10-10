import { test } from 'node:test';
import assert from 'node:assert/strict';
import {
  EXPORT_CAP,
  toExportRecord,
  toNdjson,
  exportFileName,
  fetchAllForExport,
  exportSummary,
} from './export.js';

test('toExportRecord maps a list-messages entry to the TUI export fields, in order', () => {
  const record = toExportRecord({
    sourceQueue: 'orders',
    messageId: 'ID:1',
    jmsType: 'OrderCreated',
    timestamp: '2026-10-10T15:30:00.123+02:00',
    headers: { tenant: 'ab' },
    body: '{"id":1}',
  }, 'orders');
  assert.equal(
    JSON.stringify(record),
    '{"queue":"orders","messageId":"ID:1","jmsType":"OrderCreated","timestamp":"2026-10-10T13:30:00.123Z","headers":{"tenant":"ab"},"body":"{\\"id\\":1}"}',
  );
});

test('toExportRecord omits what is unknown and nulls a missing body', () => {
  const record = toExportRecord({ messageId: 'ID:2', jmsType: '', timestamp: 'not a date' }, 'orders');
  assert.deepEqual(record, { queue: 'orders', messageId: 'ID:2', headers: {}, body: null });
});

test('toNdjson writes one JSON object per line, each ending in a newline', () => {
  const text = toNdjson([{ a: 1 }, { b: '<x>' }]);
  const lines = text.split('\n');
  assert.equal(lines.length, 3); // two records, then the empty string after the last newline
  assert.equal(lines[2], '');
  assert.deepEqual(JSON.parse(lines[1]), { b: '<x>' });
});

test('exportFileName matches the TUI rule', () => {
  const date = new Date('2026-10-10T15:30:00.456Z');
  assert.equal(exportFileName('orders', date), 'orders-20261010T153000Z.ndjson');
  assert.equal(exportFileName('a/b\\c:d*e?f"g<h>i|j', date), 'a_b_c_d_e_f_g_h_i_j-20261010T153000Z.ndjson');
});

// fakePages answers list-messages with pre-built pages and records the
// cursor and page size of each request.
function fakePages(pages) {
  const requests = [];
  const apiCallFn = async (conn, verb, opts) => {
    requests.push(opts.query.filter);
    const page = pages[requests.length - 1];
    const data = page.messages.slice();
    data.hasMore = !!page.hasMore;
    data.truncated = !!page.truncated;
    return data;
  };
  return { apiCallFn, requests };
}

function ids(prefix, n) {
  return Array.from({ length: n }, (_, i) => ({ messageId: prefix + i }));
}

test('fetchAllForExport follows the cursor until there is no next page', async () => {
  const { apiCallFn, requests } = fakePages([
    { messages: ids('a', 2), hasMore: true },
    { messages: ids('b', 2), hasMore: true },
    { messages: ids('c', 1), hasMore: false },
  ]);

  const result = await fetchAllForExport(apiCallFn, {}, 'orders', 'TypeB', 2, EXPORT_CAP);

  assert.equal(result.messages.length, 5);
  assert.deepEqual(requests.map((f) => f.afterMessageId), [undefined, 'a1', 'b1']);
  assert.ok(requests.every((f) => f.jmsType === 'TypeB' && f.maxCount === 2));
  assert.equal(result.truncated, false);
  assert.equal(result.capped, false);
});

test('fetchAllForExport reports truncated from the last page', async () => {
  const { apiCallFn } = fakePages([{ messages: ids('a', 400), hasMore: false, truncated: true }]);

  const result = await fetchAllForExport(apiCallFn, {}, 'orders', '', 500, EXPORT_CAP);

  assert.equal(result.messages.length, 400);
  assert.equal(result.truncated, true);
});

test('fetchAllForExport stops at the cap, asking only for what still fits', async () => {
  const { apiCallFn, requests } = fakePages([
    { messages: ids('a', 3), hasMore: true },
    { messages: ids('b', 2), hasMore: true },
  ]);

  const result = await fetchAllForExport(apiCallFn, {}, 'orders', '', 3, 5);

  assert.equal(result.messages.length, 5);
  assert.deepEqual(requests.map((f) => f.maxCount), [3, 2]);
  assert.equal(result.capped, true);
});

test('exportSummary says when the export is not everything', () => {
  assert.equal(exportSummary(10, 10, false, false), 'Exported 10 message(s)');
  assert.equal(exportSummary(400, 1600, true, false), "Exported 400 of 1600 message(s) (the broker's browse limit)");
  assert.equal(exportSummary(400, 0, true, false), "Exported 400 message(s), more on the queue (the broker's browse limit)");
  assert.equal(exportSummary(EXPORT_CAP, 0, false, true), 'Exported ' + EXPORT_CAP + ' message(s) — stopped at the export cap of ' + EXPORT_CAP);
});
