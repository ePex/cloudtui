import test from 'node:test';
import assert from 'node:assert/strict';
import { tierForQueue, sortMoveTargets } from './movepicker.js';

test('tierForQueue: DLQ-stripped match against the source is preferred (tier 0)', () => {
  assert.equal(tierForQueue('foo.bar', 'dlq.foo.bar'), 0);
  assert.equal(tierForQueue('foo.bar', 'imq.foo.bar'), 0);
});

test('tierForQueue: an ordinary queue is regular (tier 1)', () => {
  assert.equal(tierForQueue('orders', 'dlq.foo.bar'), 1);
  assert.equal(tierForQueue('foo.bar', 'orders'), 1);
});

test('tierForQueue: dlq./imq.-prefixed queues are tier 2 unless they are the preferred match', () => {
  assert.equal(tierForQueue('dlq.other', 'orders'), 2);
  assert.equal(tierForQueue('imq.other', 'orders'), 2);
});

test('tierForQueue: activemq./statistics.-prefixed queues are tier 3 (bottom)', () => {
  assert.equal(tierForQueue('activemq.DLQ', 'orders'), 3);
  assert.equal(tierForQueue('statistics.broker', 'orders'), 3);
});

test('sortMoveTargets orders by tier then alphabetically, excludes the source queue', () => {
  const sorted = sortMoveTargets(
    ['zeta', 'activemq.DLQ', 'dlq.foo.bar', 'foo.bar', 'alpha', 'dlq.other'],
    'dlq.foo.bar',
  );
  assert.deepEqual(sorted, ['foo.bar', 'alpha', 'zeta', 'dlq.other', 'activemq.DLQ']);
});
