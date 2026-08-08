# Spec — Bugfix 31: status bar duplicated the legend; Home listed itself

Date: 2026-08-08

## Background

Bugfix 30 gave Home its own context-panel copy of the global hotkey
legend, specifically because the bottom status bar's copy could get
silently overwritten by a transient message with nothing to restore it.
That fix was additive — it didn't touch the status bar's existing default
— which left two things wrong:

1. On Home specifically, the legend now showed in *both* places
   simultaneously (status bar idle text + the new context panel),
   which reads as confusing duplication rather than a fix.
2. Home's copy of the legend included `h → home` — a no-op reminder to do
   the thing you're already doing, since that entry is only ever visible
   while already on Home.

## Fix

- `newStatusBar` no longer sets any default text; the bar is blank at idle
  on every screen (not just Home) and shows transient messages exactly as
  before. `reapplyTheme` no longer resets it to a legend on theme switch
  either — it only recolors, leaving whatever text (blank or a transient
  message) already there alone.
- `HomeView.Shortcuts()` drops the `h`/`home` entry, keeping the other
  five (`?`/`l`/`s`/`q`/`:`).

This makes the status bar purely a transient-message strip everywhere,
full stop — no idle default to duplicate or go stale. The full hotkey
reference now lives in exactly one place per context: Home's panel for
the global set, `?` (help) for the complete list from anywhere, and each
other view's own panel for its view-specific bindings.

## Scope

### In scope

- `tui/internal/app/statusbar.go`: remove `readyStatusText`; blank
  default.
- `tui/internal/app/theme.go`: stop resetting status bar text on theme
  switch.
- `tui/internal/ui/views/home.go`: drop `h` from `Shortcuts()`.
- Tests for all of the above.

### Out of scope

- Anything about the `?` help modal (already lists all global hotkeys;
  unaffected).
- Other views' own `Shortcuttable` panels (unaffected — this is only
  about the now-removed global-legend default).

## Definition of done

1. `go build ./...`, `go vet ./...`, `go test ./...` pass in `tui/`.
2. Verified live: status bar is blank at idle on both Home and Log (not
   just Home); Home's context panel no longer lists `h`/`home`.
