# Bugfix: `:` prompt keeps the old theme's background after a live theme switch

Date: 2026-08-25

## Problem

Switching the active theme at runtime (Settings → Theme, or `:theme
<name>`) recolors the app shell via `reapplyTheme`
(`tui/internal/app/theme.go`) — but the `:` command prompt's own visible
background is left on whatever theme was active when the app started.
Everything else around it (status bar, info/divider/context/logo panels,
Home table, every view/dialog's own list or table, and — separately — the
prompt's autocomplete drop-down items, which are rebuilt fresh from
current styles whenever the prompt regains focus) recolors correctly, so
the prompt reads as a visibly stale tile sitting in an otherwise
consistently-themed shell.

## Root cause

`a.prompt` (`tui/internal/app/app.go:167-169`) is constructed once, at
startup, with:

```go
a.prompt = tview.NewInputField().
    SetLabel(" :").
    SetFieldBackgroundColor(tcell.ColorDefault)
```

`SetFieldBackgroundColor(tcell.ColorDefault)` makes the field's own
editable area transparent, so what's actually visible is the surrounding
`tview.Box`'s background — a color tview captures once, at construction,
from whatever `tview.Styles.PrimitiveBackgroundColor` was at that moment,
and never re-reads afterward. `reapplyTheme` explicitly re-colors every
other shell primitive's background on a switch (see its calls to
`SetBackgroundColor` on the status bar, info panel, divider, context
panel, logo panel, and home table) but has no equivalent call for
`a.prompt` — it only reapplies the *autocomplete drop-down's* styles via
`ui.StyleInputFieldAutocomplete(a.prompt, p)`, which affects the popup
list, not the input field's own Box background.

This is a narrow gap, not a systemic pattern: every other `tview.InputField`
in the app (each view's filter input, dialog form fields, etc.) sets an
explicit, palette-derived field background color (typically
`p.SelectionBg`) both at construction and again in its own `ApplyPalette`
— `a.prompt` is the only input field using the "transparent, inherit the
Box's background" trick, and the only one missing a background line in
its theme-switch path. Confirmed by grepping every view/dialog's
`ApplyPalette` for background handling — no other gap found.

## Fix

Add `a.prompt.SetBackgroundColor(bg)` to `reapplyTheme`, alongside the
other shell primitives it already recolors on a switch.

## Scope

- In scope: `a.prompt`'s own background on a live theme switch.
- Out of scope: any other widget's theming (systematically checked, none
  found broken); the autocomplete drop-down's styling (already correct,
  per the bugfix-autocomplete-suggestions PR); a general "reload
  everything" mechanism — a targeted one-line fix following the existing
  `reapplyTheme` pattern is sufficient and consistent with how every
  other primitive here is already handled.

## Manual verification

`tview.Box` (which `InputField` embeds) exposes `GetBackgroundColor()`,
so this is unit-testable directly: assert `a.prompt.GetBackgroundColor()`
matches the new palette's `Background` after `switchTheme`. Manual
verification (build + tmux) additionally confirms the visible color
across at least two themes, since a passing unit test alone doesn't rule
out a rendering-level mismatch.
