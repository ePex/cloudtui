# Tasks — Global hotkeys

Plan: [2026-07-10_feat-global-hotkeys-plan.md](2026-07-10_feat-global-hotkeys-plan.md)

Each task below needs explicit manual approval before it is implemented.

1. **`tui/internal/ui/filterable.go`** — `Filterable` interface
   (`Filter(query string)`). No test file (bare interface, no logic —
   exercised indirectly in task 7).
   Status: done.

2. **`tui/internal/ui/views/home.go` + `settings.go`**, and update
   **`views_test.go`** — new placeholder views `NewHome()`/
   `NewSettings()`, added to the table-driven constructor test.
   Status: done.

3. **`tui/internal/app/topbar.go` + `topbar_test.go`** — `newTopBar`
   gains a `filterInput *tview.InputField` parameter and adds the
   `"filter"` page to `topLeft`; update existing test call sites and add
   an assertion that the `"filter"` page exists.
   Status: done.

4. **`tui/internal/app/help.go` + `help_test.go`** — `newHelpModal(cfg)`
   (bordered keybinding list) and `centered(p, width, height)` (modal
   centering helper).
   Status: done.

5. **`tui/internal/app/app.go`** — `App` gains `rootPages`,
   `filterInput`, `helpVisible`; `New()` wires the 5-view slice
   (`home` first), `rootPages` (`"main"`/`"help"` via `ShowPage`/
   `HidePage`), and `filterInput`; `onGlobalKey` restructured for
   `h`/`s`/`q`/`?`/`/`; new `openHelp`/`closeHelp`/`beginFilter`/
   `onFilterDone`/`activeView`. No change to `onPromptDone`'s existing
   Enter-key routing.
   Status: done.

6. **`tui/internal/app/app_test.go`** — update existing tests for the
   new default view (`"home"`, not `"secrets"`) and 5-view count; add
   tests for `h`/`s`/`q` routing, `?` open/close (and that it swallows
   other keys while open), and `/` both as a no-op (on a non-`Filterable`
   view) and via a fake `Filterable` view appended to `a.views`/`a.pages`
   in the test.
   Status: done.

7. **Verify** — `gofmt -l .`, `go vet ./...`, `go test ./...` clean/
   passing; `go build ./cmd/tui` succeeds; best-effort run check (same
   caveat as the previous feature — no tty/tmux equivalent here for
   interactive confirmation).
   Status: done for the automatable parts — all clean/passing/succeeding.
   Running for 5s produced no error output (consistent with successful
   screen init), but interactive/visual confirmation of h/s/q/?// still
   isn't possible from this sandboxed shell. Recommend running
   `task run:tui` yourself to try the new hotkeys.
