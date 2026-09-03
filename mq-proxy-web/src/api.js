// mq-proxy REST client (spec/11) and the pure request/response shaping
// functions around it.

function base64Encode(str) {
  if (typeof btoa === 'function') return btoa(str);
  return Buffer.from(str, 'utf-8').toString('base64');
}

export function buildAuthHeader(username, password) {
  return 'Basic ' + base64Encode(username + ':' + password);
}

// Normalizes mq-proxy's two response envelope shapes (spec/11):
// list endpoints return { data, errors: [...] }, single-item endpoints
// return { data, error } or { data, error: null }. Always returns
// { data, errors: [], hasMore } with errors as an array, empty when
// there were none, so every call site has one shape to check. hasMore
// (spec/11's pagination) is only meaningful on list-messages responses
// — everywhere else it's just the server's own unused-there default,
// false.
export function parseEnvelope(json) {
  if (!json || typeof json !== 'object') {
    return { data: undefined, errors: [], hasMore: false };
  }
  if (Array.isArray(json.errors)) {
    return { data: json.data, errors: json.errors, hasMore: json.hasMore === true };
  }
  if (json.error) {
    return { data: json.data, errors: [json.error], hasMore: json.hasMore === true };
  }
  return { data: json.data, errors: [], hasMore: json.hasMore === true };
}

export function errorMessageFrom(envelope, fallback) {
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
export function buildQueryString(params) {
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

// How many messages the client asks for at once when the user hasn't set
// a filter.maxCount of their own — mirrors the TUI client's own default,
// satisfying mq-proxy's hard requirement that maxCount be a positive
// integer on list-messages (spec/11).
export var DEFAULT_MAX_COUNT = 500;

// Builds the list-messages query params object (spec/11): sourceQueue
// and returnBody stay top-level, everything else nests under filter.*.
export function buildListMessagesParams(sourceQueue, opts) {
  opts = opts || {};
  return {
    sourceQueue: sourceQueue,
    returnBody: true,
    filter: {
      jmsType: opts.jmsType || undefined,
      messageId: opts.messageId || undefined,
      maxCount: opts.maxCount || undefined,
      afterMessageId: opts.afterMessageId || undefined,
    },
  };
}

// Appends a "Load more" page onto the already-rendered list — a plain
// concat, but pulled out as its own pure function since loadMessages
// and loadMoreMessages both need the identical "existing + new" step.
// Never mutates either input array.
export function appendMessages(existing, newPage) {
  return existing.concat(newPage);
}

// How many messages the JMS Type autocomplete scans to populate its
// suggestions — mirrors the TUI's own automatic-scan cap
// (jmsTypeAutoScanCount, spec/08) rather than introducing a different
// number. returnBody is left false: only the jmsType header is needed,
// so there's no reason to pull message bodies over the wire for this.
var JMS_TYPE_SCAN_MAX_COUNT = 500;

export function buildJmsTypeScanParams(sourceQueue) {
  return { sourceQueue: sourceQueue, returnBody: false, filter: { maxCount: JMS_TYPE_SCAN_MAX_COUNT } };
}

// Sorted, de-duplicated, non-empty JMS Type values found across a list
// of messages — used to populate the JMS Type prompt's autocomplete.
export function extractDistinctJmsTypes(messages) {
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
export function buildBulkFilter(jmsType) {
  var trimmed = (jmsType || '').trim();
  return trimmed ? { jmsType: trimmed } : {};
}

// send-message's jmsType field is required by mq-proxy's DTO (spec/11)
// even though a blank entry is a normal, common case here — unlike the
// TUI (spec/09), which hardcodes "text" since it has no JMS Type field
// at all, this page exposes one but still needs a sane default when
// it's left blank.
export function resolveJmsType(jmsType) {
  var trimmed = (jmsType || '').trim();
  return trimmed || 'text';
}

// Filter for acting on exactly one already-known message (single
// delete/move) — matches the TUI proxy client's MessageFilter{MessageID,
// MaxCount: 1} pattern (tui/internal/queue/proxy/proxy.go).
export function buildSingleMessageFilter(messageId) {
  return { messageId: messageId, maxCount: 1 };
}

// delete-messages/move-messages both accept an array of per-message
// request objects (spec/11) — "the shape exists for bulk-operation
// parity", which multi-select finally puts to real use: one array
// element per selected message, each still scoped to exactly one
// message via buildSingleMessageFilter (never a wider filter, so a
// bulk action can never accidentally affect an unselected message).
export function buildBulkDeleteBody(sourceQueue, messageIds) {
  return messageIds.map(function (id) {
    return { sourceQueue: sourceQueue, filter: buildSingleMessageFilter(id) };
  });
}

export function buildBulkMoveBody(sourceQueue, targetQueue, messageIds) {
  return messageIds.map(function (id) {
    return { sourceQueue: sourceQueue, targetQueue: targetQueue, filter: buildSingleMessageFilter(id) };
  });
}

// conn: { baseUrl, username, password }. verb: e.g. "list-queues".
// opts: { method, query, body } — query is a (possibly nested) params
// object per buildQueryString; body is a plain object, JSON-encoded.
export function apiCall(conn, verb, opts) {
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
      // Callers that need hasMore (currently only list-messages'
      // pagination, loadMessages/loadMoreMessages) read it off the
      // resolved data itself rather than apiCall growing a second
      // return channel every other call site would have to ignore —
      // harmless on data of any shape (array or object), a no-op
      // where the server doesn't send hasMore at all.
      if (envelope.data && typeof envelope.data === 'object') {
        envelope.data.hasMore = envelope.hasMore;
      }
      return envelope.data;
    });
  });
}
