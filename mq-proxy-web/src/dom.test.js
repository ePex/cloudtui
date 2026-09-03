import test from 'node:test';
import assert from 'node:assert/strict';
import { truncate, sortedHeaderEntries } from './dom.js';

test('truncate leaves short strings untouched and ellipsizes long ones', () => {
  assert.equal(truncate('hello', 80), 'hello');
  assert.equal(truncate('a'.repeat(85), 80), 'a'.repeat(79) + '…');
  assert.equal(truncate(null, 80), '');
});

test('sortedHeaderEntries: sorted [key, value] pairs from the headers map', () => {
  assert.deepEqual(sortedHeaderEntries({ correlationId: 'abc', groupId: 'g1', jmsxUserId: 'svc' }), [
    ['correlationId', 'abc'],
    ['groupId', 'g1'],
    ['jmsxUserId', 'svc'],
  ]);
});

test('sortedHeaderEntries: no headers (null/undefined/empty) yields an empty list', () => {
  assert.deepEqual(sortedHeaderEntries(null), []);
  assert.deepEqual(sortedHeaderEntries(undefined), []);
  assert.deepEqual(sortedHeaderEntries({}), []);
});
