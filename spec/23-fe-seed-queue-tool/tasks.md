# Tasks — FE 23: seed-queue dev tool

Plan: [plan.md](plan.md)

1. [x] Create `tui/internal/seed/seed.go` with `Sender`, `Run`, and sample
   message generation.
2. [x] Add `tui/internal/seed/seed_test.go` covering count, JSON validity/
   sequential IDs, progress callback, and stop-on-error.
3. [x] Create `tui/cmd/seedqueue/main.go` (CLI wrapper).
4. [x] Add `seed:queue` task to `Taskfile.yml`.
5. [x] Update `tui/CLAUDE.md` package layout list.
6. [x] Run `go build ./...`, `go vet ./...`, `go test ./...` in `tui/`.
7. [x] Verify live against the real broker's `orders` queue (empty before
   and after — sent 3, confirmed via browse, purged).
