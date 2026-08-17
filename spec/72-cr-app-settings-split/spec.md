# Spec — CR 72: split `settings.go` into `settingsView` and `themePicker`

Date: 2026-08-17

## Background

Next step in phase 3's recorded roadmap (`spec/70`). `settings.go`
today defines two unrelated types: `settingsView` (the Settings screen
— a `ui.View`, stays in package `app`) and `themePicker` (one of the
10 overlays moving to `internal/dialog`). CR 71 already resolved the
one real blocker this split would have hit (`styleDropDown`, which
used to live in this file) — re-reading the file now confirms it's
otherwise clean: `themePicker` only touches its own state and `ui.Host`/
`ui.StyleList`; `datadogSettingsLabel` (used by `refreshSettingsList`)
is only called from within `settings.go` itself. `settings_test.go`'s
11 tests all exercise `settingsView`/`refreshSettingsList`/
`switchTheme` — none reference `themePicker` directly, so no test file
needs to split.

## Problem

One file, two types with different eventual homes. Not fixing this
now just defers identical work to whichever later CR actually needs
`themePicker` isolated (e.g. once `internal/dialog` exists, `settings.go`
would need `package app` and `package dialog` in one file, which is
impossible).

## Solution

Move `themePicker`, `newThemePicker`, `show`, `close`, `ApplyPalette`,
and the `var _ ui.Themeable` assertion into a new file `themepicker.go`.
`settings.go` keeps `settingsView`, `newSettingsView`,
`refreshSettingsList`, `datadogSettingsLabel`. Pure file split — same
package, same file otherwise unchanged, just fewer lines each.

## Scope

### In scope

- New file `internal/app/themepicker.go`.
- `settings.go` shrinks to `settingsView`'s half.

### Out of scope

- Exporting the 10 types/constructors/`Show` methods, the
  `.flex`/`.form`/`.visible` redesign, or the actual move to
  `internal/dialog` — later CRs per `spec/70`'s roadmap.
- Any behavior change. Pure code motion within the same package.

## Definition of done

1. `go build ./...` and `go test ./...` pass in `tui/`.
2. `themePicker` and everything it owns lives in `themepicker.go`;
   `settings.go` defines only `settingsView`-related code.
3. No behavior change — pure file split within one package, no
   renamed/retyped symbols; `go test ./...` passing is sufficient, no
   live verification needed.
