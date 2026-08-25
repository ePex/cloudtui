# Plan

## Files touched

- `tui/internal/app/theme.go` — add the missing background recolor call
  to `reapplyTheme`.
- `tui/internal/app/theme_test.go` — one new test, following the existing
  `TestReapplyThemeUpdatesStatusBarColors` pattern already in this file.
- `spec/01-repo-and-tui-shell/spec.md` — merge-back: note that the prompt's
  own background is recolored on a live theme switch, alongside the
  existing "Command prompt autocomplete" section this touches.

No other files change. No new dependencies.

## Fix

**Superseded during implementation** — see `spec.md`'s "Root cause"
section for the full trace of why. The originally-planned
`a.prompt.SetBackgroundColor(bg)` compiles and passes a naive
`GetBackgroundColor()`-based test, but has zero visible effect:
`InputField` wraps a private `*TextArea` with its own separate embedded
`*Box`, and `TextArea.Draw()` repaints from *that* Box's background,
overwriting whatever the outer `SetBackgroundColor` painted.

Actual fix, in `reapplyTheme` (`tui/internal/app/theme.go`), replacing
the `SetBackgroundColor` line:

```go
a.prompt.SetFormAttributes(0, tcell.GetColor(p.Value), bg, tcell.GetColor(p.Text), tcell.ColorDefault)
```

`SetFormAttributes` is the only exported `InputField` method that
forwards to the private `TextArea`'s own `SetFormAttributes`, which is
what actually sets the background `TextArea.Draw()` paints from. It also
fixes the label's and typed-text's foreground colors, which share the
same "frozen at construction" root cause. See `spec.md` for why each of
the five arguments is what it is.

(`bg` is already computed at the top of `reapplyTheme` — `bg :=
tcell.GetColor(p.Background)` — reused, not recomputed.)

## Testing

**Superseded**: `GetBackgroundColor()`-based assertions can't catch this
bug (see above) — a render-based test is required instead.

- `tui/internal/app/theme_test.go`:
  `TestReapplyThemeUpdatesPromptRenderedBackground`, using
  `tcell.SimulationScreen` + `Draw` + `GetContents` (the same technique
  `TestPromptAutocompleteFirstOpenIsReadable` already uses in this file)
  — construct an `App`, switch to `cyberpunk` via `reapplyTheme`, draw
  `a.prompt` to a fresh simulation screen, and assert column 0's rendered
  foreground/background match the palette's `Value`/`Background`.
- Manual verification (build + tmux, as done to confirm the bug): switch
  themes at runtime **more than once in the same process**
  (`:theme cyberpunk` then `:theme dark`) and visually confirm the
  prompt's label color matches the rest of the shell each time — the bug
  was intermittent in exactly this way during diagnosis, so a single
  switch isn't enough to trust a manual check.

## Trade-offs / risks accepted

- The originally-approved plan's fix and test were both wrong (see
  above) — corrected during implementation rather than shipped as
  planned, since shipping the ineffective version would have closed the
  bug without fixing it.
- No exported way exists to reach the private `TextArea`'s background
  directly by name — `SetFormAttributes` is a `Form`-oriented API being
  reused for a non-`Form` field. This is a bit of an odd fit
  API-shape-wise, but it's the only public surface tview exposes for
  this, and the argument values are documented in `spec.md` so a future
  reader isn't left guessing why a `Form` method appears here.
