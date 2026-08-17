# Tasks — CR 79: split view-wiring trampolines into `viewwiring.go`

1. [x] Created `internal/app/viewwiring.go` with the package-level doc
   comment per plan.md's template. `gofmt -l`, `go build ./...` clean.

2. [x] Moved `openMessages`/`wireQueuesOpensMessages` from
   `messages.go` into `viewwiring.go`. `gofmt -l`, `go build ./...`
   clean (no import changes needed in either file).

3. [x] Moved `openMessageDetail`/`wireMessagesOpensDetail` from
   `message_detail.go` into `viewwiring.go`. `gofmt -l`, `go build
   ./...` clean.

4. [x] Moved `openParamDetail`/`wireSSMParamsOpensDetail` from
   `paramdetail.go` into `viewwiring.go`; added the `awsssm` import to
   `viewwiring.go`. `gofmt -l`, `go build ./...` clean.

5. [x] Moved `openSecretDetail`/`wireSecretsOpensDetail` from
   `secretdetail.go` into `viewwiring.go`; added the `awssecrets`
   import to `viewwiring.go`. `gofmt -l`, `go build ./...` clean.

6. [x] Moved `openLogSearch`/`wireLogsOpensSearch` from `logsearch.go`
   into `viewwiring.go`. `gofmt -l`, `go build ./...` clean (no new
   imports needed).

7. [x] Moved `openLogEventDetail`/`wireLogSearchOpensEventDetail` from
   `logdetail.go` into `viewwiring.go`; added the `awslogs` import to
   `viewwiring.go`. `gofmt -l`, `go build ./...` clean.

8. [x] Moved `openDatadogLogDetail`/`wireDatadogLogsOpensDetail` from
   `datadoglogdetail.go` into `viewwiring.go`; added the `datadoglogs`
   import to `viewwiring.go`. `gofmt -l`, `go build ./...` clean.

9. [x] Moved `openCodePipelineDetail`/`wireCodePipelineListOpensDetail`
   from `codepipelinedetail.go` into `viewwiring.go`. `gofmt -l`,
   `go build ./...` initially failed with `"strings" imported and not
   used` in `codepipelinedetail.go` (its only remaining use of
   `strings` was inside the now-moved methods) — removed the import;
   clean after.

10. [x] Final verification pass: grep confirms zero remaining
    `^func (a \*App)` declarations in the 8 origin files; `gofmt -l
    tui/` clean; `go vet ./...` clean; `go build ./...` and
    `go test ./...` pass repo-wide (all packages `ok`). No live
    verification needed — pure same-package file move, per spec.md.
