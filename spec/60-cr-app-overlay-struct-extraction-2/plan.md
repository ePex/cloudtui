# Plan — CR 60: extract message filter + Datadog settings into dedicated structs

## Approach

Same shape as CR 59: a struct per overlay (`app *App` back-reference,
`form *tview.Form`, `visible bool`), methods hung off that struct instead
of `*App`, construction moved into a `new*(a *App) *T` constructor called
from `New()`. `centered(...)`/`AddPage(...)` wiring in `New()` stays as-is,
just reads the new struct's `form` field instead of a flat one.

## Files touched

### `tui/internal/app/messagefilter.go`

```go
type messageFilter struct {
	app     *App
	form    *tview.Form
	visible bool
}

func newMessageFilter(a *App) *messageFilter {
	// body = app.go:462-478's construction, assigning into mf.form;
	// AddButton closures call mf.apply/mf.clear/mf.close instead of
	// a.applyMessageFilter/a.clearMessageFilter/a.closeMessageFilter;
	// SetInputCapture's Escape branch calls mf.close()
}

func (mf *messageFilter) show() { ... }   // was showMessageFilter
func (mf *messageFilter) close() { ... }  // was closeMessageFilter
func (mf *messageFilter) apply() { ... }  // was applyMessageFilter
func (mf *messageFilter) clear() { ... }  // was clearMessageFilter
```

`formatFilterDate` and `parseMessageFilterForm` are already
package-level, `*App`-independent functions in this file — untouched,
they don't need a receiver and already live where they should.

### `tui/internal/app/datadogsettings.go`

```go
type datadogEditor struct {
	app     *App
	form    *tview.Form
	visible bool
}

func newDatadogEditor(a *App) *datadogEditor {
	// body = app.go:554-567's construction; AddButton closures call
	// de.save/de.close; SetInputCapture's Escape branch calls de.close()
}

func (de *datadogEditor) show() { ... }   // was showDatadogEditor
func (de *datadogEditor) close() { ... }  // was closeDatadogEditor
func (de *datadogEditor) save() { ... }   // was saveDatadogEditor
```

### `tui/internal/app/app.go`

- Remove the 4 fields (`messageFilterForm`, `messageFilterVisible`,
  `datadogEditorForm`, `datadogEditorVisible`), replaced by
  `messageFilter *messageFilter` and `datadogEditor *datadogEditor`.
- `New()`: the two inline construction blocks (`~462-481`, `~554-570`)
  become `a.messageFilter = newMessageFilter(a)` /
  `a.datadogEditor = newDatadogEditor(a)`, followed by the existing
  `centered(a.messageFilter.form, 64, 16)` / `centered(a.datadogEditor.form,
  56, 10)` lines (just the field path changes) — the `AddPage(...)` chain
  itself is untouched.
- Both OR-chains (`~722`, `~768` — same two lines CR 59 already touched
  three times each) get their `messageFilterVisible`/`datadogEditorVisible`
  parts updated to `a.messageFilter.visible`/`a.datadogEditor.visible`.

### `tui/internal/app/messages.go`

`a.showMessageFilter()` (the one call site, `~131`) → `a.messageFilter.show()`.

### `tui/internal/app/settings.go`

Both `a.showDatadogEditor()` call sites (`~50`, `~89`) →
`a.datadogEditor.show()`.

### `tui/internal/app/app_test.go`

`TestOnGlobalKeyPassesThroughWhenDatadogEditorVisible`: `a.showDatadogEditor()`
→ `a.datadogEditor.show()`. Same assertion, same intent — still guards the
exact OR-chain line, just through the new field path.

### `tui/internal/app/datadogsettings_test.go`

All `a.showDatadogEditor()`/`a.saveDatadogEditor()` calls and
`a.datadogEditorForm`/`a.datadogEditorVisible` field reads → the
`a.datadogEditor.*` equivalents. No assertion changes — same behavior,
same coverage.

## Testing

No new tests — pure structural move, same reasoning as CR 59. Existing
`datadogsettings_test.go` and the `app_test.go` OR-chain regression test
must keep passing with unchanged intent.

Manual (`verify-live` skill) — message filter has no unit-test safety net
for its show/apply/clear flow, so this matters more here than it did for
CR 59's overlays:

- Open the message filter from a messages view (`f` or whatever the
  current binding is — check `messages.go`), confirm it prefills from the
  active filter, `Apply` sets a new filter and reloads, `Clear` resets it
  and reloads, `Cancel`/`Esc` discards without changing anything.
- Open Datadog settings from Settings, confirm prefill from `cfg.Datadog`,
  `Save` persists and closes, `Cancel`/`Esc` discards. Type into the
  Site/Access Token fields and confirm none of the characters typed
  trigger global hotkeys (the exact regression `TestOnGlobalKeyPassesThroughWhenDatadogEditorVisible`
  guards at the unit level — worth eyeballing live too since this is
  UI-input-routing behavior).
