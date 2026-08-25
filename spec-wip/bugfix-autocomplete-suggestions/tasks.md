# Tasks

1. [x] Add `blendColors` helper and wire it into
   `StyleInputFieldAutocomplete` (`tui/internal/ui/style.go`), plus
   `TestBlendColors` and an `Accent`-bearing fixture update to
   `TestStyleInputFieldAutocompleteReturnsField` (`tui/internal/ui/style_test.go`).
2. [x] Add `globalHotkeyAliases` and filter it out of `promptSuggestions`
   (`tui/internal/app/promptcommands.go`), plus table-driven tests for the
   filtering behavior in `tui/internal/app/app_test.go`.
3. [x] Manual verification: build and run the TUI, check the `:` prompt's
   autocomplete panel background against at least two themes (`dark`,
   `cyberpunk`) for readability, and confirm `:q`, `:h`, `:s`, `:l` still
   execute via Enter while no longer appearing in the suggestion list
   (record what was checked here).

   Checked by driving the built binary in tmux:
   - `dark` theme: unselected row background rendered as `rgb(60,41,62)`
     (Background `#1a1b26` blended 15% toward Accent `#ff79c6`),
     distinctly different from the plain `#1a1b26` used everywhere else
     on screen; selected row kept `SelectionBg`/`SelectionText`
     (`rgb(42,195,222)` bg) unchanged.
   - `cyberpunk` theme (switched via `:theme cyberpunk`): unselected row
     background rendered as `rgb(49,35,29)` (Background `#0d0221` blended
     toward Accent `#ffe400`), clearly distinct from the `#0d0221` screen
     background; selected row kept the theme's `SelectionBg` (`#ffe400`).
   - Typing `q`/`h`/`s`/`l` alone into the prompt no longer suggests that
     bare letter (only the full name, e.g. `quit`, and any view name
     sharing the prefix, e.g. `queues`, are offered).
   - `:s`, `:h`, `:l` each typed in full and confirmed with two Enters
     (first accepts the highlighted suggestion into the field per
     existing tview behavior, second submits — unrelated to this fix)
     correctly switched to Settings/Home/Log respectively; `:q` correctly
     quit the app (tmux session exited).
4. [ ] Merge-back: update `spec/01-repo-and-tui-shell/spec.md`'s "Command
   prompt autocomplete" section to describe the panel background and the
   global-hotkey-alias suggestion filtering; delete
   `spec-wip/bugfix-autocomplete-suggestions/`.
