# Repo foundations and TUI shell

_Condensed from spec/01-fe-repo-foundations, spec/02-fe-tui-shell-and-starting-features — see those folders for the incremental history._

## Purpose

Establish the repository layout, Go module structure, and the interactive TUI shell (layout, navigation, theming, and the first two views) that every later feature builds on top of.

## Repo layout

```
cloudtui/
├── tui/         # Go TUI application
├── mq-proxy/    # Kotlin/Spring ActiveMQ REST proxy service
├── spec/        # Condensed, current end-state specifications (this tree)
├── spec-wip/    # Active feature/bugfix/change-request work in progress
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

- **Top bar** — three sections separated by a one-character-wide vertical divider bar (`│`, colored with the palette's border color, repeated for the top bar's full height) between the left and middle sections: a connection-info panel on the left (shows active connection/AWS profile once those features exist), a **view-specific shortcuts** panel in the middle (populated by the active view's `Shortcuts()` if it implements `ui.Shortcuttable`; empty otherwise), and an ASCII logo on the right. While the `:` command prompt is active, the left section is replaced by the input field. The shortcuts panel renders **one shortcut per line** — `<key> description`, key in the accent color — never concatenated onto a single line; the top bar's height is fixed to fit the tallest view's shortcut list (see `ShortcutPanelRows`), not scrollable.
- **Content area** — the main paged view area, backed by a `tview.Pages` container. Switching views calls `SwitchToPage` (not `ShowPage`) so the previous page is fully hidden, not just covered in z-order.
- **Status bar** — a transient message strip at the bottom. It has no idle default text (see spec/05-home-navigation for why) — it shows loading/error/confirmation text and otherwise stays blank. The full global-hotkey reference lives in the `?` help modal and in Home's own context panel, not in the status bar.

## Navigation

Two complementary mechanisms:

- **Single-key hotkeys** (active whenever no prompt/filter/form field is focused): `h` → home, `s` → settings, `l` → log, `q` → quit, `?` → help modal, `:` → command prompt.
- **`:command` prompt**: `:h`/`:home`, `:s`/`:settings`, `:l`/`:log`, `:q`/`:quit`, `:theme <name>`, plus later features add their own shortcuts (`:aq`, `:ap`, ...), and the name of any registered view (`:queues`, `:settings`, `:log`, ...). The special commands and their aliases live in a single table (`promptCommandTable()` in `internal/app`) shared by both execution and autocomplete, so the two can't drift out of sync.

### Command prompt autocomplete

While typing in the `:` prompt, a drop-down filters to matching entries as you type (prefix match), built from every special command name, `theme ` (see below), and every registered view's name. Pressing `:` with an empty prompt shows the full list immediately. Once the typed text starts with `theme `, the drop-down switches to matching built-in theme names instead of commands. The drop-down is styled to match the active theme and recolors on a live theme switch.

This uses `tview.InputField`'s built-in `SetAutocompleteFunc`/`SetAutocompleteStyles`, so its interaction model follows tview's own conventions: `↑`/`↓` navigates and live-updates the field text, `Enter`/`Tab` accepts the highlighted entry into the field and closes the drop-down (without yet submitting), and `Esc` closes the drop-down (without yet canceling the prompt) — a *second* `Enter`/`Esc` press (now with no drop-down open) submits or cancels, same as before this drop-down existed.

When switching views, focus goes to `view.Primitive()` (the view's own root primitive), not the `Pages` container — tview does not cascade keyboard events from `Pages` into a child `Form`/`List`, so focusing the container leaves interactive widgets (dropdowns, forms) unable to receive Enter/arrow-key input.

## Views

- **Home** — a dashboard/launcher. See spec/05-home-navigation for its sectioned, keyboard-navigable structure.
- **Settings** — a `tview.List` with **secondary text disabled** (`ShowSecondaryText(false)`): each row is a single `Label: value` line only, never a description or hint line underneath. It has **exactly four rows, in this order** — Theme, AMQ Connection, AWS Profile, Datadog — and no others; `config.yaml` fields with no corresponding row (e.g. `logo`, the `colors:` overrides) are not surfaced in Settings at all, edited only by hand in `config.yaml` or, for theme colors, via the theme picker's underlying YAML file. Selecting a row opens that field's editor. Changes persist immediately to `config.yaml`. Every row's editor is a **centered modal dialog overlay** (`internal/dialog`, layered on `rootPages` via `ui.Centered` — see spec/03-architecture-and-package-layout), never a page pushed into the main content `Pages` — this is true of the Theme row (picker; applies at runtime without restart), the AMQ Connection row (spec/12), the AWS Profile row (spec/14), and the Datadog row (spec/18) alike. See spec/04-theming for the theme picker specifically.

## Theming

A `config.yaml` file (gitignored; `config.example.yaml` documents the schema) controls the active palette. Three built-in themes ship: **dark** (navy background, orange labels, cyan values, pink/magenta key-binding accents, teal list selection, orange status bar), **cyberpunk** (near-black background, neon yellow `#FFE400` primary accent, neon pink/magenta `#FF003C` secondary, electric cyan `#00D4FF` labels, neon yellow selection highlight), and **gofore** (Gofore brand palette: Deep Blue `#0F3D51` background, Gofore Orange `#F7673B` labels, Digital Blue `#00819D` borders/selection, Code Blue `#44C2DE` values, Salmon Byte `#FFA572` accent). The app runs with no `config.yaml` present at all — built-in defaults apply. See spec/04-theming for the full theme-loading mechanism (themes moved from hardcoded Go functions to embedded YAML files in a later revision).

## Help modal

`?` opens a dismissable modal listing every key binding, dismissed with `Esc` or `?` again.

## Implementation notes

Current locations (post package-split — see spec/03-architecture-and-package-layout for the full end-state layout):

- `tui/internal/ui/view.go` — `View` interface (`Name`, `Title`, `Primitive`).
- `tui/internal/ui/shortcuttable.go` — `Shortcuttable` interface (`Shortcuts() []Shortcut`).
- `tui/internal/app/app.go` — shell composition root: three-row layout, top bar, status bar, global hotkey routing, help modal, view registration/switching.
- `tui/internal/ui/topbar.go` — `NewTopBar`/`TopBar`: the left info/prompt `Pages`, the divider, the context panel, and the logo, laid out in a single `tview.Flex` row.
- `tui/internal/app/promptcommands.go` — the `:` prompt's special-command table and its autocomplete suggestion function.
- `tui/internal/ui/views/home.go` — the Home view (moved out of `internal/app` as part of the later package split; originally lived at `internal/app/home.go`).
- `tui/internal/config/` — `Config`/`Palette` schema, load/save/defaults, `config.example.yaml`.

## Notable gotchas worth preserving

- **`tview.Pages.SwitchToPage` vs `ShowPage`**: always use `SwitchToPage` when navigating between top-level views — `ShowPage` leaves prior pages visible underneath in z-order, which causes stray Esc/input handling on the wrong page (see spec/08-message-browser-and-detail's Esc-navigation note for a concrete case this caused).
- **Focus target on view switch**: focus `view.Primitive()`, never the `Pages` container itself — see spec/04-theming for the bug this caused with the settings dropdown.
- **`tview.InputField.SetText` does not refresh an active `SetAutocompleteFunc` drop-down** — only a live keystroke does (via the field's own `InputHandler`). Reopening the `:` prompt (`prompt.SetText("")`) must be followed by an explicit `prompt.Autocomplete()` call, or the drop-down shows whatever suggestions were current the last time a keystroke triggered a refresh — in practice, the stale set captured at `SetAutocompleteFunc`'s own wiring time in `New()`, before view registration.
- **`tview.InputField.SetAutocompleteFunc` must be called *after* `ui.StyleInputFieldAutocomplete`, not before**: `SetAutocompleteFunc` eagerly calls `Autocomplete()`, which lazily builds the drop-down's internal `tview.List` and bakes in whatever `autocompleteStyles` are set on the `InputField` at that exact moment — later `Autocomplete()` calls only refresh the list's entries, never its style. Styling first (as `New()` now does) ensures the very first drop-down uses the palette's colors instead of tview's own unthemed defaults, which render unselected items with foreground == background (invisible text) once `applyTheme` has repointed `tview.Styles` at the active palette. Same underlying gotcha as `tview.DropDown`'s `styleDropDown` requirement (see `spec/18-datadog-logs`), but order-of-calls rather than a missing call.
