# Plan — FE 09: Queue list filtering

## Architecture

`queuesView` gains a filter input row and a cached summary list. The outer
primitive registered as the page changes from `*tview.Table` to
`*tview.Flex` (vertical), containing the table (proportion 1) and a
1-row `*tview.InputField` at the bottom.

The filter input is always present in the layout (1 row). When inactive it
is not focused and shows `" / filter: <current filter> "` (or an empty
label when no filter is set). Pressing `/` in the table focuses it.
Enter or Esc in the input confirms the filter and returns focus to the
table.

## `queuesView` changes (`queues.go`)

### New fields

```go
type queuesView struct {
    table        *tview.Table
    filterInput  *tview.InputField
    flex         *tview.Flex
    app          *App
    backend      queue.Backend
    filter       string          // active filter (empty = no filter)
    allSummaries []queue.Summary // full unfiltered list from last load
}
```

### `Primitive()` → returns `flex`

`colorBordered` in `app.go` uses a type assertion for `bordered`; Flex does
not implement it, so the assertion fails silently and no border color is set
on the flex (correct — the table's own border already has the right color,
set during construction).

`reapplyTheme` already directly accesses `a.queuesV.table`, so border/title
repaint continues to work unchanged.

### Filter input

```go
filterInput := tview.NewInputField()
filterInput.SetLabel(" / filter: ")
// Live filter on every keystroke.
filterInput.SetChangedFunc(func(text string) {
    qv.applyFilter(text)
})
// Enter/Esc confirms and returns focus to table.
filterInput.SetDoneFunc(func(_ tcell.Key) {
    qv.applyFilter(qv.filterInput.GetText())
    qv.app.tv.SetFocus(qv.table)
})
// Up/Down confirms and immediately moves the table selection.
filterInput.SetInputCapture(func(event *tcell.EventKey) *tcell.EventKey {
    if event.Key() == tcell.KeyUp || event.Key() == tcell.KeyDown {
        qv.applyFilter(qv.filterInput.GetText())
        qv.app.tv.SetFocus(qv.table)
        qv.table.InputHandler()(event, func(tview.Primitive) {})
        return nil
    }
    return event
})
```

Styled with `SelectionBg`/`SelectionText` from the palette.

### `applyFilter(s string)`

```go
func (qv *queuesView) applyFilter(s string) {
    qv.filter = s
    qv.updateTitle()
    qv.repaint(qv.allSummaries)
}
```

### `updateTitle()`

Sets the table title to `" Queues "` when filter is empty, or
`" Queues [<filter>] "` when active.

### `repaint(summaries []queue.Summary)`

Stores `summaries` into `allSummaries`. Filters before rendering:

```go
filtered := summaries
if qv.filter != "" {
    filtered = make([]queue.Summary, 0, len(summaries))
    lower := strings.ToLower(qv.filter)
    for _, s := range summaries {
        if strings.Contains(strings.ToLower(s.Name), lower) {
            filtered = append(filtered, s)
        }
    }
}
// ... render filtered
```

### `/` hotkey

Added to the table's `SetInputCapture`:

```go
case event.Rune() == '/':
    qv.filterInput.SetText(qv.filter)
    qv.app.tv.SetFocus(qv.filterInput)
    return nil
```

### `Shortcuts()`

```go
return []ui.Shortcut{
    {Key: "r", Description: "refresh"},
    {Key: "/", Description: "filter"},
}
```

### `Activate()`

Unchanged — still calls `load()`. Filter persists because it is stored on
`qv.filter` and applied in `repaint`.

## `reapplyTheme` (`theme.go`)

Add repaint of `filterInput` colors (background, label color, text color).

## Files touched

- `tui/internal/app/queues.go` — new fields, flex wrapper, filter logic
- `tui/internal/app/queues_test.go` — add filter tests
- `tui/internal/app/theme.go` — repaint filterInput
- `tui/internal/app/app.go` — `onGlobalKey` passes events through when filterInput is focused

## Key decisions

- **Always-visible 1-row input**: avoids dynamic show/hide complexity in
  tview's Flex (no `ResizeItem` API). Consistent with how many TUIs show a
  persistent filter bar.
- **Cache `allSummaries`**: re-filtering on the client side avoids an extra
  backend round-trip when the filter changes.
- **Live filtering via `SetChangedFunc`**: rows update on every keystroke, no need to press Enter.
- **Up/Down confirm + navigate**: pressing ↑/↓ confirms the filter, returns focus to the table, and forwards the key to the table's input handler so the selection moves immediately.
- **Global hotkey guard**: `onGlobalKey` in `app.go` passes events through when `filterInput` is focused, preventing keys like `s` from triggering view switches while typing.
- **Esc and Enter both confirm**: the filter stays active regardless of how
  the user closes the input — matches the spec requirement for persistence.
- **Substring, case-insensitive**: simple and sufficient for queue names.

## Testing

- `TestQueuesViewFilterShortcutPresent` — `/` in Shortcuts().
- `TestQueuesViewFilterApplied` — after `applyFilter("foo")` + `repaint`,
  only matching rows appear in the table.
- `TestQueuesViewFilterPersistsAfterRepaint` — set filter, repaint with new
  data, matching rows still filtered.
- `TestQueuesViewFilterClear` — empty filter shows all rows.
