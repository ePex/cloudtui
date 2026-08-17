# Plan — CR 78: physical move to `internal/dialog`

## Approach

### 1. Move the files

```bash
mkdir -p internal/dialog
git mv internal/app/confirm.go          internal/dialog/confirm.go
git mv internal/app/movepicker.go       internal/dialog/movepicker.go
git mv internal/app/sendmessage.go      internal/dialog/sendmessage.go
git mv internal/app/connections.go      internal/dialog/connections.go
git mv internal/app/messagefilter.go    internal/dialog/messagefilter.go
git mv internal/app/timerangemodal.go   internal/dialog/timerangemodal.go
git mv internal/app/datadogsettings.go  internal/dialog/datadogsettings.go
git mv internal/app/themepicker.go      internal/dialog/themepicker.go
git mv internal/app/awsprofiles.go      internal/dialog/awsprofiles.go
git mv internal/app/connections_test.go     internal/dialog/connections_test.go
git mv internal/app/datadogsettings_test.go internal/dialog/datadogsettings_test.go
git mv internal/app/timerangemodal_test.go  internal/dialog/timerangemodal_test.go
git mv internal/app/awsprofiles_test.go     internal/dialog/awsprofiles_test.go
git mv internal/app/hosttest_test.go        internal/dialog/hosttest_test.go
```

`git mv` first, then edit — keeps the file history attached to the
move (`git log --follow` still works) rather than showing up as
14 deletes + 14 creates.

### 2. `package app` → `package dialog`

One-line change at the top of all 14 moved files (9 production + 5
test). No other content changes needed in the 9 production files —
every reference inside them is already either a local (now
same-package) symbol or an already-qualified `ui.X`/`config.X`/
`queue.X`/`awsprofile.X`/`context.X`/`tcell.X`/`tview.X` reference.

### 3. `internal/dialog/dialogtest_test.go` (new)

```go
package dialog

import (
	"context"
	"strings"
	"testing"

	"github.com/gdamore/tcell/v2"
	"github.com/rivo/tview"

	"github.com/ePex/cloudtui/tui/internal/queue"
)

// renderedScreenText draws prim into a width×height simulation screen
// and returns its visible text. Duplicated from internal/app's
// queues_test.go (unexported, test-only, used by both sides of the
// internal/dialog split — not worth a shared package for one function).
func renderedScreenText(t *testing.T, prim tview.Primitive, width, height int) string {
	t.Helper()
	prim.SetRect(0, 0, width, height)
	screen := tcell.NewSimulationScreen("")
	if err := screen.Init(); err != nil {
		t.Fatalf("screen.Init: %v", err)
	}
	screen.SetSize(width, height)
	prim.Draw(screen)
	screen.Show() // flushes the back buffer into front; GetContents reads front

	cells, w, h := screen.GetContents()
	var b strings.Builder
	for y := 0; y < h; y++ {
		for x := 0; x < w; x++ {
			cell := cells[y*w+x]
			if len(cell.Runes) > 0 {
				b.WriteRune(cell.Runes[0])
			}
		}
	}
	return b.String()
}

// fakeQueueBackend is a zero-behavior queue.Backend, used only as
// testHost's default Backend() — no test moved into this package
// calls host.Backend() today (only movePicker/sendMessageOverlay do,
// neither has a dedicated test), so every method here is unexercised.
// Duplicated from internal/app's queues_test.go for the same reason
// as renderedScreenText above.
type fakeQueueBackend struct{}

func (f *fakeQueueBackend) List(_ context.Context) ([]queue.Summary, error) { return nil, nil }
func (f *fakeQueueBackend) BrowseMessages(_ context.Context, _ string, _ queue.MessageFilter) ([]queue.Message, error) {
	return nil, nil
}
func (f *fakeQueueBackend) PurgeQueue(_ context.Context, _ string) error        { return nil }
func (f *fakeQueueBackend) RemoveMessage(_ context.Context, _, _ string) error  { return nil }
func (f *fakeQueueBackend) MoveMessage(_ context.Context, _, _, _ string) error { return nil }
func (f *fakeQueueBackend) MoveAllMessages(_ context.Context, _, _ string) (int, error) {
	return 0, nil
}
func (f *fakeQueueBackend) SendMessage(_ context.Context, _, _ string) error { return nil }
func (f *fakeQueueBackend) DeleteMessages(_ context.Context, _ string, _ queue.MessageFilter) (int, error) {
	return 0, nil
}
func (f *fakeQueueBackend) MoveMessages(_ context.Context, _, _ string, _ queue.MessageFilter) (int, error) {
	return 0, nil
}
```

`hosttest_test.go`'s `newTestHost()` already reads
`backend: &fakeQueueBackend{}` — unchanged after the move, now
resolving to this package-local copy instead of `internal/app`'s.

### 4. `internal/app/app.go`

```go
// new import, alongside the existing internal/ui, internal/config, etc.
"github.com/ePex/cloudtui/tui/internal/dialog"
```

Field declarations (10 lines, type only — field names unchanged):

```go
// before
confirm        *ConfirmDialog
movePicker     *MovePicker
sendMessage    *SendMessageOverlay
connManager    *ConnManager
connEditor     *ConnEditor
messageFilter  *MessageFilter
timeRangeModal *TimeRangeModal
datadogEditor  *DatadogEditor
themePicker    *ThemePicker
awsProfiles    *AWSProfilesPicker

// after
confirm        *dialog.ConfirmDialog
movePicker     *dialog.MovePicker
sendMessage    *dialog.SendMessageOverlay
connManager    *dialog.ConnManager
connEditor     *dialog.ConnEditor
messageFilter  *dialog.MessageFilter
timeRangeModal *dialog.TimeRangeModal
datadogEditor  *dialog.DatadogEditor
themePicker    *dialog.ThemePicker
awsProfiles    *dialog.AWSProfilesPicker
```

Constructor calls in `New()` (10 lines, `NewX(...)` → `dialog.NewX(...)`,
arguments unchanged):

```go
a.confirm = dialog.NewConfirmDialog(a)
a.movePicker = dialog.NewMovePicker(a)
a.sendMessage = dialog.NewSendMessageOverlay(a)
a.connManager = dialog.NewConnManager(a, a.confirm)
a.connEditor = dialog.NewConnEditor(a, a.connManager)
a.messageFilter = dialog.NewMessageFilter(a)
a.timeRangeModal = dialog.NewTimeRangeModal(a)
a.datadogEditor = dialog.NewDatadogEditor(a)
a.themePicker = dialog.NewThemePicker(a)
a.awsProfiles = dialog.NewAWSProfilesPicker(a)
```

Nothing else in `app.go` changes — `a.connManager.editor =
a.connEditor` (field assignment, not a type reference), the 10
`ui.Centered(a.X.Primitive(), w, h)` sizing lines, the
`overlayVisible []visibler` slice literal, and every `.Show()`/field
read elsewhere in `app.go` are all unaffected by the move (they never
name the type).

### 5. No other file changes

`message_detail.go`, `queues.go`, `messages.go`, `logsearch.go`,
`datadoglogs.go`, `settings.go`, `host.go` — confirmed zero type-name
references to any of the 10 types in the Background audit; these
files are untouched.

### 6. `tui/CLAUDE.md`

Add one bullet to "Package layout", right after `internal/app/`:

```
- `internal/dialog/` — modal overlay types (confirm, connection
  manager/editor, message filter, time range, Datadog/theme/AWS
  profile pickers) implementing internal/ui's Host contract.
```

### 7. Verification order

1. `go build ./...` immediately after step 1+2 (move + package
   rename, before touching `app.go`) — expect failures confined to
   `internal/app` (undefined `ConfirmDialog` etc. in `app.go`) and
   nothing else; confirms the move itself didn't strand a reference.
2. Fix `app.go` (step 4) — `go build ./...` should pass repo-wide.
3. `go vet ./...`, `gofmt -l .`, `go test ./...` repo-wide.
4. Live verification (`verify-live` skill): build the binary, open
   each of the 10 overlays once via its real trigger (`x`/`d` for
   confirm+delete, `m` for move picker, `s` for send message, `:aq`
   for connection manager → `n` for editor, `f` for message filter,
   `t` for time range in log search or Datadog logs, Settings for
   Datadog/theme/AWS profiles), confirm each renders and Esc closes
   it, quit cleanly. Record exactly what was checked in `tasks.md`
   per `tui/CLAUDE.md`'s testing conventions.

## Files touched

- 9 production + 5 test files: moved `internal/app/` → `internal/dialog/`
- `internal/dialog/dialogtest_test.go` (new)
- `internal/app/app.go` (import + 10 field types + 10 constructor calls)
- `tui/CLAUDE.md` (package layout bullet)

**Corrected during implementation** (see `tasks.md` for full detail):
this list undersold the actual blast radius — the "zero type-name
references outside `app.go`" audit only checked type names, not
cross-package field/method access on an already-exported type. Three
more categories of fix were needed: (1) `connections.go` gained a
`SetEditor` method (`a.connManager.editor = a.connEditor` no longer
compiles across the package boundary); (2) the two true `App`/`Host`
integration tests (`TestSaveDatadogEditorRoundTrip`,
`TestActivateAWSProfilePersistsAndUpdatesUI`) had to split into a
pure-overlay half (staying with the moved test, via `testHost`) and a
new `internal/app/host_test.go` (the real disk-persistence/UI-update
half); (3) four more `internal/app` test files
(`app_test.go`, `messages_test.go`, `logsearch_test.go`,
`datadoglogs_test.go`) reached into unexported overlay fields/methods
and needed fixing, plus one fully misplaced test (`TestSortPickerQueues`)
relocated to a new `internal/dialog/movepicker_test.go`.

## Key decisions

- **`git mv` before editing, not delete+recreate** — preserves file
  history through the move (matches how every earlier file-touching
  CR in this series has operated, even though none moved files across
  directories before this one).
- **Duplicate `renderedScreenText`/`fakeQueueBackend` rather than
  inventing a shared test-helpers package** — both are small,
  self-contained, and this is the only pair of test-only symbols that
  turned out to be needed on both sides of the split after a full
  audit of the 5 moving test files against every other `internal/app`
  test file. A new package for two functions, one of which
  (`fakeQueueBackend`) isn't even exercised by anything that moved,
  is more machinery than the problem justifies.
- **No accessor/interface changes to `ui.Host` or the overlay types
  themselves** — this CR is purely a location + import change; every
  design decision that made the move possible (CR 67's `Host`
  interface, CR 73's `Primitive()`/`Visible()`, CR 74's export pass,
  CR 75's `ui.TimeRange`, CR 76/77's `testHost`) already happened.
- **Live verification required, unlike most of the prerequisite CRs**
  — those were pure renames verified sufficient by `go test`; this CR
  moves every file that renders one of the 10 overlays, and a mistake
  here (e.g. a botched merge during the `app.go` edit) would most
  likely still compile — only running the app catches it.

## Definition of done

Unchanged from spec.md — `go build`/`go test`/`go vet` pass, `gofmt -l`
clean, all 14 files live in `internal/dialog`, `app.go` is the only
`internal/app` file touched, all 10 overlays live-verified,
`tui/CLAUDE.md` updated, zero behavior change.
