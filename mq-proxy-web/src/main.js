// Entry point: wires every view's static controls, then restores a
// stored connection (if any) into the connect form's fields. Opening the
// page with #progress-demo plays the progress overlay's demo.
import { $ } from './dom.js';
import { initConnection, loadStoredConnection } from './connection.js';
import { initQueues } from './queues.js';
import { initMessages } from './messages.js';
import { initMovePicker } from './movepicker.js';
import { initDialogs } from './dialogs.js';
import { runProgressDemo } from './progress.js';

initConnection();
initQueues();
initMessages();
initMovePicker();
initDialogs();

var stored = loadStoredConnection();
if (stored) {
  $('connUrl').value = stored.baseUrl;
  $('connUser').value = stored.username;
  $('connPass').value = stored.password;
}

var DEMO_HASH = '#progress-demo';

// runDemoFromHash plays the demo when the URL asks for it, then drops the
// hash, so setting it again replays it. The result goes to the visible
// view's info line, if it has one.
function runDemoFromHash() {
  if (window.location.hash !== DEMO_HASH) return;
  history.replaceState(null, '', window.location.pathname + window.location.search);
  runProgressDemo().then(function (result) {
    var info = document.querySelector('main > section:not([hidden]) p.info');
    if (!info) return;
    info.textContent = result;
    info.hidden = false;
  });
}

window.addEventListener('hashchange', runDemoFromHash);
runDemoFromHash();
