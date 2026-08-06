# Tasks — FE 19: Theme files

Spec: [spec.md](spec.md) | Plan: [plan.md](plan.md)

## Foundation — config architecture (done)

1. [x] **`Palette` omitempty tags** — `omitempty` on every string field of
   `Palette` in `config.go`.

2. [x] **`ApplyPaletteOverrides` helper** — exported function; merges
   Views map rather than replacing it.

3. [x] **`PaletteUserOverrides` helper** — returns sparse delta palette.

4. [x] **Update `Load()`** — two-pass unmarshal; effective palette =
   theme base + user overrides.

5. [x] **Update `switchTheme()`** — saves sparse overrides only; user
   overrides preserved across theme switches.

6. [x] **Revise existing `config_test.go` tests** — `TestLoadThemeOverride`,
   `TestLoadViewsPartialOverrideMergesDefaults`, `TestSaveLoadRoundTrip`.

7. [x] **New tests** — `TestApplyPaletteOverrides`, `TestPaletteUserOverrides`,
   `TestLoadThemeWithColorOverride`.

8. [x] **Update `config.example.yaml`** — sparse colors documentation.

## Theme YAML files (remaining)

9. [ ] **Create `tui/themes/dark.yaml`** — full dark palette as YAML.

10. [ ] **Create `tui/themes/cyberpunk.yaml`** — full cyberpunk palette as YAML.

11. [ ] **Embed and load themes in `config.go`** — add `//go:embed themes/*.yaml`;
    replace `DarkPalette()` / `CyberpunkPalette()` / `PaletteForTheme()` switch
    with embedded-FS lookup; update `Default()` to use `PaletteForTheme("dark")`;
    add `AvailableThemes()`.

12. [ ] **Update `config_test.go`** — remove tests for deleted functions;
    add `TestAvailableThemes`; verify existing load/override tests still pass.

13. [ ] **Update `settings.go`** — replace hardcoded `themes` slice with
    `config.AvailableThemes()`.

14. [ ] **Update `config.example.yaml`** — note that `theme:` must match an
    embedded theme file name.

15. [x] **Fix dropdown popup list contrast** — unselected items were invisible
    because the popup list had no explicit text/background colors. Added
    `styleDropDown(dd, palette)` helper in `settings.go` (called from
    `newSettingsView` and from `reapplyTheme`) that calls `SetListStyles`
    with `Text`/`Background` for unselected and `SelectionText`/`SelectionBg`
    for the selected item.

16. [x] **Manual verification** — start app, open settings, confirm both
    themes appear in the dropdown with readable items; select cyberpunk,
    confirm theme applies and `config.yaml` contains only `theme: cyberpunk`.
