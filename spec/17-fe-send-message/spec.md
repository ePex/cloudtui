# Spec — FE 17: Send message to queue

Date: 2026-08-06

## What and why

The TUI currently supports browsing, moving, and deleting messages but has no
way to produce new messages. This feature adds a "send message" operation:
from the queues view or the messages view the user selects a queue, presses
`c`, fills in a message body in an inline overlay, and submits it. The message
is sent to the broker via STOMP over TCP so that it appears as a normal JMS
TextMessage and the queue remains fully browsable after sending.

## Scope

**In scope:**

- `c` hotkey in both the **queues view** and the **messages view** opens a
  send-message overlay: a bordered flex with a multi-line `tview.TextArea` for
  the message body and two action items — Submit and Cancel.
- On submit: send the message, close the overlay, reload the queues list and
  (if open) the messages list, show a status bar confirmation.
- On cancel / `Esc`: close the overlay, return focus to the caller.

- `SendMessage` on `queue.Backend` and `jolokia.Client` implemented via
  **STOMP 1.1 over TCP** (not JMX `sendTextMessage`): open a TCP connection to
  `host:stompPort`, exchange CONNECT/CONNECTED frames, send a SEND frame with
  `content-length`, then DISCONNECT. No external library — standard `net`
  package only.

- `STOMPPort int` added to `QueueConfig` (YAML: `stompPort`, default `61613`).
  Host extracted from the existing `url` field.

- `infra/activemq.xml` — custom broker config mounted via `compose.yaml`,
  enabling the STOMP transport connector alongside OpenWire so the dev
  environment supports this feature out of the box.

- **Browse robustness**: `jolokia.Client.BrowseMessages` falls back to the
  simpler `browse()` JMX operation if `browseMessages()` returns a Jolokia
  error. Fallback messages have empty IDs; a yellow status note is shown.

- **Purge robustness**: `jolokia.Client.PurgeQueue` tries `purgeQueue()` JMX
  operation first, then `removeMatchingMessages("TRUE")`, then browse-and-remove.
  This allows purging queues that cannot be browsed.

- Fix `message_detail.go` delete-error reporting: error shown in status bar.

- Unit tests for all new code paths (STOMP mock TCP server, browse fallback,
  purge chain, `c` hotkey stubs).

**Out of scope:**
- Message headers, properties, or JMS type selection (text messages only).
- STOMP over TLS.
- Persistent STOMP connections (connect-per-send).
- Message templates or history.
