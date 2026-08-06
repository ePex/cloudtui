# Tasks — CR 15: Move-picker DLQ priority and search filter

Plan: [plan.md](plan.md)

1. [x] **`sortPickerQueues` + unit tests** — add `sortPickerQueues(sourceQueue
   string, names []string) []string` to `app.go`; add
   `TestSortPickerQueues` (table-driven: DLQ source with match, DLQ source
   without match, non-DLQ source, empty list) to `app_test.go`.

2. [x] **Overlay layout restructure** — add `movePickerFlex *tview.Flex` and
   `movePickerSearch *tview.InputField` fields to `App`; build the vertical
   flex (list proportion-1, search fixed-1) with border and title on the
   flex; remove border from `movePickerList`; replace
   `centered(movePickerList, 52, 20)` with `centered(movePickerFlex, 52, 22)`
   in `rootPages`; wire `SetChangedFunc` (live filter from
   `movePickerQueues`), `SetDoneFunc` (refocus list), and `SetInputCapture`
   (`Esc` → clear + refocus list) on `movePickerSearch`.

3. [x] **`showMovePicker` update** — after the async queue load, sort names
   via `sortPickerQueues`, store in `a.movePickerQueues`, then populate
   `movePickerList`; add `'/'` case to the list's `SetInputCapture` to
   focus `movePickerSearch`; update context panel hint to show
   `<Esc> cancel  </> search`.

4. [x] **Theme** — style `movePickerSearch` in `reapplyTheme` (label color,
   field background, field text color) matching `queuesV.filterInput`.

5. [ ] **Manual verification** — open a message detail on a DLQ queue, press
   `m`; confirm the corresponding non-DLQ queue appears first in the list.
   Press `/`; type a partial name; confirm the list narrows live. Press `Esc`;
   confirm filter clears and full list is restored. Press `Enter` in the
   search field; confirm focus returns to the list. Select a queue; confirm
   the message is moved and the messages list reloads.
