# Plan — Shell color palette evolution

Spec: [spec.md](spec.md)

## Approach

### Revision 1 — per-view colors + status schema

```go
type Palette struct {
    Border, Label, Value, Accent   string
    Success, Warning, Error        string // schema-only, unused
    Views map[string]string        // view name -> color, falls back to Border
}

func (p Palette) ViewColor(name string) string {
    if c, ok := p.Views[name]; ok && c != "" { return c }
    return p.Border
}
```

Applied via a `bordered` structural interface (`SetBorderColor`/
`SetTitleColor`, satisfied by anything embedding `*tview.Box`) in
`App.New()`'s view-registration loop, so `internal/ui/views` stays free
of any `internal/config` dependency — `internal/app` (which already
depends on both) does the wiring.

### Revision 2 — global re-theme

Confirmed against the `rivo/tview` v0.42.0 source that primitives
capture `tview.Styles` once, at construction time (`NewBox`'s
`backgroundColor: Styles.PrimitiveBackgroundColor`, `NewList`'s
`selectedStyle: Foreground(Styles.PrimitiveBackgroundColor)
.Background(Styles.PrimaryTextColor)`, etc.) — so a single mutation of
`tview.Styles`, done before any primitive is built, re-themes the whole
shell:

```go
func applyTheme(p config.Palette) {
    bg := tcell.GetColor(p.Background)
    tview.Styles.PrimitiveBackgroundColor = bg
    tview.Styles.ContrastBackgroundColor = bg
    tview.Styles.MoreContrastBackgroundColor = bg
    tview.Styles.BorderColor = tcell.GetColor(p.Border)
    tview.Styles.TitleColor = tcell.GetColor(p.Border)
    tview.Styles.GraphicsColor = tcell.GetColor(p.Border)
    tview.Styles.PrimaryTextColor = tcell.GetColor(p.Text)
    tview.Styles.SecondaryTextColor = tcell.GetColor(p.Value)
    tview.Styles.TertiaryTextColor = tcell.GetColor(p.Label)
    tview.Styles.InverseTextColor = tcell.GetColor(p.SelectionText)
    tview.Styles.ContrastSecondaryTextColor = tcell.GetColor(p.Value)
}
```

Called first thing in `App.New()`, before `proxy.New(...)` or the
`a.views` slice literal. Selection color is a deliberate exception:
tview's own computed default selection style just inverts body text,
which wouldn't produce a distinct teal highlight, so `styleList` wires
`SelectionBg`/`SelectionText` explicitly onto every `tview.List` the
shell constructs (queues, messages, settings, AWS-profile picker).
`Views`' five entries all collapse to `Border`'s value — the map/
fallback mechanism from revision 1 is untouched, just re-defaulted.

Top bar: a one-column `│` divider (colored with `Border`) between the
info panel and the nav panel; the nav panel gains a `Navigation:` heading
and switches key tokens from bare (`q`) to bracketed (`<q>`) form.
`infoPanelText` relabels to three lines ("Active connection"/"User"/
"AWS Profile") reusing existing config fields — no schema change there.
Status bar: `readyStatusText(cfg)` renders the idle-state hotkey legend
on the `StatusBarBg` background; transient messages
(`setStatus(...)`) still temporarily override it exactly as before.

## Files touched

- `internal/config/config.go` (+ `config_test.go`) — both revisions'
  `Palette` fields and `Default()` values.
- `internal/app/app.go` — revision 1's `bordered` interface/coloring
  loop; revision 2's `applyTheme` call site.
- `internal/app/theme.go` (new, revision 2) — `applyTheme`/`styleList`
  (+ `theme_test.go`).
- `internal/app/{topbar,statusbar,queues,settings}.go` (+ tests) —
  revision 2's divider/heading/key format, hotkey-legend status bar,
  `styleList` wiring on every list.
- `config.example.yaml` — kept in sync at each revision.

## Key decisions / trade-offs

- Border/title coloring (revision 1) lives in `internal/app`, not
  `internal/ui/views`, via the `bordered` structural interface — avoids
  coupling the views package to config.
- Global theming (revision 2) via one `tview.Styles` mutation rather
  than per-widget calls — the intended tview extension point, at the
  cost of being order-dependent (must run before any primitive exists).
- Selection colors are wired explicitly per list rather than riding on
  `applyTheme`, since tview's own default would just invert body text.
- `Views` keeps its 5-entry map even after collapsing to one shared
  color — preserves the override mechanism (and its tests) for anyone
  who wants per-view colors back via `config.yaml`.

## Testing

- `internal/config`: `Default()`/`ViewColor` fallback (revision 1);
  updated `Default()` values and the new fields' partial-override merge
  behavior (revision 2).
- `internal/app`: registered views render with their configured (or
  fallback) border color (revision 1); `applyTheme` followed by a fresh
  `tview.NewBox()` has matching `GetBorderColor()`/`GetBackgroundColor()`;
  info panel/shortcuts panel/status bar content and colors; divider
  column exists (revision 2).
- `styleList`'s effect isn't independently unit-testable — `tview.List`
  exposes setters but no getters for the resulting selection style;
  verified by manual/visual check instead, noted explicitly rather than
  skipped silently.
