# Spec — CR 77: migrate `awsprofiles_test.go` to the `ui.Host` test double

Date: 2026-08-17

## Background

CR 76 built `testHost` (`internal/app/hosttest_test.go`, a minimal
`ui.Host` test double covering all 20 interface methods) and proved it
on `connections_test.go`, `datadogsettings_test.go`, and
`timerangemodal_test.go` — deliberately deferring
`awsprofiles_test.go` (the largest of the four, 13 test functions) to
a follow-up CR. This is that follow-up.

Re-read `awsprofiles_test.go` fresh (confirmed 13 test functions, not
14 as CR 76's spec estimated — a stale headcount from before the file
was last touched, corrected here) to classify each one, same audit
CR 76 ran for its three files:

- **12 pure `AWSProfilesPicker`-logic tests** — open/close, key
  handling (`r` refresh, `/` filter focus, Esc), filter narrowing,
  table population from an injected lister, active-profile star
  marking, scroll-to-top on repaint. `*App` is present only to get a
  `ui.Host` and to inject a fake lister via `a.listAWSProfiles` (an
  `App` field wired into `host.go`'s `ListAWSProfiles`) — nothing
  about `*App` itself is under test.
- **1 true `App`/`Host` integration test** —
  `TestActivateAWSProfilePersistsAndUpdatesUI` verifies that
  activating a profile also updates `App`'s info panel and settings
  list (`a.infoPanel`, `a.settingsList` — wiring only `App` owns) and
  persists to `config.yaml` on disk. Same shape as CR 76's
  `TestSaveDatadogEditorRoundTrip` — stays in `internal/app`,
  unmodified.

No misplaced tests this time (unlike CR 76's `TestDatadogSettingsLabel`) —
every test in this file genuinely exercises `AWSProfilesPicker`.

## Problem

Same as CR 76: once `AWSProfilesPicker` moves to `internal/dialog`,
none of these 12 tests can keep reaching into `a.awsProfiles.visible`/
`.table`/`.filterInput`/`.close()`/`.applyFilter()`/`.activate()` from
`internal/app`'s side of a package boundary. They need to construct
`AWSProfilesPicker` directly through `testHost`, like the three files
CR 76 already converted.

## Solution

Apply CR 76's established pattern:

- `newTestAWSProfiles(t) (*AWSProfilesPicker, *testHost)` helper,
  mirroring `newTestConnEditor`/`newTestDatadogEditor`/
  `newTestTimeRangeModal`.
- The lister injection moves from `a.listAWSProfiles = func(...) {...}`
  to `host.listAWSProfiles = func(...) {...}` — `testHost.ListAWSProfiles`
  already reads this field (built in CR 76 specifically for this
  purpose, unused until now).
- The two `a.tv.GetFocus() != a.awsProfiles.X` focus assertions
  (`TestShowAWSProfilesPopulatesTableFromInjectedLister`,
  `TestAWSProfilesSlashFocusesFilterInput`) become
  `host.focused != ap.X` — `testHost.SetFocus` already records this
  (also built in CR 76, unused until now).
- `TestAWSProfilesEnterActivatesRowRespectingFilter` currently asserts
  `a.cfg.ActiveAWSProfile` after `activate()` runs; becomes
  `host.activeAWSProfile` — `testHost.SetActiveAWSProfile` already
  records this.
- `renderedScreenText` (used by
  `TestApplyAWSProfilesFilterNarrowsRowsByName`) stays exactly where
  it is (`queues_test.go`, same package) — no promotion needed yet,
  since `awsprofiles_test.go` isn't changing package in this CR (only
  the physical move CR, later, would need to address that).
- `TestActivateAWSProfilePersistsAndUpdatesUI` — untouched.

## Scope

### In scope

- `awsprofiles_test.go`: add `newTestAWSProfiles(t)`; convert all 12
  pure-overlay tests to construct `AWSProfilesPicker` via `testHost`
  instead of `New(config.Default())`.
- `gofmt`/`go vet`/`go build`/`go test` clean across `tui/`.

### Out of scope

- `TestActivateAWSProfilePersistsAndUpdatesUI` — stays as a real
  `App`/`Host` integration test, unmodified.
- The physical move of any of the 10 overlay types into
  `internal/dialog`, and `testHost`'s eventual relocation there — next
  CR, now fully unblocked on the test-infrastructure side (all 4 files
  with dedicated overlay test coverage will be using `testHost`).
- Adding test coverage for the 5 overlays with none today
  (`confirmDialog`, `movePicker`, `sendMessageOverlay`,
  `messageFilter`, `themePicker`) — separate scope, not part of this
  migration.
- Any production-code behavior change — test infrastructure only.

## Definition of done

1. `go build ./...` and `go test ./...` pass in `tui/`.
2. All 12 pure-overlay tests in `awsprofiles_test.go` construct
   `AWSProfilesPicker` directly via `testHost`, with zero remaining
   `New(config.Default())` calls except in
   `TestActivateAWSProfilePersistsAndUpdatesUI`.
3. `gofmt -l` reports nothing; `go vet ./...` clean.
4. No production-code behavior change.
