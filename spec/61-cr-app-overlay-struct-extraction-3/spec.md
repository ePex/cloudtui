# Spec — CR 61: extract the remaining four overlay groups into dedicated structs

Date: 2026-08-16

## Background

CR 59 and CR 60 extracted five overlays (confirm, move picker, send-message,
message filter, Datadog settings) into per-feature structs. The backlog
table in `spec/59-.../spec.md` has four groups left:

| Group | Fields on `App` | Methods | Test coverage |
|---|---|---|---|
| Connection manager + editor | 9 | 9 (`connections.go`) | `connections_test.go`, 1 line in `app_test.go` |
| Time range modal | 8 | 8 (`timerangemodal.go`) | `timerangemodal_test.go` (12 tests), 6 lines in `datadoglogs_test.go`/`logsearch_test.go` each |
| Theme picker | 3 | 2 (`settings.go`) | none |
| AWS profiles picker | 8 | 8 (`awsprofiles.go`) | `awsprofiles_test.go` (extensive), 6 lines in `app_test.go` |

## Problem / Solution

Same as CR 59/60 — finish applying the established pattern to the rest of
the backlog. This is the last CR in the series; after this, every overlay
in `internal/app` follows the same shape as the 15 views that already did
before CR 59 started.

Per the user's direction, this CR (and CR 60, not yet pushed) land as
**commits on one branch, one PR** — `cr/60-app-overlay-struct-extraction-2`
— rather than a separate PR each, since the pattern is proven and the risk
per slice is low.

## Scope

### In scope — same target shape as CR 59/60 throughout

- **Connection manager + editor**: two structs in `connections.go`,
  `connManager` (`flex`, `list`, `hints`, `visible`) and `connEditor`
  (`form`, `visible`, `isNew`, `origName`, `brokerName` — the last three
  were already renamed once for spec/57's Broker Name work, this just
  moves them onto a struct). `connEditor` needs to reach `connManager`
  (e.g. refreshing the list after save) via its own `app *App`
  back-reference — same cross-overlay reach already used elsewhere (e.g.
  CR 59's `movePicker` reaching `app.queuesV`).
- **Time range modal**: one struct, named `timeRangeModal` — **not**
  `timeRange`, which is already the name of the value type
  (`type timeRange struct{...}`) this modal edits; colliding would shadow
  it. Fields: `flex`, `tabs`, `pages`, `relativeList`, `absoluteForm`,
  `visible`, `activeTab`, `onApply`.
- **Theme picker**: one struct, `themePicker` (`flex`, `list`, `visible`).
  Construction moves from `app.go` into `settings.go`, alongside
  `showThemePicker`/`closeThemePicker` (which become its methods) — same
  file, no new file needed (mirrors how CR 60 left message filter and
  Datadog settings in their existing files rather than creating new ones).
- **AWS profiles picker**: one struct, `awsProfilesPicker` (not
  `awsProfiles` — too close to the imported `awsprofile` package name).
  Fields: `flex`, `table`, `filterInput`, `hints`, `visible`, `filter`,
  `all`, `filtered`.
- All four: `App` gains one field each, replacing the flat fields; both
  OR-chain guards in `app.go` (`onGlobalKey`'s exemption list and
  `onPromptDone`'s focus-restore check) updated for all four flags —
  after this CR those two lines are just eight `.visible` reads, no flat
  booleans left.
- Every test file listed in the table above updated to the new access
  paths — same assertions, same intent, per CR 59/60's precedent.

### Out of scope

- Any behavior change. Pure structural move, as with CR 59/60.
- `reapplyTheme` coverage for message filter/Datadog settings (still a
  pre-existing gap, still not this CR's job — noted in CR 60's spec).
- Anything not in the table above — this closes out the CR 59 backlog
  entirely; no further overlay-extraction backlog after this.

## Definition of done

1. `go build ./...` and `go test ./...` pass in `tui/`, including every
   updated test in the table above with unchanged intent.
2. `app.go`'s two OR-chain lines contain only `.visible` struct-field
   reads, no flat booleans.
3. No behavior change in any of the four overlays — verified live
   (`verify-live` skill) for at least the connection manager/editor and
   AWS profiles picker (real broker/AWS-file interaction); time range
   modal and theme picker have thorough or straightforward-enough
   existing/added test coverage that live verification is a lighter
   sanity pass, not the primary safety net (mirrors CR 60's differentiated
   treatment of message filter vs. Datadog settings).
