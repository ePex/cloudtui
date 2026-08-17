# Spec — CR 69: swap `connManager`/`connEditor` to `host ui.Host`

Date: 2026-08-17

## Background

Fourth (and final overlay-swap) slice of phase 3 of the `internal/app`
package split (`spec/64-cr-app-package-split/spec.md`). CR 68 swapped
8 of the 10 overlays to `host ui.Host`, deliberately excluding
`connManager` and `connEditor` — the only two that reach into a
sibling overlay, which `Host` can't expose without leaking a concrete
overlay type into an otherwise-generic interface.

`connections.go` was re-read in full to enumerate every cross-reach
precisely rather than guess:

- `connManager` calls `a.connEditor.show(...)` (its `n`/`e`/`d` key
  handlers — new/edit/duplicate) and `a.confirm.show(...)` (its delete
  confirmation).
- `connEditor` calls `a.connManager.visible`/`.list`/`.populate()`
  (`close()`'s focus-fallback and `save()`'s post-save list refresh).

Checking `app.go`'s `New()` construction order
(`a.confirm` → ... → `a.connManager` → `a.connEditor`) shows this
isn't fully circular: `confirm` already exists when `connManager` is
built, and `connManager` already exists when `connEditor` is built —
both can be plain constructor parameters. Only one direction is
backward: `connManager` needs `connEditor`, which doesn't exist yet at
`connManager`'s construction time.

## Problem

Same as CR 68's excluded cases: `Host` must stay generic, so
`connManager`/`connEditor` need their sibling reaches satisfied a
different way — direct references to the concrete sibling types
(fine, since both stay in package `app`, soon both in
`internal/dialog` together — an intra-package reference either way,
never crossing into `internal/view` or back into `internal/app`).

## Solution

- `connEditor` gains a `manager *connManager` field, set via
  constructor parameter (`newConnEditor(host ui.Host, manager
  *connManager)`) — `connManager` already exists at this point in
  `New()`, so no different wiring pattern needed.
- `connManager` gains `confirm *confirmDialog` (constructor parameter,
  `confirm` already exists) and `editor *connEditor` (**not** a
  constructor parameter — `connEditor` doesn't exist yet when
  `connManager` is built). `editor` is left zero-value at construction
  and set explicitly in `New()` right after `a.connEditor` is created
  (`a.connManager.editor = a.connEditor`) — one new line, same "wire
  after both exist" pattern CR 62 already established for
  detail-view trampolines. Safe because `editor` is only read inside
  input-capture closures, which only ever fire after `New()` has fully
  returned.
- Every other touch in both types (chrome, `Config()` reads,
  `SetStatus`, `SwitchConnection`, `DeleteConnection`, `SaveConnection`)
  swaps to the matching `Host` method exactly as CR 68 did for the
  other 8 — full substitution table in plan.md.

## Scope

### In scope

- `connections.go`: `connManager` and `connEditor` both swap `app
  *App` → `host ui.Host` plus the one sibling field each needs; every
  `.app.`/`a.`-via-local-alias access updated to the matching `Host`
  method or sibling-field access.
- `app.go`: the two `newXxx(a)` construction calls gain the extra
  sibling-reference argument (`newConnManager(a, a.confirm)`,
  `newConnEditor(a, a.connManager)`); one new line
  (`a.connManager.editor = a.connEditor`) right after `a.connEditor`
  is constructed.

### Out of scope

- Moving any file to `internal/dialog` — CR 70, now that all 10
  overlays depend on `ui.Host` (plus, for these two, each other) and
  none touch `*App` directly.
- The known pre-existing `connEditor.ApplyPalette` dead type-assertion
  bug (documented, preserved as-is since CR 65) — still untouched.
- Any behavior change. Pure retype — if `Host`'s method semantics
  differ from the field access they replace, that's a defect in an
  earlier CR, not something to work around here.

## Definition of done

1. `go build ./...` and `go test ./...` pass in `tui/`.
2. `connManager` and `connEditor` both hold `host ui.Host`; `grep -n
   '\.app\.' connections.go` returns nothing.
3. No behavior change — verified live (`verify-live` skill): open the
   connection manager, create a new connection, edit an existing one
   (including toggling Backend jolokia↔proxy to exercise
   `rebuildTail`), duplicate one, delete a non-active one, delete the
   active one (confirming it switches away correctly) — the full
   connection-management flow in one pass, since both types' methods
   are entangled enough that testing them separately would miss
   exactly the interaction (`connManager` opening `connEditor`,
   `connEditor` refreshing `connManager`) this CR is about.
