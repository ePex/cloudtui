# 2026-07-10 — k9s-style shell layout

## Feature

Rework the tui shell (`internal/app`) from its current bare header/prompt/
pages stack into a three-row, k9s-style layout, with the ASCII logo and
color palette driven by a YAML config file instead of hardcoded.

## What

**Row 1 — top bar (fixed height), split into two panels:**

- **Left — connection info.** Shows the active AWS profile and the AMQ
  connection target. Placeholder values this pass (see Out of scope).
  While the user is typing a command (`:` pressed), this panel is
  temporarily replaced by the command prompt input field, k9s-style, and
  reverts to the info panel on Enter or Escape. The prompt no longer has
  its own permanent row.
- **Right — shortcuts + logo.** A static list of the app's current key
  bindings (`:` command, `q`/`quit`, `esc` cancel) plus the ASCII logo
  rendered from config.

**Row 2 — main (flexible height):** the existing `Pages` area
(queues/secrets/params views). Unchanged in this pass beyond becoming the
middle row instead of the whole body.

**Row 3 — bottom bar (fixed, minimal height):** reserved for
minimal/transient status — loading indicators, progress bars — once such
operations exist. This pass only reserves the row and renders a static
placeholder line, since there are no async operations yet to report on.

**Customization:** a YAML config file defines the ASCII logo (multi-line)
and the color palette (accents/borders/highlights, mapped to tcell
colors). Built-in defaults ship so the app runs with zero config present.
A simple placeholder logo is created as that default.

## Why

CLAUDE.md's architecture section already calls for a "k9s-style" shell
(command prompt, resource views); the current scaffold is a plain header
and doesn't reflect that. The user wants the visual structure to match
k9s (top info/shortcuts/logo bar, main content, minimal bottom status
bar) and wants the logo/colors to be user-configurable rather than
hardcoded constants.

## Scope

- Rework `internal/app/app.go`'s layout into the 3-row structure above.
- New top-left connection-info panel (placeholder text).
- New top-right shortcuts+logo panel (static shortcut list; logo from
  config).
- New bottom status bar (structural placeholder only).
- Command prompt changes from a persistent row to overlaying the
  top-left panel while active; behavior (routing, `q`/`quit`, unknown
  command) is unchanged.
- A config package/loader for the YAML file (logo + palette), with
  built-in defaults, loaded once at startup.
- Default placeholder ASCII logo and default color palette as the
  built-in fallback.
- Unit tests for the new/changed logic (config loading incl. defaults,
  prompt overlay show/hide, any new routing), per CLAUDE.md's testing
  rule.

## Out of scope

- Real AWS profile detection or real AMQ connection status — both stay
  placeholder text this pass; wiring them to the AWS SDK default
  credential chain and the `QueueBackend`/mq-proxy is a separate later
  feature.
- Real loading bars/progress indicators tied to actual async work — no
  such work exists yet, so the bottom row is structural only.
- A k9s-style breadcrumb header above the main `Pages` area (e.g.
  `Clusterroles(all)[3] <view>`) — not requested.
- Any new behavior in the queues/secrets/params views themselves — they
  remain placeholders as before.
- Config hot-reloading — loaded once at startup only.
- Full k9s keybinding parity — the shortcuts panel documents only the
  bindings the app already has.
