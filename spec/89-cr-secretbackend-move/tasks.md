# Tasks — CR 89: move `secretBackend` out of `internal/app` into `internal/queue/secretbackend`

1. [x] Create `internal/queue/secretbackend/secretbackend.go` with
   `secretCache` (unchanged), `SecretResolver`/`NewSecretResolver`/
   `Resolve`/`Invalidate`, `passwordSecretName`/`connWithPassword`/
   `buildBackend` (unchanged bodies), `New`, and `Backend` (with its
   `Profile()` accessor and all 9 `queue.Backend` methods) — all per
   plan.md. This is a new, self-contained package;
   `internal/app/connectionsecrets.go` is untouched in this task.
   `gofmt -l`, `go build ./...`, `go vet ./...` clean.

2. [x] Create `internal/queue/secretbackend/secretbackend_test.go`:
   port the 7 existing `connectionsecrets_test.go` tests, renamed and
   adapted to construct `*SecretResolver`/`*Backend` directly (no
   `*App`) per plan.md; `secretsFakeBackend` → `fakeBackend`. Do not
   port `TestSecretBackendCurrentUsesCapturedProfileNotLiveConfig`
   (dropped per plan.md — the regression it guarded against is now
   structurally impossible). `gofmt -l`, `go build ./...`,
   `go vet ./...`, `go test ./internal/queue/secretbackend/...` clean.

3. [x] `app.go`: struct field `secretCache *secretCache` →
   `secretResolver *secretbackend.SecretResolver`; construct it right
   after `a.revealSecret = awssecrets.Reveal`; update the 2
   `newBackendForConn` call sites (`New()`, `switchConnection`) to
   `secretbackend.New(a.secretResolver, ..., ...)` per plan.md; add
   the `secretbackend` import. `host.go`: update the 2
   `newBackendForConn` call sites (`SaveConnection`,
   `SetActiveAWSProfile`) the same way; add the `secretbackend`
   import. Delete `internal/app/connectionsecrets.go` and
   `internal/app/connectionsecrets_test.go` (content fully relocated
   in tasks 1-2). `gofmt -l`, `go build ./...` clean.

   Correction to this task's original wording: `go vet ./...` does
   **not** stay clean here as written — `vet` typechecks test files
   too, so `host_test.go`'s still-broken reference to the deleted
   `*secretBackend` type fails `vet` itself, not just `go test`. Same
   underlying "expected, fixed in task 4" situation, just surfacing on
   an earlier command than originally described.

4. [x] `host_test.go`: adapt
   `TestSetActiveAWSProfileRebuildsSecretBackedBackend` to
   `*secretbackend.Backend` + `.Profile()` per plan.md; add the
   `secretbackend` import. `gofmt -l`, `go build ./...`,
   `go vet ./...`, `go test ./...` clean — resolves task 3's expected
   failure.

5. [x] Final verification pass: grep confirms zero remaining
   `secretCache`/`secretBackend`/`newBackendForConn`/`resolvePassword`
   references anywhere in `internal/app`; `gofmt -l tui/` clean;
   `go vet ./...` clean; `go build ./...` and `go test ./...` pass
   repo-wide; confirm zero import cycle (`go list -deps
   ./internal/app/... ./internal/queue/...` succeeds).

   All clean. Also updated `tui/CLAUDE.md`'s package-layout section to
   add the new `internal/queue/secretbackend/` entry (in scope here,
   since this CR is what creates the package — unlike PR #39's
   unrelated doc-only fix). Noticed `internal/queue/proxy/` is also
   missing from that same list (a pre-existing gap, not caused by this
   CR) — left untouched here, same "no drive-by changes" reasoning
   that made the `internal/view` gap its own separate PR (#39) rather
   than folding it into whichever CR happened to notice it; worth a
   tiny follow-up doc PR if wanted.

   Grepped every other `spec/*.md` referencing the old symbols (24
   hits across specs 21/22/56/58/64/66/68/74/78/83-88) — all
   historical documents describing state at time of writing, same
   precedent as CR 85-88; none updated.

   No live-verification task — same reasoning as bugfix 88: pure
   backend-construction plumbing, no UI surface, fully covered by the
   relocated + adapted unit tests.
