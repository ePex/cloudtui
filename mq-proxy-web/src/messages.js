import { $, showError, clearError, showView, escapeHtml, truncate, sortedHeaderEntries } from './dom.js';
import {
  apiCall,
  buildListMessagesParams,
  appendMessages,
  buildBulkDeleteBody,
  buildBulkMoveBody,
  buildSingleMessageFilter,
  DEFAULT_MAX_COUNT,
  isAbortError,
  BULK_BATCH_SIZE,
  runBatches,
  bulkSummary,
} from './api.js';
import { state } from './state.js';
import { loadQueues } from './queues.js';
import { openMovePicker } from './movepicker.js';
import { confirmDialog, openSendModal } from './dialogs.js';
import { startOperation } from './progress.js';
import {
  EXPORT_CAP,
  toExportRecord,
  toNdjson,
  exportFileName,
  fetchAllForExport,
  exportSummary,
  downloadNdjson,
} from './export.js';

export function openMessages(queueName) {
  state.currentQueue = queueName;
  state.selectedMessageIds = new Set();
  $('msgFilterType').value = '';
  $('msgFilterMax').value = String(DEFAULT_MAX_COUNT);
  showView('messagesView');
  loadMessages();
}

// Runs under the progress overlay. Esc cancels the request and empties
// the list, since what it showed may be another queue's.
export function loadMessages() {
  var queueName = state.currentQueue;
  var maxCount = parseInt($('msgFilterMax').value, 10) || DEFAULT_MAX_COUNT;
  var jmsType = $('msgFilterType').value.trim();
  $('messagesTitle').textContent = queueName + ' (max=' + maxCount + ')';
  clearError($('messagesError'));
  $('messagesInfo').hidden = true;
  state.selectedMessageIds = new Set();
  state.messagesHasMore = false;
  $('loadMoreMessagesBtn').hidden = true;
  var tbody = $('messagesTable').querySelector('tbody');
  tbody.innerHTML = '<tr><td colspan="5">Loading…</td></tr>';
  var op = startOperation({ title: 'Loading messages from "' + queueName + '"', abortable: true });
  return apiCall(state.conn, 'list-messages', {
    query: buildListMessagesParams(queueName, { jmsType: jmsType, maxCount: maxCount }),
    signal: op.signal,
  }).then(function (messages) {
    state.messages = messages || [];
    state.messagesHasMore = !!(messages && messages.hasMore);
    renderMessages(state.messages);
    updateLoadMoreButton();
    updateBulkActionsUI();
  }, function (err) {
    if (isAbortError(err)) {
      state.messages = [];
      renderMessages(state.messages);
      updateBulkActionsUI();
      showCancelledLoad('Apply');
      return;
    }
    tbody.innerHTML = '';
    showError($('messagesError'), err);
  }).finally(op.finish);
}

// showCancelledLoad says in the info line that a load was cancelled, and
// which button retries it.
function showCancelledLoad(retryButton) {
  $('messagesInfo').textContent = 'Loading cancelled — press ' + retryButton + ' to retry';
  $('messagesInfo').hidden = false;
}

// "Load more" — fetches the next page after the last currently-rendered
// message and appends it, rather than replacing state.messages (which
// loadMessages does for every other reload: Apply, opening a queue, or
// after an action completes). The cursor is read directly off the last
// rendered row rather than tracked as separate state, so it can't drift
// out of sync with what's actually on screen. Esc on the progress overlay
// cancels it and keeps the loaded messages.
export function loadMoreMessages() {
  var queueName = state.currentQueue;
  var maxCount = parseInt($('msgFilterMax').value, 10) || DEFAULT_MAX_COUNT;
  var jmsType = $('msgFilterType').value.trim();
  var lastMessage = state.messages[state.messages.length - 1];
  if (!lastMessage) return;
  clearError($('messagesError'));
  $('messagesInfo').hidden = true;
  var op = startOperation({ title: 'Loading more messages from "' + queueName + '"', abortable: true });
  return apiCall(state.conn, 'list-messages', {
    query: buildListMessagesParams(queueName, { jmsType: jmsType, maxCount: maxCount, afterMessageId: lastMessage.messageId }),
    signal: op.signal,
  }).then(function (page) {
    page = page || [];
    state.messages = appendMessages(state.messages, page);
    state.messagesHasMore = !!page.hasMore;
    renderMessages(state.messages);
    updateLoadMoreButton();
    updateBulkActionsUI();
  }, function (err) {
    if (isAbortError(err)) {
      showCancelledLoad('Load more');
      return;
    }
    showError($('messagesError'), err);
  }).finally(op.finish);
}

function updateLoadMoreButton() {
  $('loadMoreMessagesBtn').hidden = !state.messagesHasMore;
}

function renderMessages(messages) {
  var tbody = $('messagesTable').querySelector('tbody');
  tbody.innerHTML = '';
  messages.forEach(function (m) {
    var tr = document.createElement('tr');
    tr.className = 'message-row';
    tr.innerHTML =
      '<td><input type="checkbox"></td>' +
      '<td>' + escapeHtml(m.messageId) + '</td>' +
      '<td>' + escapeHtml(m.jmsType) + '</td>' +
      '<td>' + escapeHtml(m.timestamp) + '</td>' +
      '<td>' + escapeHtml(truncate(m.body, 80)) + '</td>';
    var checkbox = tr.querySelector('input[type="checkbox"]');
    checkbox.checked = state.selectedMessageIds.has(m.messageId);
    checkbox.addEventListener('click', function (ev) {
      ev.stopPropagation();
      if (checkbox.checked) {
        state.selectedMessageIds.add(m.messageId);
      } else {
        state.selectedMessageIds.delete(m.messageId);
      }
      updateBulkActionsUI();
    });
    tr.addEventListener('click', function () { openMessageDetail(m); });
    tbody.appendChild(tr);
  });
}

// Keeps the "N selected" label, the Delete/Move selected buttons'
// disabled state, and the header checkbox's checked/indeterminate
// state all in sync with state.selectedMessageIds — called after
// every selection change instead of a full re-render, since the
// table rows themselves don't need to change.
function updateBulkActionsUI() {
  var count = state.selectedMessageIds.size;
  var total = state.messages.length;
  $('messagesSelectedCount').textContent = count + ' selected';
  $('deleteSelectedMessagesBtn').disabled = count === 0;
  $('moveSelectedMessagesBtn').disabled = count === 0;
  var headerCheckbox = $('selectAllMessagesCheckbox');
  headerCheckbox.checked = total > 0 && count === total;
  headerCheckbox.indeterminate = count > 0 && count < total;
}

function selectAllMessages() {
  state.selectedMessageIds = new Set(state.messages.map(function (m) { return m.messageId; }));
  renderMessages(state.messages);
  updateBulkActionsUI();
}

function selectNoneMessages() {
  state.selectedMessageIds = new Set();
  renderMessages(state.messages);
  updateBulkActionsUI();
}

function openMessageDetail(message) {
  state.currentMessage = message;
  clearError($('messageDetailError'));
  var fields = $('messageDetailFields');
  fields.innerHTML =
    '<dt>Queue</dt><dd>' + escapeHtml(message.sourceQueue || state.currentQueue) + '</dd>' +
    '<dt>Message ID</dt><dd>' + escapeHtml(message.messageId) + '</dd>' +
    '<dt>JMS Type</dt><dd>' + escapeHtml(message.jmsType) + '</dd>' +
    '<dt>Timestamp</dt><dd>' + escapeHtml(message.timestamp) + '</dd>';
  var headerEntries = sortedHeaderEntries(message.headers);
  if (headerEntries.length > 0) {
    fields.innerHTML += '<dt class="section-label">Headers</dt>' + headerEntries.map(function (entry) {
      return '<dt>' + escapeHtml(entry[0]) + '</dt><dd>' + escapeHtml(entry[1]) + '</dd>';
    }).join('');
  }
  $('messageDetailBody').textContent = message.body || '';
  showView('messageDetailView');
}

// "Export": the selected messages if any are selected (already loaded),
// otherwise everything the current JMS Type filter matches — all pages,
// not just the loaded ones, up to EXPORT_CAP — downloaded as NDJSON
// (spec-wip fe-message-export). The result line says when it can't be
// everything: the server reported truncated, the cap stopped it, or the
// queue's message count (from the queue list) is higher.
function exportMessages() {
  var queueName = state.currentQueue;
  var info = $('messagesInfo');
  clearError($('messagesError'));
  info.hidden = false;
  info.textContent = 'Exporting…';
  var selected = state.messages.filter(function (m) { return state.selectedMessageIds.has(m.messageId); });
  var collect = selected.length > 0
    ? Promise.resolve({ messages: selected, truncated: false, capped: false, selection: true })
    : fetchAllForExport(apiCall, state.conn, queueName, $('msgFilterType').value.trim(),
      parseInt($('msgFilterMax').value, 10) || DEFAULT_MAX_COUNT, EXPORT_CAP);
  collect.then(function (result) {
    var records = result.messages.map(function (m) { return toExportRecord(m, queueName); });
    downloadNdjson(exportFileName(queueName, new Date()), toNdjson(records));
    var queue = state.queues.find(function (q) { return q.name === queueName; });
    var total = !result.selection && !$('msgFilterType').value.trim() && queue ? queue.messageCount : 0;
    info.textContent = exportSummary(records.length, total, result.truncated, result.capped);
  }, function (err) {
    info.hidden = true;
    showError($('messagesError'), err);
  });
}

// runSelected deletes or moves the selected messages (opts.ids) in
// batches under the progress overlay — counted when there's more than
// one batch, else a spinner; Esc stops before the next batch — then
// reloads and reports the result in the info line. opts: ids, verb
// (the mq-proxy command), buildBody(batch), title, pastVerb ("Deleted",
// "Moved"), target (' to "queue"' or '').
function runSelected(opts) {
  var counted = opts.ids.length > BULK_BATCH_SIZE;
  var op = startOperation({
    title: opts.title,
    total: counted ? opts.ids.length : 0,
    stopUnit: counted ? 'batch' : undefined,
  });
  var call = function (verb, callOpts) { return apiCall(state.conn, verb, callOpts); };
  return runBatches(call, opts.ids, opts.buildBody, opts.verb, { onStep: op.step, stopped: op.stopped })
    .then(function (result) {
      var summary = bulkSummary(opts.pastVerb, opts.target, result.done, result.failed, opts.ids.length);
      // The reloads start before this operation finishes, so they take
      // the overlay over instead of closing and reopening it.
      var reload = loadMessages();
      loadQueues();
      op.finish();
      return reload.then(function () {
        if ($('messagesInfo').hidden) {
          $('messagesInfo').textContent = summary;
          $('messagesInfo').hidden = false;
        }
        if (result.lastError && $('messagesError').hidden) showError($('messagesError'), result.lastError);
      });
    });
}

// selectedTitle is the progress overlay's title for deleting or moving
// n selected messages (verb "Deleting"/"Moving", target as in
// runSelected), worded like the TUI's.
function selectedTitle(verb, n, queueName, target) {
  var what = n === 1 ? 'message' : n + ' message(s)';
  return verb + ' ' + what + ' from "' + queueName + '"' + target;
}

// Wires the messages view's own static controls — called once at startup.
export function initMessages() {
  $('exportMessagesBtn').addEventListener('click', exportMessages);
  $('loadMoreMessagesBtn').addEventListener('click', loadMoreMessages);

  $('selectAllMessagesBtn').addEventListener('click', selectAllMessages);
  $('selectNoneMessagesBtn').addEventListener('click', selectNoneMessages);
  $('selectAllMessagesCheckbox').addEventListener('change', function () {
    if ($('selectAllMessagesCheckbox').checked) {
      selectAllMessages();
    } else {
      selectNoneMessages();
    }
  });

  $('deleteSelectedMessagesBtn').addEventListener('click', function () {
    var queueName = state.currentQueue;
    var ids = Array.from(state.selectedMessageIds);
    confirmDialog('Delete ' + ids.length + ' selected message' + (ids.length === 1 ? '' : 's') + '?').then(function (confirmed) {
      if (!confirmed) return;
      runSelected({
        ids: ids,
        verb: 'delete-messages',
        buildBody: function (batch) { return buildBulkDeleteBody(queueName, batch); },
        title: selectedTitle('Deleting', ids.length, queueName, ''),
        pastVerb: 'Deleted',
        target: '',
      });
    });
  });

  $('moveSelectedMessagesBtn').addEventListener('click', function () {
    var queueName = state.currentQueue;
    var ids = Array.from(state.selectedMessageIds);
    openMovePicker(queueName, function (target) {
      var to = ' to "' + target + '"';
      return runSelected({
        ids: ids,
        verb: 'move-messages',
        buildBody: function (batch) { return buildBulkMoveBody(queueName, target, batch); },
        title: selectedTitle('Moving', ids.length, queueName, to),
        pastVerb: 'Moved',
        target: to,
      });
    }, $('messagesError'));
  });

  $('applyMsgFilterBtn').addEventListener('click', loadMessages);
  $('sendFromMessagesBtn').addEventListener('click', function () { openSendModal(state.currentQueue); });
  $('backToMessagesBtn').addEventListener('click', function () { showView('messagesView'); });

  $('deleteMessageBtn').addEventListener('click', function () {
    var message = state.currentMessage;
    var queueName = state.currentQueue;
    confirmDialog('Delete message "' + message.messageId + '"?').then(function (confirmed) {
      if (!confirmed) return;
      var op = startOperation({ title: selectedTitle('Deleting', 1, queueName, '') });
      apiCall(state.conn, 'delete-messages', {
        method: 'POST',
        body: [{ sourceQueue: queueName, filter: buildSingleMessageFilter(message.messageId) }],
      }).then(function () {
        showView('messagesView');
        loadMessages();
        loadQueues();
      }, function (err) {
        showError($('messageDetailError'), err);
      }).finally(op.finish);
    });
  });

  $('moveMessageBtn').addEventListener('click', function () {
    openMovePicker(state.currentQueue, function (target) {
      var message = state.currentMessage;
      var op = startOperation({ title: selectedTitle('Moving', 1, state.currentQueue, ' to "' + target + '"') });
      return apiCall(state.conn, 'move-messages', {
        method: 'POST',
        body: [{ sourceQueue: state.currentQueue, targetQueue: target, filter: buildSingleMessageFilter(message.messageId) }],
      }).then(function () {
        showView('messagesView');
        loadMessages();
        loadQueues();
      }).finally(op.finish);
    }, $('messageDetailError'));
  });
}
