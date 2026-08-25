# Tasks

1. [ ] Add `blendColors` helper and wire it into
   `StyleInputFieldAutocomplete` (`tui/internal/ui/style.go`), plus
   `TestBlendColors` and an `Accent`-bearing fixture update to
   `TestStyleInputFieldAutocompleteReturnsField` (`tui/internal/ui/style_test.go`).
2. [ ] Add `globalHotkeyAliases` and filter it out of `promptSuggestions`
   (`tui/internal/app/promptcommands.go`), plus table-driven tests for the
   filtering behavior in `tui/internal/app/app_test.go`.
3. [ ] Manual verification: build and run the TUI, check the `:` prompt's
   autocomplete panel background against at least two themes (`dark`,
   `cyberpunk`) for readability, and confirm `:q`, `:h`, `:s`, `:l` still
   execute via Enter while no longer appearing in the suggestion list
   (record what was checked here).
4. [ ] Merge-back: update `spec/01-repo-and-tui-shell/spec.md`'s "Command
   prompt autocomplete" section to describe the panel background and the
   global-hotkey-alias suggestion filtering; delete
   `spec-wip/bugfix-autocomplete-suggestions/`.
