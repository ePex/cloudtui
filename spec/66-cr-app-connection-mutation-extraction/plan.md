# Plan — CR 66: extract config-mutating logic into named `App` methods

## Approach

Read all four source methods in full before writing anything (done during
spec.md). Each of the four extractions below moves logic verbatim, with
one deliberate, safe reordering called out explicitly (not hidden).

### 1. `connections.go`: `SaveConnection`

```go
// SaveConnection appends conn (isNew) or replaces the connection named
// origName (edit) in Connections, rebuilding the active backend in
// place if the edited connection was the active one, then persists and
// refreshes the settings list.
func (a *App) SaveConnection(conn config.Connection, origName string, isNew bool) {
	wasActive := a.cfg.ActiveConnection == origName
	if isNew {
		a.cfg.Connections = append(a.cfg.Connections, conn)
	} else {
		for i, c := range a.cfg.Connections {
			if c.Name == origName {
				a.cfg.Connections[i] = conn
				break
			}
		}
		if wasActive {
			a.cfg.ActiveConnection = conn.Name
			a.backend = newBackendForConn(a, conn)
			a.queuesV.backend = a.backend
			a.infoPanel.SetText(ui.InfoPanelText(a.cfg))
		}
	}
	if err := config.SaveDefault(a.cfg); err != nil {
		slog.Error("SaveConnection: save failed", "error", err)
	}
	a.refreshSettingsList()
}
```

`connEditor.save()` keeps form-reading/validation and its own tail
(`ce.close()`, `a.connManager.populate()` — overlay-to-overlay, stays a
direct call since both are still in package `app`), replacing the
`wasActive`/append-or-replace/backend-rebuild block with one call:
`a.SaveConnection(conn, ce.origName, ce.isNew)`.

### 2. `connections.go`: `DeleteConnection`

```go
// DeleteConnection removes name from Connections. If it was the active
// connection, activates the first remaining one (reusing switchConnection
// for the backend-rebuild+persist+refresh path); otherwise persists
// directly. Returns whether the removed connection was active, so the
// caller knows which post-delete UI path to take (switchConnection
// already navigated to "queues"; the non-active path needs the caller
// to repaint its own list instead).
func (a *App) DeleteConnection(name string) (wasActive bool) {
	wasActive = a.cfg.ActiveConnection == name
	conns := make([]config.Connection, 0, len(a.cfg.Connections)-1)
	for _, c := range a.cfg.Connections {
		if c.Name != name {
			conns = append(conns, c)
		}
	}
	a.cfg.Connections = conns
	if wasActive {
		a.cfg.ActiveConnection = a.cfg.Connections[0].Name
		a.switchConnection(a.cfg.ActiveConnection)
		return true
	}
	if err := config.SaveDefault(a.cfg); err != nil {
		slog.Error("DeleteConnection: save failed", "error", err)
	}
	return false
}
```

`connManager.delete()`'s confirm callback shrinks to:

```go
a.confirm.show(fmt.Sprintf("Delete connection %q?", toDelete.Name), func() {
	if a.DeleteConnection(toDelete.Name) {
		cm.close()
	} else {
		cm.populate()
		a.tv.SetFocus(cm.list)
	}
})
```

The pre-confirm guards (`len(a.cfg.Connections) <= 1`, index bounds)
stay in `connManager.delete()` — they decide whether to prompt at all,
not part of the mutation itself.

### 3. `datadogsettings.go`: `SaveDatadogConfig`

```go
// SaveDatadogConfig persists cfg.Datadog and refreshes the settings list.
func (a *App) SaveDatadogConfig(cfg config.DatadogConfig) {
	a.cfg.Datadog = cfg
	if err := config.SaveDefault(a.cfg); err != nil {
		slog.Error("SaveDatadogConfig: save failed", "error", err)
	}
	a.refreshSettingsList()
}
```

`datadogEditor.save()` becomes: read form fields, then
`a.SaveDatadogConfig(config.DatadogConfig{Site: site, AccessToken: token})`
followed by `de.close()`.

### 4. `awsprofiles.go`: `SetActiveAWSProfile`

```go
// SetActiveAWSProfile sets name as the active AWS profile, updates the
// info panel, refreshes the settings list, and persists.
func (a *App) SetActiveAWSProfile(name string) {
	a.cfg.ActiveAWSProfile = name
	a.infoPanel.SetText(ui.InfoPanelText(a.cfg))
	a.refreshSettingsList()
	if err := config.SaveDefault(a.cfg); err != nil {
		slog.Error("SetActiveAWSProfile: save failed", "error", err)
	}
}
```

`awsProfilesPicker.activate()` becomes:

```go
func (ap *awsProfilesPicker) activate(name string) {
	a := ap.app
	a.SetActiveAWSProfile(name)
	ap.close()
	a.statusBar.SetText(fmt.Sprintf("AWS profile: %s", name))
}
```

**Deliberate reordering, called out explicitly**: the original persists
*after* `ap.close()` and the status-bar text; the extracted version
persists *before* them (inside `SetActiveAWSProfile`, ahead of the
overlay's own cleanup). `config.SaveDefault` has no observable UI
effect — it only writes a file — so this reordering changes nothing a
user can see, but it's a real (if immaterial) change from strict
line-for-line motion, so it's flagged here rather than silently folded
in. If review disagrees, the alternative is threading a save-callback
back out, which is more machinery for no behavioral gain.

## Files touched

- `internal/app/connections.go` (`SaveConnection`, `DeleteConnection` +
  their call sites)
- `internal/app/datadogsettings.go` (`SaveDatadogConfig` + call site)
- `internal/app/awsprofiles.go` (`SetActiveAWSProfile` + call site)

No new files, no signature changes to anything outside these three
files, no test file changes expected (existing tests exercise the
overlay methods' observable behavior, which is unchanged).

## Key decisions

- **No error returns except where the original already had branching
  behavior on failure** (`DeleteConnection`'s `wasActive bool` return,
  needed by the caller to pick a UI path) — `SaveConnection`/
  `SaveDatadogConfig`/`SetActiveAWSProfile` keep the existing
  log-and-continue pattern for `config.SaveDefault` failures, matching
  `switchConnection`/`switchTheme`'s existing precedent in this exact
  codebase. Not "fixing" that error-handling shape — out of scope.
- **Overlay-to-overlay and overlay-local UI calls stay at the call
  site** (`ce.close()`, `a.connManager.populate()`, `cm.populate()`,
  `a.tv.SetFocus(...)`, status-bar text) — only the config-mutation +
  backend-rebuild + persist + `refreshSettingsList` core moves into the
  new `App` methods. This keeps the extracted methods' scope aligned
  with what a future `Host` interface method would actually need to do
  App-side, without also making them responsible for overlay-specific
  widget repaints.
- **No new dependencies.**

## Testing

Per spec.md: all four affected flows have real broker/AWS-file/Datadog
interaction, so `go test` alone doesn't cover the actual mutation
correctness — live-verify (`verify-live` skill):
1. Edit the active connection's settings (e.g. change the broker name) —
   confirm the queues view immediately reflects the new backend.
2. Delete the active connection — confirm it switches to another and
   navigates to "queues".
3. Delete a non-active connection — confirm the manager's list updates
   in place, no navigation.
4. Save Datadog settings — confirm the settings screen shows the new
   site.
5. Activate a different AWS profile — confirm the info panel and
   settings screen update.

## Definition of done

Unchanged from spec.md — `go build`/`go test` pass, the four `App`
methods exist and are called from their overlay's method instead of
inlining the logic, all five flows above verified live with no
behavior change.
