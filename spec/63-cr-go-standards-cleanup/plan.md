# Plan — CR 63: Go standards cleanup

## Approach

Two independent changes, done as two separate commits (per CLAUDE.md
"one logical change per commit"), each verified with a full
build+vet+test pass before moving to the next.

### 1. `interface{}` → `any`

Mechanical, done with `gofmt -r 'interface{} -> any' -w <file>` on the
four in-scope files, then a manual pass over each diff to:
- reword the one doc comment that literally spells out `interface{}`
  (`internal/app/message_detail.go:134`) to say `any` instead.
- confirm no `interface{}` survives in a context `gofmt -r` can't rewrite
  (e.g. inside a string literal or a generated/vendored file — not
  expected here, but worth a final grep).

No test changes: `any` is a type alias for `interface{}`, identical at
compile time, so nothing observable changes.

### 2. Split `jolokia.go`

Order of operations, all within `internal/queue/jolokia/`:

1. Create `messages.go` and `mutate.go` as new files in the package.
2. Move the browsing/parsing group (`BrowseMessages`,
   `browseMessagesFull`, `browseBodies`, `browseMessagesFallback`,
   `parseBrowseItem`, `extractBrowseBody`, `extractMessageID`,
   `searchResponse`) into `messages.go` verbatim — cut/paste, no
   signature or body changes.
3. Move the mutation group (`PurgeQueue`, `removeMatchingMessages`,
   `RemoveMessage`, `MoveMessage`, `MoveAllMessages`, `SendMessage`,
   `bulkItem`, `bulkResponseItem`) into `mutate.go` verbatim.
4. Leave `Client`, `NewClient`, `List`, `searchQueues`, `readMetrics`,
   `queueNameFromMBean`, `splitMBean`, `splitComma`, `setHeaders` in
   `jolokia.go`, along with `searchResponse`, `bulkItem`, and
   `bulkResponseItem` — actual usage check during implementation showed
   these three are only referenced by the `List`/metrics group, not by
   the mutation group as originally guessed here. `execSimple` moved to
   `mutate.go` instead of staying in `jolokia.go`, since it's only used
   by `PurgeQueue`.
5. Run `goimports -w` on all three files (import lists will need
   adjusting — e.g. `messages.go` and `mutate.go` each need their own
   `encoding/json`, `context`, etc. based on what actually lands there).
6. `gofmt -l`, `go vet ./...`, `go build ./...`, `go test ./...` in
   `tui/`.

The existing test file `jolokia_test.go` stays as one file (per
`tui/CLAUDE.md`: "one `_test.go` file per source file, same package")
— since the tests will now span three source files, `jolokia_test.go`
should also be split to match: tests for `List`/metrics stay in
`jolokia_test.go`, tests for browsing move to `messages_test.go`, tests
for mutation move to `mutate_test.go`. Verified during implementation by
matching each existing test function to the source function it exercises
(mirroring the file split, not a fresh test-writing pass).

## Files touched

- `internal/queue/jolokia/jolokia.go` (shrinks)
- `internal/queue/jolokia/messages.go` (new)
- `internal/queue/jolokia/mutate.go` (new)
- `internal/queue/jolokia/jolokia_test.go` (shrinks)
- `internal/queue/jolokia/messages_test.go` (new)
- `internal/queue/jolokia/mutate_test.go` (new)
- `internal/queue/proxy/proxy.go` (`any` only)
- `internal/queue/backend.go` (`any` only)
- `internal/app/message_detail.go` (`any` only)

## Key decisions / trade-offs

- **Two commits, not one.** The `any` rename and the file split are
  unrelated changes that happen to have been found in the same audit;
  keeping them separate keeps each diff reviewable and revertable
  independently, per CLAUDE.md's "small, focused commits."
- **`jolokia_test.go` split mirrors the source split** rather than being
  left as one large test file, to keep the "one test file per source
  file" convention intact — otherwise the split would immediately create
  a CLAUDE.md violation in the thing meant to fix a different one.
- **No behavior change, so no new tests.** Both changes are pure
  syntax/organization; existing tests (moved, not rewritten) are the
  verification. `go test ./...` passing after the move is the acceptance
  bar — no live/manual verification needed (nothing UI- or
  broker-behavior-facing changes).
- **No new dependencies.**

## Definition of done

Unchanged from spec.md — `gofmt -l`/`go vet` clean, `go build`/`go test`
pass, zero remaining `interface{}` in the in-scope files, `jolokia.go`
reduced to roughly a third of its current size with `messages.go` and
`mutate.go` alongside it.
