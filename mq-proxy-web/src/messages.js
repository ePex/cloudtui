import { $, showError, clearError, showView, escapeHtml, truncate, sortedHeaderEntries } from './dom.js';
import {
  apiCall,
  buildListMessagesParams,
  appendMessages,
  buildBulkDeleteBody,
  buildBulkMoveBody,
  buildSingleMessageFilter,
  DEFAULT_MAX_COUNT,
} from './api.js';
import { state } from './state.js';
import { loadQueues } from './queues.js';
import { openMovePicker } from './movepicker.js';
import { confirmDialog, openSendModal } from './dialogs.js';

export function openMessages(queueName) {
  state.currentQueue = queueName;
  state.selectedMessageIds = new Set();
  $('msgFilterType').value = '';
  $('msgFilterMax').value = String(DEFAULT_MAX_COUNT);
  showView('messagesView');
  loadMessages();
}

export function loadMessages() {
  var queueName = state.currentQueue;
  var maxCount = parseInt($('msgFilterMax').value, 10) || DEFAULT_MAX_COUNT;
  var jmsType = $('msgFilterType').value.trim();
  $('messagesTitle').textContent = queueName + ' (max=' + maxCount + ')';
  clearError($('messagesError'));
  state.selectedMessageIds = new Set();
  state.messagesHasMore = false;
  $('loadMoreMessagesBtn').hidden = true;
  var tbody = $('messagesTable').querySelector('tbody');
  tbody.innerHTML = '<tr><td colspan="5">Loading…</td></tr>';
  return apiCall(state.conn, 'list-messages', {
    query: buildListMessagesParams(queueName, { jmsType: jmsType, maxCount: maxCount }),
  }).then(function (messages) {
    state.messages = messages || [];
    state.messagesHasMore = !!(messages && messages.hasMore);
    renderMessages(state.messages);
    updateLoadMoreButton();
    updateBulkActionsUI();
  }, function (err) {
    tbody.innerHTML = '';
    showError($('messagesError'), err);
  });
}

// "Load more" — fetches the next page after the last currently-rendered
// message and appends it, rather than replacing state.messages (which
// loadMessages does for every other reload: Apply, opening a queue, or
// after an action completes). The cursor is read directly off the last
// rendered row rather than tracked as separate state, so it can't drift
// out of sync with what's actually on screen.
export function loadMoreMessages() {
  var queueName = state.currentQueue;
  var maxCount = parseInt($('msgFilterMax').value, 10) || DEFAULT_MAX_COUNT;
  var jmsType = $('msgFilterType').value.trim();
  var lastMessage = state.messages[state.messages.length - 1];
  if (!lastMessage) return;
  clearError($('messagesError'));
  return apiCall(state.conn, 'list-messages', {
    query: buildListMessagesParams(queueName, { jmsType: jmsType, maxCount: maxCount, afterMessageId: lastMessage.messageId }),
  }).then(function (page) {
    page = page || [];
    state.messages = appendMessages(state.messages, page);
    state.messagesHasMore = !!page.hasMore;
    renderMessages(state.messages);
    updateLoadMoreButton();
    updateBulkActionsUI();
  }, function (err) {
    showError($('messagesError'), err);
  });
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

// Wires the messages view's own static controls — called once at startup.
export function initMessages() {
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
      apiCall(state.conn, 'delete-messages', {
        method: 'POST',
        body: buildBulkDeleteBody(queueName, ids),
      }).then(function () {
        loadMessages();
        loadQueues();
      }, function (err) {
        showError($('messagesError'), err);
      });
    });
  });

  $('moveSelectedMessagesBtn').addEventListener('click', function () {
    var queueName = state.currentQueue;
    var ids = Array.from(state.selectedMessageIds);
    openMovePicker(queueName, function (target) {
      return apiCall(state.conn, 'move-messages', {
        method: 'POST',
        body: buildBulkMoveBody(queueName, target, ids),
      }).then(function () {
        loadMessages();
        loadQueues();
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
      apiCall(state.conn, 'delete-messages', {
        method: 'POST',
        body: [{ sourceQueue: queueName, filter: buildSingleMessageFilter(message.messageId) }],
      }).then(function () {
        showView('messagesView');
        loadMessages();
        loadQueues();
      }, function (err) {
        showError($('messageDetailError'), err);
      });
    });
  });

  $('moveMessageBtn').addEventListener('click', function () {
    openMovePicker(state.currentQueue, function (target) {
      var message = state.currentMessage;
      return apiCall(state.conn, 'move-messages', {
        method: 'POST',
        body: [{ sourceQueue: state.currentQueue, targetQueue: target, filter: buildSingleMessageFilter(message.messageId) }],
      }).then(function () {
        showView('messagesView');
        loadMessages();
        loadQueues();
      });
    }, $('messageDetailError'));
  });
}
