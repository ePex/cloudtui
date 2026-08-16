# Plan — CR 64: move chrome files into `internal/ui`

## Approach

One commit: move `topbar.go`, `statusbar.go`, `help.go`, `notify.go` (and
their existing test files) from `internal/app` (package `app`) into
`internal/ui` (package `ui`, already exists — just gains files), exporting
exactly the symbols other files reach across the new package boundary and
leaving everything else unexported.

### Exported surface (confirmed by grepping every call site first)

| Old (package `app`) | New (package `ui`) | Why exported |
|---|---|---|
| `type topBar struct{...}` | `type TopBar struct{...}` (all 7 fields exported: `Root`, `Left`, `Info`, `Divider`, `ContextPanel`, `Logo`, `Height`) | `app.go` reads every field after construction |
| `newTopBar(cfg, prompt)` | `NewTopBar(cfg, prompt)` | called from `app.go` |
| `newStatusBar(cfg)` | `NewStatusBar(cfg)` | called from `app.go` |
| `newHelpModal(cfg)` | `NewHelpModal(cfg)` | called from `app.go` |
| `helpModalWidth`, `helpModalHeight` | `HelpModalWidth`, `HelpModalHeight` | read from `app.go` |
| `centered(p, w, h)` | `Centered(p, w, h)` | called 10x from `app.go` (every overlay's sizing) |
| `desktopNotify(title, msg)` | `DesktopNotify(title, msg)` | assigned to `a.notify` in `app.go` |
| `infoPanelText(cfg)` | `InfoPanelText(cfg)` | called from `app.go`, `connections.go`, `awsprofiles.go`, `theme.go` |
| `shortcutPanelRows` | `ShortcutPanelRows` | referenced from `messages_test.go` (package `app`) |

**Stay unexported** (only ever referenced within `topbar.go`/`topbar_test.go`
themselves, confirmed via grep — no cross-file callers today):
`newDivider`, `newInfoPanel`, `newLogoPanel`, `logoWidth`, `maxInt`.

### Call sites to update (all in package `app`, unchanged files otherwise)

- `app.go`: `newTopBar`→`ui.NewTopBar`, `tb.left/info/divider/contextPanel/logo/height/root`→
  `tb.Left/Info/Divider/ContextPanel/Logo/Height/Root`, `newStatusBar`→
  `ui.NewStatusBar`, `desktopNotify`→`ui.DesktopNotify`, all 10
  `centered(...)`→`ui.Centered(...)`, `newHelpModal`/`helpModalWidth`/
  `helpModalHeight`→`ui.`-qualified, `infoPanelText(a.cfg)`→
  `ui.InfoPanelText(a.cfg)`. `app.go` already imports `internal/ui` (for
  `ui.View`/`ui.Shortcut`), so no new import there.
- `connections.go`, `awsprofiles.go`, `theme.go`: `infoPanelText(a.cfg)`→
  `ui.InfoPanelText(a.cfg)`, plus adding the `internal/ui` import (none of
  the three currently import it — `theme.go` imports `internal/ui/views`
  but not `internal/ui` itself).
- `messages_test.go`: `shortcutPanelRows`→`ui.ShortcutPanelRows`, plus
  adding the `internal/ui` import.

### Test files

`topbar_test.go` (156 lines), `statusbar_test.go` (21 lines), `help_test.go`
(32 lines) move alongside their source files into package `ui`, same-package
convention preserved. No `notify_test.go` exists today — `desktopNotify` has
no direct unit test (OS-level side effect; `codepipelinewatch_test.go`'s
`fakeNotifier` tests the injection point, not the function itself) — nothing
to move there, matching `tui/CLAUDE.md`'s "genuinely untestable, say so"
guidance already implicitly followed pre-move.

## Files touched

- New: `internal/ui/topbar.go`, `internal/ui/statusbar.go`,
  `internal/ui/help.go`, `internal/ui/notify.go`, `internal/ui/topbar_test.go`,
  `internal/ui/statusbar_test.go`, `internal/ui/help_test.go`
- Deleted: the six corresponding `internal/app/*.go` originals (four source
  + three test)
- Modified: `internal/app/app.go`, `internal/app/connections.go`,
  `internal/app/awsprofiles.go`, `internal/app/theme.go`,
  `internal/app/messages_test.go`

## Key decisions

- **Export only what's actually read across the boundary**, confirmed by
  grep rather than guessing — keeps `internal/ui`'s new surface minimal,
  matching how the existing `View`/`Shortcuttable` interfaces are the only
  other things it exports today.
- **No behavior change, no new tests** — pure move + rename. Existing
  moved tests are the verification.
- **`goimports` isn't installed in this environment** (checked in CR 63) —
  imports added/removed by hand, then `gofmt -l` + `go build` catch any
  mistake.
- **No new dependencies.**

## Definition of done

Unchanged from spec.md — `go build`/`go test` pass, the four chrome files
(+ 3 tests) live in `internal/ui`, `internal/app` is four files smaller,
no behavior change.
