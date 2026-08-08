# Spec — FE 27: `:aq` command prompt shortcut for the connection manager

Date: 2026-08-08

## Background

FE 22 added the connection manager overlay, reachable via Settings →
Connection → Enter. The `:` command prompt (FE 02) already lets you jump
to a named view from anywhere (`:settings`, `:home`, `:s`, `:h`, ...), but
had no equivalent for opening an overlay.

## Problem

Switching AMQ connections from deep in another view (e.g. mid-browse in
Messages) requires backing out to Settings first.

## Solution

`:aq` (and `:connections`) opens the connection manager overlay directly
from the command prompt, from any view.

## Scope

### In scope

- `onPromptDone` in `tui/internal/app/app.go`: new case matching `"aq"` or
  `"connections"` → `a.showConnectionManager()`.
- Fixed a focus bug this surfaced (see Design notes) so the overlay is
  actually interactive immediately, not just visually on top.

### Out of scope

- A short-form global hotkey (like `s` for Settings) — `:aq` is
  intentionally prompt-only, consistent with there being no bare-key
  shortcut for the connection manager today either (it's one level under
  Settings).
- Any other new command-prompt commands.

## Design notes

`onPromptDone`'s cleanup unconditionally ran `a.tv.SetFocus(a.pages)` after
every command, which was harmless for the existing commands (`:settings`,
`:home`, ...) because they all resolve to a page inside `a.pages`, and
`tview.Pages.Focus` delegates to whatever page is frontmost — so
re-focusing `a.pages` after `switchTo` is a same-effect no-op. But the
connection manager is a `rootPages` overlay, a sibling of `a.pages`, not a
page within it. `showConnectionManager()` correctly focuses
`connManagerList`, but the unconditional `SetFocus(a.pages)` right after
would silently steal focus back to whatever main view was already showing
underneath — the overlay would render on top and look interactive, but
keystrokes would go to the hidden view instead. Fixed by skipping that
reset when any overlay's visibility flag is set (mirrors the guard already
used in `onGlobalKey`). Caught live: `n` (new connection) had no visible
effect until this fix, because it was being swallowed by the log view
underneath rather than reaching the connection manager.

## Files touched

| File | Change |
|---|---|
| `tui/internal/app/app.go` | `onPromptDone`: `aq`/`connections` case; focus-reset guard |
| `tui/internal/app/app_test.go` | tests for the new command and cross-view behavior |

## Definition of done

1. `go build ./...`, `go vet ./...`, `go test ./...` pass in `tui/`.
2. Verified live: `:aq` from the Log view opens the connection manager
   with correct focus (confirmed by pressing `n` and seeing the editor
   open, not nothing).
