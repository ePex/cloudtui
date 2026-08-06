# Tasks — FE 17: Send message to queue

Plan: [plan.md](plan.md)

1. [x] **Backend interface + stub** — add `SendMessage` to `queue.Backend`; add
   no-op stub to `fakeQueueBackend` in `queues_test.go`.

2. [x] **`SendMessage` via JMX** — `jolokia.Client.SendMessage` calls
   `sendTextMessage(java.lang.String)` on the queue MBean via Jolokia exec.
   `stomp.go` was evaluated and removed: the STOMP adapter stored all frames
   as BytesMessage regardless of headers, making bodies unreadable in the
   detail view. JMX `sendTextMessage` always produces a readable TextMessage.
   `STOMPPort` config field was not added (not needed).

3. [x] **Send-message overlay** — overlay fields on `App`; `showSendMessage`,
   `doSend` (with auto-reload of queues and messages views), `closeSendMessage`;
   `onGlobalKey` guard; page registration.

4. [x] **Queues view `c` hotkey** — `Shortcuts()` updated; `c` case calls
   `showSendMessage` with queue name and restore callback.

5. [x] **Messages view `c` hotkey** — `Shortcuts()` updated; `c` case calls
   `showSendMessage` with `mv.queueName` and restore callback.

6. [x] **Theme** — `reapplyTheme` styles `sendMessageFlex`, `sendMessageArea`,
   `sendMessageList`.

7. [x] **Browse fallback** — `browseMessagesFull` + `browseMessagesFallback`
   (`browse()` operation); `BrowseMessages` orchestrates with original-error
   fallback; yellow status note in `messagesView.load()` for empty-ID results;
   tests `TestBrowseMessagesFallback` and `TestBrowseMessagesFallbackBothFail`.

8. [x] **Purge three-tier chain** — `PurgeQueue` tries `purgeQueue()`, then
   `removeMatchingMessages("TRUE")`, then browse-and-remove; `execSimple` and
   `removeMatchingMessages` helpers added; tests
   `TestPurgeQueueDirectOperation` and `TestPurgeQueueRemoveMatchingFallback`.

9. [x] **Delete-error display** — `message_detail.go` `d` handler shows error
   in status bar alongside `slog.Error`.

10. [x] **Infrastructure** — `infra/activemq.xml` with OpenWire + STOMP
    connectors; `infra/compose.yaml` mounts config and exposes port 61613.

11. [ ] **Manual verification** — restart broker (`docker compose … down && up`);
    select a queue from the list, press `c`, type a body, submit — status bar
    shows "Message sent to …"; the messages list reloads and the new message
    appears; open the message detail and verify body is shown as text (not
    "(binary)"). Repeat from inside the messages view. Press `c` then `Esc` —
    overlay closes, focus and shortcuts restored in both call sites.
