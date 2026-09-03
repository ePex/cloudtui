// Shared DOM helpers and small pure formatting functions used across views.

export function $(id) { return document.getElementById(id); }

export function showError(el, err) {
  el.textContent = err && err.message ? err.message : String(err);
  el.hidden = false;
}

export function clearError(el) {
  el.hidden = true;
  el.textContent = '';
}

export function showView(id) {
  ['connectView', 'queuesView', 'messagesView', 'messageDetailView'].forEach(function (viewId) {
    $(viewId).hidden = viewId !== id;
  });
  $('topbar').hidden = id === 'connectView';
}

export function makeButton(label, onClick, className) {
  var btn = document.createElement('button');
  btn.textContent = label;
  btn.type = 'button';
  if (className) btn.className = className;
  btn.addEventListener('click', onClick);
  return btn;
}

export function escapeHtml(str) {
  var div = document.createElement('div');
  div.textContent = str == null ? '' : String(str);
  return div.innerHTML;
}

export function truncate(str, maxLen) {
  if (str == null) return '';
  return str.length > maxLen ? str.slice(0, maxLen - 1) + '…' : str;
}

// mq-proxy always returns a message's JMS properties as a headers map
// (spec/11), independent of returnBody — sorted [key, value] pairs for
// the message detail view's Headers section (mirrors the TUI's own
// "all captured JMS fields as sorted Key: value lines", spec/08).
export function sortedHeaderEntries(headers) {
  return Object.keys(headers || {}).sort().map(function (key) {
    return [key, headers[key]];
  });
}
