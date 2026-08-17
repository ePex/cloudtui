# Spec — CR 78: physical move of the 10 overlay types into `internal/dialog`

Date: 2026-08-17

## Background

This is the CR `spec/64`'s phase 3 has been building toward since CR
66: every prerequisite is now done — the overlays depend on `ui.Host`,
not `*App` (CR 67–69); their shared helpers/types live in `internal/ui`
(CR 71, CR 75); `app.go` reaches them only through `Primitive()`/
`Visible()` (CR 73); all 10 types/constructors/`Show` methods are
exported (CR 74); and all 4 test files with dedicated overlay coverage
build their subject through a `ui.Host` test double instead of a full
`*App` (CR 76/77). This CR does the move itself.

Re-audited the exact blast radius fresh before scoping (not reused
from `spec/70`, since several of the CRs since then touched these
exact files):

- **9 production files move as-is**: `confirm.go`, `movepicker.go`,
  `sendmessage.go`, `connections.go`, `messagefilter.go`,
  `timerangemodal.go`, `datadogsettings.go`, `themepicker.go`,
  `awsprofiles.go`. None of them declare or use any unexported
  `internal/app` symbol anymore — confirmed by grepping every
  package-level identifier each file declares against every other
  `internal/app` file (production and test) for a stray reference.
- **5 test files move alongside them**: `connections_test.go`,
  `datadogsettings_test.go`, `timerangemodal_test.go`,
  `awsprofiles_test.go`, and `hosttest_test.go` (the `testHost` type
  double these 4 depend on — CR 76/77 built it in `internal/app` for
  lack of anywhere else to put it at the time; this is where it always
  belonged).
- **Two small cross-package test dependencies need resolving**,
  found by checking every identifier the 5 moving test files
  reference against `internal/app`'s other test files:
  - `renderedScreenText` (defined in `queues_test.go`, staying) — used
    by `awsprofiles_test.go` (moving) *and* `logs_test.go`/
    `queues_test.go`/`ssmparams_test.go`/`secrets_test.go` (staying).
    A ~20-line, dependency-free (`tcell`/`tview`/`strings`/`testing`
    only) helper used by both sides of the split — same shape as CR
    71's `styleList`/`parseFilterDate`, except it's test-only code, so
    there's no non-test package to promote it to without inventing one
    for a single small function. Duplicating it into `internal/dialog`
    is the smaller footprint (see Solution).
  - `fakeQueueBackend` (defined in `queues_test.go`, staying) — only
    used as `hosttest_test.go`'s default `testHost.backend`. Confirmed
    (again) that zero tests in any of the 4 moving files call
    `host.Backend()` — `movePicker`/`sendMessageOverlay` are the only
    two overlays that touch `Backend()`, and neither has a dedicated
    test. Same resolution as `renderedScreenText`: duplicate a minimal
    version rather than invent a shared package for it.
- **External call sites need zero code changes beyond `app.go`**:
  re-checked `message_detail.go`, `queues.go`, `messages.go`,
  `logsearch.go`, `datadoglogs.go`, `settings.go`, and `host.go` for
  any reference to one of the 10 types *by name* (a variable
  declaration, a struct literal, a type assertion) — found none. Every
  external file only ever reaches an overlay through an existing
  `*App` field (`a.confirm.Show(...)`, `a.themePicker.Show()`, ...),
  never by naming `ConfirmDialog`/`ThemePicker`/etc. directly. Only
  `app.go` names these types (in its own field declarations and
  constructor calls), so it's the only file outside the moving set
  that needs an import + qualification.

## Solution

- New package `internal/dialog`, one file per moved type (paths
  unchanged, just relocated — `confirm.go` → `internal/dialog/confirm.go`,
  etc.), `package app` → `package dialog` in each.
- `renderedScreenText` and a minimal `fakeQueueBackend`-equivalent
  duplicated into `internal/dialog`'s test scope (a new
  `dialogtest_test.go`, alongside the relocated `hosttest_test.go`) —
  not promoted anywhere, not imported from `internal/app`. Two small,
  self-contained, already-written pieces of test code, copy-pasted
  once; not the start of a shared-test-helpers package.
- `app.go`: one new import
  (`"github.com/ePex/cloudtui/tui/internal/dialog"`), the 10 struct
  field types and 10 constructor calls qualified with `dialog.`
  (`*ConfirmDialog` → `*dialog.ConfirmDialog`, `NewConfirmDialog(a)` →
  `dialog.NewConfirmDialog(a)`, ...). Field *names* (`a.confirm`,
  `a.connManager`, ...) don't change — only the types they hold.
- No other file changes anywhere in `internal/app`.

## Scope

### In scope

- Move (`git mv`, preserving history) the 9 production files + 5 test
  files into `internal/dialog/`, `package app` → `package dialog`.
- New `internal/dialog/dialogtest_test.go`: duplicated
  `renderedScreenText` and a minimal fake `queue.Backend`.
- `app.go`: new import, 10 field-type qualifications, 10
  constructor-call qualifications.
- `gofmt`/`go vet`/`go build`/`go test` clean across `tui/`.
- Live verification via the `verify-live` skill: open every one of the
  10 overlays once (confirm dialog via a delete/purge action, move
  picker, send message, connection manager + editor, message filter,
  time range modal via log search or Datadog logs, Datadog settings,
  theme picker, AWS profiles) and confirm each still renders and
  closes correctly — this CR moves files and adds package
  qualification only, but a mis-typed import alias or a missed
  qualification could compile clean and still wire the wrong page
  name at runtime (`ShowPage`/`HidePage` string literals are
  unaffected by the move, but worth confirming live given the size of
  the diff).

### Out of scope

- `tui/CLAUDE.md`'s package-layout list — needs a new bullet for
  `internal/dialog/`; update as part of this CR's "spec sync" step
  (code change ⇒ spec/doc update), not a separate CR.
- Phase 4 (`internal/view` for resource views) and phase 5
  (`connectionsecrets.go`'s `secretBackend` relocation) from
  `spec/64`'s original roadmap — phase 3 closes with this CR; those
  are separate future CRs.
- Adding test coverage for the 5 overlays with none today
  (`confirmDialog`, `movePicker`, `sendMessageOverlay`,
  `messageFilter`, `themePicker`) — unrelated to this move.
- Any behavior change. Pure move + import qualification.

## Definition of done

1. `go build ./...` and `go test ./...` pass in `tui/`.
2. The 9 production files + 5 test files live in `internal/dialog`,
   package `dialog`; nothing overlay-related remains in `internal/app`
   except `app.go`'s field declarations/constructor calls and the
   unrelated `host.go`/external-caller files (unchanged, still calling
   through `*App` fields).
3. `gofmt -l` reports nothing; `go vet ./...` clean.
4. All 10 overlays live-verified opening/closing correctly (see In
   scope).
5. `tui/CLAUDE.md`'s package layout list updated with
   `internal/dialog/`.
6. No behavior change — pure move + import qualification.
