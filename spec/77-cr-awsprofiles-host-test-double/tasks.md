# Tasks — CR 77: migrate `awsprofiles_test.go` to `testHost`

1. [x] Added `newTestAWSProfiles(t)` to `awsprofiles_test.go`; converted
   `TestShowAWSProfilesPopulatesTableFromInjectedLister`,
   `TestShowAWSProfilesHandlesEmptyRegion`,
   `TestShowAWSProfilesHandlesListerError` to use it, including the
   `host.focused != ap.table` conversion in the first. `gofmt -l`,
   `go build ./...`, `go test ./internal/app/... -run TestShowAWSProfiles` pass.

2. [x] Converted `TestAWSProfilesEscCloses`, `TestAWSProfilesSlashFocusesFilterInput`
   (including its `host.focused != ap.filterInput` conversion),
   `TestAWSProfilesRefreshReinvokesLister`,
   `TestAWSProfilesActiveProfileMarkedWithStar`,
   `TestAWSProfilesEnterActivatesRowRespectingFilter` (including its
   `host.activeAWSProfile` conversion). `gofmt -l`, `go build ./...`,
   `go test ./internal/app/... -run 'TestAWSProfilesEsc|TestAWSProfilesSlash|TestAWSProfilesRefresh|TestAWSProfilesActive|TestAWSProfilesEnter'` pass.

3. [x] Converted `TestApplyAWSProfilesFilterNarrowsRowsByName` (including
   its `renderedScreenText(t, ap.table, ...)` call) and
   `TestApplyAWSProfilesFilterClearRestoresAll`. `gofmt -l`,
   `go build ./...`, `go test ./internal/app/... -run TestApplyAWSProfilesFilter` pass.

4. [x] Converted `TestShowAWSProfilesResetsFilterFromPreviousVisit` and
   `TestRepaintAWSProfilesScrollsToTopWithManyRows`. Left
   `TestActivateAWSProfilePersistsAndUpdatesUI` untouched. `gofmt -l`,
   `go build ./...`; all 3 tests verified passing individually (the
   4th task's combined `-run` regex didn't match
   `TestActivateAWSProfilePersistsAndUpdatesUI` as expected — its name
   contains "AWSProfile" singular, not "AWSProfiles" plural, so it was
   verified with its own separate `-run` instead).

5. [x] Final verification pass: confirmed exactly one remaining
   `New(config.Default())` call in `awsprofiles_test.go`
   (`TestActivateAWSProfilePersistsAndUpdatesUI`); `gofmt -l tui/`
   clean; `go vet ./...` clean; `go build ./...` and `go test ./...`
   pass repo-wide (all packages `ok`). No live verification needed —
   test-infrastructure only, no production-code behavior change.
