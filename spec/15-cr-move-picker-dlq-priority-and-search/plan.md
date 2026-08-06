# Plan — CR 15: Move-picker DLQ priority and search filter

## DLQ priority ordering

### `sortPickerQueues` function

```go
// sortPickerQueues sorts names alphabetically, but if sourceQueue has a
// "dlq." prefix (case-insensitive) the corresponding non-DLQ queue is
// pinned first.
func sortPickerQueues(sourceQueue string, names []string) []string
```

- Located in `app.go` (unexported package-level helper).
- DLQ detection: `strings.HasPrefix(strings.ToLower(sourceQueue), "dlq.")`.
- Candidate: `sourceQueue[4:]` (strip `"dlq."` preserving original case).
- Scan `names` for a case-insensitive match against the candidate.
- If found: prepend it, sort the remainder alphabetically, return
  `[candidate, ...rest]`.
- If not found or source is not a DLQ: return `names` sorted alphabetically.

Tested by `TestSortPickerQueues` in `app_test.go`.

## Search filter

### Overlay layout change

The current overlay is `centered(movePickerList, 52, 20)` with the border
on `movePickerList`. Replace with a `tview.Flex` (vertical) that owns the
border:

```
movePickerFlex  (border + title " Move to Queue ", height 22)
├── movePickerList   (no border, proportion 1)
└── movePickerSearch (no border, fixed 1 row)
```

The overlay registration becomes `centered(movePickerFlex, 52, 22)`.

`App` gains:
- `movePickerFlex   *tview.Flex`
- `movePickerSearch *tview.InputField`
- `movePickerQueues []string` — sorted list of all eligible target queues
  (populated after async load, used for re-filtering without a broker round-trip)

### Search behavior

- `movePickerSearch` has label `" / filter: "`.
- `SetChangedFunc`: on each keystroke, rebuild the list from
  `movePickerQueues` keeping only names that contain the typed substring
  (case-insensitive `strings.Contains(strings.ToLower(name), strings.ToLower(text))`).
- `SetDoneFunc` (Enter/Tab): call `a.tv.SetFocus(a.movePickerList)`.
- `SetInputCapture`: `Esc` → clear filter text, repopulate list from
  `movePickerQueues`, refocus `movePickerList`.
- `'/'` case in `movePickerList.SetInputCapture`: shift focus to
  `movePickerSearch`.
- Context panel shows `<Esc> cancel  </> search` when picker is open.

### `showMovePicker` changes

After the async queue load:
1. Sort via `sortPickerQueues(sourceQueue, names)`.
2. Store sorted names in `a.movePickerQueues`.
3. Populate `movePickerList` from the sorted slice.

`closeMovePicker` is unchanged.

### Theme

`reapplyTheme` styles `movePickerSearch` (label color, field background,
field text color) matching the `queuesV.filterInput` style.

## Files touched

- `tui/internal/app/app.go` — `movePickerFlex`, `movePickerSearch`,
  `movePickerQueues` fields; updated overlay construction; `sortPickerQueues`
  function; updated `showMovePicker` (DLQ sort, store queues, populate search);
  `/` hotkey in picker list input capture; context panel hint update
- `tui/internal/app/app_test.go` — `TestSortPickerQueues`
- `tui/internal/app/theme.go` — style `movePickerSearch`

## Key decisions

- **IMQ treated identically to DLQ**: both `dlq.*` and `imq.*` are requeue
  prefixes — `requeueQueueCandidate` strips either prefix to find the preferred
  target; both get `➖` in the list and are de-prioritized to the third tier.
- **Sorting is a pure function**: `sortPickerQueues` has no side effects and
  is trivially testable without a running TUI.
- **Search input at the bottom**: consistent with `queuesView.filterInput`
  (already familiar to users).
- **Border on flex, not list**: the list sits inside the bordered flex so the
  search field appears inside the same visual box.
- **Re-filter from cached slice**: no broker round-trip on each keystroke;
  `movePickerQueues` is populated once on open.
- **`movePickerVisible` already guards global hotkeys**: no additional focus
  guard needed — the bool is set true for the lifetime of the picker regardless
  of whether focus is on the list or the search field.

## Testing

- `TestSortPickerQueues`: table-driven; covers DLQ source with matching
  candidate, DLQ source without matching candidate, non-DLQ source, empty list.
- Manual: open picker from a DLQ message → corresponding queue appears first;
  press `/` → search field activates; type partial name → list narrows live;
  `Esc` → filter cleared, full list restored, focus returns to list; `Enter`
  in search → focus returns to list.
