# Spec — Bugfix 18: Settings theme dropdown not interactive

## What
Pressing `s` to open the Settings view showed the Theme dropdown but it
could not be opened or changed with keyboard input.

## Why
`switchTo` called `a.tv.SetFocus(a.pages)`, focusing the Pages container
rather than the Form inside it. tview does not automatically cascade
keyboard events from a Pages widget into its child Form, so the dropdown
never received Enter / arrow-key events.

## Fix
One-line change in `switchTo`: focus `v.Primitive()` (the view's own
primitive) instead of `a.pages`. For all existing views this is a no-op
in practice; for the settings Form it makes the dropdown interactive.

## Scope
- `tui/internal/app/app.go` — `switchTo` focus target

## Out of scope
- Theme persistence / config architecture (separate CR if desired)
- Adding more built-in themes
