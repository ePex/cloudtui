# Plan — CR 79: split view-wiring trampolines into `viewwiring.go`

## Approach

### 1. Create `internal/app/viewwiring.go`

Header comment naming the file's purpose, then the 16 methods moved
verbatim (unchanged bodies, unchanged doc comments) in the same order
as spec.md's table — grouped by pair, one blank line between pairs:

```go
// viewwiring.go holds the (a *App) trampolines that wire one resource
// view to open another (queues -> messages -> message detail, and the
// equivalent list/detail pairs for SSM params, secrets, CloudWatch
// Logs, Datadog Logs, and CodePipeline). Each pair reaches directly
// into the target view's unexported state — kept together, and kept
// out of the view types' own files, since neither view "owns" the
// wiring between them; App does. See spec/79.
package app

// (openMessages, wireQueuesOpensMessages — from messages.go)
// (openMessageDetail, wireMessagesOpensDetail — from message_detail.go)
// (openParamDetail, wireSSMParamsOpensDetail — from paramdetail.go)
// (openSecretDetail, wireSecretsOpensDetail — from secretdetail.go)
// (openLogSearch, wireLogsOpensSearch — from logsearch.go)
// (openLogEventDetail, wireLogSearchOpensEventDetail — from logdetail.go)
// (openDatadogLogDetail, wireDatadogLogsOpensDetail — from datadoglogdetail.go)
// (openCodePipelineDetail, wireCodePipelineListOpensDetail — from codepipelinedetail.go)
```

Imports: whatever the 16 method bodies actually reference — a subset
of each origin file's current import list (`fmt`, `strings` appear in
most; `queue.Message`/`awsssm.Parameter`/`awssecrets.Secret`/
`awslogs.LogEvent`/`datadoglogs.LogEvent` for the `openXDetail`
parameter types). Resolved per-file during implementation by running
`goimports`/`go build` after each move and fixing whatever it flags,
same as every prior CR's mechanical-move steps — not worth
hand-computing 16 methods' exact import sets in advance when the
compiler verifies it exactly.

### 2. Remove from the 8 origin files

Delete each pair's two methods (and their doc comments, now living in
`viewwiring.go` instead) from `messages.go`, `message_detail.go`,
`paramdetail.go`, `secretdetail.go`, `logsearch.go`, `logdetail.go`,
`datadoglogdetail.go`, `codepipelinedetail.go`. Run `goimports`/
`go build` per file afterward — an import used only by the
now-removed methods (e.g. `sort` in `messages.go`, if nothing else in
the file sorts) becomes unused and must be dropped; imports still
needed by the view type's own remaining code stay untouched.

### 3. Verification order

One file at a time (matches spec/79's own precedent from every prior
CR in this sequence): move a pair, `gofmt -l`, `go build ./...` —
confirm the only errors (if any) are unused imports in the file just
touched, fix those, re-build clean, move to the next pair. After all
8: `go vet ./...`, `go test ./...` repo-wide.

## Files touched

- `internal/app/viewwiring.go` (new)
- `messages.go`, `message_detail.go`, `paramdetail.go`,
  `secretdetail.go`, `logsearch.go`, `logdetail.go`,
  `datadoglogdetail.go`, `codepipelinedetail.go` (2 methods removed
  each, imports trimmed as needed)

## Key decisions

- **One consolidated file, not 8 small `*_wiring.go` files** — 16
  methods total (2 per pair) is small enough that one file stays
  easy to scan, and it makes the "this is App's cross-view wiring
  layer" concern visible as a single unit rather than scattered
  same-named files. Mirrors `host.go`'s existing precedent (a single
  file grouping a cohesive set of small `App` methods) rather than
  inventing a new per-concern-file convention.
- **Bodies unchanged, byte-for-byte** — this is explicitly a move, not
  a rewrite; any behavior change here would conflate two different
  kinds of risk (relocation vs. logic change) in one CR, and the
  actual redesign (these methods becoming `Host` interface methods
  calling into the target view's own exported API) is deliberately
  future work, not this CR's.
- **`settings.go`/`codepipelinewatch.go` excluded** — per spec.md's
  Solution section; neither fits this CR's "cross-view trampoline"
  pattern.
- **No new tests** — pure relocation, zero behavior change; existing
  tests continue exercising the same code paths under the same
  receiver/method names, just declared in a different file.
- **No new dependencies.**

## Definition of done

Unchanged from spec.md — `go build`/`go test`/`go vet` pass, `gofmt -l`
clean, all 16 methods live in `viewwiring.go`, the 8 origin files
declare no `(a *App)` method, zero behavior change.
