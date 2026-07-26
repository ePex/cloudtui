# Plan — TUI shell: layout, navigation, and theming

Spec: [spec.md](spec.md)

## Approach

### Interfaces (`internal/ui`)

`view.go` defines the `View` interface (`Name() string`, `Title() string`,
`Primitive() tview.Primitive`).

### Config (`internal/config`)

```go
type Config struct {
    Theme  string  // "dark" | "cyberpunk" | "" (falls back to dark)
    Logo   []string
    Colors Palette
}

type Palette struct {
    Background, Border, Label, Text, Value, Accent string
    SelectionBg, SelectionText                      string
    StatusBarBg, StatusBarText                      string
    Views map[string]string // view name → border color override
}
```

`DarkPalette()` and `CyberpunkPalette()` return the two built-in palettes as
constants. `Default()` returns a `Config` with `Theme: "dark"` and the dark
palette. `Load`/`LoadDefault` unmarshal YAML on top of `Default()` so partial
overrides still get defaults for unset fields. `Save`/`SaveDefault` write back
to `config.yaml` in the working directory. A missing file is not an error.

### Global theming (`internal/app/theme.go`)

`applyTheme(p Palette)` mutates `tview.Styles` before any primitive is
constructed — the intended tview extension point. `styleList(l *tview.List,
p Palette)` explicitly wires `SelectionBg`/`SelectionText` per list, because
tview's default selection style just inverts body text.

Runtime switching requires updating both `tview.Styles` and all already-built
primitives. A `reapplyTheme(a *App, p Palette)` function mutates `tview.Styles`
then walks the app's registered views and shell components (top bar, status bar,
all lists) to set background/border/text colors explicitly, then calls
`a.tapp.Draw()`.

### Layout (`internal/app/app.go`)

Root `tview.Flex` (column = false, i.e. rows): top bar, a `tview.Pages` content
area, status bar. The content `Pages` is also the routing target for view
switching.

Top bar is itself a `tview.Flex` (columns): a left `tview.Pages` with two
pages — `"info"` (connection-info placeholder: one line for now) and
`"prompt"` (`:` command `InputField`) — a single `│` divider `TextView`, a
**context panel** `TextView` (view-specific shortcuts, empty by default), and
the ASCII logo. Swapping top-left pages via `SwitchToPage` is cleaner than
trying to replace a `Flex` child, which tview doesn't support.

The context panel is updated by `switchTo` whenever the active view changes: if
the view implements `Shortcuttable`, its shortcuts are rendered there; otherwise
the panel is cleared. The status bar carries the global hotkey legend — the
context panel must not repeat it.

Status bar is a single-line unbordered `TextView`.

### Home view (`internal/ui/views/home.go`)

Takes a `[]ViewInfo{Name, Description string}` slice at construction. Renders a
`tview.Table` (two columns: name | description, no borders, styled with palette
colors). Stateless — no config or backend dependency.

### Settings view (`internal/app/settings.go`)

Lives in `internal/app` (needs config read/write and overlay control). A
`tview.Form` where each config field is a row. The `Theme` field renders as a
`DropDown` (`"dark"` / `"cyberpunk"`); selecting a new value calls
`reapplyTheme` immediately (runtime switch) and persists via `config.Save`.
Other fields use `InputField`. On any change, config is saved and the app
redraws.

### Global hotkeys and `:command` prompt

`onGlobalKey` runs through priority layers:
1. Prompt focused → pass through unchanged.
2. Help modal open → swallow everything except `?`/`Esc`.
3. Rune dispatch: `:` ��� show `"prompt"` page in top-left and focus its
   `InputField`; `h` → `switchTo("home")`; `s` → `switchTo("settings")`;
   `q` → `app.Stop()`; `?` → toggle help overlay.

`:command` prompt `SetDoneFunc` parses the input:
- `:h`/`:home` → `switchTo("home")`
- `:s`/`:settings` → `switchTo("settings")`
- `:q`/`:quit` → `app.Stop()`
- `:theme <name>` → look up built-in palette, call `reapplyTheme`, save config

### Help modal (`internal/app/help.go`)

A `tview.Modal` (or bordered `TextView` in a `tview.Pages` overlay named
`"help"`) listing all key bindings. Shown/hidden via `rootPages.ShowPage` /
`rootPages.HidePage` — draws on top of the main layout rather than replacing it.

## Files touched

- `tui/cmd/tui/main.go` — wire `app.New().Run()`
- `tui/internal/ui/view.go`
- `tui/internal/ui/shortcuttable.go` — `Shortcuttable` interface + `Shortcut` struct
- `tui/internal/ui/views/home.go` (+ `views_test.go`)
- `tui/internal/config/config.go` (+ `config_test.go`)
- `tui/internal/app/app.go` (+ `app_test.go`)
- `tui/internal/app/topbar.go` (+ `topbar_test.go`)
- `tui/internal/app/statusbar.go` (+ `statusbar_test.go`)
- `tui/internal/app/theme.go` (+ `theme_test.go`)
- `tui/internal/app/help.go` (+ `help_test.go`)
- `tui/internal/app/settings.go` (+ `settings_test.go`)
- `tui/config.example.yaml`

## Key decisions / trade-offs

- **Runtime theme switch via `reapplyTheme`** — walks all live primitives and
  calls `Draw()`. More work than a restart, but a better UX. The alternative
  (restart the tview app) would reset all view state.
- **`tview.Styles` mutation must precede primitive construction** — order-
  dependent; `applyTheme` is the first call in `App.New()`.
- **Prompt as `tview.Pages` page** — `tview.Flex` has no child-swap primitive;
  `SwitchToPage` is the clean solution. The top-left panel has two pages:
  `"info"` and `"prompt"`.
- **Home view in `internal/ui/views`** — stateless, takes view descriptors as
  constructor args, no config dependency.
- **Settings view in `internal/app`** — needs config r/w and overlay control;
  same reasoning that will apply to any view requiring a live backend.
- **`styleList` per list** — tview's default selection inverts body text, which
  doesn't produce the intended highlight; explicit wiring is the only option.
  Effect is not unit-testable (tview exposes setters, not getters, for selection
  style); verified manually and noted explicitly.
- **`gopkg.in/yaml.v3`** — no stdlib YAML; this is the Go standard choice.
- **Context panel replaces static shortcuts panel** — the top bar's middle
  section shows view-specific shortcuts via `Shortcuttable`, and is blank for
  views that don't implement it. The status bar is the sole home for global
  hotkeys, avoiding duplication.

## Testing

- `internal/config`: `Default()` values, `Load` partial-override merge,
  load/save round-trip, `DarkPalette`/`CyberpunkPalette` field completeness,
  `ViewColor` fallback.
- `internal/app`: hotkey routing (prompt-focused pass-through, help-open
  swallow, rune dispatch), help modal open/close, `:theme` command applies palette change, settings form saves on
  change; `applyTheme` followed by a fresh `tview.NewBox()` has matching
  `GetBorderColor()`/`GetBackgroundColor()`; status bar content; `switchTo`
  clears context panel for views without `Shortcuttable`, renders shortcuts for
  views that implement it.
- `internal/ui/views`: `home.go` constructor `Name()`/`Title()`.
- `styleList` effect: not unit-testable; verified manually.
