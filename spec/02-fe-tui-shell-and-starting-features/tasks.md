# Tasks — TUI shell: layout, navigation, and theming

Plan: [plan.md](plan.md)

1. [x] **`internal/ui` interfaces** — `view.go` (`View`: `Name`, `Title`,
   `Primitive`).

2. [x] **`internal/config` package** — `Config`/`Palette` structs,
   `DarkPalette()`, `CyberpunkPalette()`, `Default()`, `Load`/`LoadDefault`,
   `Save`/`SaveDefault`; unit tests for defaults, partial-override merge,
   round-trip save/load, `ViewColor` fallback.

3. [x] **App skeleton** — `internal/app/app.go`: `App` struct, `New(cfg)`,
   `Run()`; root `tview.Flex` (rows) wiring the top bar, content `tview.Pages`,
   and status bar together; `switchTo` and `activeView` helpers; view
   registration loop; unit tests for view registration and `switchTo`.

4. [x] **Global theming** — `internal/app/theme.go`: `applyTheme(Palette)`
   mutating `tview.Styles`, `styleList(*tview.List, Palette)` for explicit
   selection wiring; called as first step of `App.New()`; unit tests confirming
   a freshly constructed `tview.Box` reflects the applied palette colors.

5. [x] **Top bar** — `internal/app/topbar.go`: left `tview.Pages` with
   `"info"` page (three-line connection-info placeholder), `"prompt"` page
   (`:` `InputField`); `│` divider; right
   shortcuts + logo panel; unit tests for panel content and divider presence.

6. [x] **Status bar** — `internal/app/statusbar.go`: single-line unbordered
   `TextView` showing the idle hotkey legend; unit tests for legend text and
   background color.

7. [x] **Global hotkey routing + `:command` prompt** — `onGlobalKey` with
   focus/modal priority layers and rune dispatch (`h`, `s`, `q`, `?`, `:`);
   `:command` `SetDoneFunc` parsing `:h`/`:home`, `:s`/`:settings`,
   `:q`/`:quit`, `:theme <name>`; unit tests for each dispatch case including
   prompt-focused pass-through and help-open swallow.

8. [x] **Help modal** — `internal/app/help.go`: bordered overlay listing all
   key bindings, shown/hidden via `rootPages` `ShowPage`/`HidePage`; unit
   tests for open/close state.

9. [x] **Home view** — `internal/ui/views/home.go`: stateless `tview.Table`
   (name | description columns) constructed with a `[]ViewInfo` slice; unit
   tests for `Name()`/`Title()`.

10. [x] **Settings view** — `internal/app/settings.go`: `tview.Form` with one
    row per config field; `Theme` field as `DropDown` (`"dark"` / `"cyberpunk"`);
    on any change, save config and call `reapplyTheme`; unit tests for
    save-on-change and theme field options.

11. [x] **Runtime theme switching** — `internal/app/theme.go`: `reapplyTheme`
    walks all live shell components and registered views, updates colors
    explicitly, calls `app.Draw()`; also invoked by `:theme <name>` command;
    unit tests confirming palette change is reflected after the call.

12. [x] **Wire `main.go`** — replace the stub with
    `app.New(config.LoadDefault()).Run()`; confirm `task build:tui` and
    `task run:tui` succeed.

13. [x] **`config.example.yaml`** — document the full config schema with both
    built-in palettes as examples and comments for each field.

14. [x] **Manual verification** — run `task run:tui`; confirm layout renders
    correctly; switch themes via settings view and `:theme` command; edit a
    setting and confirm persistence across restarts; open/close help modal;
    navigate between home and settings.

15. [x] **Context panel + `Shortcuttable` interface** — replace the static
    global-shortcuts panel in the top bar with a view-specific shortcuts panel;
    add `internal/ui/shortcuttable.go` (`Shortcuttable` interface + `Shortcut`
    struct); `App.switchTo` clears the panel for views that don't implement
    `Shortcuttable` and renders shortcuts for those that do; remove
    `newShortcutsPanel`/`shortcutsPanelText`; update `reapplyTheme`; unit tests
    for panel clear and render paths.
