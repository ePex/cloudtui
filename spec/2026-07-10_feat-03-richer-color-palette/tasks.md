# Tasks — Richer color palette

Plan: [plan.md](plan.md)

Each task below needs explicit manual approval before it is implemented.

1. [x] **`tui/internal/config/config.go`** — `Palette` gains `Success`/
   `Warning`/`Error` and `Views map[string]string`, plus the
   `ViewColor(name string) string` fallback helper; `Default()` updated
   with the three status colors and a five-entry `Views` map.
   Status: done.

2. [x] **`tui/internal/config/config_test.go`** — update `TestDefault`'s
   expected value; add tests for `ViewColor` (mapped + fallback) and for
   partial-override merge behavior on both a new scalar field (e.g.
   `warning`) and a partial `colors.views` map — confirming (not
   assuming) that unmentioned `Views` entries survive.
   Status: done — confirmed yaml.v3 reuses/merges into the pre-populated
   map as hoped (no manual merge fallback needed in `Load`).

3. [x] **`tui/config.example.yaml`** — document `success`/`warning`/`error`
   and the `views:` map.
   Status: done.

4. [x] **`tui/internal/app/app.go`** — add the `bordered` interface
   (`SetBorderColor`/`SetTitleColor`) and apply it in the
   view-registration loop in `New()`, coloring each view's border/title
   via `cfg.Colors.ViewColor(v.Name())`.
   Status: done.

5. [x] **`tui/internal/app/app_test.go`** — assert a default-mapped view
   (e.g. `"secrets"`) renders with its configured `GetBorderColor()`,
   and a view not present in `Views` (a fake view, mirroring the
   existing `fakeFilterableView` pattern) falls back to `Border`'s
   color.
   Status: done. (Required adding a `cfg config.Config` field to `App`
   and extracting a `colorBordered` helper from `New()`'s loop, so the
   fallback case — a view appended after construction — could be colored
   the same way without duplicating the logic in the test.)

6. [x] **Verify** — `gofmt -l .`, `go vet ./...`, `go test ./...` clean/
   passing; `go build ./cmd/tui` succeeds; best-effort run check (same
   tty/tmux caveat as before).
   Status: done for the automatable parts — all clean/passing/succeeding.
   5s run produced no error output. Interactive/visual confirmation still
   not possible from this sandboxed shell; recommend `task run:tui`.
