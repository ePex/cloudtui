# Plan — CR 65: `Themeable` interface

## Approach

Four ordered steps, each independently buildable/testable:

### 1. Define the interface

`internal/ui/theme.go` (new file):

```go
package ui

import "github.com/ePex/cloudtui/tui/internal/config"

// Themeable is implemented by views/overlays that need to recolor
// themselves when the active theme changes.
type Themeable interface {
	ApplyPalette(p config.Palette)
}
```

`internal/ui` already imports `internal/config` (topbar.go), so this
adds no new dependency.

### 2. Convert the 18 already-covered types

For each type in the "already covered" rows of spec.md's table: add an
`ApplyPalette(p config.Palette)` method in the type's existing file,
body moved verbatim from `reapplyTheme`'s corresponding section (same
`tcell.GetColor`/`p.ViewColor(...)`/`styleList`/`styleDropDown` calls,
just addressed via the method's own receiver instead of `a.xxx`). Add a
compile-time assertion next to each type's existing `var _ ui.View = ...`
/ `var _ ui.Shortcuttable = ...` lines (the codebase's existing
convention, e.g. `queues.go`): `var _ ui.Themeable = (*queuesView)(nil)`.

`App` gains `themables []ui.Themeable` (`app.go`), built in `New()`
right after `focusExemptInputs`/`overlayVisible` (same spot, same
reasoning: everything it references must exist first). `reapplyTheme`'s
18 corresponding sections are deleted and replaced by one loop:

```go
for _, t := range a.themables {
	t.ApplyPalette(p)
}
```

Nil-guards each section currently has (`if a.queuesV != nil { ... }`)
are dropped the same way the `onGlobalKey` cleanup dropped them — every
entry in `themables` is only ever populated once, after every view/
overlay is unconditionally constructed in `New()`.

**Special case — `connEditor`:** its section includes a
`GetFormItem(2).(*tview.DropDown)` type assertion already documented in
`theme.go` as a known, pre-existing dead no-op (wrong index, unrelated
bug). Moves verbatim, unfixed — fixing it is a behavior change, out of
this CR's scope per spec.md.

Live-verify (`verify-live` skill) a representative sample after this
step: one table view (`queuesV`), one detail view (`messageDetailV`),
one form overlay (`connEditor` or `datadogEditor`... — not yet
converted, use `connEditor`), one list overlay (`confirm` or
`movePicker`). Confirm identical appearance/behavior to before.

### 3. Convert the 7 gap types

Same mechanics, new logic instead of moved logic — written by
pattern-matching the nearest already-converted sibling:

- `datadogLogsView.ApplyPalette`: background + `ViewColor("datadog-logs")`
  border/title on `table`, `queryInput` labeled/field-colored (mirrors
  `logsView`'s `filterInput` pattern), `serviceFilterDD`/`envFilterDD`
  via the existing `styleDropDown` helper (already used by `connEditor`).
- `datadogLogDetailView.ApplyPalette`: background + `ViewColor("datadog-logs")`
  on `textView` (mirrors `messageDetailView`/`paramDetailView`/etc.
  reusing their parent list's key).
- `codePipelineListView.ApplyPalette`: background + `ViewColor("codepipeline")`
  on `table`, `filterInput` labeled/field-colored (mirrors `logsView`).
- `codePipelineDetailView.ApplyPalette`: background + `ViewColor("codepipeline")`
  on `table` (mirrors other detail views).
- `messageFilter.ApplyPalette`: background + Border on `form` (mirrors
  `connEditor`'s own form-overlay pattern, pre-conversion).
- `timeRangeModal.ApplyPalette`: background + Border on `flex`; `tabs`
  background; `pages` background; `relativeList` via `styleList` +
  background; `absoluteForm` background + Border (mirrors `movePicker`'s
  multi-widget overlay pattern for the list/form split).
- `datadogEditor.ApplyPalette`: background + Border on `form` (same
  pattern as `messageFilter`/`connEditor`).

Add all 7 to `a.themables` in `New()`, alongside the 18 from step 2 (one
combined slice literal, not two).

Live-verify all 7 explicitly: open each (`:` command or hotkey per
`verify-live`'s key reference, e.g. `datadog-logs` view, then `t` for
time-range modal; `codepipeline` view; message filter via `/` or its
bound key in the messages view; Datadog settings editor via Settings),
switch theme (`:theme <other-theme>` or the theme picker), confirm the
overlay/view recolors instead of staying on the old palette — this is
the bug the CR fixes, so a before/after comparison matters more than for
step 2's pure-refactor types.

### 4. Final pass

Confirm `reapplyTheme`'s final shape (core shell + home table + settings
list + the one loop), `gofmt -l`/`go vet`/`go build`/`go test` clean
repo-wide.

## Files touched

- New: `internal/ui/theme.go`
- Modified (one `ApplyPalette` method + one `var _ ui.Themeable = ...`
  line each): `settings.go` (themePicker), `connections.go` (connManager
  + connEditor — two methods, one file), `awsprofiles.go`, `log.go`,
  `queues.go`, `messages.go`, `message_detail.go`, `ssmparams.go`,
  `paramdetail.go`, `secrets.go`, `secretdetail.go`, `logs.go`,
  `logsearch.go`, `logdetail.go`, `movepicker.go`, `confirm.go`,
  `sendmessage.go`, `datadoglogs.go`, `datadoglogdetail.go`,
  `codepipelinelist.go`, `codepipelinedetail.go`, `messagefilter.go`,
  `timerangemodal.go`, `datadogsettings.go` (24 files — connections.go
  covers 2 types)
- Modified: `app.go` (`themables` field + slice literal in `New()`),
  `theme.go` (`reapplyTheme` shrinks to core-shell/home/settings + loop)

## Key decisions

- **Interface lives in `internal/ui`, not `internal/app`**, even though
  this CR moves no files — so phases 3/4 (when these types physically
  move to `internal/dialog`/`internal/view`) inherit `ApplyPalette`
  already satisfying `ui.Themeable`, no further change needed then.
  Mirrors why `View`/`Shortcuttable` already live there.
- **One combined `themables` slice, not per-category slices** — nothing
  downstream needs to distinguish "view" from "overlay" themables, so a
  second dimension would be unused complexity.
- **The 7 gap fixes are hand-written, not copy-pasted from a template**
  — each pattern-matches its nearest sibling (noted per-type above), but
  gets its own read-through against that type's actual fields (confirmed
  during spec.md's research) rather than assumed identical.
- **No new tests.** Per `tui/CLAUDE.md`'s existing note on `styleList`
  (`tview.List` exposes no getter for selected-item style), most of this
  can't be asserted via unit test — live verification is the actual
  safety net, as it already was for `reapplyTheme` before this CR.
- **No new dependencies.**

## Definition of done

Unchanged from spec.md — `reapplyTheme` shrinks to core-shell/home/
settings + loop, no behavior change for the 18 already-covered types,
all 7 gap types verified live to now recolor correctly.
