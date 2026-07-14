# 2026-07-10 — Global hotkeys

## Feature

Add k9s-style direct single-key global shortcuts on top of the existing
`:`-prefixed command prompt: `h` (home), `s` (settings), `q` (quit), `?`
(help overlay), and `/` (filter, scaffolded for future list views).

## What

- **New `home` view** — a placeholder (same pattern as the existing
  secrets/params/queues placeholders). Becomes the app's default view at
  startup, replacing `secrets`. Bound to `h`.
- **New `settings` view** — a placeholder view. Bound to `s`.
- **`q`** — quits the app directly when pressed outside the prompt, in
  addition to the existing `:q`/`:quit` prompt commands (both remain
  unchanged).
- **`?`** — opens a dismissable modal overlay listing all keybindings
  (`h`, `s`, `q`, `?`, `/`, plus the `:` command entry), drawn on top of
  whatever view is active. `?` again or `Escape` closes it.
- **`/`** — invokes filtering on the active view via a new `Filterable`
  contract. No current view (including the two new placeholders)
  implements real filtering, so `/` is a documented no-op until a real
  list/table view exists and implements the interface.
- All five hotkeys only fire when the command prompt doesn't have focus
  — typing into the prompt (including a literal `/` or `?` as part of a
  command) is unaffected, same as the existing `:` handling.

## Why

The user wants direct single-key navigation/actions like k9s, instead of
only the `:` command prompt, plus a discoverable help overlay and a
forward-looking filter contract so real list/table views (when built)
have a contract to implement rather than bolting filtering on ad hoc.

## Scope

- `internal/ui/views`: `home` and `settings` placeholder views (reusing
  the existing `placeholder` type/constructor pattern).
- `internal/app`: register `home`/`settings`; `home` becomes the default
  view at startup; global key capture extended for `h`/`s`/`q`/`?`/`/`;
  a help modal component.
- A `Filterable` interface (exact shape decided at the plan stage) that
  a view can optionally implement; `/` checks for it on the active view
  and no-ops if absent.
- Unit tests for all new/changed logic: new views' `Name()`/`Title()`,
  key routing for all five hotkeys (including the prompt-focused no-op
  case), help modal show/hide, and the `/` no-op path.

## Out of scope

- Real filtering logic or data — no list/table views exist yet.
- `settings` view content beyond a placeholder (no config editing UI).
- `home` view content beyond a placeholder (no dashboard/aggregation
  view).
- Any change to the existing `:` command-prompt behavior — `:secrets`,
  `:params`, `:queues`, `:q`/`:quit` all continue to work exactly as
  before.
- Any change to the top bar / status bar layout from the previous
  feature.
