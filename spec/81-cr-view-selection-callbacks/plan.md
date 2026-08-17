# Plan — CR 81: constructor-injected selection callbacks

## Approach

### 1. The 8 constructor signature changes

Each gains one new trailing parameter, typed to match the
already-declared `ViewHost` method exactly (CR 80) — meaning `app.go`
passes `a.OpenX` directly, no wrapper closure needed anywhere. `a *App`
stays the receiver-construction param throughout (unchanged).

| # | Constructor (before → after) | Resolution moved from `wireXOpensY` (unchanged logic) | `app.go` call site |
|---|---|---|---|
| 1 | `newQueuesView(a *App, b queue.Backend)` → `+ onSelect func(queueName string)` | `cell := qv.table.GetCell(row, 0); if cell == nil \|\| cell.Text == "" { return }; onSelect(cell.Text)` | `newQueuesView(a, a.backend, a.OpenMessages)` |
| 2 | `newMessagesView(a *App)` → `+ onSelect func(queueName string, msg queue.Message)` | `msgIdx := row - 1; if msgIdx < 0 \|\| msgIdx >= len(mv.msgs) { return }; onSelect(mv.queueName, mv.msgs[msgIdx])` | `newMessagesView(a, a.OpenMessageDetail)` |
| 3 | `newSSMParamsView(a *App)` → `+ onSelect func(param awsssm.Parameter)` | `idx := row - 1; if idx < 0 \|\| idx >= len(sv.filtered) { return }; onSelect(sv.filtered[idx])` | `newSSMParamsView(a, a.OpenParamDetail)` |
| 4 | `newSecretsView(a *App)` → `+ onSelect func(secret awssecrets.Secret)` | same shape as #3, `sv.filtered` | `newSecretsView(a, a.OpenSecretDetail)` |
| 5 | `newLogsView(a *App)` → `+ onSelect func(logGroupName string)` | `idx := row - 1; if idx < 0 \|\| idx >= len(lv.filtered) { return }; onSelect(lv.filtered[idx].Name)` | `newLogsView(a, a.OpenLogSearch)` |
| 6 | `newLogSearchView(a *App)` → `+ onSelect func(event awslogs.LogEvent)` | `idx := row - 1; if idx < 0 \|\| idx >= len(sv.results) { return }; onSelect(sv.results[idx])` | `newLogSearchView(a, a.OpenLogEventDetail)` |
| 7 | `newDatadogLogsView(a *App)` → `+ onSelect func(event datadoglogs.LogEvent)` | same shape as #6, `dv.results` | `newDatadogLogsView(a, a.OpenDatadogLogDetail)` |
| 8 | `newCodePipelineListView(a *App)` → `+ onSelect func(pipelineName string)` | `idx := row - 1; if idx < 0 \|\| idx >= len(lv.filtered) { return }; onSelect(lv.filtered[idx].Name)` | `newCodePipelineListView(a, a.OpenCodePipelineDetail)` |

Every `onSelect` parameter's type is exactly the corresponding
`ViewHost` method's signature (already declared, CR 80) — confirmed
for all 8, no adapter/wrapper closure required at any call site.

### 2. Each constructor: add one `SetSelectedFunc` call

Inside each `newXView`, add:

```go
xv.table.SetSelectedFunc(func(row, _ int) {
	// exact resolution logic from the table above, using xv's own fields
	onSelect(...)
})
```

Placed alongside the view's other `table.SetInputCapture`/
`SetSelectedFunc` setup in the constructor (each file already has a
natural spot — right after the table's other input wiring, going by
the existing code's organization).

### 3. `viewwiring.go`: delete all 8 `wireXOpensY` methods

Nothing else in the file changes — the 8 `OpenX` methods keep their
current bodies verbatim (page switch, focus, context-panel handling
where the target isn't self-sufficient — see spec.md's Solution
section for which 4 of 6 targets already are).

### 4. `app.go`'s `New()`

Remove the 8 `a.wireXOpensY()` calls and their "must exist first"
comments; add the matching `onSelect` argument to each of the 8
source-view constructor calls (table above). No reordering forced —
the old ordering (source views, then target views, then wiring) can
collapse to just "construct every view, in whatever order groups
them most readably" since nothing depends on call-time field reads
anymore. Keep the existing view-construction order as-is (source
before its target, matching today's file) to keep the diff minimal —
reordering isn't required by the fix, so don't do it just because we
now could.

### 5. `datadoglogdetail.go` bug fix

```go
// before
dv.app.pendingCloudWatchPattern = fmt.Sprintf("%q", id)
// after
dv.app.SetPendingCloudWatchPattern(fmt.Sprintf("%q", id))
```

### 6. Verification order

One pair at a time: signature change + resolution logic move +
`viewwiring.go` deletion + `app.go` call-site update, `go build ./...`
after each (catches any mismatched signature immediately). After all
8: the `datadoglogdetail.go` fix, then `go vet ./...`, `go test
./...` repo-wide, then live verification (`verify-live`) exercising
all 8 selection paths against a real broker/AWS profile/Datadog
config.

## Files touched

- `queues.go`, `messages.go`, `ssmparams.go`, `secrets.go`, `logs.go`,
  `logsearch.go`, `datadoglogs.go`, `codepipelinelist.go` (constructor
  signature + internal `SetSelectedFunc` wiring)
- `viewwiring.go` (8 `wireXOpensY` methods removed)
- `app.go` (`New()`'s 8 construction calls + removal of the 8 wiring
  calls)
- `datadoglogdetail.go` (1-line bug fix)

## Key decisions

- **`onSelect`'s type always matches the existing `ViewHost` method
  exactly** — lets `app.go` pass `a.OpenX` directly with zero
  adapter closures, and means the view itself never needs to know
  it's "opening messages"/"opening a param detail" — it only knows
  "call this when a row is selected", same abstraction level dialogs
  already use.
- **Don't reorder `New()`'s construction beyond removing the wiring
  calls** — the ordering constraint disappearing doesn't obligate a
  cleanup pass; reordering working code without a concrete reason is
  exactly the "no drive-by changes" `CLAUDE.md` calls out.
- **`OpenX` methods in `viewwiring.go` are untouched** — this CR only
  changes how the callback gets *registered*, not what it does once
  called.
- **`datadoglogdetail.go`'s fix folded in, not a separate CR** — one
  line, same file this CR is already touching, same "reaches into
  `*App` directly instead of the exported setter" theme.
- **No new tests** — pure relocation of existing logic (row resolution
  logic is byte-for-byte identical, just moved); existing tests for
  each view's selection behavior continue exercising the same paths.
  Live verification substitutes for new automated coverage here per
  `tui/CLAUDE.md`'s "where behavior can't be fully covered by unit
  tests" guidance — selection-to-navigation is exactly the kind of
  real-`tview`-event-loop behavior this project's tests don't
  simulate.
- **No new dependencies.**

## Definition of done

Unchanged from spec.md — `go build`/`go test`/`go vet` pass, `gofmt -l`
clean, all 8 source views wire selection internally via an injected
callback, `viewwiring.go` has no `wireXOpensY` methods left, all 8
selection paths live-verified, the `datadoglogdetail.go` bug fixed.
