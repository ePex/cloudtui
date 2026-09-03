// Entry point: wires every view's static controls, then restores a
// stored connection (if any) into the connect form's fields.
import { $ } from './dom.js';
import { initConnection, loadStoredConnection } from './connection.js';
import { initQueues } from './queues.js';
import { initMessages } from './messages.js';
import { initMovePicker } from './movepicker.js';
import { initDialogs } from './dialogs.js';

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
