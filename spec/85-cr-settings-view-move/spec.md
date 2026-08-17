# Spec — CR 85: adopt `ui.ViewHost` in `settings.go` and move it to `internal/view`

Date: 2026-08-17

## Background

Phase 4's original scope (`spec/64`) lists `settings.go` alongside the
14 files CR 82–84 already moved into `internal/view`. It was deliberately
left out of that sequence — CR 79's audit flagged it as having "a
deeper, different pre-existing wrinkle" worth its own look, rather than
folding it into the mechanical per-file pattern the other 14 used.

Reading the file fresh (not reusing CR 79's summary) confirms exactly
that, and adds detail:

**1. `settingsView` is a thinner facade than any of the 14 already
moved.** Its struct is just `{ list *tview.List }`, with only
`Name()`/`Title()`/`Primitive()` — no `Shortcuts()`, no
`ApplyPalette()`. Every other view in this app implements
`ui.Themeable`; Settings doesn't.

**2. `*App` has no `settingsV` field at all** — unlike the 14 already
moved (each has an `a.xxxV *view.XxxView` field), Settings' only
handle is `a.settingsList *tview.List`, the raw `tview.List` widget
itself, set once in `newSettingsView` (`a.settingsList = l`) and never
touched again through the `settingsView` wrapper — every other read/
write goes directly through `a.settingsList`.

**3. The view's real logic lives entirely in `(a *App)`-scoped code.**
`(a *App) refreshSettingsList()` (not a method on `settingsView` at
all) rebuilds the 4 list rows from live config
(`a.cfg.Theme`/`.ActiveConn()`/`.ActiveAWSProfile`/`.Datadog`) and
wires each row's `Enter` action straight to a dialog's `Show()`
(`a.themePicker`, `a.connManager`, `a.awsProfiles`,
`a.datadogEditor` — 4 dialogs, the same "constructor-injected
`*dialog.X` params" shape CR 83 already established for `queues.go`
et al.). It's called from **6 external sites**, not just
`settings.go` itself: `app.go`'s `switchTheme`/`switchConnection`,
and `host.go`'s `SaveConnection`/`SaveDatadogConfig`/
`SetActiveAWSProfile` (all 3 `ui.Host` methods that mutate config the
settings list displays).

**4. Theme application is special-cased, not polymorphic.**
`theme.go`'s `reapplyTheme` applies the palette to every `ui.Themeable`
generically via the `a.themables` loop — except Settings, which gets a
hand-written inline block reaching `a.settingsList` directly
(`SetBackgroundColor`/`SetBorderColor`/`SetTitleColor`/
`ui.StyleList`), guarded by a nil check, sitting *before* the loop
rather than inside it. This is the one view in the entire app that
doesn't go through `ui.Themeable` — not a missing export, a missing
interface implementation.

## Problem

None of this is reachable from a `settings.go` living in a different
package: `refreshSettingsList`'s logic, the `a.settingsList` field
itself, and `theme.go`'s inline recolor block all assume same-package
access to `*App`'s internals, unlike the already-moved 14 which only
needed narrow exported accessors bolted onto an otherwise-complete
view type.

## Solution

1. **Give `SettingsView` real ownership of its own list**, matching
   every other moved view: struct gains `host ui.ViewHost` and the 4
   dialog fields (`themePicker *dialog.ThemePicker`, `connManager
   *dialog.ConnManager`, `awsProfiles *dialog.AWSProfilesPicker`,
   `datadogEditor *dialog.DatadogEditor`); `*App` gains a real
   `settingsV *view.SettingsView` field, replacing the raw
   `settingsList *tview.List` field entirely.
2. **Fold `refreshSettingsList`'s body into an exported
   `Refresh()` method** on `SettingsView`, using the injected dialogs
   instead of `a.themePicker.Show()` etc. All 6 external call sites
   become `a.settingsV.Refresh()`.
3. **Add `ApplyPalette(p config.Palette)`**, folding in `theme.go`'s
   special-cased block verbatim; add `a.settingsV` to `a.themables`
   and delete the inline block and its nil-guard entirely — this
   view stops being a special case in `reapplyTheme`, not just gains
   an export.
4. **Move the file** (`settings.go` + a new `settings_test.go` — none
   exists today, see Scope) into `internal/view`, alongside the 14
   already there, once (1)–(3) leave it self-contained.

## Scope

### In scope

- `settings.go`: struct/constructor changes per Solution; `Refresh()`
  and `ApplyPalette()` added.
- `app.go`: `settingsList` field removed, `settingsV *view.SettingsView`
  added; the 2 `refreshSettingsList()` call sites there updated;
  `a.themables` gains `a.settingsV`.
- `host.go`: the 3 `refreshSettingsList()` call sites updated.
- `theme.go`: the special-cased Settings block removed.
- `internal/app/settings_test.go` **does exist already** (12 tests) —
  a correction to this spec's first draft, which claimed otherwise
  before actually grepping for it. 9 of its 12 tests are genuinely
  view-level (row text reflects config, the Datadog-label helper) and
  port to a new `internal/view/settings_test.go` largely unchanged,
  just switched to the `fakeViewHost` construction pattern; 3
  (`TestSwitchThemeAppliesPalette`/`Unknown`/`PersistsConfig`) don't
  touch Settings at all — they test `switchTheme`'s own config
  mutation — and relocate to `theme_test.go`, their natural home once
  `settings_test.go` itself leaves `internal/app`.
  `host_test.go`'s `TestSetActiveAWSProfilePersistsAndUpdatesUI`
  already asserts the settings list updates too — adapted to the new
  accessor, not rewritten. Only `SaveConnection`/`switchConnection`/
  `SaveDatadogConfig` genuinely lack any refresh-wiring coverage today
  — those 3 get new `internal/app` wiring tests.
- New `List() *tview.List` accessor on `SettingsView`, mirroring the
  `Table()` accessor CR 84 added to every list view for the same
  reason: the relocated/adapted `internal/app` wiring tests need to
  read a row's text without reaching an unexported field across the
  package boundary.
- `gofmt`/`go vet`/`go build`/`go test` clean; live verification.

### Out of scope

- `log.go`, `codepipelinewatch.go` — phase 4's other 2 remaining
  files. `log.go` looks shaped like the already-moved 14 (a real
  `newLogView(a *App) *logView` with its own field on `*App`) and
  probably just needs CR 82-style adoption; `codepipelinewatch.go`
  has no view type of its own to move at all — CR 79 called it "a
  problem for the future accessor-methods CR." Neither is touched
  here; each gets its own audit when its turn comes.
- Phase 5 (`connectionsecrets.go`'s `secretBackend`) — explicitly
  backlogged, unrelated to this split.
- Any behavior change — `Refresh()`/`ApplyPalette()` are the exact
  existing logic relocated, not redesigned.

## Definition of done

1. `internal/view/settings.go` holds `SettingsView`, exported,
   depending on `ui.ViewHost` + 4 `*dialog.X` params; `internal/app`
   has no `settingsList` field and no `refreshSettingsList` method.
2. `theme.go` has no Settings special case; `SettingsView` recolors
   via `a.themables` like every other view.
3. `go build`/`go test`/`go vet` clean, `gofmt -l` clean, zero import
   cycle.
4. New `internal/view/settings_test.go` covers `Refresh()`/
   `ApplyPalette()`; a small `internal/app` wiring test covers the 6
   mutation-triggers-refresh call sites.
5. Live verification: Settings list shows correct live values on
   open; each of the 4 rows opens its picker; changing the theme,
   switching a connection, changing the AWS profile, and editing the
   Datadog config each visibly update the Settings list immediately;
   a live theme switch recolors the Settings list correctly.
6. No behavior change.
