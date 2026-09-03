import { $, clearError, showError } from './dom.js';
import { apiCall } from './api.js';
import { state } from './state.js';

var DLQ_IMQ_PREFIX = /^(dlq\.|imq\.)/;
var SYSTEM_PREFIX = /^(activemq\.|statistics\.)/;

// Four-tier move-target ordering, mirroring the TUI's move-picker
// (spec/09) — DLQ requeue is the dominant real workflow, so a queue
// whose name matches the source's stripped dlq./imq. prefix is pinned
// first. Tiers: 0 preferred, 1 regular, 2 dlq./imq.-prefixed, 3
// activemq./statistics.-prefixed. Sorted alphabetically within each
// tier; the source queue itself is excluded.
export function tierForQueue(name, sourceQueue) {
  var preferred = sourceQueue.match(DLQ_IMQ_PREFIX)
    ? sourceQueue.slice(sourceQueue.indexOf('.') + 1)
    : null;
  if (preferred !== null && name === preferred) return 0;
  if (SYSTEM_PREFIX.test(name)) return 3;
  if (DLQ_IMQ_PREFIX.test(name)) return 2;
  return 1;
}

export function sortMoveTargets(queueNames, sourceQueue) {
  return queueNames
    .filter(function (name) { return name !== sourceQueue; })
    .map(function (name) { return { name: name, tier: tierForQueue(name, sourceQueue) }; })
    .sort(function (a, b) {
      if (a.tier !== b.tier) return a.tier - b.tier;
      return a.name.localeCompare(b.name);
    })
    .map(function (entry) { return entry.name; });
}

var movePickerState = null;

export function openMovePicker(sourceQueue, onConfirm, errorEl) {
  clearError(errorEl);
  $('movePickerFilter').value = '';
  $('movePickerModal').hidden = false;
  movePickerState = { onConfirm: onConfirm, errorEl: errorEl, sourceQueue: sourceQueue, allNames: [] };
  apiCall(state.conn, 'list-queues').then(function (queues) {
    movePickerState.allNames = sortMoveTargets((queues || []).map(function (q) { return q.name; }), sourceQueue);
    renderMovePickerList(movePickerState.allNames);
  }, function (err) {
    $('movePickerModal').hidden = true;
    showError(errorEl, err);
  });
}

export function renderMovePickerList(names) {
  var list = $('movePickerList');
  list.innerHTML = '';
  names.forEach(function (name) {
    var li = document.createElement('li');
    li.textContent = name;
    li.addEventListener('click', function () {
      var picker = movePickerState;
      $('movePickerModal').hidden = true;
      picker.onConfirm(name).catch(function (err) {
        showError(picker.errorEl, err);
      });
    });
    list.appendChild(li);
  });
}

// Wires the move picker's own static controls (filter input, cancel
// button) — called once at startup. Per-name list items are wired inside
// renderMovePickerList itself, since they're created dynamically.
export function initMovePicker() {
  $('movePickerFilter').addEventListener('input', function () {
    var needle = $('movePickerFilter').value.toLowerCase();
    var filtered = movePickerState
      ? movePickerState.allNames.filter(function (name) { return name.toLowerCase().indexOf(needle) !== -1; })
      : [];
    renderMovePickerList(filtered);
  });

  $('movePickerCancel').addEventListener('click', function () {
    $('movePickerModal').hidden = true;
  });
}
