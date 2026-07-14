# Tasks — TUI shell: starting behavior, layout, and features

Plan: [plan.md](plan.md)

Condensed from six originally separate task files; every item below was
already implemented before this condensation, so all are checked.

1. [x] **App skeleton** — Go module, `cmd/tui/main.go`, `ui.View`
   interface, shared `placeholder` type backing `secrets`/`params`/
   `queues`.
2. [x] **`internal/config` package** — `Config`/`Palette`, `Default`/
   `Load`/`LoadDefault`, `config.example.yaml`.
3. [x] **Three-row layout** — `topbar.go` (info panel + shortcuts/logo),
   `statusbar.go`, prompt-overlay behavior via `topLeft` pages.
4. [x] **Global hotkeys** — `h`/`s`/`q`/`?`/`/` routing in
   `onGlobalKey`; new `home` (default) and `settings` placeholder views.
5. [x] **Help modal** — `rootPages` overlay (`ShowPage`/`HidePage`),
   `helpVisible` tracking, key-swallowing while open.
6. [x] **`Filterable` contract + filter overlay** — third `topLeft`
   page, `filterInput`, `beginFilter`/`onFilterDone`, no-op on
   non-`Filterable` views.
7. [x] **`internal/awsprofile` package** — `List`/`ListFrom` against
   `~/.aws/config` and `~/.aws/credentials`, env var overrides.
8. [x] **`config.Save`/`SaveDefault`** — persistence for the AWS
   profile (and later config fields).
9. [x] **Settings view rebuilt as a real list** (moved into
   `internal/app`) — "AWS Profile" row, modal picker with
   pre-selection, persistence, top bar refresh.
10. [x] **Per-view border colors** — `Palette.Views`/`ViewColor`
    fallback, schema-only `Success`/`Warning`/`Error`, `bordered`
    interface wiring in the view-registration loop. (Later superseded
    by global theming below; the `Views`/`ViewColor` mechanism itself
    stayed.)
11. [x] **Global re-theme** — `Palette` gains `Background`/`Text`/
    `SelectionBg`/`SelectionText`/`StatusBarBg`/`StatusBarText`;
    `Default()` updated to the final hex values; `Views`' five entries
    collapsed to one shared color.
12. [x] **`theme.go`** — `applyTheme` (sets `tview.Styles.*`) and
    `styleList` (per-list selection colors), wired into `App.New()` and
    every `tview.List` construction site.
13. [x] **Top bar re-theme** — divider column, `Navigation:` heading,
    `<key>` token format, relabeled info panel (Active connection/
    User/AWS Profile).
14. [x] **Status bar re-theme** — `readyStatusText`, `StatusBarBg`/
    `StatusBarText` applied, idle text becomes the hotkey legend.
15. [x] **`config.example.yaml`** kept in sync across all of the above.
16. [x] **Verify** — manual code review (no local Go toolchain available
    in the sandbox); `gofmt`/`go vet`/`go test`/`go build` deferred to
    `task test:tui`/`task build:tui` run locally.
