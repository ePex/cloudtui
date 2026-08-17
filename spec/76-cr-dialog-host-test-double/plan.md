# Plan — CR 76: `ui.Host` test double

## Approach

### 1. `internal/app/hosttest_test.go` (new file)

```go
package app

import (
	"context"

	"github.com/rivo/tview"

	"github.com/ePex/cloudtui/tui/internal/awsprofile"
	"github.com/ePex/cloudtui/tui/internal/config"
	"github.com/ePex/cloudtui/tui/internal/queue"
	"github.com/ePex/cloudtui/tui/internal/ui"
)

// testHost is a minimal ui.Host test double: side-effecting methods
// record their call, data methods return injectable/canned values. Not
// exported — internal/dialog gets its own copy when the overlays move
// there (see spec/76's Out of scope).
type testHost struct {
	cfg             config.Config
	backend         queue.Backend
	listAWSProfiles func(context.Context) ([]awsprofile.Profile, error)
	messagesFilter  queue.MessageFilter
	deleteResult    bool // DeleteConnection's return value

	shownPages         []string
	hiddenPages        []string
	focused            tview.Primitive
	focusMainCalls     int
	status             string
	contextHint        string
	switchedTheme      string
	switchedConnection string
	savedConnection    *savedConnectionCall
	deletedConnection  string
	savedDatadogConfig *config.DatadogConfig
	activeAWSProfile   string
	reloadedQueue      string
	appliedFilter      *queue.MessageFilter
	focusMessagesCalls int
}

type savedConnectionCall struct {
	conn     config.Connection
	origName string
	isNew    bool
}

// newTestHost builds a testHost with config.Default() and a
// zero-value fakeQueueBackend — matches what New(config.Default())
// gave every overlay constructor before this CR.
func newTestHost() *testHost {
	return &testHost{cfg: config.Default(), backend: &fakeQueueBackend{}}
}

func (h *testHost) ShowPage(name string)  { h.shownPages = append(h.shownPages, name) }
func (h *testHost) HidePage(name string)  { h.hiddenPages = append(h.hiddenPages, name) }
func (h *testHost) SetFocus(p tview.Primitive) { h.focused = p }
func (h *testHost) FocusMain()            { h.focusMainCalls++ }
func (h *testHost) QueueUpdateDraw(f func()) { f() }

func (h *testHost) SetStatus(text string)      { h.status = text }
func (h *testHost) SetContextHint(text string) { h.contextHint = text }

func (h *testHost) Config() config.Config      { return h.cfg }
func (h *testHost) SwitchTheme(name string)    { h.switchedTheme = name }
func (h *testHost) SwitchConnection(name string) { h.switchedConnection = name }
func (h *testHost) SaveConnection(conn config.Connection, origName string, isNew bool) {
	h.savedConnection = &savedConnectionCall{conn: conn, origName: origName, isNew: isNew}
}
func (h *testHost) DeleteConnection(name string) bool {
	h.deletedConnection = name
	return h.deleteResult
}
func (h *testHost) SaveDatadogConfig(cfg config.DatadogConfig) { h.savedDatadogConfig = &cfg }
func (h *testHost) SetActiveAWSProfile(name string)            { h.activeAWSProfile = name }
func (h *testHost) ListAWSProfiles(ctx context.Context) ([]awsprofile.Profile, error) {
	if h.listAWSProfiles == nil {
		return nil, nil
	}
	return h.listAWSProfiles(ctx)
}

func (h *testHost) Backend() queue.Backend        { return h.backend }
func (h *testHost) ReloadAfterSend(queueName string) { h.reloadedQueue = queueName }

func (h *testHost) MessagesFilter() queue.MessageFilter { return h.messagesFilter }
func (h *testHost) ApplyMessagesFilter(f queue.MessageFilter) { h.appliedFilter = &f }
func (h *testHost) FocusMessages()                         { h.focusMessagesCalls++ }

var _ ui.Host = (*testHost)(nil)
```

`Backend()` returns `&fakeQueueBackend{}` (already defined in
`queues_test.go`, same package) by default — none of the 18 tests
converted in this CR call `host.Backend()` (only `movePicker`/
`sendMessageOverlay` do, out of scope here), so this is just a safe,
correctly-typed default, not exercised behavior.

### 2. `connections_test.go`

```go
func newTestConnEditor(t *testing.T) (*ConnEditor, *testHost) {
	t.Helper()
	host := newTestHost()
	manager := NewConnManager(host, NewConfirmDialog(host))
	return NewConnEditor(host, manager), host
}
```

Both tests: replace `a := New(config.Default()); a.connEditor.Show(...)`
with `ce, _ := newTestConnEditor(t); ce.Show(...)`, and every
`a.connEditor.X` with `ce.X`. `close()` (called via the Esc capture)
reads `ce.manager.visible` — the real `NewConnManager` built above
satisfies that without any stubbing.

### 3. `datadogsettings_test.go`

```go
func newTestDatadogEditor(t *testing.T) (*DatadogEditor, *testHost) {
	t.Helper()
	host := newTestHost()
	return NewDatadogEditor(host), host
}
```

`TestDatadogEditorEscapeCloses`/`TestDatadogEditorOtherKeysPassThrough`:
same mechanical swap as `ConnEditor`'s. `TestDatadogEditorPrefillsFromConfig`:

```go
de, host := newTestDatadogEditor(t)
host.cfg.Datadog = config.DatadogConfig{Site: "datadoghq.eu", AccessToken: "tok-123"}
de.Show()
// ... same GetFormItem assertions, de. instead of a.datadogEditor.
```

`TestSaveDatadogEditorRoundTrip` — left exactly as-is (still `a :=
New(config.Default())`, still calls `a.datadogEditor.save()`
directly): it's testing that saving actually reaches disk through
`App`'s real `SaveDatadogConfig`, which a `testHost` deliberately
doesn't do (it only records the call). `TestDatadogSettingsLabel` —
moved verbatim to a new `settings_test.go` (doesn't exist yet; this
is the file's first test).

### 4. `timerangemodal_test.go`

```go
func newTestTimeRangeModal(t *testing.T) (*TimeRangeModal, *testHost) {
	t.Helper()
	host := newTestHost()
	return NewTimeRangeModal(host), host
}
```

All 13 tests: `a := New(config.Default())` → `tm, host :=
newTestTimeRangeModal(t)`; every `a.timeRangeModal.X` → `tm.X`. One
behavioral change beyond the mechanical swap —
`TestApplyTimeRangeAbsoluteInvalidDate` currently asserts via
`a.statusBar.GetText(true)`; becomes:

```go
if !strings.Contains(host.status, "invalid from") {
	t.Errorf("status = %q, want it to mention the invalid \"from\" field", host.status)
}
```

## Files touched

- `internal/app/hosttest_test.go` (new)
- `internal/app/connections_test.go`
- `internal/app/datadogsettings_test.go`
- `internal/app/settings_test.go` (new — `TestDatadogSettingsLabel` only)
- `internal/app/timerangemodal_test.go`

## Key decisions

- **`testHost` lives in `internal/app` for now, not a new package** —
  it only has test-only callers today (all inside package `app`); a
  dedicated package would be premature given nothing outside `app`
  can use it yet (`internal/dialog` doesn't exist until the move CR,
  which is exactly when `testHost` gets a copy in its new home).
- **Fields, not a builder/options API** — every field is set directly
  before constructing the overlay (`host.cfg.Datadog = ...`) or read
  directly after acting (`host.status`), matching this codebase's
  existing test style (table-driven where useful, otherwise plain
  arrange/act/assert) rather than introducing a new fluent-builder
  convention for one file.
- **Reuse `fakeQueueBackend`, don't write a new one** — same package,
  already does exactly what `Backend()` needs to default to.
- **`QueueUpdateDraw` runs synchronously** — correct for every test
  converted here (none exercise the goroutine+`QueueUpdateDraw`
  pattern); revisit only if a future CR adds tests for `movePicker`/
  `sendMessageOverlay`, which is out of scope here.
- **No new non-test dependencies** — `hosttest_test.go` only imports
  packages already imported elsewhere in `internal/app`'s test files.

## Definition of done

Unchanged from spec.md — `go build`/`go test`/`go vet` pass, `gofmt -l`
clean, `testHost` implements all 20 `ui.Host` methods, the 3 files'
pure-overlay tests construct their overlay directly via `testHost`,
the 1 integration test and 1 misplaced test are handled as described,
zero production-code behavior change.
