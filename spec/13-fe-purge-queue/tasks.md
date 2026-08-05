# Tasks — FE 13: Purge queue

Plan: [plan.md](plan.md)

1. [x] **Backend interface + stubs** — add `PurgeQueue` and `RemoveMessage`
   to `queue.Backend`; add no-op stubs to `fakeQueueBackend` in
   `queues_test.go`.

2. [x] **Jolokia implementation** — implement `PurgeQueue` (browse + remove
   each message) and `RemoveMessage` (single exec) in `jolokia.Client`.

3. [x] **Confirmation dialog** — add `confirmFlex *tview.Flex`,
   `confirmText *tview.TextView`, `confirmList *tview.List`,
   `confirmVisible bool` to `App`; implement `showConfirm(question string,
   onConfirm func())` and `closeConfirm()`; register `centered(confirmFlex,
   52, 8)` as page `"confirm"` on `rootPages`; guard global hotkeys when
   `confirmVisible`.

4. [x] **`p` hotkey in queuesView + Shortcuts update** — add `p` to
   `queuesView.SetInputCapture`; raise top bar minimum to
   `shortcutPanelRows = 4` in `topbar.go`; update `Shortcuts()` to show all
   four entries (`r`, `p`, `/`, `o/O`); update topbar_test.go.

5. [x] **`p` hotkey in messagesView** — add `p` to
   `messagesView.SetInputCapture`; add `p purge` to `Shortcuts()`.

6. [x] **`d` hotkey in messageDetailView** — store `queueName`/`msg` in
   `render()`; add `d` to `SetInputCapture` (calls `RemoveMessage`, returns
   to messages list on success); add `d delete message` to `Shortcuts()`.

7. [x] **Theme** — style `confirmFlex`, `confirmText`, `confirmList` in
   `reapplyTheme`.

8. [x] **Tests** — `TestQueuesViewShortcutRPresent`, `TestQueuesViewPurgeShortcutPresent`,
   `TestQueuesViewSortShortcutsPresent` all pass; topbar minimum-height test
   updated to expect 4.

9. [ ] **Manual verification** — press `p` on a queue (queues view and
   messages view): confirm dialog shows with "No" focused; Esc and "No"
   dismiss without change; "Yes" removes all messages and list refreshes.
   Open a message detail and press `d`: confirm dialog shows; "Yes" removes
   only that message and returns to the message list.
