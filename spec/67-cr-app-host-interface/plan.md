# Plan — CR 67: declare `Host`, add `App`'s wrapper methods

## Approach

Three steps in one commit (all low-risk — step 1 is pure addition, step
2 mechanical, step 3 the only real behavior-adjacent change and small
enough to verify directly).

### 1. `internal/ui/host.go` (new file)

```go
package ui

import (
	"context"

	"github.com/ePex/cloudtui/tui/internal/awsprofile"
	"github.com/ePex/cloudtui/tui/internal/config"
	"github.com/ePex/cloudtui/tui/internal/queue"
	"github.com/rivo/tview"
)

// Host is the contract overlays depend on instead of the concrete *App,
// so overlays can move to their own package without an import cycle
// (internal/dialog importing internal/app back for *App).
type Host interface {
	// Chrome / focus
	ShowPage(name string)
	HidePage(name string)
	SetFocus(p tview.Primitive)
	FocusMain()
	QueueUpdateDraw(f func())

	// Status / context feedback
	SetStatus(text string)
	SetContextHint(text string)

	// Config / theme
	Config() config.Config
	SwitchTheme(name string)
	SwitchConnection(name string)
	SaveConnection(conn config.Connection, origName string, isNew bool)
	DeleteConnection(name string) (wasActive bool)
	SaveDatadogConfig(cfg config.DatadogConfig)
	SetActiveAWSProfile(name string)
	ListAWSProfiles(ctx context.Context) ([]awsprofile.Profile, error)

	// Backend / queue data
	Backend() queue.Backend
	ReloadAfterSend(queueName string)

	// Messages view interaction
	MessagesFilter() queue.MessageFilter
	ApplyMessagesFilter(f queue.MessageFilter)
	FocusMessages()
}
```

### 2. `internal/app/host.go` (new file) — `App`'s wrapper methods

A dedicated file (not `app.go`) so every method satisfying `ui.Host` is
in one place — useful for CR 68/69's own reference, and keeps `app.go`
from growing again right after CR 59–62 shrank it.

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

var _ ui.Host = (*App)(nil)

func (a *App) ShowPage(name string)       { a.rootPages.ShowPage(name) }
func (a *App) HidePage(name string)       { a.rootPages.HidePage(name) }
func (a *App) SetFocus(p tview.Primitive) { a.tv.SetFocus(p) }
func (a *App) FocusMain()                 { a.tv.SetFocus(a.pages) }
func (a *App) QueueUpdateDraw(f func())   { a.tv.QueueUpdateDraw(f) }

func (a *App) SetStatus(text string)      { a.statusBar.SetText(text) }
func (a *App) SetContextHint(text string) { a.contextPanel.SetText(text) }

func (a *App) Config() config.Config { return a.cfg }

func (a *App) SwitchTheme(name string)      { a.switchTheme(name) }
func (a *App) SwitchConnection(name string) { a.switchConnection(name) }

func (a *App) ListAWSProfiles(ctx context.Context) ([]awsprofile.Profile, error) {
	return a.listAWSProfiles(ctx)
}

func (a *App) Backend() queue.Backend { return a.backend }

// ReloadAfterSend reloads the queues view and, if the messages view is
// currently showing queueName, reloads it too. Extracted from
// sendMessageOverlay.doSend, which did this inline before this method
// existed.
func (a *App) ReloadAfterSend(queueName string) {
	if a.queuesV != nil {
		a.queuesV.load()
	}
	if a.messagesV != nil && a.messagesV.queueName == queueName {
		a.messagesV.load()
	}
}

func (a *App) MessagesFilter() queue.MessageFilter {
	return a.messagesV.filter
}

// ApplyMessagesFilter sets f as the messages view's active filter,
// updates its title, and reloads. Extracted from messageFilter.apply
// and .clear, which each did this identical 3-line sequence inline
// before this method existed.
func (a *App) ApplyMessagesFilter(f queue.MessageFilter) {
	a.messagesV.filter = f
	a.messagesV.updateTitle()
	a.messagesV.load()
}

func (a *App) FocusMessages() { a.tv.SetFocus(a.messagesV.table) }
```

`SwitchTheme`/`SwitchConnection` forward to the existing unexported
`switchTheme`/`switchConnection` rather than renaming them — both
already have other internal callers (`onPromptDone`, `DeleteConnection`)
that don't need to change in this CR. Yes, this leaves two names for
the same operation; that's the accepted cost of a minimal-risk wrapper
over a rename that would touch unrelated call sites.

### 3. Two overlay call sites, updated to use the new methods

**`sendmessage.go`, `doSend`** — replace the manual reload block:

```go
// before
a.statusBar.SetText(fmt.Sprintf("Message sent to %q", queueName))
if a.queuesV != nil {
	a.queuesV.load()
}
if a.messagesV != nil && a.messagesV.queueName == queueName {
	a.messagesV.load()
}

// after
a.statusBar.SetText(fmt.Sprintf("Message sent to %q", queueName))
a.ReloadAfterSend(queueName)
```

No reordering — this one's a straight substitution.

**`messagefilter.go`, `apply` and `clear`** — both change shape:

```go
// apply, before
a.messagesV.filter = filter
a.messagesV.updateTitle()
mf.close()
a.messagesV.load()

// apply, after
a.ApplyMessagesFilter(filter)
mf.close()
```

```go
// clear, before
mf.app.messagesV.filter = queue.MessageFilter{}
mf.app.messagesV.updateTitle()
mf.close()
mf.app.messagesV.load()

// clear, after
mf.app.ApplyMessagesFilter(queue.MessageFilter{})
mf.close()
```

**Deliberate reordering, called out explicitly** (same spirit as CR
66's `SetActiveAWSProfile` note): `load()`'s dispatch moves from
*after* `mf.close()` to *before* it, since `ApplyMessagesFilter` bundles
filter-set/title-update/load into one call and preserving the original
relative position of filter-set/title-update (before `close()`) means
`load()` moves with them. `messagesView.load()` dispatches a goroutine
and returns immediately (same async pattern as everywhere else in this
codebase), and `close()` itself is synchronous UI-state-only work — so
this reorder has no observable effect, but it's a real (if immaterial)
deviation from strict line-for-line motion, flagged rather than folded
in silently.

## Files touched

- `internal/ui/host.go` (new)
- `internal/app/host.go` (new)
- `internal/app/sendmessage.go` (`doSend`'s reload block)
- `internal/app/messagefilter.go` (`apply`, `clear`)

## Key decisions

- **Wrapper, not rename**, for `SwitchTheme`/`SwitchConnection` — see
  above.
- **`Config()` returns `config.Config` by value** — `config.Config` is
  already a value type throughout this codebase (`a.cfg` is a field,
  not a pointer), so this is a natural copy-on-read, no aliasing risk.
- **No new tests** — `ReloadAfterSend`/`ApplyMessagesFilter` wrap
  already-tested `load()`/`updateTitle()` behavior; the two behavior-
  adjacent call sites need live verification (real broker interaction),
  not unit coverage, matching CR 66's approach.
- **No new dependencies.**

## Definition of done

Unchanged from spec.md — `go build`/`go test` pass, `Host` declared
with all 20 methods, `var _ ui.Host = (*App)(nil)` compiles,
`doSend`/`apply`/`clear` call the new methods, live-verified (send a
message, apply and clear a message filter) with no behavior change.
