# Tasks

1. [ ] Add `a.prompt.SetBackgroundColor(bg)` to `reapplyTheme`
   (`tui/internal/app/theme.go`), plus `TestReapplyThemeUpdatesPromptBackground`
   in `tui/internal/app/theme_test.go`.
2. [ ] Manual verification: build and run the TUI, switch themes at
   runtime (e.g. `:theme cyberpunk` then `:theme dark`) and confirm the
   `:` prompt's background matches the rest of the shell each time
   (record what was checked here).
3. [ ] Merge-back: update `spec/01-repo-and-tui-shell/spec.md`'s "Command
   prompt autocomplete" section to note the prompt's own background is
   recolored on a live theme switch; delete
   `spec-wip/bugfix-prompt-theme-switch-background/`.
