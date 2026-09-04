# Tasks

Each task is an independent, isolated diff. `go build ./...`/`go vet
./...`/`go test ./...` must pass after every task; `gofmt` before
every commit.

1. [x] New `internal/view/statuscell.go` (`clearTableBody`,
   `showStatusCell`, per `plan.md`) + `statuscell_test.go` testing both
   directly. Not yet called from any view.

2. [x] `ssmparams.go`: `showError`/`showStatus` rewritten to call
   `showStatusCell`; `repaint()`'s inline clear-loop replaced with
   `clearTableBody(pv.table)`. `ssmparams_test.go` unchanged and
   passing.

3. [x] `secrets.go`: same rewrite. `secrets_test.go` unchanged and
   passing.

4. [x] `logs.go`: same rewrite. `logs_test.go` unchanged and passing.

5. [ ] `codepipelinelist.go`: same rewrite (column 0, static title).
   `codepipelinelist_test.go` unchanged and passing.

6. [ ] `codepipelinedetail.go`: same rewrite (column 0, per-pipeline
   dynamic title, single `stages` field nulled) — plus `Open()` and
   `Render()`'s inline clear-loops also replaced with
   `clearTableBody(dv.table)`. `codepipelinedetail_test.go` unchanged
   and passing.

7. [ ] Merge-back: document `clearTableBody`/`showStatusCell` as a
   "Notable design decision worth preserving" in
   `spec/03-architecture-and-package-layout/spec.md` (same section
   that documents `runAWSLoad`/`awsauth.Do` and
   `ui.SetInputFieldText`) — what the shared shape is, which 5 views
   use it, and why `queues.go` doesn't. Delete
   `spec-wip/cr-status-cell-dedup/`. Mark the PR ready for review.
