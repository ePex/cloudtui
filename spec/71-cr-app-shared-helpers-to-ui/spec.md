# Spec — CR 71: promote shared tview-styling/parsing helpers to `internal/ui`

Date: 2026-08-17

## Background

Continuing phase 3's move-preparation (`spec/64`, `spec/70`). Before
splitting `settings.go` (the next step in CR 70's recorded roadmap),
a check of what `styleDropDown` actually is (it's defined in
`settings.go`) surfaced a wider pattern worth resolving first: four
unexported helper functions, each defined in a file that **stays** in
package `app`, are called by files that are **moving** to
`internal/dialog` — and, in two cases, also by files that stay. Found
by grepping every bare (non-method) lowercase function call in all 10
moving files and cross-checking where each is actually defined:

| Helper | Defined in (stays) | Called by (moving) | Called by (stays) |
|---|---|---|---|
| `styleList(l *tview.List, p config.Palette) *tview.List` | `theme.go` | `confirm.go`, `movepicker.go`, `sendmessage.go`, `timerangemodal.go`, `connections.go`, `settings.go` (`themePicker`) | `theme.go` itself (`reapplyTheme`'s `a.settingsList` styling) |
| `styleDropDown(dd *tview.DropDown, p config.Palette)` | `settings.go` | `connections.go` | `datadoglogs.go` (a *view*, stays) |
| `parseFilterDate(label, s string) (time.Time, error)` | `messages.go` | `timerangemodal.go` | `messages.go` itself |
| `parseMessageFilterForm(jmsType, from, to, maxCount string) (queue.MessageFilter, error)` | `messages.go` | `messagefilter.go` | — (calls `parseFilterDate` internally) |

All four are pure functions — no `App`/`Host` state, just
widgets/palettes/strings in, values out — so none of them belong on
`Host`. They're exactly the same shape of problem CR 64's phase 1
already solved once: `centered` (sizing helper, used by both views and
overlays) became `ui.Centered`. These four are the same pattern,
just not caught in that first pass because phase 1 only looked at the
4 chrome files, not every helper every overlay would eventually need.

## Problem

Once the 10 overlay files move, `internal/dialog` can't call an
unexported function defined in package `app` (`theme.go`/`settings.go`/
`messages.go` all stay), and `internal/app` can't call an unexported
function that moved into `internal/dialog` either (for `styleDropDown`
via `datadoglogs.go`, and `styleList` via `theme.go`'s own
`reapplyTheme`). Whichever package the function ends up in, the other
side breaks.

## Solution

Promote all four (plus `parseFilterDate`'s two supporting constants,
`filterDateLayout`/`filterDateTimeLayout`, used only inside it) to
`internal/ui`, exported: `ui.StyleList`, `ui.StyleDropDown`,
`ui.ParseFilterDate`, `ui.ParseMessageFilterForm`. `internal/ui`
already imports `config` (for `Themeable`/`Host`); it gains `queue` and
`time` for these (no import cycle — `internal/queue` doesn't import
`internal/ui`).

Every caller — in both moving and staying files — updates to the
`ui.`-qualified name. This is the same shape of change CR 64 phase 1
already made for `centered`/`ui.Centered`, just for four more helpers
discovered later.

## Scope

### In scope

- `internal/ui/theme.go` (or a new file — decided in plan.md): gains
  `StyleList`, `StyleDropDown`.
- `internal/ui/messages.go` (new, or wherever plan.md decides):
  `ParseFilterDate`, `ParseMessageFilterForm`, plus the two layout
  constants.
- `theme.go`: loses `styleList`; `reapplyTheme` updated to call
  `ui.StyleList`.
- `settings.go`: loses `styleDropDown`.
- `messages.go`: loses `parseFilterDate`, `parseMessageFilterForm`,
  `filterDateLayout`, `filterDateTimeLayout`.
- Every one of the ~9 caller files listed in the table above (both
  moving and staying) updated to the `ui.`-qualified name.
- The two existing tests (`TestStyleListAppliesSelectionColors` in
  `theme_test.go`, `TestParseMessageFilterForm` in `messages_test.go`)
  move to `internal/ui` alongside their functions, per `tui/CLAUDE.md`'s
  "one test file per source file, same package" convention. No test
  exists yet for `styleDropDown` or `parseFilterDate` standalone
  (`parseFilterDate` is exercised indirectly through
  `TestParseMessageFilterForm`) — not adding new coverage here, out of
  scope for a pure relocation.

### Out of scope

- Splitting `settings.go` itself — next CR, now unblocked (it no
  longer needs to keep `styleDropDown` for anyone).
- Exporting the 10 overlay types/constructors/`Show` methods, or the
  `.flex`/`.form`/`.visible` redesign — later CRs per `spec/70`'s
  roadmap.
- Any behavior change. Pure relocation of pure functions — same
  inputs, same outputs, same callers' code paths.

## Definition of done

1. `go build ./...` and `go test ./...` pass in `tui/`.
2. All four helpers (+ the two constants) live in `internal/ui`,
   exported; none of `theme.go`/`settings.go`/`messages.go` defines
   them anymore.
3. No behavior change — pure functions, verified by the moved unit
   tests passing unchanged plus a full `go test ./...`; no live
   verification needed (nothing here touches rendering timing, broker
   state, or anything a unit test can't already cover).
