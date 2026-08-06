# Plan — FE 19: Theme files

## Already implemented (config architecture foundation)

The following are done and tested:

- `Palette` yaml tags: `omitempty` on all string fields.
- `ApplyPaletteOverrides(base, overrides Palette) Palette` helper.
- `PaletteUserOverrides(effective, base Palette) Palette` helper.
- `Load()` two-pass unmarshal: effective = theme base + user overrides.
- `switchTheme()` saves sparse overrides, not the full derived palette.
- `config_test.go` updated + new helper/override tests.
- `config.example.yaml` updated to sparse documentation.

## Remaining work — theme YAML files

### Approach

1. **Theme files** — create `tui/themes/dark.yaml` and
   `tui/themes/cyberpunk.yaml`, each containing a `Palette` YAML document
   (same field names as the existing struct tags).

2. **Embed** — add `//go:embed themes/*.yaml` in `config.go` to bundle
   the files into the binary. No external files needed at runtime.

3. **`PaletteForTheme()`** — replace the `switch` on hardcoded names with
   a lookup into the embedded FS: read `themes/<name>.yaml` and unmarshal.
   Returns `(Palette, false)` for unknown names (file not found).

4. **`AvailableThemes()`** — new exported function; reads the embedded FS
   directory, strips the `.yaml` extension, returns names sorted
   alphabetically.

5. **`DarkPalette()` / `CyberpunkPalette()`** — removed. Any internal
   callers replaced with `PaletteForTheme("dark")` / `PaletteForTheme("cyberpunk")`.
   `Default()` changes `Colors: DarkPalette()` → resolves via
   `PaletteForTheme("dark")` (or a local constant fallback if the embed
   hasn't loaded yet — see note below).

6. **Settings dropdown** — `newSettingsView` replaces the hardcoded
   `themes := []string{"dark", "cyberpunk"}` with
   `config.AvailableThemes()`.

### Note on `Default()` and `Load()` bootstrap

`Default()` is called from `Load()` as the merge base, and from `main.go`
as the error fallback. Both happen after the binary is fully loaded, so
the embedded FS is available. `PaletteForTheme("dark")` will succeed.
The `ok` return is ignored in `Default()` — if the embed is somehow
absent, `Colors` will be the zero Palette, which is safe (tview falls back
to terminal defaults).

### Theme YAML format

```yaml
background: "#1a1b26"
border:     "#c0caf5"
label:      "#e0af68"
text:       "#c0caf5"
value:      "#7dcfff"
accent:     "#ff79c6"
selectionBg:   "#2ac3de"
selectionText: "#1a1b26"
statusBarBg:   "#ff9e64"
statusBarText: "#1a1b26"
views:
  home:     "#c0caf5"
  settings: "#c0caf5"
  queues:   "#c0caf5"
```

The `omitempty` tags on `Palette` have no effect on loading (yaml.v3
always reads present fields), so the theme files can include all fields
without issue.

## Files touched (remaining work)

| File | Change |
|------|--------|
| `tui/internal/config/themes/dark.yaml` | new — dark palette definition |
| `tui/internal/config/themes/cyberpunk.yaml` | new — cyberpunk palette definition |
| `tui/internal/config/config.go` | embed directive; replace `DarkPalette()` / `CyberpunkPalette()` / `PaletteForTheme()` / `Default()`; add `AvailableThemes()` |
| `tui/internal/config/config_test.go` | update tests that call removed functions; add `TestAvailableThemes` |
| `tui/internal/app/settings.go` | use `config.AvailableThemes()` |
| `tui/config.example.yaml` | update theme comment |

## No new dependencies

`embed` is part of the Go standard library (Go 1.16+).
