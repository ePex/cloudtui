// Blocking progress overlay for slow operations (spec-wip
// fe-progress-modal): a rainbow bar with a pixel cat for counted
// operations, the cat running around a circle otherwise. Esc cancels;
// clicks and other keys are blocked from the moment an operation starts,
// the overlay only shows once it has run for SHOW_DELAY_MS and then stays
// at least MIN_VISIBLE_MS. The pure helpers are exported for tests; nothing here
// touches the DOM until the first operation starts.

export var SHOW_DELAY_MS = 250;
export var MIN_VISIBLE_MS = 400;

var FOOTER_CANCEL = 'Esc to cancel';

// createShowTimer is the overlay's timing, free of the DOM. begin() is
// called when an operation starts, end() when the last one has finished:
// onShow runs SHOW_DELAY_MS after begin() unless end() came first; once
// shown, onHide runs no sooner than MIN_VISIBLE_MS after onShow (at once
// with end(true), for Esc). A begin() while shown cancels a pending
// hide, so a follow-up operation takes the overlay over without a gap;
// one while a show is pending keeps the original delay. onHide also runs
// when the overlay never showed, so the caller can unblock the page.
export function createShowTimer(setTimeoutFn, clearTimeoutFn, now, onShow, onHide) {
  var visible = false;
  var shownAt = 0;
  var timer = null;
  function clear() {
    if (timer !== null) {
      clearTimeoutFn(timer);
      timer = null;
    }
  }
  function hide() {
    timer = null;
    visible = false;
    onHide();
  }
  return {
    begin: function () {
      if (visible) {
        clear();
        return;
      }
      if (timer !== null) return;
      timer = setTimeoutFn(function () {
        timer = null;
        visible = true;
        shownAt = now();
        onShow();
      }, SHOW_DELAY_MS);
    },
    end: function (immediate) {
      clear();
      var remaining = MIN_VISIBLE_MS - (now() - shownAt);
      if (!visible || immediate || remaining <= 0) {
        hide();
        return;
      }
      timer = setTimeoutFn(hide, remaining);
    },
    visible: function () { return visible; },
  };
}

// progressCount is the "3 of 10" line, plus the failures when there are
// any. Empty for an uncounted operation.
export function progressCount(done, total, failed) {
  if (!(total > 0)) return '';
  var s = done + ' of ' + total;
  if (failed > 0) s += ' (' + failed + ' failed)';
  return s;
}

// footerText is the overlay's last line for an operation: what Esc does,
// or, once pressed on one that can't be cut off, what happens instead.
// stopUnit names the step it stops after; by default a message for a
// counted operation, a request otherwise.
export function footerText(stopped, total, stopUnit) {
  if (!stopped) return FOOTER_CANCEL;
  return 'Stopping after the current ' + (stopUnit || (total > 0 ? 'message' : 'request')) + '…';
}

// The cat's two animation frames, the same pixel grid as the TUI's
// (tui/internal/dialog/progress.go): '.' is transparent; a tail on the
// left, the tart body, the head with ears on the right, legs below.
var CAT_FRAMES = [
  ['..tttttg.g', '..tpmptggg', 'ggtppptgkg', '..tmpptggg', '..tttttgg.', '...g.g.g.g'],
  ['..tttttg.g', '..tpmptggg', '..tppptgkg', 'ggtmpptggg', '..tttttgg.', '..g.g.g.g.'],
];
var CAT_COLORS = { t: '#f5c38c', p: '#ff99ff', m: '#ff3399', g: '#999999', k: '#000000' };

// catSpriteSvg draws both cat frames side by side, one SVG unit per
// pixel, for a background-image the CSS steps through (two frames).
export function catSpriteSvg() {
  var width = CAT_FRAMES[0][0].length;
  var height = CAT_FRAMES[0].length;
  var rects = [];
  CAT_FRAMES.forEach(function (frame, f) {
    frame.forEach(function (row, y) {
      for (var x = 0; x < row.length; x++) {
        var color = CAT_COLORS[row[x]];
        if (color) {
          rects.push('<rect x="' + (f * width + x) + '" y="' + y + '" width="1" height="1" fill="' + color + '"/>');
        }
      }
    });
  });
  return '<svg xmlns="http://www.w3.org/2000/svg" viewBox="0 0 ' + (2 * width) + ' ' + height +
    '" shape-rendering="crispEdges">' + rects.join('') + '</svg>';
}

// wait resolves true after ms, or false as soon as signal aborts.
export function wait(ms, signal) {
  return new Promise(function (resolve) {
    if (signal.aborted) {
      resolve(false);
      return;
    }
    var timer = setTimeout(function () {
      signal.removeEventListener('abort', onAbort);
      resolve(true);
    }, ms);
    function onAbort() {
      clearTimeout(timer);
      resolve(false);
    }
    signal.addEventListener('abort', onAbort);
  });
}

// Running operations, oldest first; the overlay shows the newest.
var ops = [];
var showTimer = null;

function el(id) { return document.getElementById(id); }

// init sets up the overlay on the first operation.
function init() {
  if (showTimer) return;
  el('progressOverlay').style.setProperty('--cat-sprite',
    'url("data:image/svg+xml,' + encodeURIComponent(catSpriteSvg()) + '")');
  showTimer = createShowTimer(
    function (f, ms) { return setTimeout(f, ms); },
    function (t) { clearTimeout(t); },
    function () { return Date.now(); },
    function () {
      el('progressOverlay').classList.add('shown');
    },
    function () {
      var overlay = el('progressOverlay');
      overlay.classList.remove('shown');
      overlay.hidden = true;
      document.removeEventListener('keydown', onKey, true);
      document.removeEventListener('keyup', onKey, true);
      document.removeEventListener('keypress', onKey, true);
    });
}

// onKey swallows every key while an operation runs (the overlay catches
// clicks); Esc cancels.
function onKey(ev) {
  ev.preventDefault();
  ev.stopPropagation();
  if (ev.type === 'keydown' && ev.key === 'Escape') escape();
}

function blockPage() {
  el('progressOverlay').hidden = false;
  if (document.activeElement && document.activeElement.blur) document.activeElement.blur();
  document.addEventListener('keydown', onKey, true);
  document.addEventListener('keyup', onKey, true);
  document.addEventListener('keypress', onKey, true);
}

// escape cancels every running operation: each one's signal aborts, and
// an abortable one (a load) closes at once. One that can't be cut off
// stays until its caller finishes it, saying it's stopping.
function escape() {
  ops.slice().forEach(function (op) {
    if (op.stopped) return;
    op.stopped = true;
    op.controller.abort();
    if (op.abortable) remove(op, true);
  });
  render();
}

function remove(op, immediate) {
  var i = ops.indexOf(op);
  if (i === -1) return;
  ops.splice(i, 1);
  if (ops.length === 0) {
    showTimer.end(immediate);
  } else {
    render();
  }
}

function render() {
  var op = ops[ops.length - 1];
  if (!op) return;
  var counted = op.total > 0;
  el('progressTitle').textContent = counted ? op.title : op.title + '…';
  el('progressTrack').hidden = !counted;
  el('progressSpinner').hidden = counted;
  var fraction = counted ? Math.min(op.done / op.total, 1) : 0;
  el('progressTrack').style.setProperty('--p', String(fraction));
  el('progressCount').textContent = progressCount(op.done, op.total, op.failed);
  el('progressFooter').textContent = footerText(op.stopped, op.total, op.stopUnit);
}

// startOperation blocks the page for a slow operation and returns its
// handle: signal (aborted by Esc), step(done, failed) for a counted one
// (total > 0), stopped() once Esc was pressed, and finish(). abortable
// operations (loads) close at once on Esc; the others stay until
// finish(), so a request that's already sent can't be left hanging.
// stopUnit is what such an operation stops after (see footerText).
export function startOperation(opts) {
  if (ops.length === 0) {
    init();
    blockPage();
  }
  var op = {
    title: opts.title,
    total: opts.total || 0,
    abortable: !!opts.abortable,
    stopUnit: opts.stopUnit,
    done: 0,
    failed: 0,
    stopped: false,
    controller: new AbortController(),
  };
  ops.push(op);
  showTimer.begin();
  render();
  return {
    signal: op.controller.signal,
    step: function (done, failed) {
      op.done = done;
      op.failed = failed || 0;
      if (ops[ops.length - 1] === op) render();
    },
    stopped: function () { return op.stopped; },
    finish: function () { remove(op, false); },
  };
}

var DEMO_STEPS = 20;
var DEMO_STEP_MS = 300;
var DEMO_FAIL_EVERY = 7;
var DEMO_SPINNER_MS = 4000;
export var DEMO_FINISHED = 'Progress demo finished';
export var DEMO_CANCELLED = 'Progress demo cancelled';

// runProgressDemo plays the overlay without a connection, like the TUI's
// :progress-demo: a counted run of DEMO_STEPS steps (every
// DEMO_FAIL_EVERY-th "failing"), then a spinner run. Esc ends it. Resolves
// to the result line.
export function runProgressDemo() {
  var counted = startOperation({ title: 'Progress demo', total: DEMO_STEPS });
  var failed = 0;
  function next(i) {
    return wait(DEMO_STEP_MS, counted.signal).then(function (ok) {
      if (!ok) {
        counted.finish();
        return DEMO_CANCELLED;
      }
      if (i % DEMO_FAIL_EVERY === 0) failed++;
      counted.step(i, failed);
      if (i < DEMO_STEPS) return next(i + 1);
      // Started before the counted one finishes, so it takes over the
      // visible overlay instead of closing and reopening it.
      var uncounted = startOperation({ title: 'Progress demo: no count', abortable: true });
      counted.finish();
      return wait(DEMO_SPINNER_MS, uncounted.signal).then(function (done) {
        uncounted.finish();
        return done ? DEMO_FINISHED : DEMO_CANCELLED;
      });
    });
  }
  return next(1);
}
