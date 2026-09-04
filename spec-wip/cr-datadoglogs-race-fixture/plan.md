# Plan

## Approach

### 1. New `newTestDatadogLogsViewWithDrawSignal` helper

Added to `datadoglogs_test.go`, right next to `newTestDatadogLogsView`:

```go
// newTestDatadogLogsViewWithDrawSignal is newTestDatadogLogsView's
// draw-signaling counterpart — see queues_test.go's drawSignalingHost/
// newTestQueuesViewWithDrawSignal for why this exists. Needed here
// specifically for tests that call SetCurrentOption on a dropdown
// already wired with the real onSelect (via rebuildFilterOptions/
// refreshFilterDropdowns): tview's DropDown.SetCurrentOption invokes
// that callback synchronously, which calls search() and spawns a
// goroutine neither the caller nor tview waits for — without this,
// that goroutine leaks past the test and its later QueueUpdateDraw
// call races the next thing that touches the same dropdown.
func newTestDatadogLogsViewWithDrawSignal(t *testing.T, bufSize int) (*drawSignalingHost, *DatadogLogsView) {
	t.Helper()
	base := newFakeViewHost()
	host := &drawSignalingHost{fakeViewHost: base, drawn: make(chan struct{}, bufSize)}
	timeRangeModal := dialog.NewTimeRangeModal(host)
	return host, NewDatadogLogsView(host, timeRangeModal, func(datadoglogs.LogEvent) {})
}
```

### 2. Fix the 2 leaking tests

`TestRebuildFilterOptionsSelectingAnOptionRefocusesTable`:

```go
func TestRebuildFilterOptionsSelectingAnOptionRefocusesTable(t *testing.T) {
	host, dv := newTestDatadogLogsViewWithDrawSignal(t, 1)
	dv.results = []datadoglogs.LogEvent{{Service: "activemq"}}
	dv.rebuildFilterOptions()
	host.SetFocus(dv.serviceFilterDD)

	// Simulate picking "activemq" (options: 0="(any)", 1="activemq").
	// This fires the real onSelect wired by refreshFilterDropdowns,
	// which calls search() in a goroutine — wait for its
	// QueueUpdateDraw call to land so that goroutine can't leak past
	// this test (it did, before this fix; see spec/03).
	dv.serviceFilterDD.SetCurrentOption(1)
	<-host.drawn

	if got := host.focused; got != dv.table {
		t.Errorf("focus after selecting a Service option = %v, want the results table", got)
	}
}
```

`TestHandleFacetDiscoveryResultPreservesCurrentSelectionWhenValuesArrive`:

```go
func TestHandleFacetDiscoveryResultPreservesCurrentSelectionWhenValuesArrive(t *testing.T) {
	host, dv := newTestDatadogLogsViewWithDrawSignal(t, 1)
	dv.results = []datadoglogs.LogEvent{{Service: "activemq"}}
	dv.rebuildFilterOptions()
	dv.serviceFilter = "activemq"
	// Same SetCurrentOption-leaks-a-search()-goroutine hazard as
	// TestRebuildFilterOptionsSelectingAnOptionRefocusesTable above —
	// wait for it before continuing.
	dv.serviceFilterDD.SetCurrentOption(1)
	<-host.drawn

	dv.handleFacetDiscoveryResult(dv.knownServices, []string{"activemq", "bar-proxy"}, nil)

	if dv.serviceFilter != "activemq" {
		t.Errorf("serviceFilter = %q, want %q (unchanged)", dv.serviceFilter, "activemq")
	}
	_, selected := dv.serviceFilterDD.GetCurrentOption()
	if selected != "activemq" {
		t.Errorf("dropdown's selected option = %q, want %q", selected, "activemq")
	}
}
```

Both keep their existing assertions unchanged — only the host
construction and the added `<-host.drawn` wait change.

## Files touched

- `tui/internal/view/datadoglogs_test.go` only: the new helper, and
  the two tests' host construction + one added wait each.

## Testing

- The fix itself is entirely inside `datadoglogs_test.go` — no
  production code, no new behavior to unit-test beyond the existing
  assertions (unchanged) in the two fixed tests.
- Verification is repeated `-race` runs, since a leaked-goroutine race
  is inherently timing-dependent and a single clean run doesn't prove
  it's fixed (the bug itself passed plenty of individual runs before
  being caught). Plan: `go test -race ./internal/view/... -count=1`
  at least 10 times in a loop, plus targeted repeated runs of exactly
  the 2 fixed tests and their immediate neighbors in file order (the
  ones various runs "blamed" this session:
  `TestApplyFilterOptionsDoesNotFireCallbackDuringReconciliation`,
  `TestHandleFacetDiscoveryResultMergesNewValues`,
  `TestHandleFacetDiscoveryResultNoopOnError`,
  `TestHandleFacetDiscoveryResultSkipsEmptyValues`), all green with no
  `WARNING: DATA RACE` output.
- Full `go build`/`go vet`/`go test ./...` (non-race) must also stay
  green throughout, same as every other CR this session.

## Key decisions / trade-offs

- **Fix the 2 leak points, not `fakeViewHost.QueueUpdateDraw`'s
  semantics.** See `spec.md`'s "Out of scope" — confirmed via direct
  investigation that the fake's inline execution isn't itself the bug;
  it's what makes the *already-existing* leak visible quickly (and
  therefore catchable by `-race`), but the actual defect is the leaked
  goroutine having no synchronization at all, which `drawSignalingHost`
  fixes directly at the source.
- **Reuse `drawSignalingHost`, don't invent a new mechanism.** Same
  type already used by `queues_test.go`, `ssmparams_test.go`,
  `pipelinewatcher_test.go`, etc. this session for exactly this
  "don't let an async goroutine outlive the test" purpose — no new
  test infrastructure needed.
- **`TestApplyFilterOptionsDoesNotFireCallbackDuringReconciliation` and
  the `TestHandleFacetDiscoveryResult*` tests near it are left
  untouched** — confirmed they're not the source of any leak
  themselves (see `spec.md`), so "fixing" them would just be
  cosmetic/unnecessary.
