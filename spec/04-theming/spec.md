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
- `Palette` struct fields all use `yaml:"...,omitempty"` so an empty/default override never gets serialized.
- The Settings theme dropdown is keyboard-interactive: opening it, moving between options, and confirming with Enter all work. (This requires focus to land on the Settings view's own primitive when switching views — see spec-origin/01-repo-and-tui-shell's navigation note; focusing the `Pages` container instead leaves the dropdown unable to receive input, since tview does not cascade key events from `Pages` into a child `Form`.)

## Data & config

```yaml
theme: dark          # must match a file name (without .yaml) in the embedded themes/ dir
colors:              # sparse — only fields the user explicitly overrides
  accent: "#FF00FF"
```

Two built-in themes ship: `dark` and `cyberpunk` (palettes described in spec-origin/01-repo-and-tui-shell). Each theme YAML defines the full `Palette` shape: `background`, `border`, `label`, `text`, `value`, `accent`, `selectionBg`, `selectionText`, `statusBarBg`, `statusBarText`, plus a `views:` map for per-view color overrides.

## Implementation notes

- `tui/internal/config/themes/dark.yaml`, `tui/internal/config/themes/cyberpunk.yaml` — embedded theme files.
- `tui/internal/config/config.go` — `PaletteForTheme`, `AvailableThemes`, `ApplyPaletteOverrides`, `PaletteUserOverrides`, the two-pass `Load()`.
- `tui/internal/app/settings.go` (theme picker logic) — calls `config.AvailableThemes()` for the dropdown's option list.
- `tui/config.example.yaml` — documents that `theme:` must match an embedded file name.

## Notable gotchas worth preserving

- **View focus on navigation**: `switchTo`-equivalent logic must focus `view.Primitive()`, not the `Pages` container — otherwise interactive form/dropdown widgets on that view silently stop receiving keyboard input. This was originally caught via the Settings theme dropdown specifically, but the rule applies to any view with a `tview.Form`/`DropDown`/interactive widget.
- Adding a new theme is purely a YAML drop-in — no Go code changes required; the dropdown picks it up automatically via `AvailableThemes()`.
- **Runtime switch mechanism, deliberately chosen**: switching theme live works by walking every constructed primitive and recoloring it in place (`reapplyTheme`), not by tearing down and restarting the app — a restart would lose all current view state (selected rows, open overlays, scroll position, in-flight data). `reapplyTheme` does not call `tv.Draw()` itself; tview redraws automatically on the next event loop tick, and calling `Draw()` directly would block in test environments where no event loop is running.
- **`tview.Styles` must be set before any primitive is constructed** — primitives read `tview.Styles` package-level state at construction time, not on every draw, so setting it later has no effect on already-built widgets. This is why `applyTheme(cfg.Colors)` is the very first line of `App.New()`, ahead of building any view or dialog.
