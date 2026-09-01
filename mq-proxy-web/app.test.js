'use strict';

const test = require('node:test');
const assert = require('node:assert/strict');
const app = require('./app.js');

test('buildAuthHeader encodes username:password as HTTP Basic', () => {
  assert.equal(app.buildAuthHeader('cloudtui', 'changeme'), 'Basic ' + Buffer.from('cloudtui:changeme').toString('base64'));
});

test('parseEnvelope normalizes a list-style { data, errors } response', () => {
  const result = app.parseEnvelope({ data: [1, 2], errors: [] });
  assert.deepEqual(result, { data: [1, 2], errors: [] });
});

test('parseEnvelope normalizes an item-style { data, error } response', () => {
  const result = app.parseEnvelope({ data: null, error: { code: 'not_found', message: 'no such queue' } });
  assert.deepEqual(result, { data: null, errors: [{ code: 'not_found', message: 'no such queue' }] });
});

test('parseEnvelope treats a null error as no errors', () => {
  const result = app.parseEnvelope({ data: { messageId: 'ID:1' }, error: null });
  assert.deepEqual(result, { data: { messageId: 'ID:1' }, errors: [] });
});

test('parseEnvelope tolerates a missing/malformed body', () => {
  assert.deepEqual(app.parseEnvelope(null), { data: undefined, errors: [] });
  assert.deepEqual(app.parseEnvelope(undefined), { data: undefined, errors: [] });
});

test('errorMessageFrom prefers the first error message, falls back otherwise', () => {
  assert.equal(app.errorMessageFrom({ errors: [{ message: 'boom' }] }, 'fallback'), 'boom');
  assert.equal(app.errorMessageFrom({ errors: [] }, 'fallback'), 'fallback');
  assert.equal(app.errorMessageFrom({ errors: ['plain string error'] }, 'fallback'), 'plain string error');
});

test('buildQueryString flattens nested objects with dot-notation keys', () => {
  const qs = app.buildQueryString({
    sourceQueue: 'orders',
    returnBody: true,
    filter: { jmsType: 'order-created', maxCount: 50 },
  });
  assert.equal(qs, 'sourceQueue=orders&returnBody=true&filter.jmsType=order-created&filter.maxCount=50');
});

test('buildQueryString omits undefined/null values at any depth', () => {
  const qs = app.buildQueryString({
    sourceQueue: 'orders',
    filter: { jmsType: undefined, messageId: null, maxCount: 50 },
  });
  assert.equal(qs, 'sourceQueue=orders&filter.maxCount=50');
});

test('buildQueryString URL-encodes keys and values', () => {
  const qs = app.buildQueryString({ filter: { jmsType: 'a b&c' } });
  assert.equal(qs, 'filter.jmsType=a%20b%26c');
});

test('buildListMessagesParams nests filter fields, keeps sourceQueue/returnBody top-level', () => {
  const params = app.buildListMessagesParams('orders', { jmsType: 'order-created', maxCount: 50 });
  assert.deepEqual(params, {
    sourceQueue: 'orders',
    returnBody: true,
    filter: { jmsType: 'order-created', messageId: undefined, maxCount: 50 },
  });
});

test('buildBulkFilter: a blank JMS Type means "match everything" (empty filter, no maxCount)', () => {
  assert.deepEqual(app.buildBulkFilter(''), {});
  assert.deepEqual(app.buildBulkFilter('   '), {});
  assert.deepEqual(app.buildBulkFilter(undefined), {});
});

test('buildBulkFilter: a typed JMS Type narrows via filter.jmsType, still no maxCount', () => {
  assert.deepEqual(app.buildBulkFilter('order-created'), { jmsType: 'order-created' });
  assert.deepEqual(app.buildBulkFilter('  order-created  '), { jmsType: 'order-created' });
});

test('buildSingleMessageFilter targets exactly one message', () => {
  assert.deepEqual(app.buildSingleMessageFilter('ID:m1'), { messageId: 'ID:m1', maxCount: 1 });
});

test('tierForQueue: DLQ-stripped match against the source is preferred (tier 0)', () => {
  assert.equal(app.tierForQueue('foo.bar', 'dlq.foo.bar'), 0);
  assert.equal(app.tierForQueue('foo.bar', 'imq.foo.bar'), 0);
});

test('tierForQueue: an ordinary queue is regular (tier 1)', () => {
  assert.equal(app.tierForQueue('orders', 'dlq.foo.bar'), 1);
  assert.equal(app.tierForQueue('foo.bar', 'orders'), 1);
});

test('tierForQueue: dlq./imq.-prefixed queues are tier 2 unless they are the preferred match', () => {
  assert.equal(app.tierForQueue('dlq.other', 'orders'), 2);
  assert.equal(app.tierForQueue('imq.other', 'orders'), 2);
});

test('tierForQueue: activemq./statistics.-prefixed queues are tier 3 (bottom)', () => {
  assert.equal(app.tierForQueue('activemq.DLQ', 'orders'), 3);
  assert.equal(app.tierForQueue('statistics.broker', 'orders'), 3);
});

test('sortMoveTargets orders by tier then alphabetically, excludes the source queue', () => {
  const sorted = app.sortMoveTargets(
    ['zeta', 'activemq.DLQ', 'dlq.foo.bar', 'foo.bar', 'alpha', 'dlq.other'],
    'dlq.foo.bar',
  );
  assert.deepEqual(sorted, ['foo.bar', 'alpha', 'zeta', 'dlq.other', 'activemq.DLQ']);
});

test('truncate leaves short strings untouched and ellipsizes long ones', () => {
  assert.equal(app.truncate('hello', 80), 'hello');
  assert.equal(app.truncate('a'.repeat(85), 80), 'a'.repeat(79) + '…');
  assert.equal(app.truncate(null, 80), '');
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
  const data = await app.apiCall(
    { baseUrl: 'http://localhost:8080/', username: 'cloudtui', password: 'changeme' },
    'list-queues',
  );
  assert.deepEqual(data, [{ name: 'orders' }]);
  assert.equal(calls[0].url, 'http://localhost:8080/api/management/command/list-queues');
  assert.equal(calls[0].opts.headers.Authorization, app.buildAuthHeader('cloudtui', 'changeme'));
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
    () => app.apiCall({ baseUrl: 'http://localhost:8080', username: 'x', password: 'y' }, 'list-queues'),
    /Bad credentials/,
  );
  delete global.fetch;
});
