/*
 * cloudtui AMQ web console — all page logic.
 *
 * Loaded as a classic (non-module) <script src="app.js"> so it works when
 * index.html is opened directly via file:// (Chrome blocks ES module
 * loading over file://, but has no such restriction on classic scripts).
 * The UMD-lite wrapper below makes the same file's pure functions
 * importable from Node for app.test.js, without a bundler or build step.
 */
(function (exports) {
  'use strict';

  // ---------------------------------------------------------------------
  // Pure helpers — no DOM, unit tested directly (app.test.js).
  // ---------------------------------------------------------------------

  function base64Encode(str) {
    if (typeof btoa === 'function') return btoa(str);
    return Buffer.from(str, 'utf-8').toString('base64');
  }

  function buildAuthHeader(username, password) {
    return 'Basic ' + base64Encode(username + ':' + password);
  }

  // Normalizes mq-proxy's two response envelope shapes (spec/11):
  // list endpoints return { data, errors: [...] }, single-item endpoints
  // return { data, error } or { data, error: null }. Always returns
  // { data, errors: [] } with errors as an array, empty when there were
  // none, so every call site has one shape to check.
  function parseEnvelope(json) {
    if (!json || typeof json !== 'object') {
      return { data: undefined, errors: [] };
    }
    if (Array.isArray(json.errors)) {
      return { data: json.data, errors: json.errors };
    }
    if (json.error) {
      return { data: json.data, errors: [json.error] };
    }
    return { data: json.data, errors: [] };
  }

  function errorMessageFrom(envelope, fallback) {
    var first = envelope.errors[0];
    if (!first) return fallback;
    if (typeof first === 'string') return first;
    if (first.message) return first.message;
    return fallback;
  }

  // Flattens a (possibly nested) params object into a mq-proxy-style query
  // string, e.g. { sourceQueue: 'orders', filter: { maxCount: 50 } } ->
  // "sourceQueue=orders&filter.maxCount=50". Nested keys only — this is
  // what makes list-messages' filter.* binding work (spec/11); mq-proxy
  // does not accept a flat/legacy duplicate form. undefined/null values
  // (at any depth) are omitted entirely rather than sent as "undefined".
  function buildQueryString(params) {
    var parts = [];
    function walk(obj, prefix) {
      Object.keys(obj).forEach(function (key) {
        var value = obj[key];
        if (value === undefined || value === null) return;
        var qualified = prefix ? prefix + '.' + key : key;
        if (typeof value === 'object' && !Array.isArray(value)) {
          walk(value, qualified);
        } else {
          parts.push(encodeURIComponent(qualified) + '=' + encodeURIComponent(value));
        }
      });
    }
    walk(params, '');
    return parts.join('&');
  }

  // Builds the list-messages query params object (spec/11): sourceQueue
  // and returnBody stay top-level, everything else nests under filter.*.
  function buildListMessagesParams(sourceQueue, opts) {
    opts = opts || {};
    return {
      sourceQueue: sourceQueue,
      returnBody: true,
      filter: {
        jmsType: opts.jmsType || undefined,
        messageId: opts.messageId || undefined,
        maxCount: opts.maxCount || undefined,
      },
    };
  }

  // How many messages the JMS Type autocomplete scans to populate its
  // suggestions — mirrors the TUI's own automatic-scan cap
  // (jmsTypeAutoScanCount, spec/08) rather than introducing a different
  // number. returnBody is left false: only the jmsType header is needed,
  // so there's no reason to pull message bodies over the wire for this.
  var JMS_TYPE_SCAN_MAX_COUNT = 500;

  function buildJmsTypeScanParams(sourceQueue) {
    return { sourceQueue: sourceQueue, returnBody: false, filter: { maxCount: JMS_TYPE_SCAN_MAX_COUNT } };
  }

  // Sorted, de-duplicated, non-empty JMS Type values found across a list
  // of messages — used to populate the JMS Type prompt's autocomplete.
  function extractDistinctJmsTypes(messages) {
    var seen = {};
    messages.forEach(function (m) {
      if (m.jmsType) seen[m.jmsType] = true;
    });
    return Object.keys(seen).sort();
  }

  // Shared by purge and move-all/drain: a blank JMS Type means "match
  // everything" (filter.maxCount stays unset, never sent), a typed one
  // narrows via filter.jmsType — maxCount is deliberately never set here
  // either, mirroring the TUI's PurgeQueue/MoveAllMessages vs.
  // DeleteMessages/MoveMessages distinction (spec/09): a blank filter is
  // still "match everything", not "match zero".
  function buildBulkFilter(jmsType) {
    var trimmed = (jmsType || '').trim();
    return trimmed ? { jmsType: trimmed } : {};
  }

  // send-message's jmsType field is required by mq-proxy's DTO (spec/11)
  // even though a blank entry is a normal, common case here — unlike the
  // TUI (spec/09), which hardcodes "text" since it has no JMS Type field
  // at all, this page exposes one but still needs a sane default when
  // it's left blank.
  function resolveJmsType(jmsType) {
    var trimmed = (jmsType || '').trim();
    return trimmed || 'text';
  }

  // Filter for acting on exactly one already-known message (single
  // delete/move) — matches the TUI proxy client's MessageFilter{MessageID,
  // MaxCount: 1} pattern (tui/internal/queue/proxy/proxy.go).
  function buildSingleMessageFilter(messageId) {
    return { messageId: messageId, maxCount: 1 };
  }

  // delete-messages/move-messages both accept an array of per-message
  // request objects (spec/11) — "the shape exists for bulk-operation
  // parity", which multi-select finally puts to real use: one array
  // element per selected message, each still scoped to exactly one
  // message via buildSingleMessageFilter (never a wider filter, so a
  // bulk action can never accidentally affect an unselected message).
  function buildBulkDeleteBody(sourceQueue, messageIds) {
    return messageIds.map(function (id) {
      return { sourceQueue: sourceQueue, filter: buildSingleMessageFilter(id) };
    });
  }

  function buildBulkMoveBody(sourceQueue, targetQueue, messageIds) {
    return messageIds.map(function (id) {
      return { sourceQueue: sourceQueue, targetQueue: targetQueue, filter: buildSingleMessageFilter(id) };
    });
  }

  var DLQ_IMQ_PREFIX = /^(dlq\.|imq\.)/;
  var SYSTEM_PREFIX = /^(activemq\.|statistics\.)/;

  // Four-tier move-target ordering, mirroring the TUI's move-picker
  // (spec/09) — DLQ requeue is the dominant real workflow, so a queue
  // whose name matches the source's stripped dlq./imq. prefix is pinned
  // first. Tiers: 0 preferred, 1 regular, 2 dlq./imq.-prefixed, 3
  // activemq./statistics.-prefixed. Sorted alphabetically within each
  // tier; the source queue itself is excluded.
  function tierForQueue(name, sourceQueue) {
    var preferred = sourceQueue.match(DLQ_IMQ_PREFIX)
      ? sourceQueue.slice(sourceQueue.indexOf('.') + 1)
      : null;
    if (preferred !== null && name === preferred) return 0;
    if (SYSTEM_PREFIX.test(name)) return 3;
    if (DLQ_IMQ_PREFIX.test(name)) return 2;
    return 1;
  }

  function sortMoveTargets(queueNames, sourceQueue) {
    return queueNames
      .filter(function (name) { return name !== sourceQueue; })
      .map(function (name) { return { name: name, tier: tierForQueue(name, sourceQueue) }; })
      .sort(function (a, b) {
        if (a.tier !== b.tier) return a.tier - b.tier;
        return a.name.localeCompare(b.name);
      })
      .map(function (entry) { return entry.name; });
  }

  function truncate(str, maxLen) {
    if (str == null) return '';
    return str.length > maxLen ? str.slice(0, maxLen - 1) + '…' : str;
  }

  // mq-proxy always returns a message's JMS properties as a headers map
  // (spec/11), independent of returnBody — sorted [key, value] pairs for
  // the message detail view's Headers section (mirrors the TUI's own
  // "all captured JMS fields as sorted Key: value lines", spec/08).
  function sortedHeaderEntries(headers) {
    return Object.keys(headers || {}).sort().map(function (key) {
      return [key, headers[key]];
    });
  }

  // Client-side only — list-queues (spec/11) has no server-side filter
  // param, so this narrows the already-fetched list. Case-insensitive
  // substring match on queue name.
  function filterQueues(queues, needle) {
    var trimmed = (needle || '').trim().toLowerCase();
    if (!trimmed) return queues.slice();
    return queues.filter(function (q) { return q.name.toLowerCase().indexOf(trimmed) !== -1; });
  }

  var QUEUE_SORT_COLUMNS = ['name', 'messageCount', 'consumerCount', 'enqueuedCount', 'dequeuedCount', 'producerCount'];

  // column: one of QUEUE_SORT_COLUMNS. direction: 'asc' or 'desc'. `name`
  // compares as text (localeCompare); every other column is numeric.
  // Returns a new array — never mutates the input.
  function sortQueues(queues, column, direction) {
    var sign = direction === 'desc' ? -1 : 1;
    return queues.slice().sort(function (a, b) {
      var cmp = column === 'name' ? a.name.localeCompare(b.name) : a[column] - b[column];
      return cmp * sign;
    });
  }

  // ---------------------------------------------------------------------
  // API client
  // ---------------------------------------------------------------------

  // conn: { baseUrl, username, password }. verb: e.g. "list-queues".
  // opts: { method, query, body } — query is a (possibly nested) params
  // object per buildQueryString; body is a plain object, JSON-encoded.
  function apiCall(conn, verb, opts) {
    opts = opts || {};
    var url = conn.baseUrl.replace(/\/$/, '') + '/api/management/command/' + verb;
    var qs = opts.query ? buildQueryString(opts.query) : '';
    if (qs) url += '?' + qs;
    var headers = { Authorization: buildAuthHeader(conn.username, conn.password) };
    var fetchOpts = { method: opts.method || 'GET', headers: headers };
    if (opts.body !== undefined) {
      headers['Content-Type'] = 'application/json';
      fetchOpts.body = JSON.stringify(opts.body);
    }
    return fetch(url, fetchOpts).then(function (res) {
      return res.json().catch(function () { return null; }).then(function (json) {
        var envelope = parseEnvelope(json);
        if (!res.ok) {
          throw new Error(errorMessageFrom(envelope, 'HTTP ' + res.status + ' ' + res.statusText));
        }
        if (envelope.errors.length > 0) {
          throw new Error(errorMessageFrom(envelope, 'mq-proxy reported an error'));
        }
        return envelope.data;
      });
    });
  }

  exports.buildAuthHeader = buildAuthHeader;
  exports.parseEnvelope = parseEnvelope;
  exports.errorMessageFrom = errorMessageFrom;
  exports.buildQueryString = buildQueryString;
  exports.buildListMessagesParams = buildListMessagesParams;
  exports.buildJmsTypeScanParams = buildJmsTypeScanParams;
  exports.extractDistinctJmsTypes = extractDistinctJmsTypes;
  exports.buildBulkFilter = buildBulkFilter;
  exports.resolveJmsType = resolveJmsType;
  exports.buildSingleMessageFilter = buildSingleMessageFilter;
  exports.buildBulkDeleteBody = buildBulkDeleteBody;
  exports.buildBulkMoveBody = buildBulkMoveBody;
  exports.tierForQueue = tierForQueue;
  exports.sortMoveTargets = sortMoveTargets;
  exports.truncate = truncate;
  exports.sortedHeaderEntries = sortedHeaderEntries;
  exports.filterQueues = filterQueues;
  exports.sortQueues = sortQueues;
  exports.apiCall = apiCall;

  // ---------------------------------------------------------------------
  // DOM wiring — skipped entirely outside a browser (e.g. under Node for
  // app.test.js), since none of the above needs it.
  // ---------------------------------------------------------------------

  if (typeof document === 'undefined') {
    return;
  }

  var STORAGE_KEY = 'cloudtui-mq-proxy-connection';
  var DEFAULT_MAX_COUNT = 500;

  var state = {
    conn: null,
    currentQueue: null,
    currentMessage: null,
    queues: [],
    queueSort: { column: 'name', direction: 'asc' },
    messages: [],
    selectedMessageIds: new Set(),
  };

  function $(id) { return document.getElementById(id); }

  function showError(el, err) {
    el.textContent = err && err.message ? err.message : String(err);
    el.hidden = false;
  }

  function clearError(el) {
    el.hidden = true;
    el.textContent = '';
  }

  function showView(id) {
    ['connectView', 'queuesView', 'messagesView', 'messageDetailView'].forEach(function (viewId) {
      $(viewId).hidden = viewId !== id;
    });
    $('topbar').hidden = id === 'connectView';
  }

  function loadStoredConnection() {
    try {
      var raw = window.localStorage.getItem(STORAGE_KEY);
      return raw ? JSON.parse(raw) : null;
    } catch (e) {
      return null;
    }
  }

  function storeConnection(conn) {
    try {
      window.localStorage.setItem(STORAGE_KEY, JSON.stringify(conn));
    } catch (e) {
      // localStorage unavailable (private mode, quota) — connection just
      // won't be remembered next visit; not fatal to the current session.
    }
  }

  function forgetConnection() {
    try {
      window.localStorage.removeItem(STORAGE_KEY);
    } catch (e) {
      // ignore
    }
  }

  // ---- Connect ----

  function tryConnect(conn) {
    clearError($('connectError'));
    return apiCall(conn, 'list-queues').then(function () {
      state.conn = conn;
      storeConnection(conn);
      $('connLabel').textContent = conn.baseUrl + ' (' + conn.username + ')';
      showView('queuesView');
      return loadQueues();
    }, function (err) {
      showError($('connectError'), err);
    });
  }

  $('connectForm').addEventListener('submit', function (ev) {
    ev.preventDefault();
    tryConnect({
      baseUrl: $('connUrl').value.trim(),
      username: $('connUser').value,
      password: $('connPass').value,
    });
  });

  $('disconnectBtn').addEventListener('click', function () {
    forgetConnection();
    state.conn = null;
    $('connUrl').value = '';
    $('connUser').value = '';
    $('connPass').value = '';
    showView('connectView');
  });

  // ---- Queues ----

  function loadQueues() {
    clearError($('queuesError'));
    var tbody = $('queuesTable').querySelector('tbody');
    tbody.innerHTML = '<tr><td colspan="7">Loading…</td></tr>';
    return apiCall(state.conn, 'list-queues').then(function (queues) {
      state.queues = queues || [];
      applyQueueView();
    }, function (err) {
      tbody.innerHTML = '';
      showError($('queuesError'), err);
    });
  }

  // Applies the current filter text + sort column/direction to
  // state.queues (the last-fetched raw list) and re-renders — never
  // refetches, so typing in the filter or clicking a header is instant.
  function applyQueueView() {
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
      actions.appendChild(makeButton('Purge', function () { purgeQueue(q.name); }));
      actions.appendChild(makeButton('Move all…', function () { moveAllMessages(q.name); }));
      actions.appendChild(makeButton('Send…', function () { openSendModal(q.name); }));
      tbody.appendChild(tr);
    });
  }

  function makeButton(label, onClick) {
    var btn = document.createElement('button');
    btn.textContent = label;
    btn.type = 'button';
    btn.addEventListener('click', onClick);
    return btn;
  }

  function escapeHtml(str) {
    var div = document.createElement('div');
    div.textContent = str == null ? '' : String(str);
    return div.innerHTML;
  }

  $('refreshQueuesBtn').addEventListener('click', loadQueues);
  $('backToQueuesBtn').addEventListener('click', function () { showView('queuesView'); });

  // ---- Messages ----

  function openMessages(queueName) {
    state.currentQueue = queueName;
    state.selectedMessageIds = new Set();
    $('msgFilterType').value = '';
    $('msgFilterMax').value = String(DEFAULT_MAX_COUNT);
    showView('messagesView');
    loadMessages();
  }

  function loadMessages() {
    var queueName = state.currentQueue;
    var maxCount = parseInt($('msgFilterMax').value, 10) || DEFAULT_MAX_COUNT;
    var jmsType = $('msgFilterType').value.trim();
    $('messagesTitle').textContent = queueName + ' (max=' + maxCount + ')';
    clearError($('messagesError'));
    state.selectedMessageIds = new Set();
    var tbody = $('messagesTable').querySelector('tbody');
    tbody.innerHTML = '<tr><td colspan="5">Loading…</td></tr>';
    return apiCall(state.conn, 'list-messages', {
      query: buildListMessagesParams(queueName, { jmsType: jmsType, maxCount: maxCount }),
    }).then(function (messages) {
      state.messages = messages || [];
      renderMessages(state.messages);
      updateBulkActionsUI();
    }, function (err) {
      tbody.innerHTML = '';
      showError($('messagesError'), err);
    });
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
      }).then(loadMessages, function (err) {
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
      }).then(loadMessages);
    }, $('messagesError'));
  });

  $('applyMsgFilterBtn').addEventListener('click', loadMessages);
  $('sendFromMessagesBtn').addEventListener('click', function () { openSendModal(state.currentQueue); });
  $('backToMessagesBtn').addEventListener('click', function () { showView('messagesView'); });

  // ---- Message detail ----

  function openMessageDetail(message) {
    state.currentMessage = message;
    clearError($('messageDetailError'));
    var fields = $('messageDetailFields');
    fields.innerHTML =
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
      });
    }, $('messageDetailError'));
  });

  // ---- Purge / move-all (shared JMS Type prompt) ----

  // sourceQueue: the queue to scan for JMS Type autocomplete suggestions.
  // The scan runs in the background the moment the prompt opens and
  // populates the <datalist> whenever it resolves — it never blocks or
  // gates Continue/Cancel, which stay usable immediately regardless of
  // whether the scan has finished (or failed; a failed scan just leaves
  // the datalist empty, silently — it's a nice-to-have, not worth
  // surfacing an error for).
  function jmsTypePrompt(title, sourceQueue) {
    return new Promise(function (resolve) {
      $('jmsTypePromptTitle').textContent = title;
      $('jmsTypePromptInput').value = '';
      $('jmsTypeSuggestions').innerHTML = '';
      $('jmsTypePromptModal').hidden = false;
      apiCall(state.conn, 'list-messages', { query: buildJmsTypeScanParams(sourceQueue) }).then(function (messages) {
        var datalist = $('jmsTypeSuggestions');
        extractDistinctJmsTypes(messages || []).forEach(function (jmsType) {
          var option = document.createElement('option');
          option.value = jmsType;
          datalist.appendChild(option);
        });
      }, function () {
        // scan failed — leave the datalist empty, see comment above.
      });
      function cleanup(result) {
        $('jmsTypePromptModal').hidden = true;
        continueBtn.removeEventListener('click', onContinue);
        cancelBtn.removeEventListener('click', onCancel);
        resolve(result);
      }
      var continueBtn = $('jmsTypePromptContinue');
      var cancelBtn = $('jmsTypePromptCancel');
      function onContinue() { cleanup({ cancelled: false, jmsType: $('jmsTypePromptInput').value.trim() }); }
      function onCancel() { cleanup({ cancelled: true }); }
      continueBtn.addEventListener('click', onContinue);
      cancelBtn.addEventListener('click', onCancel);
    });
  }

  function confirmDialog(message) {
    return new Promise(function (resolve) {
      $('confirmMessage').textContent = message;
      $('confirmModal').hidden = false;
      function cleanup(result) {
        $('confirmModal').hidden = true;
        yesBtn.removeEventListener('click', onYes);
        noBtn.removeEventListener('click', onNo);
        resolve(result);
      }
      var yesBtn = $('confirmYes');
      var noBtn = $('confirmNo');
      function onYes() { cleanup(true); }
      function onNo() { cleanup(false); }
      yesBtn.addEventListener('click', onYes);
      noBtn.addEventListener('click', onNo);
    });
  }

  function purgeQueue(queueName) {
    jmsTypePrompt('Purge "' + queueName + '" — JMS Type (optional)', queueName).then(function (result) {
      if (result.cancelled) return;
      var filter = buildBulkFilter(result.jmsType);
      var question = result.jmsType
        ? 'Purge "' + queueName + '"? All ' + result.jmsType + ' messages will be deleted.'
        : 'Purge "' + queueName + '"? All messages will be deleted.';
      confirmDialog(question).then(function (confirmed) {
        if (!confirmed) return;
        apiCall(state.conn, 'delete-messages', {
          method: 'POST',
          body: [{ sourceQueue: queueName, filter: filter }],
        }).then(loadQueues, function (err) {
          showError($('queuesError'), err);
        });
      });
    });
  }

  function moveAllMessages(queueName) {
    jmsTypePrompt('Move All "' + queueName + '" — JMS Type (optional)', queueName).then(function (result) {
      if (result.cancelled) return;
      var filter = buildBulkFilter(result.jmsType);
      openMovePicker(queueName, function (target) {
        return apiCall(state.conn, 'move-messages', {
          method: 'POST',
          body: [{ sourceQueue: queueName, targetQueue: target, filter: filter }],
        }).then(loadQueues);
      }, $('queuesError'));
    });
  }

  // ---- Move-target picker ----

  var movePickerState = null;

  function openMovePicker(sourceQueue, onConfirm, errorEl) {
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

  function renderMovePickerList(names) {
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

  // ---- Send message ----

  var sendModalState = null;

  function openSendModal(targetQueue) {
    sendModalState = { targetQueue: targetQueue };
    $('sendModalTitle').textContent = 'Send message to "' + targetQueue + '"';
    $('sendJmsType').value = '';
    $('sendBody').value = '';
    clearError($('sendError'));
    $('sendModal').hidden = false;
  }

  $('sendSubmit').addEventListener('click', function () {
    var body = $('sendBody').value;
    var jmsType = resolveJmsType($('sendJmsType').value);
    apiCall(state.conn, 'send-message', {
      method: 'POST',
      body: { targetQueue: sendModalState.targetQueue, jmsType: jmsType, body: body },
    }).then(function () {
      $('sendModal').hidden = true;
      if (state.currentQueue === sendModalState.targetQueue && !$('messagesView').hidden) {
        loadMessages();
      }
      loadQueues();
    }, function (err) {
      showError($('sendError'), err);
    });
  });

  $('sendCancel').addEventListener('click', function () {
    $('sendModal').hidden = true;
  });

  // ---- Startup ----

  var stored = loadStoredConnection();
  if (stored) {
    $('connUrl').value = stored.baseUrl;
    $('connUser').value = stored.username;
    $('connPass').value = stored.password;
  }
})(typeof module !== 'undefined' ? module.exports : (window.CloudtuiMQ = {}));
