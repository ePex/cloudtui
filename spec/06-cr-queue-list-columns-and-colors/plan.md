# Plan — CR 06: Queue list extended columns, colors, sorting, navigation

## Files touched

- `tui/internal/queue/backend.go` — add `EnqueueCount`, `DequeueCount` to `Summary`
- `tui/internal/queue/jolokia/jolokia.go` — fetch two extra attributes per queue
- `tui/internal/queue/jolokia/jolokia_test.go` — update happy-path test
- `tui/internal/app/queues.go` — 5-column repaint, header style, color logic, selectable rows, sort
- `tui/internal/app/queues_test.go` — update/extend tests

## Summary struct

```go
type Summary struct {
    Name          string
    PendingCount  int64
    ConsumerCount int64
    EnqueueCount  int64
    DequeueCount  int64
}
```

## Jolokia bulk read

Currently two attributes per queue (`QueueSize`, `ConsumerCount`). Extend to
four: `QueueSize`, `ConsumerCount`, `EnqueueCount`, `DequeueCount`. The bulk
response slice grows to 4 entries per queue; decode accordingly.

## Queues view repaint

### Header row
Single fixed header at row 0, non-selectable. Each cell gets:
- Background: `tcell.GetColor(p.Label)` (matches the cyan/teal in the screenshot)
- Foreground: `tcell.GetColor(p.Background)` (dark, to contrast the bright header)
- Text: `NAME ▲`, `PENDING`, `CONSUMERS`, `ENQUEUED`, `DEQUEUED`

### Data rows
Sorted by `Name` ascending before render. Selectable rows. Per-cell color:

| Column | Color condition |
|--------|----------------|
| Name   | `p.Value` (teal/cyan) |
| Pending | `p.Accent` if > 0, else `p.Text` |
| Consumers | `p.Accent` if = 0, else `p.Text` |
| Enqueued | `p.Text` |
| Dequeued | `p.Text` |

### Selection
`table.SetSelectable(true, false)` — row selection. No explicit
`SetSelectedStyle` needed; tview.Styles (set by `applyTheme`) provides the
correct selection colors globally.

### Sort
`sort.Slice(summaries, func(i, j int) bool { return summaries[i].Name < summaries[j].Name })`
applied inside `repaint()` before rendering.

### j/k navigation
Wire `SetInputCapture` to forward j/k to ↑/↓ (same pattern as home view).

## Key decisions

- **Header colors from palette**: using `p.Label` as header background and
  `p.Background` as header text gives the bright-on-dark header visible in the
  screenshot and adapts correctly to theme changes.
- **Accent for warnings**: `p.Accent` is already the "highlight" color in both
  palettes (pink in dark, yellow in cyberpunk). Reusing it for warning cells is
  consistent.
- **`p.Value` for queue names**: the teal/cyan color used in the screenshot for
  queue names matches `p.Value` in the dark palette (`#7dcfff`).
- **No `SetSelectedStyle`**: tview.Styles is already configured by `applyTheme`
  with the palette's selection colors, so the table picks them up automatically.

## Testing

- `TestQueuesViewHeaderLabels` — updated to check all 5 headers including `▲`
- `TestQueuesViewColumnCount` — updated to expect 5
- `TestQueuesViewRepaintSortsAlphabetically` — new: out-of-order input, check row order
- `TestQueuesViewPendingColorWhenNonZero` / `TestQueuesViewConsumerColorWhenZero` — new: color logic
- Jolokia happy-path test updated to return 4 attributes per queue and check `EnqueueCount`/`DequeueCount`
