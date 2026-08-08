# Spec — Bugfix 30: global hotkeys disappear from the home screen

Date: 2026-08-08

## Problem

The global hotkey legend (`?: Help  h: Home  l: Log  s: Settings  q: Quit
:: Command`) only ever lived in the bottom status bar. `ui.Shortcuttable`'s
own doc comment stated the design assumption plainly: "the status bar
already carries the global hotkey legend, so there is nothing to repeat."
That assumption doesn't hold — any transient status message (a validation
error, a delete confirmation, "Marked N message(s)") overwrites the status
bar with no automatic revert, so the legend can vanish for the rest of the
session, on every screen, including Home, which has nothing of its own in
the top bar's context panel to fall back on.

## Fix

Home now implements `ui.Shortcuttable`, returning the same six global
hotkeys. `updateContextPanel` (unchanged — this is exactly the mechanism it
already existed for) renders them in Home's own context panel, which is
view-driven and immune to whatever the status bar happens to be showing.

The bottom status bar is untouched: still shows the legend at idle, still
gets overwritten by transient messages on every screen. That's an accepted
tradeoff for a status bar, not something to fix — only Home lacked any
alternative place to see the legend when the status bar wasn't showing it.

## Scope

### In scope

- `HomeView.Shortcuts()` in `tui/internal/ui/views/home.go`.
- `ui.Shortcuttable`'s doc comment, which stated the now-corrected
  assumption.

### Out of scope

- Changing the status bar's behavior (auto-revert, etc.).
- Adding shortcuts to any other already-Shortcuttable view — this is
  specifically about Home having nothing at all.

## Files touched

| File | Change |
|---|---|
| `tui/internal/ui/views/home.go` | `Shortcuts()` implementation |
| `tui/internal/ui/shortcuttable.go` | doc comment update |
| `tui/internal/ui/views/views_test.go` | tests |
| `tui/internal/app/app_test.go` | updated the non-Shortcuttable example (settings, not home) + new test for Home's panel content |

## Definition of done

1. `go build ./...`, `go vet ./...`, `go test ./...` pass in `tui/`.
2. Verified live: Home's context panel shows the global hotkeys on
   launch; after triggering a transient status-bar message elsewhere and
   returning to Home, the status bar stays stuck on that stale message
   (expected, unchanged) while Home's context panel still correctly shows
   the hotkey legend — demonstrating the actual bug scenario, not just
   the happy path.
