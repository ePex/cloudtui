import test from 'node:test';
import assert from 'node:assert/strict';
import { filterQueues, sortQueues } from './queues.js';

const SAMPLE_QUEUES = [
  { name: 'orders', messageCount: 5, consumerCount: 2, enqueuedCount: 10, dequeuedCount: 5, producerCount: 1 },
  { name: 'dlq.orders', messageCount: 20, consumerCount: 0, enqueuedCount: 20, dequeuedCount: 0, producerCount: 0 },
  { name: 'archive', messageCount: 1, consumerCount: 1, enqueuedCount: 100, dequeuedCount: 99, producerCount: 3 },
];

test('filterQueues: blank needle returns every queue, unfiltered', () => {
  assert.deepEqual(filterQueues(SAMPLE_QUEUES, ''), SAMPLE_QUEUES);
  assert.deepEqual(filterQueues(SAMPLE_QUEUES, '   '), SAMPLE_QUEUES);
});

test('filterQueues: case-insensitive substring match on queue name', () => {
  const result = filterQueues(SAMPLE_QUEUES, 'ORD');
  assert.deepEqual(result.map((q) => q.name), ['orders', 'dlq.orders']);
});

test('filterQueues: no matches returns an empty array', () => {
  assert.deepEqual(filterQueues(SAMPLE_QUEUES, 'nonexistent'), []);
});

test('sortQueues: name column sorts alphabetically ascending by default', () => {
  const sorted = sortQueues(SAMPLE_QUEUES, 'name', 'asc');
  assert.deepEqual(sorted.map((q) => q.name), ['archive', 'dlq.orders', 'orders']);
});

test('sortQueues: descending reverses the order', () => {
  const sorted = sortQueues(SAMPLE_QUEUES, 'name', 'desc');
  assert.deepEqual(sorted.map((q) => q.name), ['orders', 'dlq.orders', 'archive']);
});

test('sortQueues: a numeric column (e.g. messageCount) sorts numerically, not lexically', () => {
  const sorted = sortQueues(SAMPLE_QUEUES, 'messageCount', 'asc');
  assert.deepEqual(sorted.map((q) => q.messageCount), [1, 5, 20]);
});

test('sortQueues does not mutate its input', () => {
  const original = SAMPLE_QUEUES.map((q) => ({ ...q }));
  sortQueues(SAMPLE_QUEUES, 'name', 'asc');
  assert.deepEqual(SAMPLE_QUEUES, original);
});
