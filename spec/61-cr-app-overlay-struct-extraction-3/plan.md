# Plan — CR 61: extract the remaining four overlay groups into dedicated structs

## Approach

Same shape as CR 59/60 for all four: a struct with an `app *App`
back-reference, methods hung off it, one `App` field replacing N flat
fields, construction moved into a `new*(a *App) *T` constructor. Four
independent sub-slices, each build-and-test clean before the next.

## 1. Connection manager + editor (`connections.go`)

```go
type connManager struct {
	app     *App
	flex    *tview.Flex
	list    *tview.List
	hints   *tview.TextView
	visible bool
}

type connEditor struct {
	app        *App
	form       *tview.Form
	visible    bool
	isNew      bool
	origName   string
	brokerName string // see spec/57-bugfix-broker-name-proxy-hidden
}
```

`showConnectionManager`/`closeConnManager`/`populateConnManagerList` →
`connManager` methods. `showConnEditor`/`setConnEditorPasswordField`/
`rebuildConnEditorTail`/`closeConnEditor`/`saveConnEditor` →
`connEditor` methods. `deleteConnFromManager` stays a `connManager`
method (it operates on the manager's list, just constructs/opens the
editor via `app.connEditor` for the "d"/duplicate path — same
cross-struct reach `connEditor` needs for `app.connManager`).

`App` fields: `connManager *connManager`, `connEditor *connEditor`.
Construction in `New()`: `a.connManager = newConnManager(a)`,
`a.connEditor = newConnEditor(a)`.

Call sites to update: `app.go` (`AddPage`/height wiring — field paths
only, order unchanged), the OR-chains, and any place outside
`connections.go` calling into these (checked via
`grep -rn 'a\.showConnectionManager\|a\.showConnEditor\|qv\.app\.showConnectionManager'`
— expected in `app.go`'s command-prompt handling for `:aq`/`:connections`
and possibly `queues.go`). `connections_test.go` and `app_test.go`'s
`:aq`/`:connections` tests updated to the new paths.

## 2. Time range modal (`timerangemodal.go`)

```go
type timeRangeModal struct {
	app          *App
	flex         *tview.Flex
	tabs         *tview.TextView
	pages        *tview.Pages
	relativeList *tview.List
	absoluteForm *tview.Form
	visible      bool
	activeTab    timeRangeMode
	onApply      func(timeRange)
}
```

`showTimeRangeModal` → `show`, `closeTimeRangeModal` → `close`,
`switchTimeRangeTab` → `switchTab`, `renderTimeRangeTabs` → `renderTabs`,
`applyTimeRangeRelative` → `applyRelative`, `applyTimeRangeAbsolute` →
`applyAbsolute`. `formatTimeRangeDateTime` stays a package-level function
(no `*App`/struct dependency).

`App` field: `timeRangeModal *timeRangeModal`. Two callers —
`logSearchView` and `datadogLogsView` — call
`a.showTimeRangeModal(current, onApply)` today; becomes
`a.timeRangeModal.show(current, onApply)`. Update
`timerangemodal_test.go` (12 tests), plus the `~6` lines each in
`datadoglogs_test.go` and `logsearch_test.go` that read
`a.timeRangeVisible`/`a.timeRangeRelativeList`/`a.timeRangeOnApply` —
`timeRange{...}` struct literals elsewhere in those files are the
**value type**, untouched.

## 3. Theme picker (`app.go` construction → `settings.go`)

```go
type themePicker struct {
	app     *App
	flex    *tview.Flex
	list    *tview.List
	visible bool
}
```

`showThemePicker`/`closeThemePicker` (currently in `settings.go`, already
the right file) become methods; construction (currently in `app.go`)
moves into a `newThemePicker(a *App) *themePicker` in `settings.go` — no
new file, matching CR 60's precedent of not creating a file for logic
that already lives in the right one. `App` field: `themePicker
*themePicker`. No test file today — none to update.

## 4. AWS profiles picker (`awsprofiles.go`)

```go
type awsProfilesPicker struct {
	app         *App
	flex        *tview.Flex
	table       *tview.Table
	filterInput *tview.InputField
	hints       *tview.TextView
	visible     bool
	filter      string
	all         []awsprofile.Profile
	filtered    []awsprofile.Profile
}
```

Named `awsProfilesPicker`, not `awsProfiles` — too easy to confuse with
the imported `awsprofile` package (singular) at a glance. `showAWSProfiles`/
`closeAWSProfiles`/`populateAWSProfilesTable`/`applyAWSProfilesFilter`/
`repaintAWSProfiles`/`activateAWSProfile`/`setAWSProfilesHeader` all
become methods (shortened: `show`/`close`/`populate`/`applyFilter`/
`repaint`/`activate`/`setHeader`). `App` field: `awsProfiles
*awsProfilesPicker`. Update `awsprofiles_test.go` (extensive — every
`a.awsProfilesX`/`a.xAWSProfiles(...)` reference) and the `~6` lines in
`app_test.go` covering `:ap`/`:awsprofiles`.

## Shared across all four

- `app.go`'s two OR-chain lines (`onGlobalKey`'s exemption list,
  `onPromptDone`'s focus-restore check) updated for all four flags in one
  pass at the end, once all four structs exist — after this they read
  purely as eight `.visible` checks.
- `theme.go`'s `reapplyTheme` already has sections for connection
  manager/editor, theme picker, and AWS profiles (from FE 22/28) — those
  get their field paths updated the same way CR 59 did for confirm/
  move-picker/send-message. **Correction from the first draft of this
  plan**: theme picker *does* have a `reapplyTheme` section (missed on
  first pass) — updated along with the rest, not skipped. Time range
  modal is the only one of the four with no `reapplyTheme` section today
  (checked — confirmed not covered); leaving that gap alone, same
  reasoning as CR 60's message filter/Datadog settings.

## Testing

Existing test suites for connection manager/editor, time range modal, and
AWS profiles are substantial — access-path updates only, same assertions.
No new tests. Theme picker has no existing coverage and this CR doesn't
add any (pure structural move of already-untested code — out of scope to
newly test it here, though `verify-live` covers it, see below).

Manual (`verify-live` skill):

- Connection manager/editor: open manager, new/edit/duplicate/delete a
  connection, confirm activate/save/cancel all still work against the
  real local broker.
- AWS profiles: open, filter, activate a profile, confirm the info panel
  and Settings list update.
- Time range modal and theme picker: lighter sanity pass (switch tabs,
  apply a relative preset, switch theme) — existing/added test coverage
  is the primary safety net for these two.
