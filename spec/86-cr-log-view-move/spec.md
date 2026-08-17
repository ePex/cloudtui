# Spec — CR 86: move `log.go` into `internal/view`

Date: 2026-08-17

## Background

Phase 4's original scope (`spec/64`) leaves 2 files after CR 85:
`log.go` and `codepipelinewatch.go`. This CR covers `log.go`.

Audited fresh (fields, call sites, test reach-ins — not reused from
CR 85's out-of-scope note, which only predicted the shape):

**1. `logView` already implements all 3 interfaces completely.** Unlike
`settingsView` (CR 85's thin facade), `logView` already has
`Name()`/`Title()`/`Primitive()`, a working `Shortcuts()`
(`ui.Shortcuttable`), and a working `ApplyPalette()` (`ui.Themeable`).
No interface gap to fill.

**2. The `app *App` field is entirely unused.**
`grep -n 'lv\.app\|\.app\b' internal/app/log.go` returns zero matches.
`logView`'s own logic (`Activate`, `load`, `colorizeLog`,
`logLevelColor`, `ApplyPalette`) never touches `lv.app` — it only reads
`~/.cloudtui/cloudtui.log` (or `lv.path` in tests) via `os.ReadFile`
and displays it. `logView` needs no host interaction at all.

**3. Zero raw-field reach-ins from any other file.**
`grep -rn 'logV\b|newLogView|newLogViewWithPath' internal/app/*.go`
(excluding `log.go`/`log_test.go` themselves) finds only 3 references,
all in `app.go`: the `logV *logView` field declaration, the
construction call `a.logV = newLogView(a)`, and membership in the
generic `a.views`/`a.themables` slices. No detail view or sibling file
reaches into `logView`'s internals the way CR 82's 5 detail views
reached into their siblings.

**4. Zero test-file reach-ins outside `log_test.go` itself**, confirmed
via `grep -rln 'logV\b|newLogView|newLogViewWithPath|\.path\b|\.load()|colorizeLog|logLevelColor' internal/app/*_test.go`
excluding `log_test.go` — empty result. The existing 6 tests already
construct via `newLogViewWithPath` and assert only through
`lv.textView`, well-encapsulated already.

This makes `log.go` structurally the simplest file left in phase 4 —
the same "adopt a host type + move" shape as CR 82/84's 9 dialog-free
views, except here the host dependency turns out to be dead weight
rather than something to swap for `ui.ViewHost`.

## Problem

None of this blocks a move mechanically, but carrying `app *App` (or
swapping it 1:1 for an equally-unused `host ui.ViewHost`) forward would
carry over dead code, which the root `CLAUDE.md` flags as a standing
rule ("No dead code. Delete unused code instead of commenting it out.")
— not a new rule invented for this CR. Since every line of the
struct/constructor is already being touched for the move, this is the
natural point to drop it rather than leave it for a later pass.

## Solution

1. **Drop the unused `app *App` field and constructor parameter
   entirely.** `newLogView()`/`newLogViewWithPath(path string)` no
   longer take an App/host argument at all — `logView` becomes a
   plain, dependency-free struct (`{ textView *tview.TextView; path
   string }`). `app.go`'s construction call becomes
   `a.logV = view.NewLogView()`.
2. **Move `log.go` + `log_test.go`** into `internal/view` via `git mv`;
   package line `app` → `view`; export the type and constructors:
   `logView` → `LogView`, `newLogView` → `NewLogView`,
   `newLogViewWithPath` → `NewLogViewWithPath`.
3. **`app.go`**: field `logV *logView` → `logV *view.LogView`;
   construction call and the `a.views`/`a.themables` slice entries
   updated to the new type/call.

## Scope

### In scope

- `log.go`: drop the unused `app`/host field and constructor
  parameter; export the type and both constructors; move to
  `internal/view`.
- `log_test.go`: move to `internal/view/log_test.go`; update the
  package line and the two `newLogViewWithPath(a, ...)` call sites to
  drop the now-removed first argument; `TestLogViewName`/
  `TestLogViewImplementsShortcuttable`/`TestLogViewShortcutsIncludeR`
  (which go through `a.logV` on a real `New(config.Default())`) move
  to `internal/app` as thin wiring tests instead, since they exercise
  `app.go`'s registration of the view, not `LogView` itself — matching
  how CR 84's `viewwiring_test.go` already separates "does app.go wire
  this view into pages/views/themables" from "does the view's own
  logic work" test-by-test.
- `app.go`: field type, construction call, and `a.views`/`a.themables`
  membership updated.
- `gofmt`/`go vet`/`go build`/`go test` clean; live verification.

### Out of scope

- `codepipelinewatch.go` — phase 4's other remaining file. It has no
  view type of its own to move (CR 79: "a problem for the future
  accessor-methods CR") — a differently-shaped audit than this file's
  mechanical case. Untouched here.
- Phase 5 (`connectionsecrets.go`'s `secretBackend`) — explicitly
  backlogged, unrelated.
- Any behavior change — `Activate`/`load`/`colorizeLog`/
  `logLevelColor`/`ApplyPalette`/`Shortcuts` all relocate byte-for-byte;
  the only thing removed is the already-dead `app` reference.

### Live verification

Lightweight — this view has no dialog/backend coupling to exercise.
Via `verify-live`: open Log (`l`), confirm the file's content loads
(or "No log file found." if absent) with level-based coloring, press
`r` to refresh, switch theme and confirm the Log view's border/title
recolor live via `ApplyPalette`.

## Definition of done

1. `internal/view/log.go` holds `LogView`, `NewLogView`,
   `NewLogViewWithPath` — no App/host parameter anywhere.
2. `internal/app` has no `logView`/`newLogView`/`newLogViewWithPath`
   symbols; `app.go`'s `a.logV` is `*view.LogView`.
3. `go build`/`go test`/`go vet` clean, `gofmt -l` clean, zero import
   cycle.
4. All 6 existing behavioral tests pass, relocated/adapted per Scope;
   the 3 wiring tests move to `internal/app`.
5. Live verification per above.
6. No behavior change beyond dropping the dead `app` field.
