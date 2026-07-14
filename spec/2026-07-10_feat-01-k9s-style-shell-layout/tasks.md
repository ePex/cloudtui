# Tasks — k9s-style shell layout

Plan: [plan.md](plan.md)

Each task below needs explicit manual approval before it is implemented.

1. [x] **`.gitignore`** — add `tui/config.yaml` under "Local configuration &
   secrets".
   Status: done.

2. [x] **Add `gopkg.in/yaml.v3` dependency** — `go get`, updating
   `tui/go.mod`/`go.sum`.
   Status: done (v3.0.1; marked `// indirect` until task 3 imports it).

3. [x] **`tui/internal/config/config.go` + `config_test.go`** — `Config`,
   `Palette`, `Default()`, `Load(path)`, `LoadDefault()`, with the
   documented default logo/palette and partial-override merge behavior.
   Status: done.

4. [x] **`tui/config.example.yaml`** — committed schema template (not
   auto-loaded).
   Status: done.

5. [x] **`tui/internal/app/topbar.go` + `topbar_test.go`** — connection-info
   panel (placeholder `Profile:`/`Queue Broker:` lines), shortcuts+logo
   panel, and the `topLeft` `Pages` wrapper (`"info"`/`"prompt"`).
   Status: done.

6. [x] **`tui/internal/app/statusbar.go` + `statusbar_test.go`** — bottom
   status bar (single-line, unbordered, placeholder text).
   Status: done.

7. [x] **`tui/internal/app/app.go`** — restructure `App`/`New()` into the
   3-row layout (`topBar` / `pages` / `statusBar`), wire `config.LoadDefault()`,
   update `onGlobalKey`/`onPromptDone` to switch `topLeft` between
   `"info"`/`"prompt"`. No change to view-routing/quit/unknown-command
   logic itself.
   Status: done.

8. [x] **`tui/internal/app/app_test.go`** — update existing tests for the new
   structure; add assertions that `topLeft`'s front page is `"prompt"`
   while active and `"info"` afterward.
   Status: done.

9. [x] **Verify** — `gofmt -l .`, `go vet ./...`, `go test ./...` all
   clean/passing; visually confirm the layout via `task run:tui` (or the
   `run`/`verify` skill) since this is a UI change.
   Status: done for the automatable parts — `gofmt`/`vet`/`test` all
   clean, `go build ./cmd/tui` succeeds, and running it for 5s produced
   no error output (consistent with the screen initializing and entering
   its event loop). Interactive/visual confirmation not done: this
   sandboxed shell has no tty/tmux equivalent for a Windows console TUI
   (same limitation the original scaffold spec documented) — recommend
   running `task run:tui` in a real terminal to eyeball the layout.
