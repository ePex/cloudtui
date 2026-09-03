import { $, clearError, showError } from './dom.js';
import { apiCall, buildJmsTypeScanParams, extractDistinctJmsTypes, resolveJmsType } from './api.js';
import { state } from './state.js';
import { loadQueues } from './queues.js';
import { loadMessages } from './messages.js';

// sourceQueue: the queue to scan for JMS Type autocomplete suggestions.
// The scan runs in the background the moment the prompt opens and
// populates the <datalist> whenever it resolves — it never blocks or
// gates Continue/Cancel, which stay usable immediately regardless of
// whether the scan has finished (or failed; a failed scan just leaves
// the datalist empty, silently — it's a nice-to-have, not worth
// surfacing an error for).
export function jmsTypePrompt(title, sourceQueue) {
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

export function confirmDialog(message) {
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

var sendModalState = null;

export function openSendModal(targetQueue) {
  sendModalState = { targetQueue: targetQueue };
  $('sendModalTitle').textContent = 'Send message to "' + targetQueue + '"';
  $('sendJmsType').value = '';
  $('sendBody').value = '';
  clearError($('sendError'));
  $('sendModal').hidden = false;
}

// Wires the send modal's own static controls — called once at startup.
export function initDialogs() {
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
}
