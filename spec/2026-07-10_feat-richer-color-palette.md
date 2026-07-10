# 2026-07-10 — Richer color palette

## Feature

Expand the config-driven color palette (`internal/config.Palette`) with
per-view accent colors and semantic status colors, and finally wire up
the `Border` color (defined since the shell-layout feature, never
applied anywhere).

## What

- **Per-view accent colors.** Each registered view (`home`, `secrets`,
  `params`, `queues`, `settings`) gets its own color, used for that
  view's border and title — k9s-style, so views are visually
  distinguishable at a glance. A view not listed in the config falls
  back to the existing shared `Border` color, so adding a new view later
  doesn't require a palette change.
- **`Border` gets applied.** Resource-view borders currently render in
  tview's default color, ignoring `Border`. This pass wires it up as the
  fallback described above.
- **Semantic status colors.** `Palette` gains `Success`/`Warning`/`Error`
  colors. Nothing currently renders success/warning/error state (the
  status bar is still a static placeholder), so these are schema-only
  additions this pass — the same forward-looking pattern `Border`
  originally followed.
- `config.example.yaml` documents all the new fields.

## Why

The user wants a visually richer, more distinguishable UI: different
resource views recognizable by color, and the palette schema
future-proofed with status tones so an upcoming status-bar/help feature
doesn't require yet another schema change later. This also closes the
gap flagged (but deliberately deferred) in the shell-layout feature's
plan, where `Border` was defined but nothing read it.

## Scope

- `internal/config`: `Palette` gains a per-view color map and
  `Success`/`Warning`/`Error` fields; `Default()` ships sensible values
  for all five current views plus the three status colors; the existing
  partial-override merge behavior extends naturally to the new fields.
- `config.example.yaml` updated to document the new schema.
- Resource views render their border/title in their configured per-view
  color, falling back to `Border` if unlisted.
- Unit tests: new config fields' defaults and merge behavior, per-view
  color lookup with fallback, and that each view's rendered border color
  matches what's configured.

## Out of scope

- Actually displaying success/warning/error anywhere — no feature shows
  that state yet; `Success`/`Warning`/`Error` are schema-only this pass.
- Selection/highlight colors for list rows — no real list/table views
  exist yet (still out of scope from the shell-layout feature).
- Any change to the top bar's existing `Label`/`Value`/`Accent` usage
  (connection-info panel, shortcuts panel) or the help modal's coloring.
- An in-app settings UI for editing colors — the `settings` view stays a
  placeholder.
