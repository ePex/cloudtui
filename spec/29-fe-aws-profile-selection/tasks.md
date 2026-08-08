# Tasks — FE 29: select an active AWS profile, filter the list, `:ap`

Plan: [plan.md](plan.md)

1. [x] `config.Config.ActiveAWSProfile` field + round-trip/default tests.
2. [x] `infoPanelText` third line + tests.
3. [x] Filter input, filter/repaint split (`awsProfilesAll`/`Filtered`),
   `applyAWSProfilesFilter` + tests.
4. [x] `activateAWSProfile` (persist, info panel, close, status bar) +
   ⭐ marker + tests.
5. [x] `SetSelectedFunc` wired to the *filtered* row, not the unfiltered
   index + regression test for that specific case.
6. [x] `:ap` / `:awsprofiles` command + tests (including "works from a
   non-Settings view", mirroring `:aq`'s test).
7. [x] `config.example.yaml`: document `activeAWSProfile`.
8. [x] `go build ./...`, `go vet ./...`, `go test ./...` — all pass.
9. [x] Manual verification per `verify-live`: backed up `config.yaml`,
   drove the real binary — `:ap` from the Log view, filtered 69 real
   profiles down to 2 matches, activated one, confirmed the info panel/
   status bar/⭐-on-reopen all updated correctly — then restored
   `config.yaml` byte-for-byte.
10. [x] Updated `spec/28-fe-aws-profile-discovery/spec.md`'s "out of
    scope" section to point at this spec (it explicitly ruled out
    exactly what this spec does).
