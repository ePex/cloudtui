# Spec — FE 19: Theme files

Date: 2026-08-07

## What

Built-in themes (`dark`, `cyberpunk`) are currently hardcoded as Go
functions (`DarkPalette()`, `CyberpunkPalette()`). This feature moves them
to YAML files embedded in the binary and extends the system so the
settings dropdown discovers available themes automatically rather than
relying on a hardcoded list.

The config architecture work (see *Already shipped* below) is the
foundation: `colors:` in `config.yaml` means user overrides only, and
`theme:` drives the base palette at startup.

## Why

- Adding or tweaking a theme today requires editing Go source, rebuilding,
  and shipping a new binary — even for pure palette changes.
- The hardcoded theme list in the settings form must be manually kept in
  sync with the Go palette functions; they can diverge silently.
- YAML-defined themes are easier to read and compare side-by-side.
- Embedding the files in the binary keeps the single-binary distribution
  model: no external files required at runtime.

## Already shipped (config architecture foundation)

The following changes were implemented as part of this feature and are
already in the codebase:

- `Palette` yaml tags have `omitempty` so sparse palettes don't write
  empty strings.
- `Load()` uses a two-pass unmarshal: effective palette = theme base +
  explicit user overrides from `colors:`.
- `switchTheme()` saves only the theme name and sparse user overrides —
  never the full derived palette.
- `ApplyPaletteOverrides` / `PaletteUserOverrides` helpers in `config.go`.

## Desired behaviour (remaining work)

- `tui/themes/dark.yaml` and `tui/themes/cyberpunk.yaml` define the
  built-in palettes; the hardcoded Go palette functions are removed.
- `config.PaletteForTheme(name)` loads the palette from the corresponding
  embedded file; it continues to return `(Palette, bool)`.
- A new `config.AvailableThemes()` function returns the sorted list of
  embedded theme names. The settings dropdown calls this instead of a
  hardcoded slice.

## Scope (remaining)

- `tui/themes/dark.yaml` and `tui/themes/cyberpunk.yaml` — new files.
- `tui/internal/config/config.go` — embed `themes/*.yaml`; replace
  `DarkPalette()` / `CyberpunkPalette()` with file-based loading in
  `PaletteForTheme()`; add `AvailableThemes()`.
- `tui/internal/app/settings.go` — replace hardcoded `themes` slice with
  `config.AvailableThemes()`.
- `tui/config.example.yaml` — update theme comment to note that the name
  must match a file in the embedded `themes/` directory.

## Out of scope

- User-defined themes in `~/.cloudtui/themes/` (future extension).
- Theme validation / descriptive error messages for malformed YAML.
- Theme hot-reload without restart.
- Exposing individual color pickers in the Settings UI.
