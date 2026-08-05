# Plan — CR 05: Home view navigatable sections

## Approach

Keep the `tview.Table` primitive but enable row selection and add section
header rows. tview.Table supports `SetSelectable(rows, cols bool)` at the
table level and `cell.SetSelectable(false)` per-cell, so section headers can
be non-selectable while entry rows are selectable. `SetSelectedFunc` handles
Enter.

## Section structure

A new `SectionInfo` type replaces the flat `[]ViewInfo`:

```go
type SectionInfo struct {
    Title   string
    Entries []ViewInfo
}
```

Passed to `NewHome` and stored for repaint. Sections are defined in `app.go`
(hard-coded for now):

```go
[]views.SectionInfo{
    {Title: "Apps",   Entries: []views.ViewInfo{{Name:"queues", ...}}},
    {Title: "System", Entries: []views.ViewInfo{{Name:"settings",...},{Name:"log",...}}},
}
```

"Home" is not included.

## Visual layout

```
─── Apps ────────────────────────────────
  queues      List ActiveMQ queues via Jolokia
─── System ──────────────────────────────
  settings    Edit and persist app configuration
  log         View the application log
```

Section header: full-width cell spanning both columns, styled with the
`label` palette color, text `─── <Title> ` followed by `─` padding.
Entry rows: same two-column layout as today (name | description), selectable.

## Navigation

- `SetSelectable(true, false)` on the table enables row-level selection.
- tview's built-in ↑/↓ (and PageUp/PageDown) move the cursor; j/k are wired
  via `SetInputCapture` to emit synthetic ↑/↓ events.
- Section header rows have `SetSelectable(false)` on their cell, so the cursor
  skips them automatically (tview skips non-selectable rows when navigating).
- `SetSelectedFunc` fires when the user presses Enter; the callback receives
  the row index, which maps back to a view name and calls into app via a
  `func(name string)` passed to `NewHome`.

## Files touched

- `tui/internal/ui/views/home.go` — `SectionInfo` type; `NewHome` signature
  updated; `RepaintHomeTable` updated; section header rendering; navigation
  wiring.
- `tui/internal/app/app.go` — define sections, remove "home", pass
  `a.switchTo` callback.
- `tui/internal/app/theme.go` — `RepaintHomeTable` call updated.
- `tui/internal/ui/views/home_test.go` — update for new API.
- `tui/internal/app/app_test.go` — update `homeViewInfos`-dependent tests.

## Key decisions

- **`tview.Table` over `tview.List`**: List has no concept of non-selectable
  items, making section headers awkward. Table gives full per-cell control.
- **Callback `func(name string)`**: home view lives in `ui/views`, which has no
  knowledge of `App`; passing a callback keeps the layer boundary clean.
- **j/k forwarding**: emit a synthetic `tcell.EventKey` with `KeyUp`/`KeyDown`
  via `table.InputHandler()(event, nil)` — this is idiomatic tview and avoids
  duplicating selection logic.
- **Section headers span both columns** via `SetExpansion` + `SetMaxWidth(0)`
  on a single wide cell in column 0, with column 1 left empty.

## Testing

Unit tests (no running app required):
- Section headers present and non-selectable.
- Entry rows selectable, correct names/descriptions.
- Callback fired with correct view name on selection.
- `RepaintHomeTable` resets colors without changing structure.

Manual:
- Home screen shows Apps and System sections with styled headers.
- Arrow keys and j/k move cursor, skipping headers.
- Enter on an entry switches to that view.
- Theme switch repaints section header colors.
