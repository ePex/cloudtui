# Tasks — CR 63: Go standards cleanup

1. [x] Replace `interface{}` with `any` in `internal/queue/jolokia/jolokia.go`,
   `internal/queue/proxy/proxy.go`, `internal/queue/backend.go`, and
   `internal/app/message_detail.go`. Reword the doc comment at
   `message_detail.go:134` to say `any` instead of `interface{}`. Verify
   with a final grep that no non-comment `interface{}` remains in these
   four files. `gofmt -l`, `go vet ./...`, `go build ./...`,
   `go test ./...` all clean. Commit.

2. [x] Create `internal/queue/jolokia/messages.go` and move
   `BrowseMessages`, `browseMessagesFull`, `browseBodies`,
   `browseMessagesFallback`, `parseBrowseItem`, `extractBrowseBody`,
   `extractMessageID`, and the `searchResponse` struct into it verbatim.
   Fix imports (`goimports -w`) in both `jolokia.go` and `messages.go`.
   `gofmt -l`, `go vet ./...`, `go build ./...`, `go test ./...` all
   clean — no test changes yet, just source motion.

3. [x] Create `internal/queue/jolokia/mutate.go` and move `PurgeQueue`,
   `removeMatchingMessages`, `RemoveMessage`, `MoveMessage`,
   `MoveAllMessages`, `SendMessage`, `bulkItem`, and `bulkResponseItem`
   into it verbatim. Fix imports in `jolokia.go` and `mutate.go`.
   `gofmt -l`, `go vet ./...`, `go build ./...`, `go test ./...` all
   clean. Commit tasks 2+3 together as one "split jolokia.go" commit
   (both are pure code motion toward the same end state).

4. [x] Split `internal/queue/jolokia/jolokia_test.go` to match: move
   tests exercising the functions now in `messages.go` into a new
   `messages_test.go`, and tests exercising the functions now in
   `mutate.go` into a new `mutate_test.go`, leaving tests for
   `List`/`searchQueues`/`readMetrics`/etc. in `jolokia_test.go`. Fix
   imports in all three test files. `go test ./...` clean, same test
   count/names as before the split (verify with `go test -v
   ./internal/queue/jolokia/... | grep -c '^--- PASS'` before/after
   matching). Commit.

5. [x] Final verification pass: confirm `internal/queue/jolokia/jolokia.go`
   is roughly a third of its original 949 lines, `messages.go` and
   `mutate.go` both exist alongside it, `gofmt -l tui/` and
   `go vet ./...` are clean repo-wide, `go build ./...` and
   `go test ./...` pass repo-wide. No commit needed unless this surfaces
   something to fix.
