import { $, showError, clearError, escapeHtml, makeButton, showView } from './dom.js';
import { apiCall, buildBulkFilter, isAbortError } from './api.js';
import { state } from './state.js';
import { openMessages } from './messages.js';
import { openMovePicker } from './movepicker.js';
import { jmsTypePrompt, confirmDialog, openSendModal } from './dialogs.js';
import { startOperation } from './progress.js';

// Client-side only — list-queues (spec/11) has no server-side filter
// param, so this narrows the already-fetched list. Case-insensitive
// substring match on queue name.
export function filterQueues(queues, needle) {
  var trimmed = (needle || '').trim().toLowerCase();
  if (!trimmed) return queues.slice();
  return queues.filter(function (q) { return q.name.toLowerCase().indexOf(trimmed) !== -1; });
}

// column: one of QUEUE_SORT_COLUMNS. direction: 'asc' or 'desc'. `name`
// compares as text (localeCompare); every other column is numeric.
// Returns a new array — never mutates the input.
export function sortQueues(queues, column, direction) {
  var sign = direction === 'desc' ? -1 : 1;
  return queues.slice().sort(function (a, b) {
    var cmp = column === 'name' ? a.name.localeCompare(b.name) : a[column] - b[column];
    return cmp * sign;
  });
}

// Runs under the progress overlay; Esc cancels the request and puts the
// last-loaded list back, saying how to retry.
export function loadQueues() {
  clearError($('queuesError'));
  $('queuesInfo').hidden = true;
  var tbody = $('queuesTable').querySelector('tbody');
  tbody.innerHTML = '<tr><td colspan="7">Loading…</td></tr>';
  var op = startOperation({ title: 'Loading queues', abortable: true });
  return apiCall(state.conn, 'list-queues', { signal: op.signal }).then(function (queues) {
    state.queues = queues || [];
    applyQueueView();
  }, function (err) {
    if (isAbortError(err)) {
      applyQueueView();
      $('queuesInfo').textContent = 'Loading cancelled — press Refresh to retry';
      $('queuesInfo').hidden = false;
      return;
    }
    tbody.innerHTML = '';
    showError($('queuesError'), err);
  }).finally(op.finish);
}

// Applies the current filter text + sort column/direction to
// state.queues (the last-fetched raw list) and re-renders — never
// refetches, so typing in the filter or clicking a header is instant.
export function applyQueueView() {
  var filtered = filterQueues(state.queues, $('queueFilter').value);
  var sorted = sortQueues(filtered, state.queueSort.column, state.queueSort.direction);
  renderQueues(sorted);
  updateSortIndicators();
}

function updateSortIndicators() {
  Array.prototype.forEach.call($('queuesTable').querySelectorAll('th.sortable'), function (th) {
    var indicator = th.querySelector('.sort-indicator');
    if (indicator) indicator.remove();
    if (th.dataset.column === state.queueSort.column) {
      var span = document.createElement('span');
      span.className = 'sort-indicator';
      span.textContent = state.queueSort.direction === 'asc' ? '▲' : '▼';
      th.appendChild(span);
    }
  });
}

function renderQueues(queues) {
  var tbody = $('queuesTable').querySelector('tbody');
  tbody.innerHTML = '';
  queues.forEach(function (q) {
    var tr = document.createElement('tr');
    tr.innerHTML =
      '<td class="queue-name">' + escapeHtml(q.name) + '</td>' +
      '<td>' + q.messageCount + '</td>' +
      '<td>' + q.consumerCount + '</td>' +
      '<td>' + q.enqueuedCount + '</td>' +
      '<td>' + q.dequeuedCount + '</td>' +
      '<td>' + q.producerCount + '</td>' +
      '<td class="row-actions"></td>';
    tr.querySelector('.queue-name').addEventListener('click', function () {
      openMessages(q.name);
    });
    var actions = tr.querySelector('.row-actions');
    actions.appendChild(makeButton('Purge', function () { purgeQueue(q.name); }, 'btn-danger'));
    actions.appendChild(makeButton('Move all…', function () { moveAllMessages(q.name); }));
    actions.appendChild(makeButton('Send…', function () { openSendModal(q.name); }, 'btn-primary'));
    tbody.appendChild(tr);
  });
}

// purgeTitle and moveAllTitle are the progress overlay's titles, worded
// like the TUI's.
export function purgeTitle(queueName, jmsType) {
  return jmsType ? 'Purging ' + jmsType + ' messages from "' + queueName + '"' : 'Purging "' + queueName + '"';
}

export function moveAllTitle(queueName, target, jmsType) {
  var what = jmsType ? jmsType + ' messages' : 'all messages';
  return 'Moving ' + what + ' from "' + queueName + '" to "' + target + '"';
}

// Purge and move all are one request each, under the progress overlay:
// one that's sent can't be taken back, so Esc only says it's stopping.
export function purgeQueue(queueName) {
  jmsTypePrompt('Purge "' + queueName + '" — JMS Type (optional)', queueName).then(function (result) {
    if (result.cancelled) return;
    var filter = buildBulkFilter(result.jmsType);
    var question = result.jmsType
      ? 'Purge "' + queueName + '"? All ' + result.jmsType + ' messages will be deleted.'
      : 'Purge "' + queueName + '"? All messages will be deleted.';
    confirmDialog(question).then(function (confirmed) {
      if (!confirmed) return;
      var op = startOperation({ title: purgeTitle(queueName, result.jmsType) });
      apiCall(state.conn, 'delete-messages', {
        method: 'POST',
        body: [{ sourceQueue: queueName, filter: filter }],
      }).then(loadQueues, function (err) {
        showError($('queuesError'), err);
      }).finally(op.finish);
    });
  });
}

export function moveAllMessages(queueName) {
  jmsTypePrompt('Move All "' + queueName + '" — JMS Type (optional)', queueName).then(function (result) {
    if (result.cancelled) return;
    var filter = buildBulkFilter(result.jmsType);
    openMovePicker(queueName, function (target) {
      var op = startOperation({ title: moveAllTitle(queueName, target, result.jmsType) });
      return apiCall(state.conn, 'move-messages', {
        method: 'POST',
        body: [{ sourceQueue: queueName, targetQueue: target, filter: filter }],
      }).then(loadQueues).finally(op.finish);
    }, $('queuesError'));
  });
}

// Wires the queues view's own static controls — called once at startup.
export function initQueues() {
  Array.prototype.forEach.call($('queuesTable').querySelectorAll('th.sortable'), function (th) {
    th.addEventListener('click', function () {
      var column = th.dataset.column;
      if (state.queueSort.column === column) {
        state.queueSort.direction = state.queueSort.direction === 'asc' ? 'desc' : 'asc';
      } else {
        state.queueSort = { column: column, direction: 'asc' };
      }
      applyQueueView();
    });
  });

  $('queueFilter').addEventListener('input', applyQueueView);
  $('refreshQueuesBtn').addEventListener('click', loadQueues);
  $('backToQueuesBtn').addEventListener('click', function () { showView('queuesView'); });
}
