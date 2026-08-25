# Plan

## Files touched

- `tui/internal/ui/style.go` — Problem 1: add a `blendColors` helper and use
  it in `StyleInputFieldAutocomplete`.
- `tui/internal/ui/style_test.go` — unit test for `blendColors` (the one
  piece of this fix with a directly assertable return value).
- `tui/internal/app/promptcommands.go` — Problem 2: filter global-hotkey
  aliases out of `promptSuggestions`.
- `tui/internal/app/app_test.go` (or a new `promptcommands_test.go`,
  decided during implementation — see below) — unit tests for
  `promptSuggestions`' filtering.
- `spec/01-repo-and-tui-shell/spec.md` — merge-back once both problems are
  implemented: update the "Command prompt autocomplete" section to
  describe the panel background and the alias-filtering behavior.

No other files change. No new dependencies.

## Problem 1: panel-background contrast

`StyleInputFieldAutocomplete` currently does:

```go
i.SetAutocompleteStyles(
    tcell.GetColor(p.Background),
    tcell.StyleDefault.Foreground(tcell.GetColor(p.Text)).Background(tcell.GetColor(p.Background)),
    tcell.StyleDefault.Foreground(tcell.GetColor(p.SelectionText)).Background(tcell.GetColor(p.SelectionBg)),
)
```

New: compute `panelBg := blendColors(tcell.GetColor(p.Background), tcell.GetColor(p.Accent), 0.15)`
once, and use it for both the `background` argument and the unselected
(`main`) style's background. Selected style is untouched.

```go
func blendColors(a, b tcell.Color, t float64) tcell.Color {
    ar, ag, ab := a.RGB()
    br, bg, bb := b.RGB()
    if ar < 0 || br < 0 {
        return a // either color isn't RGB-representable; don't blend toward garbage
    }
    lerp := func(x, y int32) int32 { return x + int32(float64(y-x)*t) }
    return tcell.NewRGBColor(lerp(ar, br), lerp(ag, bg), lerp(ab, bb))
}
```

### Key decisions

- **Blend factor 0.15 (15% toward Accent), fixed.** Chosen by eye against
  the three existing themes (`dark`, `cyberpunk`, `gofore`) to be clearly
  distinguishable from the plain background without pulling text
  contrast down — no theme gets a new field, no config knob. If a theme
  added later looks wrong at 0.15, that's a follow-up tuning change, not
  a blocker here.
- **Blend toward `Accent`, not `Border` or `SelectionBg`.** `Border` sits
  too close to `Text` in some themes (e.g. `dark`: `#c0caf5` for both),
  which would wreck readability if used as a fill. `SelectionBg` is
  reserved for the highlighted row, and reusing it (even diluted) risks
  the unselected/selected rows reading as similar. `Accent` is
  deliberately a "pop" color in every theme and is otherwise unused by
  this drop-down, so blending toward it doesn't compete with an existing
  meaning.
- **`blendColors` degrades to returning `a` unchanged if either color
  isn't RGB-decodable** (e.g. an unset palette field in a test fixture
  or a named/default `tcell.Color`), rather than computing garbage from
  `-1` components. All real palettes always populate every field (loaded
  through `config.PaletteForTheme` + `ApplyPaletteOverrides`), so this
  only matters for hand-built `config.Palette{}` values in tests.
- **No change to `StyleDropDown`** (the `tview.DropDown` popup used by
  Settings' pickers) — out of scope per `spec.md`.

## Problem 2: filter global-hotkey aliases from suggestions

Add a small, explicitly-named set in `promptcommands.go`:

```go
// globalHotkeyAliases are promptCommand names that duplicate a global
// single-key hotkey handled by onGlobalKey (tui/internal/app/app.go,
// the switch around lines 456-483). They stay valid to type-and-execute
// in the prompt (see promptCommandTable) but are excluded from
// promptSuggestions: the global hotkey already covers that need, and
// suggesting them just clutters the list under their own full name.
// Keep this in sync with onGlobalKey's switch if a hotkey is added,
// removed, or reassigned.
var globalHotkeyAliases = map[string]bool{"h": true, "s": true, "l": true, "q": true}
```

In `promptSuggestions`, in the loop over `promptCommandTable()`'s names,
skip adding a name when `globalHotkeyAliases[n]` is true:

```go
for _, pc := range promptCommandTable() {
    for _, n := range pc.names {
        if globalHotkeyAliases[n] {
            continue
        }
        if strings.HasPrefix(n, currentText) {
            add(n)
        }
    }
}
```

`onPromptDone`'s execution loop is untouched — it still iterates every
name in `pc.names`, so `:q`/`:h`/`:s`/`:l` keep executing when typed in
full and confirmed with Enter (see spec.md's "double-checked" note on why
this can't fire early).

### Key decisions

- **A name-keyed map, not a struct field on `promptCommand`.** Only 4 of
  the table's names need this treatment, and they're identified by their
  *value* (the literal alias string), not by which `promptCommand` they
  belong to — a lookup set is simpler than threading a new field through
  every table entry (most of which wouldn't use it).
- **Not auto-derived from `onGlobalKey`'s switch.** Extracting the actual
  hotkey runes from the switch statement's AST (or restructuring
  `onGlobalKey` to loop over a shared table) would remove the
  duplication risk entirely, but it's a bigger, separate refactor of
  `onGlobalKey` itself — out of scope for a suggestion-list bugfix. The
  cross-referencing comment (with an explicit line range) is the
  lighter-weight mitigation, matching how `promptCommandTable`'s own
  existing doc comment already cross-references
  `spec-wip/90-fe-command-autocomplete/plan.md` by pointer rather than
  by code sharing.
- **`aq`/`ap` untouched** — not in the set, so they keep suggesting, per
  spec.md.

## Testing

- `tui/internal/ui/style_test.go`: table-driven `TestBlendColors` cases —
  `t=0` returns `a` unchanged, `t=1` returns `b` unchanged, a midpoint
  case with hand-computed expected RGB, and an invalid-color case (e.g.
  `tcell.ColorDefault` as `b`) returning `a` unchanged. Existing
  `TestStyleInputFieldAutocompleteReturnsField` gets an `Accent` field
  added to its fixture palette so it exercises the real code path (it
  still can't assert colors — `tview` exposes no getter — so it stays a
  non-nil/no-panic check as before).
- `tui/internal/app`: table-driven test(s) for `promptSuggestions`
  asserting: typing `"q"`/`"h"`/`"s"`/`"l"` exactly does not include that
  bare letter in the result; typing a longer prefix (`"qu"`, `"ho"`,
  `"se"`, `"lo"`) still suggests `"quit"`/`"home"`/`"settings"`/`"log"`;
  typing `"a"` still suggests `"aq"` and `"ap"`. Placed alongside existing
  prompt tests in `app_test.go` (no existing `promptcommands_test.go` to
  extend, and the table this exercises is small enough not to warrant a
  new file) unless review prefers a dedicated file — decided in the task
  breakdown.
- Manual verification as already specified in `spec.md`'s "Manual
  verification" section (visual check across themes for Problem 1;
  `:q`+Enter etc. still executing for Problem 2). No broker/queue
  interaction, so the `verify-live` skill doesn't apply here.

## Trade-offs / risks accepted

- The 0.15 blend factor is a judgment call, not derived from a contrast
  formula (e.g. WCAG-style luminance ratio) — acceptable for a terminal
  UI improvement over the status quo (identical-to-background), and
  simpler to reason about than adding a contrast-ratio solver for three
  themes.
- The `globalHotkeyAliases` set can drift from `onGlobalKey` if someone
  changes hotkeys without reading the cross-referencing comment — accepted
  given the alternative (a shared-table refactor of `onGlobalKey`) is out
  of scope for this bugfix.
