import { $, clearError, showError, showView } from './dom.js';
import { apiCall } from './api.js';
import { state } from './state.js';
import { loadQueues } from './queues.js';

var STORAGE_KEY = 'cloudtui-mq-proxy-connection';

export function loadStoredConnection() {
  try {
    var raw = window.localStorage.getItem(STORAGE_KEY);
    return raw ? JSON.parse(raw) : null;
  } catch (e) {
    return null;
  }
}

export function storeConnection(conn) {
  try {
    window.localStorage.setItem(STORAGE_KEY, JSON.stringify(conn));
  } catch (e) {
    // localStorage unavailable (private mode, quota) — connection just
    // won't be remembered next visit; not fatal to the current session.
  }
}

export function forgetConnection() {
  try {
    window.localStorage.removeItem(STORAGE_KEY);
  } catch (e) {
    // ignore
  }
}

export function tryConnect(conn) {
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

// Wires the connect screen's own static controls — called once at startup.
export function initConnection() {
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
}
