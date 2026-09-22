# Theming

_Condensed from spec/18-bugfix-settings-theme-dropdown, spec/19-fe-theme-files — see those folders for the incremental history._

## Purpose

A palette-driven color system, configurable per-theme and per-user, switchable at runtime without a rebuild or restart.

## Behavior

- Built-in themes are defined as **YAML files embedded in the binary** via `//go:embed` (not hardcoded Go functions) — adding or tweaking a theme is a YAML edit, not a Go code change + rebuild.
- `config.PaletteForTheme(name)` loads a palette from the embedded theme files, returning `(Palette, bool)`.
- `config.AvailableThemes()` returns the sorted list of embedded theme names, discovered from the embedded filesystem — the settings dropdown calls this directly, so it can never drift out of sync with the actual theme files.
- `config.yaml`'s `theme:` field selects the base palette at startup; its `colors:` section holds **sparse user overrides only** (fields the user explicitly customized), never a full derived palette.
- `config.Load()` does a **two-pass unmarshal**: effective `Colors` = theme base (from `PaletteForTheme(theme)`) merged with the sparse `colors:` overrides on top (`ApplyPaletteOverrides`).
- Switching theme at runtime (via the Settings dropdown or `:theme <name>`) applies immediately, no restart needed, and persists by writing only the theme name plus sparse overrides back to `config.yaml` (`PaletteUserOverrides` computes the sparse diff against the new base) — never the full resolved palette. This keeps `config.yaml` legible and keeps a theme's own file as the single source of truth for its unmodified colors.
- **A live switch recolors everything exactly as a restart would.**
  - **What's covered:** every list, form (labels, fields, buttons),
    input field, dropdown (label, field, pop-up list, focused/disabled
    state), autocomplete drop-down, table header and separator, and text
    view.
  - **Where:** in overlays, views, and the top bar (info, context and
    logo panels, and the `:` prompt).
  - **Overlays** are seen in the new theme when next opened; the theme
    picker is recolored while it stays on screen.
  - **One exception:** table **data rows** (queue names, message types,
    the favorites star, ...) keep the previous theme's colors until the
    view next repaints (a refresh, filter change, reload, or the queue
    list's auto-refresh). Repainting them on a switch would reset the
    cursor to the first row, since every view's `repaint` does.
- **Startup applies every widget's palette too:** `App.New` calls every
  themable's `ApplyPalette(cfg.Colors)` once after building them. Widgets
  whose palette colors come only from `ApplyPalette` (e.g. the pickers'
  selected-row highlight) therefore look the same right after startup as
  after a live switch.
- `Palette` struct fields all use `yaml:"...,omitempty"` so an empty/default override never gets serialized.
- The Settings theme dropdown is keyboard-interactive: opening it, moving between options, and confirming with Enter all work. (This requires focus to land on the Settings view's own primitive when switching views — see spec/01-repo-and-tui-shell's navigation note; focusing the `Pages` container instead leaves the dropdown unable to receive input, since tview does not cascade key events from `Pages` into a child `Form`.)

## Data & config

```yaml
theme: dark          # must match a file name (without .yaml) in the embedded themes/ dir
colors:              # sparse — only fields the user explicitly overrides
  accent: "#FF00FF"
```

Three built-in themes ship: `dark`, `cyberpunk` (palettes described in spec/01-repo-and-tui-shell), and `gofore` — the Gofore brand palette: Deep Blue (`#0F3D51`) background, Gofore Orange (`#F7673B`) labels, Digital Blue (`#00819D`) borders/selection, Code Blue (`#44C2DE`) values, Salmon Byte (`#FFA572`) accent. Each theme YAML defines the full `Palette` shape: `background`, `border`, `label`, `text`, `value`, `accent`, `selectionBg`, `selectionText`, `statusBarBg`, `statusBarText`, plus a `views:` map for per-view color overrides.

## Implementation notes

- `tui/internal/config/themes/dark.yaml`, `tui/internal/config/themes/cyberpunk.yaml`, `tui/internal/config/themes/gofore.yaml` — embedded theme files.
- `tui/internal/config/config.go` — `PaletteForTheme`, `AvailableThemes`, `ApplyPaletteOverrides`, `PaletteUserOverrides`, the two-pass `Load()`.
- `tui/internal/dialog/themepicker.go` (`ThemePicker`) — calls `config.AvailableThemes()` for the dropdown's option list; `tui/internal/app/promptcommands.go` calls it too, for the `:theme ` autocomplete.
- `tui/config.example.yaml` — documents that `theme:` must match an embedded file name.
- `tui/internal/ui/theme.go` — `ApplyTviewStyles(p)`: the single palette → `tview.Styles` mapping (`Background` for the primitive/contrast backgrounds, `Text` primary, `Value` secondary and contrast-secondary, `Label` tertiary, `Border` border/title/graphics, `SelectionText` inverse). `app.applyTheme` calls it at startup and on every switch.
- `tui/internal/ui/style.go` — the helpers every `ApplyPalette` uses to re-apply what a widget copied at construction, with the same colors a freshly built widget gets:
  - `StyleList`: main, secondary and shortcut text, plus the selection highlight
  - `StyleForm`: label, field, the three button states, and each item's and button's own box background
  - `StyleFilterInput`: stand-alone `/` inputs — `Label` label, `SelectionText` on `SelectionBg` field, via `SetFormAttributes`, plus the outer box background
  - `StyleDropDown` (stand-alone) and `StyleFormDropDown` (inside a form: pop-up list, focused, prefix, disabled)
  - `StyleInputFieldAutocomplete`
- `tui/internal/app/theme.go` — `reapplyTheme`: sets `tview.Styles`, recolors the shell chrome (status bar, top-bar panels including their base text color, the `:` prompt's inner text area *and* outer box), then calls every themable's `ApplyPalette`.
- Table views redraw their header row (`setHeader()`) and reset their borders color in `ApplyPalette`, since both are colored only when created.

## Notable gotchas worth preserving

- **View focus on navigation**: `switchTo`-equivalent logic must focus `view.Primitive()`, not the `Pages` container — otherwise interactive form/dropdown widgets on that view silently stop receiving keyboard input. This was originally caught via the Settings theme dropdown specifically, but the rule applies to any view with a `tview.Form`/`DropDown`/interactive widget.
- Adding a new theme is purely a YAML drop-in — no Go code changes required; the dropdown picks it up automatically via `AvailableThemes()`.
- **Runtime switch mechanism, deliberately chosen**: switching theme live works by walking every constructed primitive and recoloring it in place (`reapplyTheme`), not by tearing down and restarting the app — a restart would lose all current view state (selected rows, open overlays, scroll position, in-flight data). `reapplyTheme` does not call `tv.Draw()` itself; tview redraws automatically on the next event loop tick, and calling `Draw()` directly would block in test environments where no event loop is running.
- **`tview.Styles` must be set before any primitive is constructed** — primitives read `tview.Styles` package-level state at construction time, not on every draw, so setting it later has no effect on already-built widgets. This is why `applyTheme(cfg.Colors)` is the very first line of `App.New()`, ahead of building any view or dialog.
- **The same fact is why a live switch needs the `ui.Style*` helpers.** Here's what keeps its construction-time colors unless re-applied:
  - `List`: unselected rows
  - `Form`: label color, field style, button styles
  - `DropDown`: focused, prefix and disabled styles
  - `TextView`: base text color, which untagged characters use (spaces between color tags, the logo, log lines without a level)
  - table cells: colored when set

  Every widget kind added to a view or dialog needs its colors re-applied in `ApplyPalette`, through the matching helper where one exists.
- **`InputField` has two backgrounds.** It wraps a private `TextArea` whose own box background is what the label is drawn on; only `SetFormAttributes` reaches it. The outer box background paints wherever the field doesn't (beside it, below it). Both need resetting, which `StyleFilterInput`/`StyleForm` and `reapplyTheme` (for the `:` prompt) do.
- **Autocomplete drop-downs are cached.** tview builds the drop-down's internal list once, with the styles set at that moment, and drops it only when the field loses focus or has no suggestions. `SetAutocompleteFunc` builds it immediately. So `StyleInputFieldAutocomplete` blurs an unfocused field after restyling, to force a rebuild with the new styles.
- **Testing a theme fix:**
  - compare a live switch against a restart cell by cell, on a simulation screen
  - and/or scan for colors only the old palette has; the regression tests in `internal/dialog` and `internal/view` build every overlay and view under `dark` and switch to `cyberpunk`
  - mutation-check that the test fails without the fix: several first attempts here passed while testing nothing
- **Palette colors are literal hex strings, resolved via `tcell.GetColor` (true 24-bit RGB)** — never tcell's named color constants (`tcell.ColorNavy`, `tcell.ColorDarkCyan`, etc.), which are a fixed 16/256-color approximation whose actual on-screen result depends on the terminal's color profile and can look nothing like the intended hex value (e.g. rendering navy as cyan). Every `Palette` field is a `"#RRGGBB"` string end to end, from the theme YAML through to the `tcell.GetColor(p.X)` calls in `applyTheme`/`reapplyTheme` — introducing a named-constant color anywhere in that path reintroduces this bug. The few hardcoded `tcell.ColorRed`/`ColorGreen`/etc. uses for semantic status indicators (errors, pipeline health) are a deliberate exception — they're not part of the theme palette and aren't expected to shift with it.
- **A selected row must have its foreground recolored to `selectionText`, not just its background to `selectionBg`** — `selectionText` is chosen for contrast against `selectionBg` specifically; leaving a row's normal per-field text color (label/text/value) in place on top of `selectionBg` can be unreadable (e.g. a light value color on a bright cyan selection highlight). Any selectable list/table applies both uniformly on selection (see `tview.Table.SetSelectedStyle` usage in `tui/internal/ui/views/home.go`), overriding whatever per-cell colors the row normally uses.
