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
├── cmd/cloudtui/   # main package — entry point
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
- **`:command` prompt**: `:h`/`:home`, `:s`/`:settings`, `:l`/`:log`, `:q`/`:quit`, `:theme <name>`, plus later features add their own shortcuts (`:aq`, `:ap`, ...), and the name of any registered view (`:queues`, `:settings`, `:log`, ...). The special commands and their aliases live in a single table (`promptCommandTable()` in `internal/app`) shared by both execution and autocomplete, so the two can't drift out of sync. `h`/`s`/`l`/`q` (but not the two-letter aliases) are excluded from autocomplete *suggestions* specifically because they duplicate the single-key hotkeys above — see "Command prompt autocomplete" below.

### Command prompt autocomplete

While typing in the `:` prompt, a drop-down filters to matching entries as you type (prefix match), built from every special command name, `theme ` (see below), and every registered view's name — **except** the single-letter aliases (`h`, `s`, `l`, `q`) that duplicate a global hotkey: those stay valid to type-and-execute in the prompt, they just aren't offered as suggestions, since the global hotkey already covers that need and suggesting them back just clutters the list. Two-letter aliases with no global-hotkey equivalent (`aq`, `ap`) keep suggesting. Pressing `:` with an empty prompt shows the full list immediately (minus the excluded aliases). Once the typed text starts with `theme `, the drop-down switches to matching built-in theme names instead of commands. The drop-down is styled to match the active theme and recolors on a live theme switch; unselected rows use a background blended 15% from the theme's `background` toward its `accent` color (`ui.BlendColors`/`ui.AutocompletePanelBlend`) rather than a flat copy of the screen background, so the popup reads as a distinct panel (see the gotcha below on why a real drawn border isn't used instead).

This uses `tview.InputField`'s built-in `SetAutocompleteFunc`/`SetAutocompleteStyles`, so its interaction model follows tview's own conventions: `↑`/`↓` navigates and live-updates the field text, `Enter`/`Tab` accepts the highlighted entry into the field and closes the drop-down (without yet submitting), and `Esc` closes the drop-down (without yet canceling the prompt) — a *second* `Enter`/`Esc` press (now with no drop-down open) submits or cancels, same as before this drop-down existed.

The prompt's own label color, background, and typed-command text color also recolor on a live theme switch (`reapplyTheme` → `a.prompt.SetFormAttributes(...)`, in `internal/app/theme.go`) — see the gotcha below on why this specific call is required instead of the more obvious `InputField.SetBackgroundColor`.

When switching views, focus goes to `view.Primitive()` (the view's own root primitive), not the `Pages` container — tview does not cascade keyboard events from `Pages` into a child `Form`/`List`, so focusing the container leaves interactive widgets (dropdowns, forms) unable to receive Enter/arrow-key input.

## Views

- **Home** — a dashboard/launcher. See spec/05-home-navigation for its sectioned, keyboard-navigable structure.
- **Settings** — a `tview.List` with **secondary text disabled** (`ShowSecondaryText(false)`): each row is a single `Label: value` line only, never a description or hint line underneath. It has **exactly four rows, in this order** — Theme, AMQ Connection, AWS Profile, Datadog — and no others; `config.yaml` fields with no corresponding row (e.g. `logo`, the `colors:` overrides) are not surfaced in Settings at all, edited only by hand in `config.yaml` or, for theme colors, via the theme picker's underlying YAML file. Selecting a row opens that field's editor. Changes persist immediately on save — to `config.yaml` for Theme/AWS Profile/Datadog, and to `connections/jolokia.yaml`/`connections/proxy.yaml` for the AMQ Connection row's connection data (see spec/12-named-connections and, below, the settings/connections/favorites file split those separate locations come from). Every row's editor is a **centered modal dialog overlay** (`internal/dialog`, layered on `rootPages` via `ui.Centered` — see spec/03-architecture-and-package-layout), never a page pushed into the main content `Pages` — this is true of the Theme row (picker; applies at runtime without restart), the AMQ Connection row (spec/12), the AWS Profile row (spec/14), and the Datadog row (spec/18) alike. See spec/04-theming for the theme picker specifically.

## Theming

A `config.yaml` file at `~/.cloudtui/config.yaml` (`tui/config.example.yaml` documents the schema) controls the active palette. `config.LoadDefault`/`SaveDefault` resolve this path unconditionally — no current-working-directory fallback, matching `~/.cloudtui/cloudtui.log`'s (spec/06) precedent of a single fixed location, since the binary can be launched from anywhere once installed via any of spec/02's methods. A pre-relocation, cwd-relative `config.yaml` (e.g. a dev checkout's `tui/config.yaml` from before this location existed) is copied into place automatically the first time `~/.cloudtui/config.yaml` doesn't yet exist, so existing setups aren't silently discarded.

`config.yaml` holds appearance/settings only (theme, logo, colors, `activeConnection`, `activeAWSProfile`, `datadog`) — connections and AWS favorites live in their own sibling files under the same `~/.cloudtui/` directory, so either can be copied or shared independently of appearance settings and of each other (a favorites file has no secrets in it, just names, so it's a genuinely safe thing to hand to a teammate or commit somewhere on its own — see `tui/favorites.example.yaml`):
- `~/.cloudtui/connections/jolokia.yaml`, `~/.cloudtui/connections/proxy.yaml` — the `connections` list, split by backend type (`tui/connections/*.example.yaml` document the schema; see spec/12-named-connections).
- `~/.cloudtui/favorites.yaml` — the `awsFavorites` content (see spec/15/16/17 for the SSM Parameters/Secrets Manager/CloudWatch Logs views that read/write it).

Each file's top-level content *is* the section — no wrapper key inside the file, since the filename already scopes it. `Load`/`Save` still take a single settings-file path (unchanged signature); the sibling files are derived from that path's directory. A config written before this split (still combining everything into one file) loads correctly as a fallback — used only for whichever piece doesn't have its own split file yet — and gets rewritten into the current 3-file shape the next time anything saves; nothing is lost or requires manual action. Three built-in themes ship: **dark** (navy background, orange labels, cyan values, pink/magenta key-binding accents, teal list selection, orange status bar), **cyberpunk** (near-black background, neon yellow `#FFE400` primary accent, neon pink/magenta `#FF003C` secondary, electric cyan `#00D4FF` labels, neon yellow selection highlight), and **gofore** (Gofore brand palette: Deep Blue `#0F3D51` background, Gofore Orange `#F7673B` labels, Digital Blue `#00819D` borders/selection, Code Blue `#44C2DE` values, Salmon Byte `#FFA572` accent). The app runs with no `config.yaml` present at all — built-in defaults apply. See spec/04-theming for the full theme-loading mechanism (themes moved from hardcoded Go functions to embedded YAML files in a later revision).

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
- **`tview.InputField.SetBackgroundColor` doesn't actually recolor a labeled field on a live theme switch.** `InputField` wraps a *private* `*TextArea` (unexported field, inaccessible outside `tview`) which owns its **own, separate** embedded `*Box` — baked in at `TextArea` construction from whatever `tview.Styles.PrimitiveBackgroundColor` was at that moment, and never touched again. `InputField.Draw()` calls the outer `i.Box.DrawForSubclass` first (this is what `SetBackgroundColor` updates), then `i.textArea.Draw()`, whose first line is `t.Box.DrawForSubclass` — repainting the exact same cells from the inner, never-updated Box. So `SetBackgroundColor` compiles, and even passes a naive `GetBackgroundColor()`-based unit test, but has zero visible effect once the field has drawn even once. The only exported `InputField` method that reaches the private `TextArea`'s actual background is `SetFormAttributes(labelWidth, labelColor, bgColor, fieldTextColor, fieldBgColor)` — a `Form`-oriented API, reused here for a non-`Form` field because it's the only public surface that forwards to `TextArea.SetFormAttributes`, which is what `TextArea.Draw()` actually paints from. This bit the `:` prompt specifically because it's the app's only `tview.InputField` with a label (every other input field — filter inputs, dialog forms — has no label and colors its editable area directly via `SetFieldBackgroundColor`, which *does* work since it targets `TextArea.textStyle` rather than its Box). Verifying this class of bug requires rendering to a `tcell.SimulationScreen` and reading `GetContents()` back (see `TestPromptAutocompleteFirstOpenIsReadable`'s technique) — `GetBackgroundColor()` alone will pass right through the bug.
- **`tview.InputField`'s autocomplete popup can't be given a real drawn border.** `InputField` owns the popup internally as a private `*tview.List` with no exported accessor, and `Draw()` recomputes the popup's rect on every frame to exactly fit its entries (widest entry × entry count), with no padding reserved for a border — `SetBorder(true)` isn't reachable through the public API, and even if it were, the existing rect math would clip content rather than frame it. Getting an actual border would require vendoring/forking `tview` to patch that geometry. Instead, `ui.StyleInputFieldAutocomplete` blends the popup's background 15% toward the palette's `accent` color (`ui.BlendColors`) so unselected rows read as a distinct panel rather than reusing the plain screen background — see "Command prompt autocomplete" above. The same tight-fit rect rules out left/right *padding* too, and padding is a worse case than the border: the only lever is padding each suggestion *string* (since `lwidth` is derived straight from entry text width), but that same string is what tview inserts into the field on accept/navigate — stripping the padding back out requires `SetAutocompletedFunc`, which loses tview's built-in dodge (a private variable this package can't reach) around re-filtering the list on every text change. Concretely: arrow-navigating to an entry live-sets the field text to that entry's name, which `SetAutocompletedFunc` can't help re-triggering `Autocomplete()`, collapsing the list to whatever narrowly matches that longer text — breaking multi-item arrow-key browsing after the first keystroke. Decided not to pursue for this reason.
