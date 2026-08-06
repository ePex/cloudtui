# Tasks — FE 16: Move all messages from one queue to another

Plan: [plan.md](plan.md)

1. [x] **Backend interface + stub** — add `MoveAllMessages(ctx context.Context,
   sourceQueue, targetQueue string) (int, error)` to `queue.Backend`; add a
   no-op stub returning `(0, nil)` to `fakeQueueBackend` in `queues_test.go`.

2. [x] **Jolokia implementation + test** — implement `MoveAllMessages` in
   `jolokia.Client` using `moveMatchingMessagesTo(java.lang.String,java.lang.String)`
   with selector `"TRUE"`; parse the integer `value` from the response and
   return it. Add `TestMoveAllMessages` to `jolokia_test.go`.

3. [x] **`showMovePicker` / `fillPickerList` refactor** — add
   `movePickerOnSelect func(string)` field to `App`; change `showMovePicker`
   signature to `showMovePicker(sourceQueue string, onSelect func(string))`;
   update `fillPickerList` to take only `filter string` and use
   `a.movePickerOnSelect`; update the `message_detail.go` call site to pass
   the move-message logic as the callback.

4. [x] **Queues view `M` hotkey** — add `{Key: "M", Description: "move queue"}`
   to `queuesView.Shortcuts()`; add `M` case to `table.SetInputCapture` that
   reads the selected queue name and calls `a.showMovePicker(name, func(target)
   { MoveAllMessages → status bar count + qv.load() })`.

5. [ ] **Manual verification** — select a queue with messages, press `M`;
   picker opens (DLQ/system tiers apply); select target; status bar shows
   "Moved N messages from \<src\> to \<dst\>"; queues list reloads with
   updated pending counts; source queue shows 0 pending.
