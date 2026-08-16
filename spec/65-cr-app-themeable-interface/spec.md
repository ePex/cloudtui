# Spec — CR 65: replace `reapplyTheme`'s hardcoded recolor with a `Themeable` interface

Date: 2026-08-16

## Background

This is phase 2 of the `internal/app` package split roadmap
(`spec/64-cr-app-package-split/spec.md`). Phase 1 moved the four chrome
files with zero coupling into `internal/ui`. `theme.go`'s `reapplyTheme`
was identified as the single biggest remaining coupling point — 249
lines that reach into ~18 other types by name (`a.queuesV.table`,
`a.confirm.flex`, `a.movePicker.list`, ...) to recolor them on a live
theme switch. That's fine as long as everything is one package; it's
exactly the kind of unexported cross-type access that blocks moving
those types into `internal/dialog`/`internal/view` later (phase 3/4).

Read in full for this spec: every one of `reapplyTheme`'s ~18 sections
follows the same shape — set background, set border/title color (via
`config.Palette.ViewColor(name)` or plain `p.Border`), and if the type
owns a filter input or dropdown, color that too. This is a pattern, not
18 unrelated pieces of logic.

### The existing gap

`reapplyTheme` does **not** recolor seven types that exist today:
`datadogLogsV`, `datadogLogDetailV`, `codePipelineListV`,
`codePipelineDetailV`, `messageFilter`, `timeRangeModal`,
`datadogEditor`. These were added (FE 39/43/46/53 and others) after
`reapplyTheme` was last extended, and nothing enforces that every new
themeable type gets added to this function — it's a hand-maintained
list with no compiler check. Switching themes live while any of these
seven is open leaves it showing the old palette until closed and
reopened.

## Problem

Two problems, one fix: (1) the hardcoded reach into 18 types is the
blocker for phases 3/4 of the package split, and (2) the same
hand-maintained-list pattern is *why* 7 types silently fell out of
coverage — the exact failure mode `onGlobalKey`'s dead nil-checks and
duplicated OR-chain had (fixed recently in PR #14) before this CR.

## Solution

Add a `Themeable` interface to `internal/ui` (already houses the other
shell↔view contracts, `View`/`Shortcuttable`):

```go
// Themeable is implemented by views/overlays that need to recolor
// themselves when the active theme changes.
type Themeable interface {
	ApplyPalette(p config.Palette)
}
```

Give each of the 25 view/overlay types below an `ApplyPalette(p
config.Palette)` method, in the same file as the type's other methods
(mirrors CR 59–61's "same file, no new file needed" precedent) — moving
existing logic out of `reapplyTheme` verbatim for the 18 already
covered, writing new (but pattern-matched) logic for the 7 gap types.
`App` gains one field, `themables []ui.Themeable`, built once in `New()`
after every view/overlay exists (mirrors the `focusExemptInputs`/
`overlayVisible` pattern from the `onGlobalKey` cleanup, PR #14).
`reapplyTheme` shrinks to the handful of things that aren't a separate
struct type (status bar, info panel, divider, context panel, logo panel
— core shell primitives directly on `App`; home table and settings list
— raw `*tview.Table`/`*tview.List` fields with no owning struct) plus
one loop: `for _, t := range a.themables { t.ApplyPalette(p) }`.

### Full type inventory

| Type | File | Already covered? | ViewColor key / notes |
|---|---|---|---|
| `themePicker` | settings.go | Yes | Border |
| `connManager` | connections.go | Yes | Border |
| `connEditor` | connections.go | Yes | Border (+ existing dropdown style call, preserved as-is incl. its known-dead type assertion — not this CR's job to fix) |
| `awsProfilesPicker` | awsprofiles.go | Yes | Border |
| `logView` | log.go | Yes | `ViewColor("log")` |
| `queuesView` | queues.go | Yes | `ViewColor("queues")` |
| `messagesView` | messages.go | Yes | `ViewColor("queues")` |
| `messageDetailView` | message_detail.go | Yes | `ViewColor("queues")` |
| `ssmParamsView` | ssmparams.go | Yes | `ViewColor("ssm-parameters")` |
| `paramDetailView` | paramdetail.go | Yes | `ViewColor("ssm-parameters")` |
| `secretsView` | secrets.go | Yes | `ViewColor("secrets-manager")` |
| `secretDetailView` | secretdetail.go | Yes | `ViewColor("secrets-manager")` |
| `logsView` | logs.go | Yes | `ViewColor("cloudwatch-logs")` |
| `logSearchView` | logsearch.go | Yes | `ViewColor("cloudwatch-logs")` |
| `logDetailView` | logdetail.go | Yes | `ViewColor("cloudwatch-logs")` |
| `movePicker` | movepicker.go | Yes | Border |
| `confirmDialog` | confirm.go | Yes | Border |
| `sendMessageOverlay` | sendmessage.go | Yes | Border |
| `datadogLogsView` | datadoglogs.go | **No (gap)** | `ViewColor("datadog-logs")`; also colors `queryInput` + both dropdowns via existing `styleDropDown` |
| `datadogLogDetailView` | datadoglogdetail.go | **No (gap)** | `ViewColor("datadog-logs")` (mirrors detail views reusing their list's key) |
| `codePipelineListView` | codepipelinelist.go | **No (gap)** | `ViewColor("codepipeline")`; also colors `filterInput` |
| `codePipelineDetailView` | codepipelinedetail.go | **No (gap)** | `ViewColor("codepipeline")` |
| `messageFilter` | messagefilter.go | **No (gap)** | Border (matches connEditor/datadogEditor form-overlay pattern) |
| `timeRangeModal` | timerangemodal.go | **No (gap)** | Border; colors `flex`/`tabs`/`pages`/`relativeList`/`absoluteForm` |
| `datadogEditor` | datadogsettings.go | **No (gap)** | Border (matches connEditor/messageFilter pattern) |

**Not converted** (stay in `reapplyTheme` as-is): `a.statusBar`,
`a.infoPanel`, `a.divider`, `a.contextPanel`, `a.logoPanel` (core shell
primitives, not owned by any view/overlay struct), `a.homeTable` (raw
table from a package-level `views.NewHome`/`views.RepaintHomeTable`
call, no owning struct), `a.settingsList` (raw list field, no
`settingsView` struct instance retained on `App` today).

## Scope

### In scope

- New file `internal/ui/theme.go`: the `Themeable` interface.
- 25 files gain one `ApplyPalette(p config.Palette) ` method each (18
  moved verbatim from `reapplyTheme`, 7 newly written per the table
  above).
- `app.go`: `App` gains `themables []ui.Themeable`, built once in `New()`.
- `theme.go`: `reapplyTheme` shrinks to core-shell/home/settings +
  the `themables` loop.
- Live verification (`verify-live` skill) that theme switching still
  works for a sample of the 18 already-covered types, **and** that all 7
  gap types now correctly recolor live — this second part is a real
  behavior change (a bug fix), not pure refactor, so needs explicit
  manual confirmation, not just "no regression."

### Out of scope

- Any package move — everything stays in `internal/app`/`internal/ui`
  as today; phase 3 (new `internal/dialog` package) is separate.
- Fixing `connEditor`'s known-dead `GetFormItem(2)` type assertion
  (documented as intentionally preserved, pre-existing, unrelated bug).
- Any other behavior change beyond closing the 7-type recolor gap.

## Definition of done

1. `go build ./...` and `go test ./...` pass in `tui/`.
2. `theme.go`'s `reapplyTheme` is meaningfully shorter (from 249 lines
   to roughly the core-shell/home/settings sections + a ~5-line loop).
3. No behavior change for the 18 already-covered types — verified live
   for a representative sample (at least one table view, one detail
   view, one form overlay, one list overlay).
4. The 7 previously-uncovered types now correctly recolor on a live
   theme switch — verified live for all 7, since this is the one part
   of this CR that's an actual bug fix, not pure refactor.
