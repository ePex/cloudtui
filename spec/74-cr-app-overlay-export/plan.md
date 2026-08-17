# Plan — CR 74: export the 10 overlay types, constructors, `Show` methods

## Approach

Mechanical, per-type, in this order so the build stays green between
steps (verify after each type, not just at the end):

### 1. Rename within each type's own file (10 renames, 9 files)

For each of the 10 types, in its defining file:

- Type declaration: `type x struct` → `type X struct`
- Every receiver's type: `func (x *x) ...` → `func (x *X) ...`
  (receiver variable name unchanged — e.g. `cd *confirmDialog` →
  `cd *ConfirmDialog`, not renamed to `cd *ConfirmDialog` with a new
  receiver letter)
- Constructor: `func newX(...) *x` → `func NewX(...) *X`
- `show` method: `func (x *x) show(...)` → `func (x *X) Show(...)`
- Doc comments naming the type/constructor/method (e.g. "// confirmDialog
  is..." → "// ConfirmDialog is...", "// show opens..." → "// Show
  opens...")
- `var _ ui.Themeable = (*x)(nil)` → `(*X)(nil)`

`connections.go` holds two types (`connManager`, `connEditor`) —
both renamed in the same pass over that file. The one cross-reference
between them, `connManager.editor *connEditor` /
`connEditor.manager *connManager`, keeps its lowercase *field* names;
only the field's *type* changes to `*ConnEditor`/`*ConnManager`.

| Type | File |
|---|---|
| `confirmDialog` → `ConfirmDialog` | `confirm.go` |
| `movePicker` → `MovePicker` | `movepicker.go` |
| `sendMessageOverlay` → `SendMessageOverlay` | `sendmessage.go` |
| `connManager` → `ConnManager` | `connections.go` |
| `connEditor` → `ConnEditor` | `connections.go` |
| `messageFilter` → `MessageFilter` | `messagefilter.go` |
| `timeRangeModal` → `TimeRangeModal` | `timerangemodal.go` |
| `datadogEditor` → `DatadogEditor` | `datadogsettings.go` |
| `themePicker` → `ThemePicker` | `themepicker.go` |
| `awsProfilesPicker` → `AWSProfilesPicker` | `awsprofiles.go` |

`close` and every other method/field (`doSend`, `save`, `delete`,
`activate`, `applyRelative`, `applyAbsolute`, `fillList`,
`setPasswordField`, `rebuildTail`, `renderTabs`, `switchTab`, internal
fields) — untouched, stay unexported and lowercase-receiver as today.

### 2. `app.go`

- 10 struct field type declarations, e.g.:
  ```go
  // before
  confirm *confirmDialog
  // after
  confirm *ConfirmDialog
  ```
- 10 constructor calls in `New()`, e.g.:
  ```go
  // before
  a.confirm = newConfirmDialog(a)
  a.connEditor = newConnEditor(a, a.connManager)
  // after
  a.confirm = NewConfirmDialog(a)
  a.connEditor = NewConnEditor(a, a.connManager)
  ```
- `onPromptDone`'s two calls:
  ```go
  // before
  a.connManager.show()
  a.awsProfiles.show()
  // after
  a.connManager.Show()
  a.awsProfiles.Show()
  ```
- `overlayVisible`'s slice literal (`a.confirm`, `a.movePicker`, ...) —
  values unchanged, their static type is now `*ConfirmDialog` etc.
  instead of `*confirmDialog`; the literal itself needs no edits, only
  the field declarations feeding it (already covered above).

### 3. `settings.go`

8 call sites total, all `.show()` → `.Show()`, no other change:

- `newSettingsView`: `a.themePicker.show()`, `a.connManager.show()`,
  `a.awsProfiles.show()`, `a.datadogEditor.show()` (lines 35–38)
- `refreshSettingsList`: same 4 calls, same order (lines 73–76)

### 4. External view files (`.show(` → `.Show(`, verified call sites)

| File | Call sites |
|---|---|
| `message_detail.go` | `a.confirm.show(...)`, `a.movePicker.show(...)` |
| `queues.go` | `qv.app.confirm.show(...)`, `qv.app.movePicker.show(...)`, `qv.app.sendMessage.show(...)` |
| `messages.go` | `a.confirm.show(...)` ×2, `a.movePicker.show(...)`, `a.sendMessage.show(...)`, `a.messageFilter.show()` |
| `logsearch.go` | `a.timeRangeModal.show(...)` |
| `datadoglogs.go` | `a.timeRangeModal.show(...)` |

Each is a straight `.show(` → `.Show(` substitution; arguments and
surrounding logic untouched.

### 5. Test files (7 files, same-package references)

Every occurrence of a renamed type name, and every `.show(` call on
one of these 10 types, updated to the new exported name in:
`app_test.go`, `messages_test.go`, `logsearch_test.go`,
`datadoglogs_test.go`, `connections_test.go`,
`datadogsettings_test.go`, `timerangemodal_test.go`.

`.close(` call sites in these files are unchanged (method stays
unexported). Where a test declares a local variable typed as one of
the 10 (e.g. `var cm *connManager`), the type annotation updates too;
inferred (`:=`) declarations need no edit beyond the constructor-call
site itself.

### 6. Verification after each type (not just at the end)

After each of the 10 renames, run `gofmt -l .`, `go build ./...`,
`go vet ./...` before moving to the next type — a missed call site
fails the build immediately and pinpoints exactly which file still
uses the old name, which is faster to fix one type at a time than
after all 10 are done. Final pass: `go test ./...` across `tui/`.

## Files touched

- 9 overlay files (10 type renames — `connections.go` holds 2):
  `confirm.go`, `movepicker.go`, `sendmessage.go`, `connections.go`,
  `messagefilter.go`, `timerangemodal.go`, `datadogsettings.go`,
  `themepicker.go`, `awsprofiles.go`
- `app.go` (field types, constructor calls, `onPromptDone`)
- `settings.go` (8 `.show()` → `.Show()`)
- `message_detail.go`, `queues.go`, `messages.go`, `logsearch.go`,
  `datadoglogs.go` (`.show(` → `.Show(`)
- 7 test files: `app_test.go`, `messages_test.go`, `logsearch_test.go`,
  `datadoglogs_test.go`, `connections_test.go`,
  `datadogsettings_test.go`, `timerangemodal_test.go`

**Corrected during implementation** (see `tasks.md` for the full
notes): field-type declarations for a renamed type (in `app.go` and,
for `confirmDialog`, in `connManager`'s own struct) couldn't be
deferred to a separate step as originally planned — Go requires the
type to exist wherever it's referenced, so each rename's field-type
fallout was fixed in the same task as the rename itself, not batched
into task 10. `connections.go:154` (`cm.confirm.show(...)`) was an
external-style call site missed by `spec/70`'s original audit,
fixed alongside `connections.go`'s own rename. `awsprofiles_test.go`
turned out to need updating too (16 `.show(` sites) — it wasn't on
the original 7-file list because it references overlays only via the
`a.X` field path, never the bare type name, so the type-name grep
that produced that list missed it.

## Key decisions

- **Rename one type at a time, verifying the build after each** —
  rather than a single repo-wide search/replace pass — because a
  missed call site (there are ~40 across production + test code) is
  much easier to isolate immediately after touching one type than
  after all 10 are renamed and the build has many simultaneous
  failures.
- **`close` stays unexported** — zero external callers today (verified
  in `spec/70` and reconfirmed in `spec.md`'s background section), and
  CR 75's physical move brings the whole file (including its internal
  callers of `close`) along together, so there's never a point where
  `close` needs to be reachable from outside its own file.
- **No `sed`-based bulk rename** — given the BSD `sed`/`\b`
  word-boundary gotcha hit in CR 71, and that some renames need
  case-sensitive, word-bounded matching that differs per type
  (`connManager`/`connEditor` share a file and a substring
  relationship doesn't exist, but `confirm` as a field name vs.
  `confirmDialog`/`ConfirmDialog` as a type name could false-positive
  under a loose pattern) — manual `Edit` calls per file, guided by
  fresh greps, are safer here than a scripted pass.
- **No new tests** — pure rename, zero behavior change; existing tests
  continue to exercise the same code paths under new names.
- **No new dependencies.**

## Definition of done

Unchanged from spec.md — `go build`/`go test`/`go vet` pass, `gofmt -l`
clean, all 10 types/constructors/`Show` methods exported, every call
site (production and test) updated, `close` and all other members
still unexported, zero behavior change.
