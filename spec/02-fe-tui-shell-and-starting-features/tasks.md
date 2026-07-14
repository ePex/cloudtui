# Tasks — TUI shell: starting behavior, layout, and features

Plan: [plan.md](plan.md)

Condensed from four originally separate task files; every item below was
already implemented before this condensation, so all are checked.

1. [x] **App skeleton** — Go module, `cmd/tui/main.go`, `ui.View`
   interface, shared `placeholder` type backing `secrets`/`params`/
   `queues`.
2. [x] **`internal/config` package** — `Config`/`Palette`, `Default`/
   `Load`/`LoadDefault`, `config.example.yaml`.
3. [x] **Three-row layout** — `topbar.go` (info panel + shortcuts/logo),
   `statusbar.go` (structural placeholder), prompt-overlay behavior via
   `topLeft` pages.
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
10. [x] **Verify** — `task build`/`test` clean; `gofmt`/`go vet` clean;
    best-effort headless run check.
