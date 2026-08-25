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

In `reapplyTheme` (`tui/internal/app/theme.go`), add one line near the
other shell-primitive recolors:

```go
// Command prompt's own background (the field itself renders
// transparently via SetFieldBackgroundColor(tcell.ColorDefault) in
// New(), so what's visible is this Box-level background).
a.prompt.SetBackgroundColor(bg)

// Command prompt's autocomplete drop-down
ui.StyleInputFieldAutocomplete(a.prompt, p)
```

(`bg` is already computed at the top of `reapplyTheme` — `bg :=
tcell.GetColor(p.Background)` — reused, not recomputed.)

## Testing

- `tui/internal/app/theme_test.go`: `TestReapplyThemeUpdatesPromptBackground`,
  mirroring `TestReapplyThemeUpdatesStatusBarColors` — construct an `App`
  with `config.Default()`, switch to the `cyberpunk` theme via
  `reapplyTheme`, and assert `a.prompt.GetBackgroundColor() ==
  tcell.GetColor(p.Background)`.
- Manual verification (build + tmux, as done to confirm the bug): switch
  themes at runtime (e.g. `:theme cyberpunk` then `:theme dark`) and
  visually confirm the prompt's background matches the rest of the shell
  each time, across at least two themes.

## Trade-offs / risks accepted

None beyond what's already noted in `spec.md` — this is a single missing
call, fixed by adding the call other primitives already use, with a test
that mirrors an existing one in the same file.
