# Plan — CR 69: swap `connManager`/`connEditor` to `host ui.Host`

## Approach

One combined change to `connections.go` (the two types are entangled
enough — `connManager` needs `connEditor` and vice versa — that
splitting into two commits would leave an intermediate state where one
references a field the other doesn't have yet), plus the two `app.go`
construction-site updates.

### Struct/constructor changes

```go
type connManager struct {
	host    ui.Host
	confirm *confirmDialog
	editor  *connEditor // set after construction — see New()
	flex    *tview.Flex
	list    *tview.List
	hints   *tview.TextView
	visible bool
}

func newConnManager(host ui.Host, confirm *confirmDialog) *connManager {
	cm := &connManager{host: host, confirm: confirm}
	// ... unchanged widget setup ...
}
```

```go
type connEditor struct {
	host    ui.Host
	manager *connManager
	form     *tview.Form
	visible  bool
	isNew    bool
	origName string
	brokerName string
}

func newConnEditor(host ui.Host, manager *connManager) *connEditor {
	ce := &connEditor{host: host, manager: manager}
	// ... unchanged widget setup, styleDropDown(dd, host.Config().Colors) ...
}
```

### Method-by-method substitutions

**`connManager`**

- `newConnManager`'s `SetInputCapture` closure: `a.connEditor.show(...)`
  (3 call sites: `n`/`e`/`d`) → `cm.editor.show(...)`; `a.cfg.Connections[idx]`
  (2 read sites, `e`/`d`) → `cm.host.Config().Connections[idx]`.
- `SetSelectedFunc` closure: `a.cfg.Connections[idx].Name` →
  `cm.host.Config().Connections[idx].Name`; `a.switchConnection(name)` →
  `cm.host.SwitchConnection(name)`.
- `show()`: `cm.app.cfg.Colors.Accent` → `cm.host.Config().Colors.Accent`;
  `cm.app.rootPages.ShowPage` → `cm.host.ShowPage`; `cm.app.tv.SetFocus` →
  `cm.host.SetFocus`.
- `close()`: `cm.app.rootPages.HidePage` → `cm.host.HidePage`;
  `cm.app.tv.SetFocus(cm.app.pages)` → `cm.host.FocusMain()`.
- `populate()`: `cm.app.cfg.Connections` → `cm.host.Config().Connections`;
  `cm.app.cfg.ActiveConnection` → the same `Config()` call's
  `.ActiveConnection` (one read, reused — see snippet below);
  `cm.app.switchConnection(c.Name)` → `cm.host.SwitchConnection(c.Name)`.
- `delete()`: `a.cfg.Connections`/`a.statusBar.SetText` →
  `cm.host.Config().Connections`/`cm.host.SetStatus`; `a.confirm.show(...)` →
  `cm.confirm.show(...)`; `a.DeleteConnection(...)` →
  `cm.host.DeleteConnection(...)`; `a.tv.SetFocus(cm.list)` →
  `cm.host.SetFocus(cm.list)`.

```go
// populate, after:
func (cm *connManager) populate() {
	cm.list.Clear()
	cfg := cm.host.Config()
	for _, conn := range cfg.Connections {
		c := conn
		star := "   "
		if c.Name == cfg.ActiveConnection {
			star = "⭐ "
		}
		label := fmt.Sprintf("%s%-24s (%s)", star, c.Name, c.Backend)
		cm.list.AddItem(label, "", 0, func() {
			cm.close()
			cm.host.SwitchConnection(c.Name)
		})
	}
}
```

**`connEditor`**

- Constructor: both `styleDropDown(dd, a.cfg.Colors)` calls →
  `styleDropDown(dd, host.Config().Colors)`.
- `show()`: `a := ce.app` local alias removed; `a.rootPages.ShowPage` →
  `ce.host.ShowPage`; `a.tv.SetFocus` → `ce.host.SetFocus`.
- `rebuildTail()`: `ce.app.cfg.Colors` → `ce.host.Config().Colors`.
- `close()`: `a := ce.app` removed; `a.rootPages.HidePage` →
  `ce.host.HidePage`; `a.connManager.visible` → `ce.manager.visible`;
  `a.tv.SetFocus(a.connManager.list)` → `ce.host.SetFocus(ce.manager.list)`;
  else-branch `a.tv.SetFocus(a.pages)` → `ce.host.FocusMain()`.
- `save()`: `a := ce.app` removed; both `a.statusBar.SetText(...)` →
  `ce.host.SetStatus(...)`; `a.cfg.Connections` (duplicate-name loop) →
  `ce.host.Config().Connections`; `a.SaveConnection(...)` →
  `ce.host.SaveConnection(...)`; trailing `a.connManager.populate()` →
  `ce.manager.populate()`.

`ApplyPalette` on both types is unchanged — neither touches `app`/`host`,
only `p config.Palette` and their own widgets. `DeleteConnection` and
`SaveConnection` (the `App`-side, `Host`-implementing methods defined
lower in the same file) are unchanged — they're the target of the calls
above, not the caller.

### `app.go`

```go
// before
a.connManager = newConnManager(a)
...
a.connEditor = newConnEditor(a)

// after
a.connManager = newConnManager(a, a.confirm)
...
a.connEditor = newConnEditor(a, a.connManager)
a.connManager.editor = a.connEditor
```

`a.confirm` is already constructed earlier in `New()` (line 270,
before `connManager` at line 279) — confirmed by re-checking
construction order, so no reordering needed there.

## Files touched

- `internal/app/connections.go`
- `internal/app/app.go` (two construction-call sites + one new wiring
  line)

## Key decisions

- **`editor` wired post-construction, not via a setter method** — a
  plain field assignment in `New()` is the smallest change that works;
  introducing a `SetEditor(*connEditor)` method for one internal,
  same-package call site would be ceremony with no benefit.
- **`populate()` calls `Config()` once, not per-field** — avoids two
  separate `Host` method calls (`.Connections` then `.ActiveConnection`)
  doing the same underlying work twice; matches the existing style of
  caching a `Config()`/`Colors` read at the top of a method when used
  more than once (already the pattern in `awsprofiles.go`'s `repaint()`
  after CR 68).
- **No new tests, no new dependencies.**

## Testing

Per spec.md: live-verify (`verify-live` skill) the full connection-
management flow in one pass — open the manager, create a new
connection, edit an existing one (toggling Backend jolokia↔proxy to
exercise `rebuildTail`, which reads/writes through `host.Config()`),
duplicate one, delete a non-active one, delete the active one. This
single pass exercises every changed line in both types, including the
`connManager`↔`connEditor` interaction the sibling-reference wiring
exists for.

## Definition of done

Unchanged from spec.md — both types hold `host ui.Host` (+ sibling
refs), `go build`/`go test` pass, zero remaining `.app.` access in
`connections.go`, full connection-management flow verified live.
