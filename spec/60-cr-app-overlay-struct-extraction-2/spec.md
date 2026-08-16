# Spec — CR 60: extract message filter + Datadog settings into dedicated structs

Date: 2026-08-16

## Background

CR 59 (`spec/59-cr-app-overlay-struct-extraction`) extracted the three
overlays that had no file of their own (confirm, move picker,
send-message) into per-feature structs, matching the pattern already used
by 15 existing views, and documented the remaining backlog:

| Group | Fields on `App` | Has its own file already? |
|---|---|---|
| Connection manager + editor | 9 | Yes — `connections.go` |
| Message filter | 2 | Yes — `messagefilter.go` |
| Time range modal | 8 | Yes — `timerangemodal.go` |
| Datadog settings editor | 2 | Yes — `datadogsettings.go` |
| Theme picker | 3 | Logic in `settings.go`, construction in `app.go` |
| AWS profiles picker | 8 | Yes — `awsprofiles.go` |

## Problem

Same as CR 59: these six groups already have their own file (or nearly —
theme picker), but their state still lives as flat fields on `App`
instead of a dedicated struct, and their logic is `(a *App)` methods
mixed into `app.go`/their file rather than hung off that struct.

## Solution

Continue the CR 59 pattern, one more slice. This CR picks the two
smallest, most self-contained groups — message filter and Datadog
settings editor (2 fields each) — as the next step, in preference to the
four larger groups. Same target shape as CR 59: a struct with an
`app *App` back-reference, methods hung off that struct, `App` holds one
field per overlay instead of N flat fields.

Two things specific to this slice, found while surveying the code before
writing this spec:

- **Message filter has no dedicated test file** (`parseMessageFilterForm`,
  the one pure function it uses, is tested via `messages_test.go`, but
  nothing exercises `showMessageFilter`/`applyMessageFilter`/
  `clearMessageFilter` directly) — so this extraction has less existing
  test coverage as a safety net than CR 59 did, and leans more on live
  verification.
- **Datadog settings has real regression-test coverage worth preserving
  carefully**: `datadogsettings_test.go` (prefill, cancel-discards,
  save-persists) and a specific `app_test.go` case,
  `TestOnGlobalKeyPassesThroughWhenDatadogEditorVisible`, guarding a real
  bug found live before ("unlike the other overlays, `datadogEditorVisible`
  was missing from `onGlobalKey`'s overlay-exemption block, so typing into
  the Datadog editor's Site/Access Token fields hit global hotkeys instead
  of being typed — `q` quit the whole app mid-edit"). That test directly
  exercises the exact OR-chain line this CR edits — it must keep passing,
  unchanged in intent, after the field-path rename.
- **Neither overlay is currently retheme-live**: `theme.go`'s
  `reapplyTheme` has no section for `messageFilterForm` or
  `datadogEditorForm` at all today (unlike the three CR 59 overlays, which
  it does cover). That's a pre-existing gap, not something this CR
  introduces or needs to fix — `theme.go` needs no changes for this CR.

## Scope

### In scope

- New files `messagefilter.go` (already exists — refactored in place,
  not renamed) gains a `messageFilter` struct; `datadogsettings.go`
  (same, in place) gains a `datadogEditor` struct.
- `app.go`: remove the 4 extracted fields (2 + 2), replaced by
  `messageFilter *messageFilter` and `datadogEditor *datadogEditor`;
  construction in `New()` calls the new constructors; the two OR-chains
  updated for both flags.
- Call sites: `messages.go` (`showMessageFilter`), `settings.go` (two
  `showDatadogEditor` call sites) updated to the new method paths.
- `app_test.go`'s `TestOnGlobalKeyPassesThroughWhenDatadogEditorVisible`
  and `datadogsettings_test.go`'s field/method references updated to the
  new paths — same assertions, same intent, access-path change only.

### Out of scope

- The four remaining backlog groups (connection manager/editor, time
  range modal, theme picker, AWS profiles picker) — still backlog, still
  listed in `spec/59-.../spec.md`, not duplicated here.
- Adding `reapplyTheme` coverage for these two overlays — a real, separate
  gap, but a behavior change (not a refactor) and not what this CR is
  about. Worth a follow-up bugfix if it's actually bothering anyone in
  practice.
- Any behavior change. Pure structural move, as with CR 59.

## Definition of done

1. `go build ./...` and `go test ./...` pass in `tui/`, including
   `TestOnGlobalKeyPassesThroughWhenDatadogEditorVisible` continuing to
   pass with its original intent intact.
2. `app.go` shorter by roughly the moved construction blocks (~35 lines).
3. No behavior change: message filter (Apply/Clear/Cancel, Esc) and
   Datadog settings (prefill, save, cancel-discards) work identically —
   verified live (`verify-live` skill), same as CR 59, since message
   filter in particular has no unit-test safety net for its
   show/apply/clear flow.
