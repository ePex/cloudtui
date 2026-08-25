# Tasks

1. [x] Add `a.prompt.SetBackgroundColor(bg)` to `reapplyTheme`
   (`tui/internal/app/theme.go`), plus `TestReapplyThemeUpdatesPromptBackground`
   in `tui/internal/app/theme_test.go`.

   **Corrected during implementation**: the plan above turned out to be
   ineffective — `SetBackgroundColor` only updates `InputField`'s outer
   `*Box`, which `TextArea.Draw()` immediately overwrites from its own,
   separate, never-updated private `*Box`. Replaced with
   `a.prompt.SetFormAttributes(0, tcell.GetColor(p.Value), bg,
   tcell.GetColor(p.Text), tcell.ColorDefault)`, the only exported method
   that reaches the private `TextArea`'s actual background — see
   `spec.md`'s "Root cause"/"Fix" and `plan.md`'s "Fix"/"Testing" sections
   (both updated in place, superseded text kept visible) for the full
   trace. Test replaced with
   `TestReapplyThemeUpdatesPromptRenderedBackground`, which renders to a
   `tcell.SimulationScreen` and reads the cell back — the only way to
   actually catch this, since the private Box isn't reachable from a
   `GetBackgroundColor()` assertion.
2. [x] Manual verification: build and run the TUI, switch themes at
   runtime (e.g. `:theme cyberpunk` then `:theme dark`) and confirm the
   `:` prompt's background matches the rest of the shell each time
   (record what was checked here).

   Checked by driving the built binary in tmux, from an isolated scratch
   directory (not the repo's own `tui/config.yaml`, which holds real
   local connection credentials — testing was moved there after
   accidentally running earlier checks against it):
   - First switch in a fresh process (dark → cyberpunk): label rendered
     fg `rgb(0,212,255)` / bg `rgb(13,2,33)`, matching cyberpunk's
     `Value`/`Background` exactly.
   - Second switch in the *same* process (cyberpunk → dark): label
     rendered fg `rgb(125,207,255)` / bg `rgb(26,27,38)`, matching dark's
     `Value`/`Background`. This second-switch case is the one that
     exposed the bug during diagnosis: the private `TextArea`'s Box
     freezes at whatever theme was active when the app process started
     (read from `config.yaml` at startup), so a switch *into* that same
     already-frozen theme looks correct by coincidence — only a switch to
     a *different* theme within the same running process actually proves
     the fix works. The first diagnostic run happened to reuse a
     `config.yaml` already left on `cyberpunk` by earlier testing, so its
     first "switch to cyberpunk" wasn't a real change and masked the bug;
     a clean config plus a same-process second switch is what actually
     surfaces it. Checking both cases here matters for that reason.
3. [ ] Merge-back: update `spec/01-repo-and-tui-shell/spec.md`'s "Command
   prompt autocomplete" section to note the prompt's own background is
   recolored on a live theme switch; delete
   `spec-wip/bugfix-prompt-theme-switch-background/`.
