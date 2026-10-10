import test from 'node:test';
import assert from 'node:assert/strict';
import {
  createShowTimer,
  progressCount,
  footerText,
  catSpriteSvg,
  wait,
  SHOW_DELAY_MS,
  MIN_VISIBLE_MS,
} from './progress.js';

// fakeClock is a manual clock for createShowTimer: advance(ms) runs the
// timers that fall due, in order.
function fakeClock() {
  var t = 0;
  var nextId = 1;
  var timers = new Map();
  return {
    setTimeout: function (f, ms) {
      var id = nextId++;
      timers.set(id, { at: t + ms, f: f });
      return id;
    },
    clearTimeout: function (id) { timers.delete(id); },
    now: function () { return t; },
    advance: function (ms) {
      var end = t + ms;
      for (;;) {
        var due = null;
        timers.forEach(function (timer, id) {
          if (timer.at <= end && (!due || timer.at < due.timer.at)) due = { id: id, timer: timer };
        });
        if (!due) break;
        timers.delete(due.id);
        t = due.timer.at;
        due.timer.f();
      }
      t = end;
    },
  };
}

function timerWithLog() {
  var clock = fakeClock();
  var log = [];
  var timer = createShowTimer(clock.setTimeout, clock.clearTimeout, clock.now,
    function () { log.push('show@' + clock.now()); },
    function () { log.push('hide@' + clock.now()); });
  return { clock: clock, log: log, timer: timer };
}

test('createShowTimer shows only after the show delay', () => {
  const { clock, log, timer } = timerWithLog();
  timer.begin();
  clock.advance(SHOW_DELAY_MS - 1);
  assert.equal(timer.visible(), false);
  clock.advance(1);
  assert.equal(timer.visible(), true);
  assert.deepEqual(log, ['show@250']);
});

test('createShowTimer never shows an operation that ends before the delay, and unblocks at once', () => {
  const { clock, log, timer } = timerWithLog();
  timer.begin();
  clock.advance(100);
  timer.end();
  clock.advance(1000);
  assert.deepEqual(log, ['hide@100']);
});

test('createShowTimer keeps a shown overlay up for the minimum time', () => {
  const { clock, log, timer } = timerWithLog();
  timer.begin();
  clock.advance(300);
  timer.end();
  clock.advance(MIN_VISIBLE_MS - 51);
  assert.equal(timer.visible(), true);
  clock.advance(1);
  assert.deepEqual(log, ['show@250', 'hide@650']);
});

test('createShowTimer hides at once after the minimum time has passed', () => {
  const { clock, log, timer } = timerWithLog();
  timer.begin();
  clock.advance(1000);
  timer.end();
  assert.deepEqual(log, ['show@250', 'hide@1000']);
});

test('createShowTimer end(true) hides at once, for Esc', () => {
  const { clock, log, timer } = timerWithLog();
  timer.begin();
  clock.advance(300);
  timer.end(true);
  assert.deepEqual(log, ['show@250', 'hide@300']);
});

test('createShowTimer: a new operation during the minimum time takes the overlay over', () => {
  const { clock, log, timer } = timerWithLog();
  timer.begin();
  clock.advance(300);
  timer.end();
  timer.begin();
  clock.advance(5000);
  assert.equal(timer.visible(), true);
  assert.deepEqual(log, ['show@250']);
});

test('createShowTimer: a second begin while the show is pending keeps the first delay', () => {
  const { clock, log, timer } = timerWithLog();
  timer.begin();
  clock.advance(200);
  timer.begin();
  clock.advance(50);
  assert.deepEqual(log, ['show@250']);
});

test('progressCount says how far a counted operation is, with failures', () => {
  assert.equal(progressCount(3, 10, 0), '3 of 10');
  assert.equal(progressCount(12, 20, 1), '12 of 20 (1 failed)');
  assert.equal(progressCount(0, 0, 0), '');
});

test('footerText says what Esc does, then what happens instead', () => {
  assert.equal(footerText(false, 0), 'Esc to cancel');
  assert.equal(footerText(false, 10), 'Esc to cancel');
  assert.equal(footerText(true, 10), 'Stopping after the current message…');
  assert.equal(footerText(true, 0), 'Stopping after the current request…');
  assert.equal(footerText(true, 100, 'batch'), 'Stopping after the current batch…');
});

test('catSpriteSvg draws both frames side by side on a 20x6 grid', () => {
  const svg = catSpriteSvg();
  assert.match(svg, /viewBox="0 0 20 6"/);
  const xs = [...svg.matchAll(/<rect x="(\d+)"/g)].map((m) => Number(m[1]));
  assert.ok(xs.some((x) => x < 10) && xs.some((x) => x >= 10));
  assert.ok(xs.every((x) => x < 20));
  // The eye ('k') is in the same place in both frames.
  assert.match(svg, /x="8" y="2" width="1" height="1" fill="#000000"/);
  assert.match(svg, /x="18" y="2" width="1" height="1" fill="#000000"/);
});

test('wait resolves true once the time is up', async () => {
  assert.equal(await wait(5, new AbortController().signal), true);
});

test('wait resolves false as soon as the signal aborts', async () => {
  const controller = new AbortController();
  const started = Date.now();
  const result = wait(10000, controller.signal);
  controller.abort();
  assert.equal(await result, false);
  assert.ok(Date.now() - started < 1000);
});

test('wait resolves false for an already aborted signal', async () => {
  const controller = new AbortController();
  controller.abort();
  assert.equal(await wait(10000, controller.signal), false);
});
