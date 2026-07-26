# TUI shell: layout, navigation, and theming

Date: 2026-07-26

## What

Stand up the interactive TUI shell with its core layout, navigation model,
two built-in themes, and two initial views.

### Layout

A fixed three-row layout:

- **Top bar** — three sections: a connection-info panel on the left (placeholder
  for now; an AWS profile feature will fill it in later), a
  **view-specific shortcuts** panel in the middle (empty when the active view
  declares none), and an ASCII logo on the right. While the `:` command prompt
  is active the left section is replaced by the input field.
- **Content area** — the main paged view area.
- **Status bar** — a persistent global hotkey legend at the bottom
  (`?: Help  h: Home  s: Settings  q: Quit  :: Command`),
  temporarily replaced by transient loading/error text.

### Navigation

Two complementary mechanisms:

- **Single-key hotkeys** (active when no prompt is focused):
  `h` → home, `s` → settings, `q` → quit, `?` → help modal.
- **`:command` prompt**: `:h`/`:home`, `:s`/`:settings`,
  `:q`/`:quit`, `:theme <name>`.

### Views

- **Home** — a dashboard listing all available views with their name and a short
  description. The top bar and status bar already carry the logo and shortcut
  hints, so the dashboard is pure content: what's here and what each view is for.
- **Settings** — an editable representation of the active `config.yaml`. Rows
  map to config fields; selecting a row lets the user change its value inline.
  Changes are persisted immediately to `config.yaml`. The theme row opens a
  picker (dark / cyberpunk); selecting a new theme applies it at runtime without
  restart.

### Theming

A `config.yaml` file (gitignored; `config.example.yaml` documents the schema)
controls the active palette. Two built-in themes ship as hardcoded defaults:

- **dark** — navy background, orange labels, cyan values, pink/magenta
  key-binding accents, teal list selection, orange status bar.
- **cyberpunk** — inspired by Cyberpunk 2077: near-black background, neon yellow
  (`#FFE400`) as primary accent, neon pink/magenta (`#FF003C`) as secondary,
  electric cyan (`#00D4FF`) for labels, light text, neon yellow selection
  highlight.

The app runs without any `config.yaml` present; built-in defaults apply.
Theme can be switched at runtime via the settings view or `:theme <name>`.

### Help modal

`?` opens a dismissable modal listing every key binding. Dismissed with `Esc`
or `?` again.

## Why

Every other feature builds on top of this shell. Establishing the layout,
navigation model, and theming system first means all subsequent views slot into
a consistent chrome without rework. Two themes at launch (one functional dark,
one expressive cyberpunk) demonstrate the config-driven palette system end to
end and give the app personality from day one.

## Scope

- `internal/ui/view.go` — `View` interface (`Name`, `Title`, `Primitive`).
- `internal/ui/shortcuttable.go` — `Shortcuttable` interface (`Shortcuts() []Shortcut`) for views that declare their own key bindings for display in the top bar's middle panel.
- `internal/app/` — three-row layout, top bar, status bar, global hotkey
  routing, help modal, `home` and `settings` views, runtime theme switching.
- `internal/config/` — `Config`/`Palette` schema, load/save/defaults,
  `config.example.yaml`.
- Unit tests: layout wiring, hotkey routing, help modal show/hide, config
  load/save/defaults, theme application, settings view edit/persist.
- Manual verification: run the app, switch themes at runtime, edit a setting,
  confirm persistence across restarts.

## Out of scope

- AWS profile selection — separate feature.
- Resource views (secrets, params, queues) — separate features.
- Filter functionality — view-specific; implemented per view as needed, not globally.
- Any cloud or network calls.
