# Tasks — Shell color palette evolution

Plan: [plan.md](plan.md)

Condensed from two originally separate task files; every item below was
already implemented before this condensation, so all are checked.

**Revision 1 — per-view colors + status schema (2026-07-10):**

1. [x] **`Palette`** gains `Views map[string]string`, `ViewColor`
   fallback helper, and schema-only `Success`/`Warning`/`Error`.
2. [x] **`internal/app`** — `bordered` interface + coloring applied in
   the view-registration loop.
3. [x] **`config.example.yaml`** documents the new fields.

**Revision 2 — global re-theme (2026-07-14):**

4. [x] **`Palette`** gains `Background`/`Text`/`SelectionBg`/
   `SelectionText`/`StatusBarBg`/`StatusBarText`; `Default()` updated to
   the new hex values; `Views`' five entries collapsed to one shared
   color.
5. [x] **`theme.go`** — `applyTheme` (sets `tview.Styles.*`) and
   `styleList` (per-list selection colors), wired into `App.New()` and
   every `tview.List` construction site.
6. [x] **Top bar** — divider column, `Navigation:` heading, `<key>`
   token format, relabeled info panel (Active connection/User/AWS
   Profile).
7. [x] **Status bar** — `readyStatusText`, `StatusBarBg`/`StatusBarText`
   applied, idle text becomes the hotkey legend.
8. [x] **`config.example.yaml`** updated to match.
9. [x] **Verify** — manual code review (no local Go toolchain available
   at revision-2 implementation time); `gofmt`/`go vet`/`go test`/
   `go build` deferred to `task test:tui`/`task build:tui` run locally.
