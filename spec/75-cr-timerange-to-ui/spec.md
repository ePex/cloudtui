# Spec — CR 75: promote `timeRange`/`timeRangeMode`/`timeRangePresets` to `internal/ui`

Date: 2026-08-17

## Background

With CR 74 done, the next step toward closing phase 3 of `spec/64`
(spec/64/spec.md) is the physical move of the 10 overlay files into a
new `internal/dialog` package. Before scoping that move, re-audited
every package-level symbol the 10 moving files (`confirm.go`,
`movepicker.go`, `sendmessage.go`, `connections.go`,
`messagefilter.go`, `timerangemodal.go`, `datadogsettings.go`,
`themepicker.go`, `awsprofiles.go`) declare, checking whether anything
outside those files depends on it — the same check CR 71 ran before
finding `styleList`/`styleDropDown`/`parseFilterDate`/
`parseMessageFilterForm` blocking that move.

Everything checked out self-contained **except one**: `timerangemodal.go`
uses `timeRange`, `timeRangeMode`, `timeRangePresets`,
`timeRangeRelative`, `timeRangeAbsolute` — all defined in
`logsearch.go`, a resource-view file that is *not* part of this move
(it stays in `internal/app`, or moves later in `spec/64`'s phase 4).
`datadoglogs.go` (also staying, external caller of `timeRangeModal`)
uses the same type directly too (`dv.tr`, the `func(tr timeRange)`
callback). 82 occurrences total across 6 files:
`logsearch.go`, `logsearch_test.go`, `datadoglogs.go`,
`datadoglogs_test.go`, `timerangemodal.go`, `timerangemodal_test.go`.

## Problem

Once `timerangemodal.go` moves to `internal/dialog`, it can't keep
using a type defined in `internal/app` — `internal/app` will import
`internal/dialog` (for the overlay types themselves), so
`internal/dialog` importing back from `internal/app` would be a cycle.
The type has to live somewhere both packages can import without either
importing the other.

## Solution

Promote the whole `timeRange` group to `internal/ui` (new file
`timerange.go`) — the same destination CR 71 used for
`styleList`/`parseFilterDate`, for the same reason: it's the shared,
dependency-free base both `internal/app` and `internal/dialog` sit
above. Exported (required to be usable from another package, per
`internal/ui`'s existing convention for cross-package symbols):

| Before (unexported, `logsearch.go`) | After (exported, `internal/ui`) |
|---|---|
| `timeRangePreset` | `ui.TimeRangePreset` |
| `timeRangePresets` | `ui.TimeRangePresets` |
| `defaultPresetIdx` | `ui.DefaultPresetIdx` |
| `timeRangeMode` | `ui.TimeRangeMode` |
| `timeRangeRelative` | `ui.TimeRangeRelative` |
| `timeRangeAbsolute` | `ui.TimeRangeAbsolute` |
| `timeRange` (struct + `bounds`/`label` methods) | `ui.TimeRange` |

Both methods (`bounds`, `label`) are fully self-contained (only touch
`timeRangePresets`/the receiver's own fields) — they move and get
exported (`Bounds`, `Label`) alongside the type, same treatment as
`ParseFilterDate` in CR 71.

## Scope

### In scope

- `internal/ui/timerange.go` (new file): the 7 promoted/exported
  identifiers, moved verbatim from `logsearch.go` plus `export`
  renaming.
- `internal/ui/timerange_test.go` (new file): existing coverage for
  `bounds`/`label`, if any exists today under `logsearch_test.go` —
  reverify during implementation and move+rename whatever's there.
- `logsearch.go`, `logsearch_test.go`: remove the promoted
  declarations, update all call sites to `ui.TimeRange`/
  `ui.TimeRangeRelative`/etc.
- `datadoglogs.go`, `datadoglogs_test.go`: same call-site update.
- `timerangemodal.go`, `timerangemodal_test.go`: same call-site
  update (this file doesn't declare any of the 7 — it only consumes
  them — so this is pure rename, no logic move).
- `gofmt`/`go vet`/`go build`/`go test` clean across `tui/`.

### Out of scope

- The physical move of the 10 overlay files into `internal/dialog` —
  next CR, now unblocked for this specific dependency (still need to
  separately confirm no other blockers before scoping it — see below).
- Redesigning how the 4 test files that construct a full `*App` via
  `New()` and reach into unexported overlay fields
  (`connections_test.go`, `datadogsettings_test.go`,
  `timerangemodal_test.go`, `awsprofiles_test.go`) will work once
  those types live in a different package — noted as an open question
  for the move CR's plan.md, not resolved here.
- Any behavior change. Pure promotion + rename; `go test ./...`
  passing is sufficient, no live verification needed.

## Definition of done

1. `go build ./...` and `go test ./...` pass in `tui/`.
2. `timeRangePreset`/`timeRangePresets`/`defaultPresetIdx`/
   `timeRangeMode`/`timeRangeRelative`/`timeRangeAbsolute`/`timeRange`
   no longer exist in `internal/app`; their exported equivalents live
   in `internal/ui` and are used by every former call site.
3. `gofmt -l` reports nothing; `go vet ./...` clean.
4. No behavior change — promotion + rename only.
