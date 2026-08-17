# Spec — CR 74: export the 10 overlay types, constructors, and `Show` methods

Date: 2026-08-17

## Background

`spec/70`'s recorded roadmap has one piece left before the physical
move to `internal/dialog`: exporting all 10 overlay types, their
constructors, and their `show` methods. CR 73 deliberately cleared the
way for this by replacing every raw `.flex`/`.form`/`.visible` access
from `app.go` with `Primitive()`/`Visible()` calls, so this pass is now
a rename, not a redesign — no new methods, no behavior change, nothing
that requires re-deriving how any overlay works.

Re-verified the exact current shape before scoping this CR (fresh
greps, not reused from `spec/70`, since CR 71–73 touched some of these
files):

- Every `.show(` external call site from `spec/70`'s audit table is
  still accurate — no CR since has changed a call site's file or
  shape.
- `close()` still has zero callers outside each overlay's own file
  (confirmed again by grep) — stays unexported.
- Every one of the 10 types, and every one of their `new*` constructors,
  is referenced only from within `internal/app` (production code) —
  there is no package boundary yet, so exporting them has no runtime
  effect today. It's pure preparation for CR 75 (the physical move),
  which does introduce a package boundary.
- Test coverage is heavier than `spec/70`'s table suggested — a fresh
  grep found *seven* test files reaching into one of the 10 type names
  or calling `.show(`/`.close(` directly, not four:
  `app_test.go`, `messages_test.go`, `logsearch_test.go`,
  `datadoglogs_test.go`, `connections_test.go`,
  `datadogsettings_test.go`, `timerangemodal_test.go`. All are in
  package `app` (no separate `_test` package), so today they compile
  by referencing the unexported names directly — every one of those
  references needs updating to the new exported name in this CR, since
  the old unexported identifiers stop existing.

## Problem

`internal/dialog` (CR 75) cannot import these 10 types, their
constructors, or their `show` methods while they're unexported —
Go simply doesn't allow it. The rename has to happen before the move,
not during it, so the move itself stays a pure file-relocation with
import-path fixes, not a rename-while-moving that's harder to verify.

## Solution

Rename, 1:1, nothing else:

| Type | Constructor | `show` → |
|---|---|---|
| `confirmDialog` → `ConfirmDialog` | `newConfirmDialog` → `NewConfirmDialog` | `Show` |
| `movePicker` → `MovePicker` | `newMovePicker` → `NewMovePicker` | `Show` |
| `sendMessageOverlay` → `SendMessageOverlay` | `newSendMessageOverlay` → `NewSendMessageOverlay` | `Show` |
| `connManager` → `ConnManager` | `newConnManager` → `NewConnManager` | `Show` |
| `connEditor` → `ConnEditor` | `newConnEditor` → `NewConnEditor` | `Show` |
| `messageFilter` → `MessageFilter` | `newMessageFilter` → `NewMessageFilter` | `Show` |
| `timeRangeModal` → `TimeRangeModal` | `newTimeRangeModal` → `NewTimeRangeModal` | `Show` |
| `datadogEditor` → `DatadogEditor` | `newDatadogEditor` → `NewDatadogEditor` | `Show` |
| `themePicker` → `ThemePicker` | `newThemePicker` → `NewThemePicker` | `Show` |
| `awsProfilesPicker` → `AWSProfilesPicker` | `newAWSProfilesPicker` → `NewAWSProfilesPicker` | `Show` |

Everything else on each type stays unexported: `close`, internal
fields (`flex`, `form`, `visible`, `host`, etc.), and every other
helper method (`doSend`, `save`, `delete`, `activate`, `applyRelative`,
...) — none of them are called from outside their own file today, and
none of them need to be callable from `internal/dialog` after the move
either (the move brings the whole file, callers included, along with
it).

`connManager.editor *connEditor` and `connEditor.manager *connManager`
(the sibling-overlay fields from CR 69) keep their lowercase field
names — only the *type* they point to is renamed.

## Scope

### In scope

- The 10 overlay files: rename each type, its constructor, and its
  `show` method per the table above. Update each file's own doc
  comments that name the type/constructor/method.
- `app.go`: the 10 struct field type declarations, the 10 constructor
  call sites in `New()`, the `onPromptDone` calls
  (`a.connManager.Show()`, `a.awsProfiles.Show()`), the `visibler`
  slice literal (already holds the overlay values, not field
  addresses, from CR 73 — no shape change, just the types involved are
  now exported).
- `host.go`: no `show`/`close` calls here, but check for any type
  reference (e.g. in `Host` method bodies) — verify during
  implementation.
- `settings.go`: 6 `.show()` call sites (2 in the settings-page item
  builder, 4 in `refreshSettingsList` per `spec/70`'s numbers —
  reverify exact count during implementation since CR 72 split this
  file).
- External view files with `.show(` call sites per the reverified
  audit: `message_detail.go`, `queues.go`, `messages.go`,
  `logsearch.go`, `datadoglogs.go`.
- All 7 test files identified above: update every reference to a
  renamed type, constructor, or `.show(` call to the new exported
  name. `.close(` call sites in test files stay as-is (name unchanged).
- `gofmt`/`go vet`/`go build`/`go test` clean across `tui/`.

### Out of scope

- The physical move of any file into `internal/dialog` — CR 75.
- Renaming `close` or any other still-unexported method/field — no
  external caller needs them exported, and exporting unused surface
  area isn't justified by anything in this CR.
- Any behavior change. Pure rename; `go test ./...` passing is
  sufficient — no live verification needed (nothing here changes what
  runs, only what it's called).
- Phase 4 (`internal/view`) and phase 5 (`connectionsecrets.go`) of
  `spec/64`'s original roadmap.

## Definition of done

1. `go build ./...` and `go test ./...` pass in `tui/`.
2. All 10 types, their constructors, and their `Show` methods are
   exported; every internal and external call site updated; `close`
   and all other members remain unexported.
3. `gofmt -l` reports nothing; `go vet ./...` clean.
4. No behavior change — rename only.
