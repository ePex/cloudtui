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
} from './api.js';

test('buildAuthHeader encodes username:password as HTTP Basic', () => {
  assert.equal(buildAuthHeader('cloudtui', 'changeme'), 'Basic ' + Buffer.from('cloudtui:changeme').toString('base64'));
});

test('parseEnvelope normalizes a list-style { data, errors } response', () => {
  const result = parseEnvelope({ data: [1, 2], errors: [] });
  assert.deepEqual(result, { data: [1, 2], errors: [], hasMore: false });
});

test('parseEnvelope normalizes an item-style { data, error } response', () => {
  const result = parseEnvelope({ data: null, error: { code: 'not_found', message: 'no such queue' } });
  assert.deepEqual(result, { data: null, errors: [{ code: 'not_found', message: 'no such queue' }], hasMore: false });
});

test('parseEnvelope treats a null error as no errors', () => {
  const result = parseEnvelope({ data: { messageId: 'ID:1' }, error: null });
  assert.deepEqual(result, { data: { messageId: 'ID:1' }, errors: [], hasMore: false });
});

test('parseEnvelope carries hasMore through for list-messages pagination', () => {
  const result = parseEnvelope({ data: [1, 2], errors: [], hasMore: true });
  assert.deepEqual(result, { data: [1, 2], errors: [], hasMore: true });
});

test('parseEnvelope tolerates a missing/malformed body', () => {
  assert.deepEqual(parseEnvelope(null), { data: undefined, errors: [], hasMore: false });
  assert.deepEqual(parseEnvelope(undefined), { data: undefined, errors: [], hasMore: false });
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
