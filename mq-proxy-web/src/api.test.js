import test from 'node:test';
import assert from 'node:assert/strict';
import {
  buildAuthHeader,
  parseEnvelope,
  errorMessageFrom,
  buildQueryString,
  buildListMessagesParams,
  appendMessages,
  buildJmsTypeScanParams,
  extractDistinctJmsTypes,
  buildBulkFilter,
  resolveJmsType,
  buildSingleMessageFilter,
  buildBulkDeleteBody,
  buildBulkMoveBody,
  apiCall,
  isAbortError,
  BULK_BATCH_SIZE,
  chunk,
  runBatches,
  bulkSummary,
} from './api.js';

test('buildAuthHeader encodes username:password as HTTP Basic', () => {
  assert.equal(buildAuthHeader('cloudtui', 'changeme'), 'Basic ' + Buffer.from('cloudtui:changeme').toString('base64'));
});

test('parseEnvelope normalizes a list-style { data, errors } response', () => {
  const result = parseEnvelope({ data: [1, 2], errors: [] });
  assert.deepEqual(result, { data: [1, 2], errors: [], hasMore: false, truncated: false });
});

test('parseEnvelope normalizes an item-style { data, error } response', () => {
  const result = parseEnvelope({ data: null, error: { code: 'not_found', message: 'no such queue' } });
  assert.deepEqual(result, { data: null, errors: [{ code: 'not_found', message: 'no such queue' }], hasMore: false, truncated: false });
});

test('parseEnvelope treats a null error as no errors', () => {
  const result = parseEnvelope({ data: { messageId: 'ID:1' }, error: null });
  assert.deepEqual(result, { data: { messageId: 'ID:1' }, errors: [], hasMore: false, truncated: false });
});

test('parseEnvelope carries hasMore through for list-messages pagination', () => {
  const result = parseEnvelope({ data: [1, 2], errors: [], hasMore: true });
  assert.deepEqual(result, { data: [1, 2], errors: [], hasMore: true, truncated: false });
});

test('parseEnvelope carries truncated through, separately from hasMore', () => {
  const result = parseEnvelope({ data: [], errors: [], hasMore: false, truncated: true });
  assert.deepEqual(result, { data: [], errors: [], hasMore: false, truncated: true });
});

test('parseEnvelope tolerates a missing/malformed body', () => {
  assert.deepEqual(parseEnvelope(null), { data: undefined, errors: [], hasMore: false, truncated: false });
  assert.deepEqual(parseEnvelope(undefined), { data: undefined, errors: [], hasMore: false, truncated: false });
});

test('errorMessageFrom prefers the first error message, falls back otherwise', () => {
  assert.equal(errorMessageFrom({ errors: [{ message: 'boom' }] }, 'fallback'), 'boom');
  assert.equal(errorMessageFrom({ errors: [] }, 'fallback'), 'fallback');
  assert.equal(errorMessageFrom({ errors: ['plain string error'] }, 'fallback'), 'plain string error');
});

test('buildQueryString flattens nested objects with dot-notation keys', () => {
  const qs = buildQueryString({
    sourceQueue: 'orders',
    returnBody: true,
    filter: { jmsType: 'order-created', maxCount: 50 },
  });
  assert.equal(qs, 'sourceQueue=orders&returnBody=true&filter.jmsType=order-created&filter.maxCount=50');
});

test('buildQueryString omits undefined/null values at any depth', () => {
  const qs = buildQueryString({
    sourceQueue: 'orders',
    filter: { jmsType: undefined, messageId: null, maxCount: 50 },
  });
  assert.equal(qs, 'sourceQueue=orders&filter.maxCount=50');
});

test('buildQueryString URL-encodes keys and values', () => {
  const qs = buildQueryString({ filter: { jmsType: 'a b&c' } });
  assert.equal(qs, 'filter.jmsType=a%20b%26c');
});

test('buildListMessagesParams nests filter fields, keeps sourceQueue/returnBody top-level', () => {
  const params = buildListMessagesParams('orders', { jmsType: 'order-created', maxCount: 50 });
  assert.deepEqual(params, {
    sourceQueue: 'orders',
    returnBody: true,
    filter: { jmsType: 'order-created', messageId: undefined, maxCount: 50, afterMessageId: undefined },
  });
});

test('buildListMessagesParams nests afterMessageId as the pagination cursor', () => {
  const params = buildListMessagesParams('orders', { maxCount: 50, afterMessageId: 'ID:m1' });
  assert.deepEqual(params.filter.afterMessageId, 'ID:m1');
});

test('appendMessages concatenates without mutating either input array', () => {
  const existing = [{ messageId: 'ID:1' }];
  const newPage = [{ messageId: 'ID:2' }, { messageId: 'ID:3' }];
  const result = appendMessages(existing, newPage);
  assert.deepEqual(result, [{ messageId: 'ID:1' }, { messageId: 'ID:2' }, { messageId: 'ID:3' }]);
  assert.deepEqual(existing, [{ messageId: 'ID:1' }]);
  assert.deepEqual(newPage, [{ messageId: 'ID:2' }, { messageId: 'ID:3' }]);
});

test('appendMessages onto an empty list just returns the new page', () => {
  assert.deepEqual(appendMessages([], [{ messageId: 'ID:1' }]), [{ messageId: 'ID:1' }]);
});

test('buildJmsTypeScanParams: scans without bodies, capped at the shared auto-scan count', () => {
  assert.deepEqual(buildJmsTypeScanParams('orders'), {
    sourceQueue: 'orders',
    returnBody: false,
    filter: { maxCount: 500 },
  });
});

test('extractDistinctJmsTypes: sorted, de-duplicated, non-empty values only', () => {
  const messages = [
    { jmsType: 'order-created' },
    { jmsType: 'order-cancelled' },
    { jmsType: 'order-created' },
    { jmsType: '' },
    {},
  ];
  assert.deepEqual(extractDistinctJmsTypes(messages), ['order-cancelled', 'order-created']);
});

test('extractDistinctJmsTypes: an empty message list yields an empty list', () => {
  assert.deepEqual(extractDistinctJmsTypes([]), []);
});

test('buildBulkFilter: a blank JMS Type means "match everything" (empty filter, no maxCount)', () => {
  assert.deepEqual(buildBulkFilter(''), {});
  assert.deepEqual(buildBulkFilter('   '), {});
  assert.deepEqual(buildBulkFilter(undefined), {});
});

test('resolveJmsType: a blank entry defaults to "text" (mq-proxy requires the field)', () => {
  assert.equal(resolveJmsType(''), 'text');
  assert.equal(resolveJmsType('   '), 'text');
  assert.equal(resolveJmsType(undefined), 'text');
});

test('resolveJmsType: a typed value is used as-is (trimmed)', () => {
  assert.equal(resolveJmsType('order-created'), 'order-created');
  assert.equal(resolveJmsType('  order-created  '), 'order-created');
});

test('buildBulkFilter: a typed JMS Type narrows via filter.jmsType, still no maxCount', () => {
  assert.deepEqual(buildBulkFilter('order-created'), { jmsType: 'order-created' });
  assert.deepEqual(buildBulkFilter('  order-created  '), { jmsType: 'order-created' });
});

test('buildSingleMessageFilter targets exactly one message', () => {
  assert.deepEqual(buildSingleMessageFilter('ID:m1'), { messageId: 'ID:m1', maxCount: 1 });
});

test('buildBulkDeleteBody: one request element per selected message, each scoped to exactly that message', () => {
  assert.deepEqual(buildBulkDeleteBody('orders', ['ID:m1', 'ID:m2']), [
    { sourceQueue: 'orders', filter: { messageId: 'ID:m1', maxCount: 1 } },
    { sourceQueue: 'orders', filter: { messageId: 'ID:m2', maxCount: 1 } },
  ]);
});

test('buildBulkDeleteBody: an empty selection yields an empty request body', () => {
  assert.deepEqual(buildBulkDeleteBody('orders', []), []);
});

test('buildBulkMoveBody: one request element per selected message, carrying the shared target queue', () => {
  assert.deepEqual(buildBulkMoveBody('orders', 'archive', ['ID:m1', 'ID:m2']), [
    { sourceQueue: 'orders', targetQueue: 'archive', filter: { messageId: 'ID:m1', maxCount: 1 } },
    { sourceQueue: 'orders', targetQueue: 'archive', filter: { messageId: 'ID:m2', maxCount: 1 } },
  ]);
});

test('apiCall sends Basic auth, unwraps envelope data, and rejects on a non-ok response', async () => {
  const calls = [];
  global.fetch = async (url, opts) => {
    calls.push({ url, opts });
    return {
      ok: true,
      json: async () => ({ data: [{ name: 'orders' }], errors: [] }),
    };
  };
  const data = await apiCall(
    { baseUrl: 'http://localhost:8080/', username: 'cloudtui', password: 'changeme' },
    'list-queues',
  );
  assert.deepEqual([...data], [{ name: 'orders' }]);
  assert.equal(calls[0].url, 'http://localhost:8080/api/management/command/list-queues');
  assert.equal(calls[0].opts.headers.Authorization, buildAuthHeader('cloudtui', 'changeme'));
  delete global.fetch;
});

test('apiCall surfaces hasMore on the resolved data, for list-messages pagination', async () => {
  global.fetch = async () => ({
    ok: true,
    json: async () => ({ data: [{ messageId: 'ID:1' }], errors: [], hasMore: true }),
  });
  const data = await apiCall(
    { baseUrl: 'http://localhost:8080', username: 'cloudtui', password: 'changeme' },
    'list-messages',
  );
  assert.equal(data.hasMore, true);
  delete global.fetch;
});

test('apiCall surfaces truncated on the resolved data', async () => {
  global.fetch = async () => ({
    ok: true,
    json: async () => ({ data: [{ messageId: 'ID:1' }], errors: [], hasMore: false, truncated: true }),
  });
  const data = await apiCall(
    { baseUrl: 'http://localhost:8080', username: 'cloudtui', password: 'changeme' },
    'list-messages',
  );
  assert.equal(data.truncated, true);
  assert.equal(data.hasMore, false);
  delete global.fetch;
});

test('apiCall rejects with the server error message on a non-ok HTTP response', async () => {
  global.fetch = async () => ({
    ok: false,
    status: 401,
    statusText: 'Unauthorized',
    json: async () => ({ error: 'Bad credentials' }),
  });
  await assert.rejects(
    () => apiCall({ baseUrl: 'http://localhost:8080', username: 'x', password: 'y' }, 'list-queues'),
    /Bad credentials/,
  );
  delete global.fetch;
});

test('apiCall passes the abort signal to fetch', async () => {
  const controller = new AbortController();
  let seen;
  global.fetch = async (url, opts) => {
    seen = opts.signal;
    return { ok: true, status: 200, json: async () => ({ data: [], errors: [] }) };
  };
  await apiCall({ baseUrl: 'http://localhost:8080', username: 'x', password: 'y' }, 'list-queues', { signal: controller.signal });
  assert.equal(seen, controller.signal);
  delete global.fetch;
});

test('apiCall rejects with an AbortError when the request is cancelled', async () => {
  const controller = new AbortController();
  global.fetch = (url, opts) => new Promise((resolve, reject) => {
    opts.signal.addEventListener('abort', () => reject(new DOMException('aborted', 'AbortError')));
  });
  const call = apiCall({ baseUrl: 'http://localhost:8080', username: 'x', password: 'y' }, 'list-queues', { signal: controller.signal });
  controller.abort();
  await assert.rejects(call, (err) => isAbortError(err));
  delete global.fetch;
});

test('apiCall rejects with an AbortError when cancelled while reading the body', async () => {
  global.fetch = async () => ({
    ok: true,
    status: 200,
    json: async () => { throw new DOMException('aborted', 'AbortError'); },
  });
  await assert.rejects(
    apiCall({ baseUrl: 'http://localhost:8080', username: 'x', password: 'y' }, 'list-queues'),
    (err) => isAbortError(err),
  );
  delete global.fetch;
});

test('isAbortError tells a cancel from a real failure', () => {
  assert.equal(isAbortError(new DOMException('aborted', 'AbortError')), true);
  assert.equal(isAbortError(new Error('HTTP 500')), false);
  assert.equal(isAbortError(null), false);
});

test('chunk splits into batches of at most size, in order', () => {
  assert.deepEqual(chunk([1, 2, 3, 4, 5], 2), [[1, 2], [3, 4], [5]]);
  assert.deepEqual(chunk([1, 2], 25), [[1, 2]]);
  assert.deepEqual(chunk([], 25), []);
});

// fakeBatchCall answers each request with a message entry per ID in its
// body, minus those in missing; it fails the request for a batch listed
// in failBatches (by index). It records each batch's IDs.
function fakeBatchCall(missing = [], failBatches = []) {
  const batches = [];
  const call = async (verb, opts) => {
    const ids = opts.body.map((r) => r.filter.messageId);
    batches.push({ verb, ids });
    if (failBatches.includes(batches.length - 1)) throw new Error('HTTP 500 boom');
    return ids.filter((id) => !missing.includes(id)).map((id) => ({ messageId: id }));
  };
  return { call, batches };
}

const ids = (n) => Array.from({ length: n }, (_, i) => 'ID:' + i);
const body = (batch) => buildBulkDeleteBody('orders', batch);

test('runBatches sends batches of 25 one after another and reports each step', async () => {
  const { call, batches } = fakeBatchCall();
  const steps = [];
  const result = await runBatches(call, ids(60), body, 'delete-messages', {
    onStep: (done, failed) => steps.push([done, failed]),
    stopped: () => false,
  });
  assert.equal(BULK_BATCH_SIZE, 25);
  assert.deepEqual(batches.map((b) => b.ids.length), [25, 25, 10]);
  assert.ok(batches.every((b) => b.verb === 'delete-messages'));
  assert.deepEqual(steps, [[25, 0], [50, 0], [60, 0]]);
  assert.deepEqual(result, { done: 60, failed: 0, cancelled: false, lastError: null });
});

test('runBatches counts messages a response does not report as failed', async () => {
  const { call } = fakeBatchCall(['ID:3', 'ID:30', 'ID:31']);
  const result = await runBatches(call, ids(50), body, 'delete-messages', { onStep: () => {}, stopped: () => false });
  assert.equal(result.done, 50);
  assert.equal(result.failed, 3);
});

test('runBatches does not count ID-less entries', async () => {
  const call = async () => [{ messageId: 'ID:0' }, {}, { messageId: null }];
  const result = await runBatches(call, ids(3), body, 'delete-messages', { onStep: () => {}, stopped: () => false });
  assert.equal(result.failed, 2);
});

test('runBatches counts a failed request as a failed batch and goes on', async () => {
  const { call, batches } = fakeBatchCall([], [1]);
  const result = await runBatches(call, ids(60), body, 'delete-messages', { onStep: () => {}, stopped: () => false });
  assert.equal(batches.length, 3);
  assert.equal(result.done, 60);
  assert.equal(result.failed, 25);
  assert.equal(result.lastError.message, 'HTTP 500 boom');
});

test('runBatches stops before the next batch once stopped', async () => {
  const { call, batches } = fakeBatchCall();
  let stop = false;
  const result = await runBatches(call, ids(100), body, 'delete-messages', {
    onStep: (done) => { if (done === 50) stop = true; },
    stopped: () => stop,
  });
  assert.equal(batches.length, 2);
  assert.deepEqual(result, { done: 50, failed: 0, cancelled: true, lastError: null });
});

test('bulkSummary words the result like the TUI', () => {
  assert.equal(bulkSummary('Deleted', '', 100, 0, 100), 'Deleted 100 message(s)');
  assert.equal(bulkSummary('Deleted', '', 1, 0, 1), 'Deleted message');
  assert.equal(bulkSummary('Deleted', '', 50, 0, 100), 'Deleted 50 of 100 message(s); cancelled');
  assert.equal(bulkSummary('Moved', ' to "dlq"', 50, 3, 100), 'Moved 47 of 100 message(s) to "dlq"; 3 failed; cancelled');
  assert.equal(bulkSummary('Deleted', '', 100, 3, 100), 'Deleted 97/100 message(s); 3 failed');
});
