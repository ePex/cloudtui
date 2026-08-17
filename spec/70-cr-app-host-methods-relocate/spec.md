# Spec — CR 70: relocate `App`'s Host-implementing methods out of the moving overlay files

Date: 2026-08-17

## Background

CR 66–69 finished making all 10 modal overlays depend on `ui.Host`
instead of the concrete `*App`. The remaining step of phase 3
(`spec/64-cr-app-package-split/spec.md`) is physically moving those 10
types (`confirmDialog`, `movePicker`, `sendMessageOverlay`,
`messageFilter`, `timeRangeModal`, `datadogEditor`, `themePicker`,
`awsProfilesPicker`, `connManager`, `connEditor` — files `confirm.go`,
`movepicker.go`, `sendmessage.go`, `messagefilter.go`,
`timerangemodal.go`, `datadogsettings.go`, `connections.go`, plus the
`themePicker`/`awsProfilesPicker` sections of `settings.go`/
`awsprofiles.go`) into a new `internal/dialog` package.

Before scoping that move, a full audit was done of everything *outside*
these 10 types that touches them directly (not through `ui.Host`) —
grepped across the whole `internal/app` package, not just guessed. It
found a wider blast radius than expected:

- **10 constructor calls** in `app.go`, each also declaring the
  matching `App` struct field type.
- **`app.go`'s `New()`** reads `.flex`/`.form` directly (10 lines, for
  `ui.Centered(...)` sizing) and takes the address of the raw
  `.visible` field directly (10 lines, building `overlayVisible
  []*bool`) — neither goes through a method.
- **`.Show(...)`-equivalent calls from 5 view files** outside the 10:
  `queues.go` (confirm, movePicker, sendMessage — missed by the first
  automated pass, since it accesses them via `qv.app.X` rather than a
  local `a :=` alias; caught by re-grepping directly), `messages.go`
  (confirm, movePicker, sendMessage, messageFilter), `message_detail.go`
  (confirm, movePicker), `logsearch.go`/`datadoglogs.go`
  (timeRangeModal), `settings.go`'s `settingsView` part (themePicker,
  connManager, datadogEditor, awsProfiles), `app.go`'s `onPromptDone`
  (connManager, awsProfiles, via `:aq`/`:ap`). No external `.close()`
  calls exist anywhere — every overlay closes itself internally, so
  `close` doesn't need to be exported.
- **A structural problem, not just missing exports**: `SaveConnection`/
  `DeleteConnection` (in `connections.go`), `SaveDatadogConfig` (in
  `datadogsettings.go`), and `SetActiveAWSProfile` (in `awsprofiles.go`)
  are `(a *App)` methods — the `Host`-implementing side, added in CR
  66 — living in files that are about to move. They **can't** move
  with those files; `App` stays in package `app` forever. This isn't
  an export problem, it's a "these 4 methods are in the wrong file"
  problem that has to be fixed before the move, not during it.
- **`settings.go` is a split file**: `settingsView` (stays in `app`)
  and `themePicker` (moves) are both defined in it today.
- **Test files reach into unexported fields too**
  (`app_test.go`→connManager/awsProfilesPicker/datadogEditor,
  `messages_test.go`→confirmDialog/movePicker,
  `datadoglogs_test.go`/`logsearch_test.go`→timeRangeModal, including
  reading `.onApply` and calling it directly as a func) — these break
  too, once fields move/get renamed, on top of the moving types' own
  test files.

## Problem

This is too much to do safely in one CR. The 4 misplaced `App` methods
are the one genuinely blocking issue — everything else (exports,
`.flex`/`.form` access, `.visible` access, the `settings.go` split,
test fixups) can be sequenced as separate, individually-verifiable
CRs, but those 4 methods must move out of `connections.go`/
`datadogsettings.go`/`awsprofiles.go` *before* those files can move
anywhere, or the move itself becomes tangled with an unrelated fix.

## Solution

Relocate `SaveConnection`, `DeleteConnection`, `SaveDatadogConfig`, and
`SetActiveAWSProfile` verbatim into `host.go` — the file that already
holds every other `Host`-implementing `App` method (added across CR
66/67, all in one place already). Pure code motion: same method
bodies, same receiver, same package — only the file changes.

This is the smallest, safest, and only strictly *required* first step
toward the move. It doesn't touch any of the 10 overlay types
themselves, doesn't export anything, and doesn't move any overlay file
— just clears the one real blocker.

### Roadmap for the rest of the move (recorded here so the next CR
doesn't have to re-derive the audit above)

| Step | What |
|---|---|
| **This CR (70)** | Relocate the 4 misplaced `App` methods into `host.go` |
| 71 | Split `settings.go` into `settings.go` (`settingsView`, stays) + a new file for `themePicker` (moves) |
| 72 | Export all 10 types, their constructors, and `.Show(...)` (not `close` — nothing external calls it); design and apply a fix for `app.go`'s direct `.flex`/`.form` reads (likely each overlay gains a `Primitive() tview.Primitive` method, mirroring the existing `ui.View` convention) and direct `.visible` address-of (likely `overlayVisible []*bool` becomes a slice of a small `interface { Visible() bool }`, with each overlay gaining that method) — all still within package `app`, no move yet, so this is verifiable as a pure rename+redesign before the added risk of a package move |
| 73 | Physically move the 10 (now fully-exported) files + their tests into `internal/dialog`; update imports in `app.go` and the 5 view files; fix the test files that reach into fields directly |

## Scope

### In scope

- `host.go`: gains `SaveConnection`, `DeleteConnection`,
  `SaveDatadogConfig`, `SetActiveAWSProfile` (moved verbatim).
- `connections.go`: loses `SaveConnection`/`DeleteConnection`.
- `datadogsettings.go`: loses `SaveDatadogConfig`.
- `awsprofiles.go`: loses `SetActiveAWSProfile`.

### Out of scope

- Everything in the roadmap table above (steps 71–73) — future CRs.
- Any export, rename, or file move of the 10 overlay types themselves.
- Any behavior change. Pure code motion, four methods, one file each.

## Definition of done

1. `go build ./...` and `go test ./...` pass in `tui/`.
2. All four methods live in `host.go`; `connections.go`,
   `datadogsettings.go`, `awsprofiles.go` no longer define any
   `(a *App)` method.
3. No behavior change — this is pure file motion of already-tested
   logic (each method's behavior was already live-verified when it was
   introduced, in CR 66); `go test ./...` passing is sufficient
   verification, no live re-check needed.
