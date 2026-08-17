# Spec — CR 68: swap 8 overlays' `app *App` field to `host ui.Host`

Date: 2026-08-17

## Background

Third slice of phase 3 of the `internal/app` package split
(`spec/64-cr-app-package-split/spec.md`). CR 67 declared `ui.Host` (20
methods) and proved `*App` satisfies it, but no overlay uses it yet —
all 10 still hold a concrete `app *App` field. This CR is the first
one to actually swap that field, which is also the first real test of
whether `Host`'s 20 methods are complete: if an overlay needs something
`Host` doesn't expose, the swap won't compile.

Before scoping this CR, all 10 overlays were checked for calls into
*another* overlay's fields/methods (not `*App` itself) — grepping every
`.confirm.`/`.movePicker.`/`.connManager.`/etc. pattern across all 10
files. Only 2 have any: `connManager` reaches into `confirm.show(...)`,
and `connEditor` reaches into `connManager.visible`/`.populate()`. The
other 8 (`confirmDialog`, `movePicker`, `sendMessageOverlay`,
`messageFilter`, `timeRangeModal`, `datadogEditor`, `themePicker`,
`awsProfilesPicker`) touch only `*App` — every one of those touches was
re-verified line by line against `Host`'s method list, with zero gaps.

## Problem

`connManager`/`connEditor`'s sibling-overlay reaches need a design
decision `Host` shouldn't make for them: `Host` is deliberately generic
(any future implementation, not just `*App`, could satisfy it), so it
must not grow methods like `Confirm() *confirmDialog` that leak a
concrete overlay type into the interface — that would defeat the point
of the interface for exactly the two overlays that need it least
(everyone else would inherit a wider interface for someone else's
dependency). The other 8 have no such problem and can swap today.

## Solution

Swap `app *App` → `host ui.Host` for the 8 clean overlays, one per
task, each independently buildable (each overlay type's field is
private to that type — changing one doesn't affect the others, and
`New()`'s construction call for that one overlay is updated in the same
step). Every internal `a.xxx`/`c.app.xxx`/etc. call becomes
`c.host.Xxx()` per `Host`'s method names (table in plan.md).

`connManager`/`connEditor` are explicitly **out of scope** — they need
constructor-injected sibling references alongside `host ui.Host`
(`newConnManager(host ui.Host, confirm *confirmDialog)`, etc.), a
different-shaped change worth its own CR (69) once this one has proven
the simple case works.

## Scope

### In scope

- `confirm.go`, `movepicker.go`, `sendmessage.go`, `messagefilter.go`,
  `timerangemodal.go`, `datadogsettings.go`, `settings.go` (the
  `themePicker` section only — `settingsView` is unrelated), `awsprofiles.go`
  (the `awsProfilesPicker` section — `secretBackend` etc. in
  `connectionsecrets.go` is unrelated): `app *App` field → `host
  ui.Host`, every constructor renamed param, every call site updated to
  the `Host` method name.
- `app.go`'s `New()`: the 8 corresponding `newXxx(a)` constructor calls
  — `a` already satisfies `ui.Host`, so these become `newXxx(a)` still
  (no call-site change) if the constructor signature keeps taking
  `ui.Host`-typed `a` implicitly via Go's structural typing — **actually
  worth confirming in plan.md**: passing a `*App` where `ui.Host` is
  expected works automatically (interface satisfaction), so `New()`
  itself may need zero changes. If any constructor needs an explicit
  `ui.Host` cast for clarity, that's decided there, not here.
- Each overlay's own struct/method receivers, comments referencing
  `a.xxx` or `.app.xxx`, updated to match.

### Out of scope

- `connManager`, `connEditor` — CR 69.
- Moving any file to `internal/dialog` — CR 70 (after both overlay
  sub-groups are swapped).
- Any behavior change. Pure rename/retype — if `Host`'s method
  semantics differ even slightly from the field access they replace,
  that's a bug in CR 67, not something to silently work around here.

## Definition of done

1. `go build ./...` and `go test ./...` pass in `tui/`.
2. All 8 overlays hold `host ui.Host`, not `app *App`; `go vet` and a
   full-text grep confirm no leftover `.app.` access in these 8 files.
3. No behavior change — verified live (`verify-live` skill) for a
   representative sample given the volume (8 overlays): `confirmDialog`
   (a delete confirmation), `movePicker` (moving a message), `sendMessageOverlay`
   (sending a message — already covered live in CR 67, re-check after
   this retype), `messageFilter` (apply/clear — same), `themePicker`
   (switching theme via the picker, not just `:theme`), `datadogEditor`
   and `awsProfilesPicker` (already covered live in CR 66, re-check
   after this retype). `timeRangeModal` gets at least a visual open/
   apply check.
