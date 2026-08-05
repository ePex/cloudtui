# Tasks — FE 09: Queue list filtering

Plan: [plan.md](plan.md)

1. [x] **Filter input + flex wrapper** — add `filterInput *tview.InputField`,
   `flex *tview.Flex`, `filter string`, `allSummaries []queue.Summary` fields
   to `queuesView`; construct the Flex (table + 1-row input) in
   `newQueuesView`; change `Primitive()` to return `flex`; add `/` to table's
   `SetInputCapture` to focus the filter input; add `</> filter` to
   `Shortcuts()`; wire `filterInput.SetChangedFunc` for live filtering;
   `filterInput.SetDoneFunc` to confirm and return focus; `filterInput.SetInputCapture`
   for Up/Down to confirm + forward key to table; fix `onGlobalKey` in `app.go`
   to pass events through when filterInput is focused.

2. [x] **Filter logic** — add `applyFilter(s string)`, `updateTitle()`;
   update `repaint` to store into `allSummaries` and filter by
   case-insensitive substring before rendering; update `updateTitle` call
   after filter change.

3. [x] **`reapplyTheme`** — repaint `filterInput` background, label color,
   and field text color in `theme.go`.

4. [x] **Tests** — add to `queues_test.go`: filter shortcut present; after
   `applyFilter` + repaint only matching rows rendered; filter persists after
   second repaint; empty filter restores all rows.

5. [ ] **Manual verification** — run TUI; press `/` in queues view; type a
   partial name; confirm only matching rows shown and title updated; press
   Enter/Esc; confirm filter persists; switch to another view and back;
   confirm filter still active; clear filter; confirm all queues shown.
