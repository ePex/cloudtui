# Repo foundations and TUI shell

_Condensed from spec/01-fe-repo-foundations, spec/02-fe-tui-shell-and-starting-features — see those folders for the incremental history._

## Purpose

Establish the repository layout, Go module structure, and the interactive TUI shell (layout, navigation, theming, and the first two views) that every later feature builds on top of.

## Repo layout

```
cloudtui/
├── tui/         # Go TUI application
├── spec/        # Feature/bugfix/change-request specifications (incremental record)
├── spec-origin/ # Condensed, rebuild-from-scratch specifications (this tree)
├── Taskfile.yml # Cross-platform task runner (build, run, test)
└── CLAUDE.md    # Project instructions for agents and contributors
```

`tui/` follows standard Go module conventions:

```
tui/
├── cmd/tui/   # main package — entry point
├── internal/  # private application packages (not importable from outside)
├── go.mod
└── go.sum
```

`cmd/` holds the binary entry point only; all application logic lives under `internal/`.

## TUI shell layout

A fixed three-row layout:

- **Top bar** — three sections: a connection-info panel on the left (shows active connection/AWS profile once those features exist), a **view-specific shortcuts** panel in the middle (populated by the active view's `Shortcuts()` if it implements `ui.Shortcuttable`; empty otherwise), and an ASCII logo on the right. While the `:` command prompt is active, the left section is replaced by the input field.
- **Content area** — the main paged view area, backed by a `tview.Pages` container. Switching views calls `SwitchToPage` (not `ShowPage`) so the previous page is fully hidden, not just covered in z-order.
- **Status bar** — a transient message strip at the bottom. It has no idle default text (see spec-origin/05-home-navigation for why) — it shows loading/error/confirmation text and otherwise stays blank. The full global-hotkey reference lives in the `?` help modal and in Home's own context panel, not in the status bar.

## Navigation

Two complementary mechanisms:

- **Single-key hotkeys** (active whenever no prompt/filter/form field is focused): `h` → home, `s` → settings, `l` → log, `q` → quit, `?` → help modal, `:` → command prompt.
- **`:command` prompt**: `:h`/`:home`, `:s`/`:settings`, `:l`/`:log`, `:q`/`:quit`, `:theme <name>`, plus later features add their own shortcuts (`:aq`, `:ap`, ...).

When switching views, focus goes to `view.Primitive()` (the view's own root primitive), not the `Pages` container — tview does not cascade keyboard events from `Pages` into a child `Form`/`List`, so focusing the container leaves interactive widgets (dropdowns, forms) unable to receive Enter/arrow-key input.

## Views

- **Home** — a dashboard/launcher. See spec-origin/05-home-navigation for its sectioned, keyboard-navigable structure.
- **Settings** — an editable representation of the active `config.yaml`. Rows map to config fields; selecting a row lets the user change its value inline. Changes persist immediately to `config.yaml`. The theme row opens a picker; selecting a new theme applies it at runtime without restart. See spec-origin/04-theming.

## Theming

A `config.yaml` file (gitignored; `config.example.yaml` documents the schema) controls the active palette. Three built-in themes ship: **dark** (navy background, orange labels, cyan values, pink/magenta key-binding accents, teal list selection, orange status bar), **cyberpunk** (near-black background, neon yellow `#FFE400` primary accent, neon pink/magenta `#FF003C` secondary, electric cyan `#00D4FF` labels, neon yellow selection highlight), and **gofore** (Gofore brand palette: Deep Blue `#0F3D51` background, Gofore Orange `#F7673B` labels, Digital Blue `#00819D` borders/selection, Code Blue `#44C2DE` values, Salmon Byte `#FFA572` accent). The app runs with no `config.yaml` present at all — built-in defaults apply. See spec-origin/04-theming for the full theme-loading mechanism (themes moved from hardcoded Go functions to embedded YAML files in a later revision).

## Help modal

`?` opens a dismissable modal listing every key binding, dismissed with `Esc` or `?` again.

## Implementation notes

Current locations (post package-split — see spec-origin/03-architecture-and-package-layout for the full end-state layout):

- `tui/internal/ui/view.go` — `View` interface (`Name`, `Title`, `Primitive`).
- `tui/internal/ui/shortcuttable.go` — `Shortcuttable` interface (`Shortcuts() []Shortcut`).
- `tui/internal/app/app.go` — shell composition root: three-row layout, top bar, status bar, global hotkey routing, help modal, view registration/switching.
- `tui/internal/ui/views/home.go` — the Home view (moved out of `internal/app` as part of the later package split; originally lived at `internal/app/home.go`).
- `tui/internal/config/` — `Config`/`Palette` schema, load/save/defaults, `config.example.yaml`.

## Notable gotchas worth preserving

- **`tview.Pages.SwitchToPage` vs `ShowPage`**: always use `SwitchToPage` when navigating between top-level views — `ShowPage` leaves prior pages visible underneath in z-order, which causes stray Esc/input handling on the wrong page (see spec-origin/08-message-browser-and-detail's Esc-navigation note for a concrete case this caused).
- **Focus target on view switch**: focus `view.Primitive()`, never the `Pages` container itself — see spec-origin/04-theming for the bug this caused with the settings dropdown.
