# Plan — FE 10: Queue list column sorting

## `queuesView` changes (`queues.go`)

### New fields

```go
type queuesView struct {
    // ... existing fields ...
    sortCol int  // 0=NAME, 1=PENDING, 2=CONSUMERS, 3=ENQUEUED, 4=DEQUEUED
    sortAsc bool // true = ascending
}
```

Default: `sortCol=0`, `sortAsc=true` (preserves current behaviour).

### Column metadata

```go
var queueColumns = []string{"NAME", "PENDING", "CONSUMERS", "ENQUEUED", "DEQUEUED"}
```

Used in `setHeader` and `repaint` to build header labels dynamically.

### `setHeader()`

Rebuilds all 5 header cells. The active sort column gets a `▲` or `▼`
suffix; all others show the plain label.

```go
for i, col := range queueColumns {
    label := col
    if i == qv.sortCol {
        if qv.sortAsc { label += " ▲" } else { label += " ▼" }
    }
    // SetCell(0, i, ...)
}
```

### `repaint(summaries []queue.Summary)`

Replaces the hardcoded `sort.Slice` by name with a sort driven by
`sortCol` and `sortAsc`:

```go
sort.SliceStable(filtered, func(i, j int) bool {
    a, b := filtered[i], filtered[j]
    var less, equal bool
    switch qv.sortCol {
    case 1: less, equal = a.PendingCount < b.PendingCount, a.PendingCount == b.PendingCount
    case 2: less, equal = a.ConsumerCount < b.ConsumerCount, a.ConsumerCount == b.ConsumerCount
    case 3: less, equal = a.EnqueueCount < b.EnqueueCount, a.EnqueueCount == b.EnqueueCount
    case 4: less, equal = a.DequeueCount < b.DequeueCount, a.DequeueCount == b.DequeueCount
    default: less, equal = a.Name < b.Name, a.Name == b.Name
    }
    if equal { return a.Name < b.Name } // stable tiebreaker
    if qv.sortAsc { return less }
    return !less
})
```

After sorting, call `qv.setHeader()` to update the sort indicator.

### `o` / `O` hotkeys

Added to the table's `SetInputCapture`:

```go
case event.Rune() == 'o':
    qv.sortCol = (qv.sortCol + 1) % len(queueColumns)
    qv.repaint(qv.allSummaries)
    return nil
case event.Rune() == 'O':
    qv.sortAsc = !qv.sortAsc
    qv.repaint(qv.allSummaries)
    return nil
```

### `Shortcuts()`

```go
return []ui.Shortcut{
    {Key: "r", Description: "refresh"},
    {Key: "/", Description: "filter"},
    {Key: "o/O", Description: "sort col/dir"},
}
```

Combined into one entry: the context panel height equals the logo height (3 lines), so 4 separate shortcut lines would overflow.

## Files touched

- `tui/internal/app/queues.go` — new fields, dynamic header, sort logic, hotkeys
- `tui/internal/app/queues_test.go` — sort tests

## Key decisions

- **Name tiebreaker**: when two rows are equal on the active sort column,
  they are always ordered by name ascending. This prevents direction toggling
  from changing the order when all values are identical (e.g. DEQUEUED all 0),
  which would otherwise produce `!false = true` for every comparison —
  an inconsistent comparator that causes unpredictable output.
- **`sort.SliceStable`**: preserves relative order of equal rows, giving
  predictable output for numeric columns where many queues share the same value.
- **`o/O` combined shortcut entry**: small `o` cycles column, capital `O` flips
  direction. Combined into one `Shortcuts()` entry so the hint fits the 3-line
  context panel.
- **`setHeader` called inside `repaint`**: keeps the indicator in sync
  whenever data changes or sort state changes, with no separate call site.
- **Default preserved**: `sortCol=0, sortAsc=true` → ascending by name,
  same as before.

## Testing

- `TestQueuesViewSortByPending` — after setting `sortCol=1`, `sortAsc=false`
  and calling `repaint`, rows appear highest-pending first.
- `TestQueuesViewSortDirectionToggle` — toggle `sortAsc` changes row order.
- `TestQueuesViewSortHeaderMarker` — active column header contains `▲`/`▼`;
  others do not.
- `TestQueuesViewSortShortcutsPresent` — `o/O` combined entry in `Shortcuts()`.
