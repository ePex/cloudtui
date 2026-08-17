# Plan — CR 83: `ui.ViewHost` adoption, 5 dialog-coupled views

## Approach

### 1. `app.go`: move the 5 dialogs' construction earlier

Today (`New()`, around what's currently line ~275):

```go
a.confirm = dialog.NewConfirmDialog(a)
confirmOverlay := ui.Centered(a.confirm.Primitive(), 52, 8)

a.movePicker = dialog.NewMovePicker(a)
movePickerOverlay := ui.Centered(a.movePicker.Primitive(), 52, 22)

a.sendMessage = dialog.NewSendMessageOverlay(a)
sendMessageOverlay := ui.Centered(a.sendMessage.Primitive(), 70, 14)
```
...(later) `a.messageFilter = dialog.NewMessageFilter(a)` and
`a.timeRangeModal = dialog.NewTimeRangeModal(a)`, each immediately
followed by its own `xOverlay := ui.Centered(...)` line.

Split each of these 5 into two pieces: the `a.X = dialog.NewX(a)`
construction line moves up to right after `a.backend =
newBackendForConn(a, cfg.ActiveConn())` (before `a.queuesV =
newQueuesView(...)`) — none of the 5 dialogs depend on any view, only
on `a` (`ui.Host`) or another already-built dialog, so this is safe.
The `xOverlay := ui.Centered(a.X.Primitive(), ...)` line (plus its
sizing comments) stays exactly where it is today; it still compiles
unchanged since `a.X` is simply already set by the time it runs.
`a.connManager = dialog.NewConnManager(a, a.confirm)` is unaffected
(unrelated to this CR, and `a.confirm` is still available to it either
way).

### 2. Per-file constructor signatures

```go
func newQueuesView(a ui.ViewHost, b queue.Backend,
	confirm *dialog.ConfirmDialog, movePicker *dialog.MovePicker,
	sendMessage *dialog.SendMessageOverlay,
	onSelect func(queueName string)) *queuesView

func newMessagesView(a ui.ViewHost,
	messageFilter *dialog.MessageFilter, sendMessage *dialog.SendMessageOverlay,
	confirm *dialog.ConfirmDialog, movePicker *dialog.MovePicker,
	onSelect func(queueName string, msg queue.Message)) *messagesView

func newMessageDetailView(a ui.ViewHost,
	movePicker *dialog.MovePicker, confirm *dialog.ConfirmDialog,
	onBack func(), onReload func()) *messageDetailView

func newLogSearchView(a ui.ViewHost, timeRangeModal *dialog.TimeRangeModal,
	onSelect func(event awslogs.LogEvent), onBack func()) *logSearchView

func newDatadogLogsView(a ui.ViewHost, timeRangeModal *dialog.TimeRangeModal,
	onSelect func(event datadoglogs.LogEvent)) *datadogLogsView
```

Each struct gains one field per new dialog parameter (named after the
dialog, same as the parameter — matches `ConnEditor`'s bare `manager`
field), alongside `host ui.ViewHost` replacing `app *App`. `import
"github.com/ePex/cloudtui/tui/internal/dialog"` added to each of the 5
files (all already import `internal/ui`).

### 3. Rename table (raw access → `ui.ViewHost`/`ui.Host`, applies
   wherever the symbol appears across the 5 files)

| Old | New |
|---|---|
| `.tv.SetFocus(` | `.SetFocus(` |
| `.tv.QueueUpdateDraw(` | `.QueueUpdateDraw(` |
| `.cfg.Colors` / `.cfg.ActiveAWSProfile` / `.cfg.Datadog` | `.Config().Colors` / `.Config().ActiveAWSProfile` / `.Config().Datadog` |
| `.contextPanel.SetText(` | `.SetContextHint(` |
| `.statusBar.SetText(` | `.SetStatus(` |
| `.pages.SwitchToPage(` | `.SwitchToPage(` |
| `.backend` (bare field read) | `.Backend()` |
| `.filterLogEvents(` | `.FilterLogEvents(` |
| `.searchDatadogLogs(` | `.SearchDatadogLogs(` |
| `.listDatadogFacetValues(` | `.ListDatadogFacetValues(` |
| `.SwitchTo(` | unchanged (already exported) |

Dialog calls (`.confirm.Show(`, `.movePicker.Show(`,
`.sendMessage.Show(`, `.messageFilter.Show(`, `.timeRangeModal.Show(`)
switch from `a.X`/`xv.app.X` to the new constructor-injected field
(`xv.confirm`, `xv.movePicker`, etc. — same name as the struct field).

### 4. Per-file notes

- **`queues.go`** (17 sites): `.tv`×8, `.cfg`×4, `.contextPanel`×2,
  `.statusBar`×1, `.confirm`×1, `.movePicker`×1, `.sendMessage`×1.
  Note the `'M'`/`'c'` handlers' `restoreQueues`/inline restore
  closures rebuild *this view's own* context panel after a dialog
  closes — self-restore, not a sibling reach, stays inline using
  `qv.host`/`qv.Config()`/`qv.SetContextHint`.
- **`messages.go`** (largest, ~30 sites): mixes `a.X` (inside closures
  built before `mv` exists) and `mv.app.X` for the same underlying
  value — per CR 82's established rule, preserve this existing mix
  (which access point is used where), only change what the symbol
  resolves to. Esc handler's `a.SwitchTo("queues")` is untouched — it
  already only uses the exported method, so no `onBack` needed here.
- **`message_detail.go`**: the 3 sibling-reaching blocks (`'m'`
  success, `'d'` success, Esc) collapse to `onBack()` (all 3) plus
  `onReload()` (the `'m'`/`'d'` sites only, right after `onBack()` —
  preserves the existing difference, doesn't invent a reload on Esc).
  The `restoreDetail` closure (used when the move picker is
  cancelled, staying on this same page) is self-restore, not
  sibling-reaching: `a.tv.SetFocus(a.pages)` becomes
  `dv.host.SetFocus(dv.textView)` — `ui.ViewHost` has no raw `.pages`,
  and focusing the view's own primitive directly is equivalent to
  focusing the pages container while this page is the active one.
- **`logsearch.go`**: single `onBack()` replaces the Esc handler's
  3-line body. Unlike `message_detail.go`'s target, `logsV` *is* a
  registered `ui.View`, so the closure uses `UpdateContextPanel(a.logsV)`
  directly (no manual `Shortcuts()` rebuild) — same as CR 82's
  non-`logdetail.go` cases.
- **`datadoglogs.go`**: no `onBack` (top-level registered view, no
  Esc-back at all). Only the rename table + `timeRangeModal` field.

### 5. `app.go` construction call sites

```go
a.queuesV = newQueuesView(a, a.backend, a.confirm, a.movePicker, a.sendMessage, a.OpenMessages)
a.messagesV = newMessagesView(a, a.messageFilter, a.sendMessage, a.confirm, a.movePicker, a.OpenMessageDetail)
a.messageDetailV = newMessageDetailView(a, a.movePicker, a.confirm,
	func() {
		a.pages.SwitchToPage("messages")
		a.tv.SetFocus(a.messagesV.table)
		lines := make([]string, 0, len(a.messagesV.Shortcuts()))
		for _, sc := range a.messagesV.Shortcuts() {
			lines = append(lines, fmt.Sprintf("[%s]<%s>[-] %s", a.Config().Colors.Accent, sc.Key, sc.Description))
		}
		a.SetContextHint(strings.Join(lines, "\n"))
	},
	func() { a.messagesV.load() },
)
...
a.logSearchV = newLogSearchView(a, a.timeRangeModal, a.OpenLogEventDetail, func() {
	a.pages.SwitchToPage("cloudwatch-logs")
	a.tv.SetFocus(a.logsV.table)
	a.UpdateContextPanel(a.logsV)
})
...
a.datadogLogsV = newDatadogLogsView(a, a.timeRangeModal, a.OpenDatadogLogDetail)
```

`a.queuesV`/`a.messagesV`/`a.messageDetailV` construction stays in its
current relative position (right after `a.backend = ...`), now able to
reference the already-built `a.confirm`/`a.movePicker`/`a.sendMessage`
per step 1. `a.logSearchV`/`a.datadogLogsV` likewise stay where they
are, now referencing `a.timeRangeModal`.

### 6. Test updates

- **`queues_test.go`** (9 call sites, all `newQueuesView(a,
  &fakeQueueBackend{}, func(string) {})`): every real `a` in these
  tests comes from `a := New(config.Default())`, which already builds
  `a.confirm`/`a.movePicker`/`a.sendMessage` before the test's own
  `newQueuesView` call runs — a mechanical `replace_all` to
  `newQueuesView(a, &fakeQueueBackend{}, a.confirm, a.movePicker, a.sendMessage, func(string) {})`
  covers every site identically.
- **`messages_test.go`** (6 call sites, all `newMessagesView(a,
  func(string, queue.Message) {})`): same reasoning, `replace_all` to
  `newMessagesView(a, a.messageFilter, a.sendMessage, a.confirm, a.movePicker, func(string, queue.Message) {})`.
- **`message_detail_test.go`** (1 call site, the `newTestMessageDetailView`
  helper): `newMessageDetailView(a)` → `newMessageDetailView(a, a.movePicker, a.confirm, func() {}, func() {})`
  — existing tests only check title/shortcuts/render, not
  navigation, so no-op stub closures are correct here.
- **`logsearch_test.go`**/**`datadoglogs_test.go`**: no direct
  constructor calls (all go through `a.logSearchV`/`a.datadogLogsV`
  from a real `New()`-built app) — no changes needed.

### 7. New tests (gap CR 82 didn't have)

CR 82's 5 detail views each already had a pre-existing
`TestXViewEscReturnsToY` test (driven through a real `New()`-built
app, same shape as `TestParamDetailViewEscReturnsToSSMParameters`) —
CR 82 relied on those, unmodified. `message_detail.go` and
`logsearch.go` have no equivalent today, so this CR adds:

- `TestMessageDetailViewEscReturnsToMessages` — drive Esc via the real
  app, assert `a.pages.GetFrontPage() == "messages"`.
- `TestLogSearchViewEscReturnsToCloudWatchLogs` — same shape as
  `TestParamDetailViewEscReturnsToSSMParameters`, target page
  `"cloudwatch-logs"`.

**Revised during implementation**: the `'m'`/`'d'` success-path tests
originally planned here (asserting the page switch *and* that
`messagesV.load()` ran) turned out infeasible — that path runs inside
a goroutine + `QueueUpdateDraw`, which needs a running tview event
loop to ever complete (same constraint `logsearch.go`'s
`handleSearchResult` doc comment already documents for its own
goroutine+`QueueUpdateDraw` path), and no test anywhere in this
codebase drives one. Added `TestMessageDetailViewMoveOpensPickerWithSourceQueue`/
`TestMessageDetailViewDeleteOpensConfirmWithPrompt` instead, covering
the synchronous half (which dialog opens, with what prompt) — same
shape as `messages_test.go`'s existing
`TestMessagesViewMoveFallsBackToCursorWhenNothingMarked`/
`TestMessagesViewDeleteFallsBackToCursorWhenNothingMarked`, which stop
at the same boundary. The actual reload-on-success is covered by live
verification (task 8) instead.

### 8. Verification order

One file at a time: `datadoglogs.go` first (smallest — rename table
only, no dialog-field wiring beyond `timeRangeModal`, no `onBack`),
then `logsearch.go` (adds `onBack` + `timeRangeModal`), then
`queues.go`, then `messages.go`, then `message_detail.go` (largest —
3 dialogs' worth of fields plus `onBack`/`onReload`), each followed by
`gofmt -l`/`go build ./...`. `app.go`'s construction reordering and
call-site updates happen alongside whichever file's turn introduces
the dependency (dialogs moved up front once, before touching
`queues.go`). Final pass: add the 4 new tests, then repo-wide `gofmt
-l`, `go vet ./...`, `go build ./...`, `go test ./...`.

## Files touched

- `queues.go`, `messages.go`, `message_detail.go`, `logsearch.go`,
  `datadoglogs.go`
- `app.go` (dialog construction reordering + 5 construction call sites)
- `queues_test.go`, `messages_test.go`, `message_detail_test.go`
  (call-site signature updates)
- `message_detail_test.go`, `logsearch_test.go` (new tests per step 7)

## Key decisions

- **Dialogs are constructor parameters, not `ui.ViewHost` methods** —
  CR 80's design decision, reaffirmed: keeps `ui.ViewHost` from
  growing 5 more `Show`-shaped methods for something that's really a
  sibling reference, exactly like `ConnEditor`/`ConnManager` already
  does across the dialog/view boundary.
- **`onBack`/`onReload` split for `message_detail.go`, not a single
  combined callback** — the existing code already treats "return to
  messages" and "reload the messages list" as separable (Esc does the
  former only); collapsing them into one callback would force every
  caller to reload even when the original code didn't, a real
  behavior change this CR must not make.
- **New tests for `message_detail.go`/`logsearch.go`'s `onBack` seam**
  — CR 82 could lean on pre-existing Esc-return tests; these two files
  have no equivalent today, and per `tui/CLAUDE.md`, relocating this
  logic behind a new callback parameter is a new code path worth its
  own coverage, not just a hope that existing render/title tests
  happen to catch a regression here.
- **Field/parameter naming matches the dialog itself** (`confirm`,
  `movePicker`, `sendMessage`, `messageFilter`, `timeRangeModal`) —
  same convention as `ConnEditor`'s `manager` field, no `dialog`-
  prefixed or `ui.Host`-style naming needed since each field's type
  already disambiguates it.
- **No new dependencies** — `internal/dialog` is already an
  `internal/app` dependency (that's exactly what's being made
  explicit per-file instead of implicit through `*App`).

## Definition of done

Unchanged from spec.md — `go build`/`go test`/`go vet` pass, `gofmt -l`
clean, all 5 files hold `host ui.ViewHost` plus their specific
`*dialog.X` fields, `app.go`'s dialog construction precedes these 5
views', `onBack`/`onReload` replace the sibling reaches, live
verification done, zero behavior change.
