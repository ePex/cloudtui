# Plan — CR 84: move the 14 `ui.ViewHost`-adopted views into `internal/view`

## Approach

### 0. Sequencing strategy — stage the risk out of the move

A 7,500+ LOC, 28-file move is too large to verify as one step. Since
Go doesn't care about export status for same-package access, every
piece of this CR *except the actual file relocation* can be done
while the 14 files still live in `internal/app`, verified
incrementally the same way CR 82/83 were:

1. **Phase 1 — exports.** Add/rename the ~30 symbols from spec.md's
   table, still inside `internal/app`. Each rename is safe and
   inert on its own (same-package callers don't care about
   capitalization).
2. **Phase 2 — capitalize the 14 types + constructors.** Also inert
   inside `internal/app` — purely mechanical, `gofmt`-verified.
3. **Phase 3 — the test fake + all 161 rewrites**, plus splitting out
   the 15 wiring-integration tests into a new
   `internal/app/viewwiring_test.go` (which stays in `internal/app`
   permanently, unaffected by Phase 4). Still inside `internal/app`.
4. **Phase 4 — the actual move**, now nearly mechanical: by this
   point every one of the 14 files is self-contained (exported
   surface, exported names, self-sufficient tests) — moving them is
   `git mv` + a package-line change + updating `app.go`'s 14 field
   types/constructor calls to the `view.` prefix.

By the end of Phase 3, `internal/app` and the eventual `internal/view`
are already logically separate; Phase 4 just makes it physically true.

### 1. Phase 1 — exports (per spec.md's table)

For the pure renames (`open`→`Open`, `render`→`Render` where no
folding is needed, `repaint`... see below), update the method
signature and every call site within the same file, then update the
1–3 external call sites in `app.go`/`host.go`/`viewwiring.go`/
`codepipelinewatch.go`.

For the folded methods, the new bodies:

```go
// messages.go — replaces the entire body OpenMessages used to inline.
// queueName-change detection, filter/quickSearch reset, and load all
// move here verbatim; only the page-switch/focus/context-panel chrome
// stays in viewwiring.go.
func (mv *messagesView) Open(queueName string) {
	if mv.queueName != queueName {
		mv.filter = queue.MessageFilter{}
		mv.quickSearch = ""
		mv.searchInput.SetText("")
	}
	mv.queueName = queueName
	mv.updateTitle()
	mv.setHeader()
	mv.load()
}

// messages.go — replaces host.go's ApplyMessagesFilter body.
func (mv *messagesView) ApplyFilter(f queue.MessageFilter) {
	mv.filter = f
	mv.updateTitle()
	mv.load()
}

// message_detail.go — folds in the title-set OpenMessageDetail used
// to do via raw .textView access.
func (dv *messageDetailView) Render(queueName string, msg queue.Message) {
	dv.queueName = queueName
	dv.msg = msg
	dv.textView.SetTitle(fmt.Sprintf(" Message Details — %s ", queueName))
	// ...existing render body unchanged below this point...
}

// paramdetail.go — same fold; existing render() logic unchanged,
// SetTitle line added using the param already in scope.
func (dv *paramDetailView) Render(param awsssm.Parameter) {
	dv.param = param
	dv.displayed = param.Type != awsssm.TypeSecureString
	dv.textView.SetTitle(fmt.Sprintf(" Parameter — %s ", param.Name))
	dv.renderBody()
}

// secretdetail.go — same fold, mirrors paramdetail.go's shape.
func (dv *secretDetailView) Render(secret awssecrets.Secret) {
	dv.secret = secret
	dv.textView.SetTitle(fmt.Sprintf(" Secret — %s ", secret.Name))
	dv.renderBody()
}

// codepipelinelist.go — replaces codepipelinewatch.go's direct
// `.all`/`.repaint(.all)` reach-in; no field exposed.
func (lv *codePipelineListView) Repaint() { lv.repaint(lv.all) }
```

`logDetailView.render`/`datadogLogDetailView.render` become
`Render` as pure renames — neither sets a per-instance title today
(both use a static title), so nothing to fold.
`logSearchView.open`/`codePipelineDetailView.open` become `Open` as
pure renames — both already set their own title internally.

`Table()`/`FilterInputs()`/`Primitive()` are new thin accessor
methods, one per applicable type, each returning the already-existing
field — no logic change:

```go
func (qv *queuesView) Table() *tview.Table                { return qv.table }
func (qv *queuesView) FilterInputs() []tview.Primitive     { return []tview.Primitive{qv.filterInput} }
func (dv *datadogLogsView) FilterInputs() []tview.Primitive {
	return []tview.Primitive{dv.queryInput, dv.serviceFilterDD, dv.envFilterDD}
}
func (mv *messagesView) Primitive() tview.Primitive { return mv.flex }
```

`app.go`'s `focusExemptInputs` construction becomes:

```go
a.focusExemptInputs = append([]tview.Primitive{a.prompt},
	a.queuesV.FilterInputs()[0],
	a.messagesV.FilterInputs()[0],
	a.ssmParamsV.FilterInputs()[0],
	a.secretsV.FilterInputs()[0],
	a.logsV.FilterInputs()[0],
	a.logSearchV.FilterInputs()[0],
	a.datadogLogsV.FilterInputs()...,
	a.codePipelineListV.FilterInputs()[0],
)
```

`app.go`'s `AddPage` block and the initial `.table` chrome-wiring loop
switch to `.Primitive()`/`.Table()`; `switchConnection`'s
`a.queuesV.backend = a.backend` becomes `a.queuesV.SetBackend(a.backend)`;
`host.go`'s 4 methods become one-line forwards
(`func (a *App) ReloadAfterSend(queueName string) { if a.queuesV != nil { a.queuesV.Load() }; if a.messagesV != nil && a.messagesV.QueueName() == queueName { a.messagesV.Load() } }`,
etc.); `codepipelinewatch.go`'s `statusLabel(...)` call becomes
`StatusLabel(...)`.

Verification: `gofmt -l`, `go build ./...`, `go test ./...` after each
file (or small group of files touching the same dependent file).

### 2. Phase 2 — capitalize types + constructors

Mechanical rename table (applied via `gofmt`-safe find/replace, one
file pair at a time, `go build ./...` after each):

| Old | New |
|---|---|
| `queuesView` / `newQueuesView` | `QueuesView` / `NewQueuesView` |
| `messagesView` / `newMessagesView` | `MessagesView` / `NewMessagesView` |
| `messageDetailView` / `newMessageDetailView` | `MessageDetailView` / `NewMessageDetailView` |
| `ssmParamsView` / `newSSMParamsView` | `SSMParamsView` / `NewSSMParamsView` |
| `paramDetailView` / `newParamDetailView` | `ParamDetailView` / `NewParamDetailView` |
| `secretsView` / `newSecretsView` | `SecretsView` / `NewSecretsView` |
| `secretDetailView` / `newSecretDetailView` | `SecretDetailView` / `NewSecretDetailView` |
| `logsView` / `newLogsView` | `LogsView` / `NewLogsView` |
| `logSearchView` / `newLogSearchView` | `LogSearchView` / `NewLogSearchView` |
| `logDetailView` / `newLogDetailView` | `LogDetailView` / `NewLogDetailView` |
| `datadogLogsView` / `newDatadogLogsView` | `DatadogLogsView` / `NewDatadogLogsView` |
| `datadogLogDetailView` / `newDatadogLogDetailView` | `DatadogLogDetailView` / `NewDatadogLogDetailView` |
| `codePipelineListView` / `newCodePipelineListView` | `CodePipelineListView` / `NewCodePipelineListView` |
| `codePipelineDetailView` / `newCodePipelineDetailView` | `CodePipelineDetailView` / `NewCodePipelineDetailView` |

`app.go`'s struct-field declarations (`queuesV *queuesView`, etc.) and
its 14 construction call sites update in lockstep — same package, so
this is a same-file, same-step edit, not a separate task.

### 3. Phase 3 — the test fake + 161 rewrites

New file `internal/app/viewhosttestfake_test.go` (test-only, deleted
from `internal/app` and re-created at `internal/view/testfake_test.go`
during Phase 4 — see below):

```go
package app

import (
	"context"
	"time"

	"github.com/rivo/tview"

	"github.com/ePex/cloudtui/tui/internal/awscodepipeline"
	"github.com/ePex/cloudtui/tui/internal/awslogs"
	"github.com/ePex/cloudtui/tui/internal/awsprofile"
	"github.com/ePex/cloudtui/tui/internal/awssecrets"
	"github.com/ePex/cloudtui/tui/internal/awsssm"
	"github.com/ePex/cloudtui/tui/internal/config"
	"github.com/ePex/cloudtui/tui/internal/datadoglogs"
	"github.com/ePex/cloudtui/tui/internal/queue"
	"github.com/ePex/cloudtui/tui/internal/ui"
)

var _ ui.ViewHost = (*fakeViewHost)(nil)

// fakeViewHost is a minimal ui.ViewHost double for view-level tests:
// it records what a view asked for instead of driving a real *App.
// Every data-fetcher method returns zero values unless a test sets
// the matching func field — same "inject a func, override per test"
// shape this codebase already used for App.listParameters etc.
// pre-CR-80.
type fakeViewHost struct {
	cfg     config.Config
	backend queue.Backend

	focused     tview.Primitive
	shownPage   string
	status      string
	contextHint string
	watching    map[string]bool

	listParametersFn         func(ctx context.Context, profile, path string) ([]awsssm.Parameter, error)
	revealParameterFn        func(ctx context.Context, profile, name string) (string, error)
	listSecretsFn             func(ctx context.Context, profile string) ([]awssecrets.Secret, error)
	revealSecretFn             func(ctx context.Context, profile, name string) (string, bool, error)
	listLogGroupsFn            func(ctx context.Context, profile string) ([]awslogs.LogGroup, error)
	filterLogEventsFn          func(ctx context.Context, profile, logGroupName string, start, end time.Time, pattern string) ([]awslogs.LogEvent, bool, error)
	searchDatadogLogsFn        func(ctx context.Context, cfg config.DatadogConfig, query string, from, to time.Time) ([]datadoglogs.LogEvent, bool, error)
	listDatadogFacetValuesFn   func(ctx context.Context, cfg config.DatadogConfig, facet string, from, to time.Time) ([]string, error)
	listPipelinesFn            func(ctx context.Context, profile string) ([]awscodepipeline.Pipeline, error)
	getPipelineStateFn         func(ctx context.Context, profile, pipelineName string) ([]awscodepipeline.StageStatus, error)
	awsAuthTypeForFn           func(ctx context.Context, profile string) (awsprofile.AuthType, error)
	awsSSOLoginFn              func(ctx context.Context, profile string) error
}

func newFakeViewHost() *fakeViewHost {
	return &fakeViewHost{cfg: config.Default(), watching: map[string]bool{}}
}

// -- ui.Host: recorded state where a test needs to assert on it --
func (f *fakeViewHost) SetFocus(p tview.Primitive)  { f.focused = p }
func (f *fakeViewHost) SetStatus(text string)       { f.status = text }
func (f *fakeViewHost) SetContextHint(text string)  { f.contextHint = text }
func (f *fakeViewHost) Config() config.Config       { return f.cfg }
func (f *fakeViewHost) Backend() queue.Backend      { return f.backend }
func (f *fakeViewHost) QueueUpdateDraw(fn func())   { fn() } // no real event loop; run inline
// -- ui.Host: no-ops (nothing under test needs these) --
func (f *fakeViewHost) ShowPage(name string)        {}
func (f *fakeViewHost) HidePage(name string)        {}
func (f *fakeViewHost) FocusMain()                  {}
func (f *fakeViewHost) SwitchTheme(name string)     {}
func (f *fakeViewHost) SwitchConnection(name string) {}
func (f *fakeViewHost) SaveConnection(conn config.Connection, origName string, isNew bool) {}
func (f *fakeViewHost) DeleteConnection(name string) (wasActive bool) { return false }
func (f *fakeViewHost) SaveDatadogConfig(cfg config.DatadogConfig)    {}
func (f *fakeViewHost) SetActiveAWSProfile(name string)               { f.cfg.ActiveAWSProfile = name }
func (f *fakeViewHost) ListAWSProfiles(ctx context.Context) ([]awsprofile.Profile, error) { return nil, nil }
func (f *fakeViewHost) ReloadAfterSend(queueName string)               {}
func (f *fakeViewHost) MessagesFilter() queue.MessageFilter            { return queue.MessageFilter{} }
func (f *fakeViewHost) ApplyMessagesFilter(fl queue.MessageFilter)     {}
func (f *fakeViewHost) FocusMessages()                                  {}

// -- ui.ViewHost chrome --
func (f *fakeViewHost) SwitchToPage(name string)     { f.shownPage = name }
func (f *fakeViewHost) UpdateContextPanel(v ui.View) {}
func (f *fakeViewHost) SwitchTo(name string)         { f.shownPage = name }
func (f *fakeViewHost) CopyToClipboard(data string)  {}

// -- ui.ViewHost cross-view navigation: never called by a view under
// test in isolation (a view invokes its injected onSelect/onBack
// closure, not host.OpenX) — pure stubs, present only to satisfy the
// interface. --
func (f *fakeViewHost) OpenMessages(queueName string)                        {}
func (f *fakeViewHost) OpenMessageDetail(queueName string, msg queue.Message) {}
func (f *fakeViewHost) OpenParamDetail(param awsssm.Parameter)               {}
func (f *fakeViewHost) OpenSecretDetail(secret awssecrets.Secret)            {}
func (f *fakeViewHost) OpenLogSearch(logGroupName string)                    {}
func (f *fakeViewHost) OpenLogEventDetail(event awslogs.LogEvent)            {}
func (f *fakeViewHost) OpenDatadogLogDetail(event datadoglogs.LogEvent)      {}
func (f *fakeViewHost) OpenCodePipelineDetail(pipelineName string)           {}
func (f *fakeViewHost) SetPendingCloudWatchPattern(pattern string)           {}

func (f *fakeViewHost) IsWatchingPipeline(name string) bool { return f.watching[name] }
func (f *fakeViewHost) StartWatchingPipeline(name string)   { f.watching[name] = true }
func (f *fakeViewHost) StopWatchingPipeline(name string)    { delete(f.watching, name) }

// -- ui.ViewHost data-fetchers: injectable func field, zero value if unset --
func (f *fakeViewHost) ListParameters(ctx context.Context, profile, path string) ([]awsssm.Parameter, error) {
	if f.listParametersFn != nil {
		return f.listParametersFn(ctx, profile, path)
	}
	return nil, nil
}
// ...same shape for RevealParameter/ListSecrets/RevealSecret/
// ListLogGroups/FilterLogEvents/SearchDatadogLogs/
// ListDatadogFacetValues/ListPipelines/GetPipelineState/
// AWSAuthTypeFor/AWSSSOLogin — 11 more, each a 3-line nil-check-then-
// call-or-zero-value forward to its `*Fn` field.
```

**Rewrite pattern for the 161 call sites.** Every
`a := New(config.Default()); ...newXView(a, ...)` becomes
`host := newFakeViewHost(); ...NewXView(host, ...)`. Where a test
needs dialogs (`queues_test.go`, `messages_test.go`,
`message_detail_test.go`), it constructs them directly on the fake:
`confirm := dialog.NewConfirmDialog(host)` etc. — real dialog
instances, since `dialog.NewX` only needs `ui.Host`, which the fake
satisfies. Where a test needs a custom backend
(`a.backend = &fakeQueueBackend{}`), it sets `host.backend =
&fakeQueueBackend{}` directly instead.

**`fakeQueueBackend` duplication**: `connectionsecrets_test.go` (not
moving) keeps its own copy; `internal/view`'s `queues_test.go` gets an
independent copy too — ~10 trivial stub methods, not worth a shared
package for.

**The 15 tests that relocate** (verified by reading each — every one
asserts a cross-view outcome via `a.pages.GetFrontPage()`, which only
means something against the real, fully-wired app):

| Current file | Test |
|---|---|
| `paramdetail_test.go` | `TestOpenParamDetailSwitchesPageAndSetsTitle`, `TestParamDetailViewEscReturnsToSSMParameters` |
| `secretdetail_test.go` | `TestOpenSecretDetailSwitchesPageAndSetsTitle`, `TestSecretDetailViewEscReturnsToSecretsManager` |
| `logdetail_test.go` | `TestOpenLogEventDetailSwitchesPage`, `TestLogDetailViewEscReturnsToLogSearch` |
| `datadoglogdetail_test.go` | `TestOpenDatadogLogDetailSwitchesPage`, `TestDatadogLogDetailViewEscReturnsToDatadogLogs`, `TestDatadogLogDetailViewGoToCloudWatchWithCorrelationID`, `TestDatadogLogDetailViewGoToCloudWatchWithoutCorrelationID` |
| `codepipelinelist_test.go` | `TestCodePipelineListViewSelectedFuncOpensDetail` |
| `codepipelinedetail_test.go` | `TestOpenCodePipelineDetailSwitchesPage`, `TestCodePipelineDetailViewEscReturnsToList` |
| `message_detail_test.go` | `TestMessageDetailViewEscReturnsToMessages` |
| `logsearch_test.go` | `TestLogSearchViewEscReturnsToCloudWatchLogs` |

They move verbatim into a new `internal/app/viewwiring_test.go`
(colocated with `viewwiring.go`, the file most of them actually
exercise), still driven by the real `New(config.Default())` — this
file never moves in Phase 4.

**Two adjustments during the move, not scope creep — a direct
consequence of the type now living in a different package:**

- `*SwitchesPageAndSetsTitle` tests today also assert
  `a.paramDetailV.textView.GetTitle()` directly — `.textView` becomes
  unexported-in-a-different-package and unreachable from
  `internal/app`. Drop the title assertion here; it's already covered
  by the view-level `Render` test (same-package access to `.textView`
  is unaffected — that test already exists for `TestParamDetailViewRenderShowsStringValueImmediately`-style
  cases, or a small assertion is added there if the title itself
  wasn't already checked at that level).
- The `*EscReturnsTo*` tests currently do
  `capture := a.paramDetailV.textView.GetInputCapture(); capture(tcell.NewEventKey(...))`
  — also unreachable post-move. Replace with
  `a.paramDetailV.Primitive().InputHandler()(tcell.NewEventKey(tcell.KeyEscape, 0, tcell.ModNone), func(tview.Primitive) {})`
  — `InputHandler()` is part of the `tview.Primitive` interface (which
  `Primitive()` already returns per Phase 1), and tview's own
  `SetInputCapture` machinery wraps a widget's `InputHandler` to run
  the custom capture first — this is the exact same re-dispatch
  pattern already used elsewhere in these files (e.g.
  `qv.table.InputHandler()(event, func(tview.Primitive) {})` in
  `queues.go`'s arrow-key redirection), not a new technique.

Verification: `go build ./...`, `go test ./internal/app/...` after
each test file's rewrite; final `go vet ./...` once all 14 are done.

### 4. Phase 4 — the physical move

Per view (14 iterations, each independently buildable since
`app.go`'s 14 fields are independent declarations):

1. `git mv internal/app/X.go internal/app/X_test.go internal/view/`
   (`internal/view/` created on the first iteration).
2. `package app` → `package view` in both moved files.
3. `app.go`: the one field's type (`XV *XView` → `XV *view.XView`) and
   the one construction call (`NewXView(...)` → `view.NewXView(...)`);
   add the `"github.com/ePex/cloudtui/tui/internal/view"` import on
   the first iteration.
4. `go build ./...`, `go test ./...`.

The shared `fakeViewHost` test file
(`internal/app/viewhosttestfake_test.go`) moves to
`internal/view/testfake_test.go` in the same step as whichever view
it's first needed by (or as its own small first step, since every one
of the 14 test files needs it) — `package app` → `package view`, no
other change.

`internal/view`'s new `import` needs in `app.go`/`host.go`/
`viewwiring.go`/`codepipelinewatch.go`: just the one
`internal/view` import; nothing else changes in those 4 files beyond
what Phase 1 already did (they already call only exported `view.X`
methods by this point — Phase 4 is purely the type/package-qualifier
swap).

### 5. Verification order

Phase 1 → Phase 2 → Phase 3 → Phase 4, in that order, each phase's
steps verified individually (`gofmt -l`, `go build ./...`, relevant
`go test`) before moving to the next phase. Final pass: repo-wide
`gofmt -l`, `go vet ./...`, `go build ./...`, `go test ./...`, then
live verification per spec.md.

## Files touched

- The 14 production + 14 test files (moved to `internal/view/`,
  renamed types/constructors, new exported methods).
- New `internal/view/testfake_test.go` (created in `internal/app` in
  Phase 3, relocated in Phase 4).
- New `internal/app/viewwiring_test.go` (the 15 relocated tests).
- `internal/app/app.go`, `host.go`, `viewwiring.go`,
  `codepipelinewatch.go` (exports adopted, `view.` import added).
- `internal/app/paramdetail_test.go`, `secretdetail_test.go`,
  `logdetail_test.go`, `datadoglogdetail_test.go`,
  `codepipelinelist_test.go`, `codepipelinedetail_test.go`,
  `message_detail_test.go`, `logsearch_test.go` — trimmed (the 15
  relocated tests removed) before those files themselves move in
  Phase 4.
- `internal/app/connectionsecrets_test.go` — gets its own
  `fakeQueueBackend` copy.

## Key decisions

- **Stage everything except the file move itself while still inside
  `internal/app`** — de-risks a large refactor into a sequence of
  individually-buildable, individually-testable steps, the same
  discipline CR 82/83 already established at smaller scale.
- **Fold trampoline logic into the view's own exported method
  (`Open`, `Render`, `ApplyFilter`) rather than exposing raw fields**
  — matches CR 82's `onBack` precedent (relocate the reach-in behind a
  method, don't just make the field public) and keeps each view's
  invariants (e.g. "title always matches the rendered param") owned
  by the view itself.
- **`FilterInputs() []tview.Primitive` (plural) uniformly**, even for
  the 7 types with only one input — one consistent shape beats a
  singular method on most types plus a special case for
  `datadogLogsView`'s 3.
- **The relocated wiring tests drop the title/content assertions
  they used to piggyback** — a direct, unavoidable consequence of
  `.textView` becoming a different package's unexported field, not a
  coverage loss: that assertion now lives in the view's own `Render`
  test, where it's actually easier to reach.
- **`fakeQueueBackend` is duplicated, not shared** — no test-fixtures
  package exists in this repo; introducing one for a ~10-method stub
  is disproportionate.
- **No behavior change anywhere** — every Phase 1 fold moves existing
  logic verbatim; Phase 2 is a pure rename; Phase 3 changes test
  *construction*, not test *assertions* (except the two documented,
  unavoidable adjustments above); Phase 4 is a package-qualifier swap.

## Definition of done

Unchanged from spec.md — `internal/view` holds all 14 types
(exported) + tests, `internal/app` has none of them, `go build`/
`go test`/`go vet` pass with zero import cycle, `gofmt -l` clean, the
4 dependent files use only exported `view.X` access, the 15 relocated
wiring tests live in `internal/app/viewwiring_test.go`, live
verification confirms no behavior change.
