# Tasks — FE 14: Move message to another queue

Plan: [plan.md](plan.md)

1. [x] **Backend interface + stub** — add `MoveMessage(ctx context.Context,
   sourceQueue, messageID, targetQueue string) error` to `queue.Backend`;
   add a no-op stub to `fakeQueueBackend` in `queues_test.go`.

2. [x] **Jolokia implementation** — implement `MoveMessage` in
   `jolokia.Client` using `moveMessageTo(java.lang.String,java.lang.String)`
   exec on the source queue MBean.

3. [x] **Queue-picker overlay** — add `movePickerList *tview.List` and
   `movePickerVisible bool` to `App`; implement `showMovePicker` (clear,
   show "Loading…", async load queues, populate list excluding source queue)
   and `closeMovePicker`; register `centered(movePickerList, 52, 20)` as
   page `"move-picker"` on `rootPages`; guard `onGlobalKey` for
   `movePickerVisible`; wire `j`/`k` and Esc in picker input capture.

4. [x] **`m` hotkey in messageDetailView** — add `m` to `SetInputCapture`
   calling `a.showMovePicker`; add `{Key: "m", Description: "move"}` to
   `Shortcuts()`.

5. [x] **Theme** — style `movePickerList` in `reapplyTheme` (selection
   colors, background, border/title).

6. [x] **Bugfix: message ID reconstruction** — `extractMessageID` was
   using `producerId.producerSessionId.connectionId.value` (wrong); corrected
   to `producerId.connectionId.value` and extended to include `sessionId` and
   `producerId.value` in the ID string. Added value-field check to both
   `RemoveMessage` and `MoveMessage` (Jolokia returns `false` when not found).
   Added `TestBrowseMessagesCompositeDataMessageID` to pin the correct format.
   Added error display in status bar when move fails.

7. [ ] **Manual verification** — open a message detail, press `m`; picker
   appears with "Loading…" then queue list (current queue absent); Esc
   cancels and returns to detail; selecting a queue moves the message and
   reloads the messages list.
