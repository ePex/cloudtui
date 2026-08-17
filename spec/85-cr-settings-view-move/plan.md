# Plan — CR 85: adopt `ui.ViewHost` in `settings.go` and move it to `internal/view`

## Approach

Same staged strategy CR 84 used: do everything except the physical
move while the file still lives in `internal/app`, verified
incrementally; the move itself is the last, purely mechanical step.

### 1. `SettingsView`'s new shape

```go
type SettingsView struct {
	list          *tview.List
	host          ui.ViewHost
	themePicker   *dialog.ThemePicker
	connManager   *dialog.ConnManager
	awsProfiles   *dialog.AWSProfilesPicker
	datadogEditor *dialog.DatadogEditor
}

var _ ui.View = (*SettingsView)(nil)
var _ ui.Themeable = (*SettingsView)(nil)

func (s *SettingsView) Name() string               { return "settings" }
func (s *SettingsView) Title() string              { return "Settings" }
func (s *SettingsView) Primitive() tview.Primitive { return s.list }

func NewSettingsView(a ui.ViewHost, themePicker *dialog.ThemePicker, connManager *dialog.ConnManager, awsProfiles *dialog.AWSProfilesPicker, datadogEditor *dialog.DatadogEditor) *SettingsView {
	l := tview.NewList().ShowSecondaryText(false)
	l.SetBorder(true).SetTitle(" Settings ")

	s := &SettingsView{list: l, host: a, themePicker: themePicker, connManager: connManager, awsProfiles: awsProfiles, datadogEditor: datadogEditor}

	// Items are populated by Refresh; add placeholders here so indices
	// are stable.
	l.AddItem("", "", 0, func() { s.themePicker.Show() })
	l.AddItem("", "", 0, func() { s.connManager.Show() })
	l.AddItem("", "", 0, func() { s.awsProfiles.Show() })
	l.AddItem("", "", 0, func() { s.datadogEditor.Show() })

	l.SetInputCapture(func(event *tcell.EventKey) *tcell.EventKey {
		switch event.Rune() {
		case 'j':
			return tcell.NewEventKey(tcell.KeyDown, 0, tcell.ModNone)
		case 'k':
			return tcell.NewEventKey(tcell.KeyUp, 0, tcell.ModNone)
		}
		return event
	})

	ui.StyleList(l, a.Config().Colors)
	s.Refresh()
	return s
}
```

### 2. `Refresh()` — folds in `(a *App) refreshSettingsList`'s body verbatim

```go
// Refresh rebuilds the displayed text of all settings-list items to
// reflect the current config values (theme name, active connection
// name, active AWS profile, Datadog site). The AWS profile name comes
// straight from cfg.ActiveAWSProfile — no need to re-read ~/.aws
// here, unlike opening the overlay itself, which lists every
// discoverable profile.
func (s *SettingsView) Refresh() {
	cfg := s.host.Config()
	cur := s.list.GetCurrentItem()
	conn := cfg.ActiveConn()
	awsProfile := cfg.ActiveAWSProfile
	if awsProfile == "" {
		awsProfile = "(none)"
	}
	s.list.Clear()
	s.list.AddItem(fmt.Sprintf("Theme: %s", cfg.Theme), "", 0, func() { s.themePicker.Show() })
	s.list.AddItem(fmt.Sprintf("AMQ Connection: %s", conn.Name), "", 0, func() { s.connManager.Show() })
	s.list.AddItem(fmt.Sprintf("AWS Profile: %s", awsProfile), "", 0, func() { s.awsProfiles.Show() })
	s.list.AddItem(fmt.Sprintf("Datadog: %s", datadogSettingsLabel(cfg.Datadog)), "", 0, func() { s.datadogEditor.Show() })
	if cur >= 0 && cur < s.list.GetItemCount() {
		s.list.SetCurrentItem(cur)
	}
}
```

`datadogSettingsLabel` (package-level helper, used only within this
file) moves unchanged.

### 3. `ApplyPalette` — folds in `theme.go`'s special-cased block, minus its nil guard

```go
// ApplyPalette recolors the settings list for a live theme switch.
func (s *SettingsView) ApplyPalette(p config.Palette) {
	bg := tcell.GetColor(p.Background)
	s.list.SetBackgroundColor(bg)
	s.list.SetBorderColor(tcell.GetColor(p.ViewColor("settings")))
	s.list.SetTitleColor(tcell.GetColor(p.ViewColor("settings")))
	ui.StyleList(s.list, p)
}
```

No nil guard: unlike `theme.go`'s inline block (which could in theory
run before `a.settingsList` existed), `ApplyPalette` is only ever
called via the `a.themables` loop, after every themable — including
this one — is already constructed, same as all 14 already-moved
views.

### 4. `app.go` changes

**Construction order**: `NewSettingsView` now needs its 4 dialogs to
already exist, unlike today's version (its closures capture `a` and
look up `a.themePicker` etc. lazily at call time, so construction
order never mattered before — same "order disappears under callback
injection" fact CR 81 already established, just newly relevant here).
`connManager`/`datadogEditor`/`themePicker`/`awsProfiles`'s
construction lines move into the existing early-dialog block (added
in CR 83, right after `a.backend = newBackendForConn(...)`), same
split CR 83 used: only the `a.X = dialog.NewX(...)` line moves, each
`xOverlay := ui.Centered(a.X.Primitive(), ...)` line stays exactly
where it is:

```go
a.confirm = dialog.NewConfirmDialog(a)
a.movePicker = dialog.NewMovePicker(a)
a.sendMessage = dialog.NewSendMessageOverlay(a)
a.messageFilter = dialog.NewMessageFilter(a)
a.timeRangeModal = dialog.NewTimeRangeModal(a)
a.connManager = dialog.NewConnManager(a, a.confirm)
a.datadogEditor = dialog.NewDatadogEditor(a)
a.themePicker = dialog.NewThemePicker(a)
a.awsProfiles = dialog.NewAWSProfilesPicker(a)
```

`connManagerOverlay := ui.Centered(a.connManager.Primitive(), 64, 20)`
etc. (and `a.connEditor = dialog.NewConnEditor(a, a.connManager)`,
which only needs `a.connManager` to already exist — true either way)
all stay at their current position further down.

**Settings construction moves** from its current position (before
`a.backend` is even set, at what's currently the very top of view
construction) to right after the early-dialog block, alongside the
other views:

```go
a.settingsV = view.NewSettingsView(a, a.themePicker, a.connManager, a.awsProfiles, a.datadogEditor)
```

replacing the local `settingsView := newSettingsView(a)` and its
now-stale "safe at this point because all live primitives are set"
comment (that reasoning doesn't apply once construction takes
concrete dialog pointers instead of doing lazy lookups). `a.logV =
newLogView(a)` and `a.backend = newBackendForConn(...)` are otherwise
unaffected — they can stay wherever this reordering leaves them,
neither depends on nor is depended on by Settings.

**Struct field**: `settingsList *tview.List` → `settingsV
*view.SettingsView`.

**`a.views` literal**: `settingsView` (local var) → `a.settingsV`.

**`a.themables` literal**: gains `a.settingsV` (anywhere in the list —
position doesn't matter, it's just a slice `reapplyTheme` ranges
over).

**The 2 `refreshSettingsList()` call sites in `app.go`**
(`switchTheme`, `switchConnection`) → `a.settingsV.Refresh()`.

### 5. `host.go` changes

The 3 `refreshSettingsList()` call sites (`SaveConnection`,
`SaveDatadogConfig`, `SetActiveAWSProfile`) → `a.settingsV.Refresh()`.

### 6. `theme.go` changes

Delete the entire "Settings list" block (the `if a.settingsList !=
nil { ... }` guard and its 4 lines) from `reapplyTheme` — replaced by
`a.settingsV` now being an ordinary member of the `a.themables` loop
just below it.

### 7. Tests — `settings_test.go` already exists (12 tests); this is
   mostly a port + adapt, not fresh coverage

**Correction to this plan's earlier draft** (and spec.md's first
draft): grepping for it before writing this section turned up
`internal/app/settings_test.go` already exists, 12 tests. Sorting
them by what they actually exercise:

**Port to `internal/view/settings_test.go`, largely unchanged**
(genuinely view-level — row text from `Refresh()`, or the
`datadogSettingsLabel` helper directly):
`TestSettingsListHasFourItems`, `TestSettingsListItemThreeIsDatadog`,
`TestSettingsListItemThreeShowsConfiguredDatadogSite`,
`TestSettingsListItemTwoIsAWSProfile`,
`TestSettingsListItemTwoShowsActiveAWSProfile`,
`TestSettingsListItemsShowCurrentThemeAndConnection`,
`TestDatadogSettingsLabel`, plus `TestSettingsListHasBorderAndTitle`
(already accesses the list via `a.pages.GetPage("settings")`, ports
as `TestSettingsViewNameAndTitle` in the same style already used by
the 14 other views). Construction switches to the `fakeViewHost` +
real-dialog pattern:

```go
func newTestSettingsView(t *testing.T) (*fakeViewHost, *SettingsView) {
	t.Helper()
	host := newFakeViewHost()
	confirm := dialog.NewConfirmDialog(host)
	connManager := dialog.NewConnManager(host, confirm)
	awsProfiles := dialog.NewAWSProfilesPicker(host)
	datadogEditor := dialog.NewDatadogEditor(host)
	themePicker := dialog.NewThemePicker(host)
	return host, NewSettingsView(host, themePicker, connManager, awsProfiles, datadogEditor)
}
```

Two additions beyond the port, matching what CR 82–84 already added
for every other view: `TestSettingsViewRefreshPreservesCursorPosition`
(select item 2, call `Refresh()` again, assert the cursor didn't
reset to 0 — the one piece of `Refresh()`'s own logic none of the 12
existing tests happens to cover) and `TestSettingsViewApplyPalette`
skipped — `ApplyPalette` stays untested, matching every other view (no
`ApplyPalette` in this codebase has a direct test; recoloring is a
`verify-live` concern), so removing the old inline block's behavior
doesn't need to gain a test it never had.

**Adapt in place** (already testing exactly the App-level wiring this
CR needs, just via the old raw field):
`TestRefreshSettingsListUpdatesTheme` (stays in `internal/app`,
renamed implicitly by what it now asserts through —
`a.settingsList.GetItemText(0)` → `a.settingsV.List().GetItemText(0)`)
and `host_test.go`'s `TestSetActiveAWSProfilePersistsAndUpdatesUI`
(same field-access fix, one line).

**Relocate to `theme_test.go`** (already there in spirit — they
exercise `switchTheme`'s own config mutation, not Settings; they
happened to live in `settings_test.go` only because that's where
`switchTheme`'s tests were first added, and now need a home once
`settings_test.go` itself leaves `internal/app`):
`TestSwitchThemeAppliesPalette`, `TestSwitchThemeUnknownIsNoOp`,
`TestSwitchThemePersistsConfig`.

**Genuinely new** (confirmed via grep: no existing test touches
whether `SaveConnection`/`switchConnection`/`SaveDatadogConfig`
refresh the settings list — unlike `SetActiveAWSProfile` and
`switchTheme`, which already had coverage): 3 new `internal/app`
wiring tests (in `viewwiring_test.go`, matching CR 84's precedent for
this shape of test) — `TestSwitchConnectionRefreshesSettingsList`,
`TestSaveConnectionRefreshesSettingsList`,
`TestSaveDatadogConfigRefreshesSettingsList`. Each drives the real
`a := New(config.Default())`, calls the mutator, and asserts
`a.settingsV.List()`'s relevant row text changed.

**New accessor needed for all of the above**: `List() *tview.List` on
`SettingsView`, mirroring `Table()` on the 8 list views CR 84 already
added for the identical reason (an `internal/app`-level test needs to
read the underlying primitive without unexported cross-package
access).

### 8. The physical move

Once (1)–(7) leave `settings.go` self-contained (no more
`a.cfg`/`a.settingsList`/`a.themePicker` reach-ins from outside),
`git mv internal/app/settings.go internal/view/settings.go` +
`git mv internal/app/settings_test.go internal/view/settings_test.go`
(the new test file created in step 7), `package app` → `package
view`, `app.go`'s field type and construction call gain the `view.`
qualifier — same mechanical last step as each of CR 84's 14.

### 9. Verification order

Steps 1–3 (the view's own new shape) → step 4–6 (`app.go`/`host.go`/
`theme.go` updated to match, still same package) → step 7 (tests,
still same package) → step 8 (the move). `gofmt -l`/`go build ./...`/
`go vet ./...`/`go test ./...` after each step. Final repo-wide pass,
then live verification.

## Files touched

- `internal/app/settings.go` → `internal/view/settings.go` (moved,
  rewritten per steps 1–3).
- New `internal/view/settings_test.go`.
- `internal/app/app.go` (dialog-construction reorder, field rename,
  construction call, `a.views`/`a.themables` literals, 2
  `Refresh()`-call-site updates).
- `internal/app/host.go` (3 `Refresh()`-call-site updates).
- `internal/app/theme.go` (special-cased block deleted).
- New wiring tests in `internal/app` (step 7).

## Key decisions

- **`Refresh()`/`ApplyPalette()` are relocations, not redesigns** —
  same reasoning CR 82–84 already established: the exact existing
  logic, moved onto the type it's actually about, not reworked.
- **Settings construction moves later in `New()`, not the 4 dialogs
  moved to before Settings while Settings itself stays put** — the
  early-dialog block already exists (CR 83) and already holds
  everything else `queues.go`/`messages.go` etc. need; extending it 4
  more entries is a smaller, more consistent diff than introducing a
  second distinct "early" zone.
- **`ApplyPalette` gets no live test, matching every other view** —
  this codebase has never unit-tested a view's recoloring; adding one
  just for Settings would be inconsistent with the established
  convention (`verify-live` for anything touching real rendering), not
  an improvement.
- **New `internal/app` wiring tests are additive, not relocated** —
  unlike CR 84's `viewwiring_test.go` entries (which moved existing
  assertions), nothing tested "does mutating config refresh the
  Settings list" before; this is new coverage the move surfaces a
  reason to add, matching `tui/CLAUDE.md`'s "a change without tests is
  not done."
- **No new dependencies** — `internal/dialog` is already an
  `internal/app` dependency; `settings.go` gains the same import the
  14 already-moved dialog-coupled views already have.

## Definition of done

Unchanged from spec.md — `SettingsView` exported in `internal/view`,
depending on `ui.ViewHost` + 4 `*dialog.X` params; no
`settingsList`/`refreshSettingsList` left in `internal/app`;
`theme.go` has no Settings special case; `go build`/`go test`/`go vet`
clean, `gofmt -l` clean, zero import cycle; new `Refresh()`/wiring
tests pass; live verification confirms no behavior change.
