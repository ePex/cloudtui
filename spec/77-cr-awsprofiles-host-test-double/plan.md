# Plan — CR 77: migrate `awsprofiles_test.go` to `testHost`

## Approach

### 1. Helper

```go
func newTestAWSProfiles(t *testing.T) (*AWSProfilesPicker, *testHost) {
	t.Helper()
	host := newTestHost()
	return NewAWSProfilesPicker(host), host
}
```

### 2. Mechanical swap, all 12 pure-overlay tests

Every occurrence of:

```go
a := New(config.Default())
a.listAWSProfiles = func(context.Context) ([]awsprofile.Profile, error) { ... }
a.awsProfiles.Show()
// ... a.awsProfiles.X ...
```

becomes:

```go
ap, host := newTestAWSProfiles(t)
host.listAWSProfiles = func(context.Context) ([]awsprofile.Profile, error) { ... }
ap.Show()
// ... ap.X ...
```

Three tests need one extra substitution beyond the mechanical
`a.awsProfiles.X` → `ap.X` swap:

- `TestShowAWSProfilesPopulatesTableFromInjectedLister`: `a.tv.GetFocus()
  != a.awsProfiles.table` → `host.focused != ap.table`.
- `TestAWSProfilesSlashFocusesFilterInput`: `a.tv.GetFocus() !=
  a.awsProfiles.filterInput` → `host.focused != ap.filterInput`.
- `TestAWSProfilesEnterActivatesRowRespectingFilter`: `a.cfg.ActiveAWSProfile
  != "personal"` → `host.activeAWSProfile != "personal"`.

`TestApplyAWSProfilesFilterNarrowsRowsByName` calls
`renderedScreenText(t, a.awsProfiles.table, 60, 10)` (helper in
`queues_test.go`, same package) — becomes `renderedScreenText(t,
ap.table, 60, 10)`, no other change; the helper itself isn't touched.

`TestRepaintAWSProfilesScrollsToTopWithManyRows` doesn't reference
`a` beyond `a.listAWSProfiles`/`a.awsProfiles` — both covered by the
mechanical swap above.

### 3. Untouched

`TestActivateAWSProfilePersistsAndUpdatesUI` — left exactly as-is
(still `a := New(config.Default())`, still calls `a.awsProfiles.activate("work")`
directly): it verifies `App`'s real `SetActiveAWSProfile` wiring
updates `a.infoPanel`/`a.settingsList` and persists to `config.yaml`,
none of which `testHost` does (it only records the call) — identical
reasoning to CR 76's `TestSaveDatadogEditorRoundTrip`.

### 4. Verification order

Convert and run `-run TestShowAWSProfiles` first (3 tests, exercises
the lister-injection + focus-assertion conversions), then
`-run TestAWSProfilesEsc|TestAWSProfilesSlash|TestAWSProfilesRefresh|TestAWSProfilesActive|TestAWSProfilesEnter`
(5 tests), then `-run TestApplyAWSProfilesFilter` (2 tests), then
`-run TestRepaintAWSProfiles` (1 test) — catches a broken conversion
in one group before it's obscured by the next. Final `gofmt -l`,
`go vet ./...`, `go build ./...`, `go test ./...` repo-wide.

## Files touched

- `internal/app/awsprofiles_test.go`

## Key decisions

- **Same pattern as CR 76, no new design** — this CR is purely
  applying already-proven infrastructure to the one remaining file;
  nothing here required a new `testHost` capability (the `listAWSProfiles`
  field, `focused` tracking, and `activeAWSProfile` recording were all
  built in CR 76 anticipating exactly this).
- **`renderedScreenText` stays in `queues_test.go`, not promoted** —
  `awsprofiles_test.go` isn't changing package in this CR, so there's
  no cross-package need yet; promoting it now would be speculative
  ahead of the CR that actually needs it (the physical move).
- **No new tests** — pure test-infrastructure migration, identical
  behavior coverage.
- **No new dependencies.**

## Definition of done

Unchanged from spec.md — `go build`/`go test`/`go vet` pass, `gofmt -l`
clean, all 12 pure-overlay tests construct `AWSProfilesPicker` via
`testHost`, `TestActivateAWSProfilePersistsAndUpdatesUI` untouched,
zero production-code behavior change.
