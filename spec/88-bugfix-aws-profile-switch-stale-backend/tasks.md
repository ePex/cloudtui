# Tasks — Bugfix 88: switching AWS profile doesn't refresh a Secrets-Manager-backed connection

1. [x] `connectionsecrets.go`: add `profile string` field to
   `secretBackend` (with the doc comment from plan.md explaining why
   it's captured once rather than read live); `newBackendForConn`
   captures `a.cfg.ActiveAWSProfile` into it at construction;
   `current()`/`refresh()` read `b.profile` instead of
   `b.app.cfg.ActiveAWSProfile`. `gofmt -l`, `go build ./...` clean.
   `go test ./...` will **fail** (not fail to compile) — the 3
   existing `secretBackend` tests construct it via a struct literal
   with no `profile:` set, so it silently zero-values to `""` and
   `resolvePassword` now correctly rejects it as "no profile
   selected" — expected, fixed in task 3.

2. [x] `host.go`: `SetActiveAWSProfile` rebuilds `a.backend` via
   `newBackendForConn(a, a.cfg.ActiveConn())` and calls
   `a.queuesV.SetBackend(a.backend)`, per plan.md — same shape as
   `switchConnection`/`SaveConnection`. `gofmt -l`, `go build ./...`,
   `go vet ./...` clean (this task doesn't touch anything task 1's
   test failures depend on, so `go test ./...` still shows the same
   task-1 failures, nothing new).

3. [x] `connectionsecrets_test.go`: add a `profile` parameter to
   `newTestSecretBackend`, update its 3 existing call sites
   (`TestSecretBackendListRetriesOnceOnFailure`,
   `TestSecretBackendListSurfacesErrorAfterRetryExhausted`,
   `TestSecretBackendRemoveMessageDoesNotRetryButInvalidatesCache`) to
   pass `"prof"`, matching each test's existing
   `newTestAppForSecrets("prof", ...)` call; add
   `TestSecretBackendCurrentUsesCapturedProfileNotLiveConfig` per
   plan.md. `gofmt -l`, `go build ./...`, `go vet ./...`,
   `go test ./...` clean — this resolves task 1's expected failures.

4. [x] `host_test.go`: add
   `TestSetActiveAWSProfileRebuildsSecretBackedBackend` per plan.md.
   `gofmt -l`, `go build ./...`, `go vet ./...`, `go test ./...` clean.

5. [x] Final verification pass: grep confirms `secretBackend` no
   longer reads `a.cfg`/`b.app.cfg` anywhere in
   `connectionsecrets.go`; `gofmt -l tui/` clean; `go vet ./...`
   clean; `go build ./...` and `go test ./...` pass repo-wide.

   All clean. Also grepped every other `spec/*.md` referencing
   `SetActiveAWSProfile`/`secretBackend` (24 hits across specs 56-87)
   — all historical documents describing state at time of writing,
   same precedent as CR 85-87; none updated.

   No live-verification task for this bugfix — the fix is pure backend
   wiring (no new UI surface, no tview rendering change), fully
   covered by the 2 new unit tests
   (`TestSecretBackendCurrentUsesCapturedProfileNotLiveConfig`,
   `TestSetActiveAWSProfileRebuildsSecretBackedBackend`) plus the 3
   existing `secretBackend` tests, which now pass with the profile
   correctly threaded through. Exercising this live would need a real
   AWS Secrets-Manager-backed connection configured against 2 distinct
   profiles, which isn't part of this session's available test setup.
