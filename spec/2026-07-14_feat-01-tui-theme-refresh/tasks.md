# Tasks — TUI theme refresh

Plan: [plan.md](plan.md)

Each task below needs explicit manual approval before it is implemented.
Philipp approved implementing all 14 in one pass rather than gating each
individually — statuses below reflect that.

1. [x] **`tui/internal/config/config.go`** — `Palette` gains `Background`,
   `Text`, `SelectionBg`, `SelectionText`, `StatusBarBg`, `StatusBarText`;
   `Default()` updated to the new hex values, including collapsing
   `Views`' five entries to `Border`'s value.
   Status: done.

2. [x] **`tui/internal/config/config_test.go`** — update `TestDefault` and
   `TestLoadFullOverride` to the new field set/values.
   Status: done.

3. [x] **`tui/config.example.yaml`** — document every new/changed field under
   `colors:`, matching `Default()`.
   Status: done.

4. [x] **`tui/internal/app/theme.go`** (new) — `applyTheme(p config.Palette)`
   (sets `tview.Styles.*`) and `styleList(l *tview.List, p config.Palette)
   *tview.List` (selection colors).
   Status: done.

5. [x] **`tui/internal/app/theme_test.go`** (new) — `applyTheme` followed by
   `tview.NewBox()` has `GetBorderColor()`/`GetBackgroundColor()` matching
   the palette passed in.
   Status: done.

6. [x] **`tui/internal/app/app.go`** — call `applyTheme(cfg.Colors)` right
   after `cfg` is resolved, before any primitive is constructed;
   `newStatusBar(cfg)` instead of the parameterless call; new
   `readyText()` method (`return readyStatusText(a.cfg)`).
   Status: done.

7. [x] **`tui/internal/app/topbar.go`** — one-column divider between the info
   panel and the nav panel; `Navigation:` heading + `<key>`-bracketed
   tokens in the shortcuts panel; `infoPanelText` relabeled to "Active
   connection" / "User" / "AWS Profile" (existing config fields, no
   schema change).
   Status: done.

8. [x] **`tui/internal/app/topbar_test.go`** — update
   `TestInfoPanelContainsPlaceholders` /
   `TestInfoPanelTextShowsConfiguredProfile` (now checking the third
   line) / `TestShortcutsPanelContainsBindings` for the new text; add a
   test asserting the divider column exists between `left` and the nav
   panel.
   Status: done.

9. [x] **`tui/internal/app/statusbar.go`** — `readyStatusText(cfg)` replaces
   the `statusReadyText` const; `newStatusBar(cfg)` sets the idle text,
   `StatusBarBg` background, and `StatusBarText` foreground.
   Status: done.

10. [x] **`tui/internal/app/statusbar_test.go`** — assert idle text matches
    `readyStatusText(config.Default())` and the bar's background color
    matches `cfg.Colors.StatusBarBg`.
    Status: done.

11. [x] **`tui/internal/app/queues.go`** — wrap `queuesList`/`messagesList`
    construction with `styleList`; recolor the detail-pane hint from
    hardcoded `green` to `a.cfg.Colors.Accent`; the 5
    `a.setStatus(statusReadyText)` call sites become
    `a.setStatus(a.readyText())`.
    Status: done.

12. [x] **`tui/internal/app/queues_test.go`** — update the 3 references to the
    removed `statusReadyText` constant to `a.readyText()`.
    Status: done.

13. [x] **`tui/internal/app/settings.go`** — wrap the settings list and the
    AWS-profile picker list construction with `styleList`.
    Status: done.

14. [x] **Verify** — `gofmt -l .`, `go vet ./...`, `go test ./...`,
    `go build ./cmd/tui`. Note: this sandbox has no Go toolchain and no
    network path to install one (`go.dev`/`ports.ubuntu.com` are both
    blocked), so this task is careful manual code review here, not an
    actual command run. Recommend Philipp runs `task test:tui` /
    `task build:tui` locally after task 13 lands; happy to fix anything
    that comes back.
    Status: done for the manual-review part — re-read every changed file
    for import/type/signature correctness (incl. against the tview
    v0.42.0 source for `List`/`Box`/`TextView`/`Styles` APIs, fetched
    directly since no local copy exists), and grepped for leftover
    references to removed identifiers (`statusReadyText`, hardcoded
    `[green]`, "Queue Broker"). Could not run `gofmt`/`go vet`/`go
    test`/`go build` — no Go toolchain here and no network path to
    install one. Please run `task test:tui` / `task build:tui` locally;
    I'll fix anything that comes back.
