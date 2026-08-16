# Spec — CR 62: move detail-navigation trampolines out of app.go

Date: 2026-08-16

## Background

CR 59–61 extracted overlay state out of `App`/`app.go` into per-feature
structs, cutting `app.go` from 1390 to 762 lines. The overlay wiring left
in `New()` is now minimal — one `newXxx(a)` call plus sizing per overlay.
The remaining bulk in `New()` (276 of `app.go`'s 762 lines) is a different
kind of clutter: not state that needs its own struct, but **logic that
already belongs to a specific view, sitting in the shared file instead of
that view's own file** — the same underlying complaint as CR 59, applied
to a different part of the file.

## Problem

`New()` contains 8 nearly-identical blocks of the shape "wire Enter on
view X's table to open view Y's detail", e.g.:

```go
// Wire Enter in the SSM parameters table to open the detail view for
// the selected parameter. Done here because paramDetailV must exist first.
a.ssmParamsV.table.SetSelectedFunc(func(row, _ int) {
	idx := row - 1
	if idx < 0 || idx >= len(a.ssmParamsV.filtered) {
		return
	}
	a.openParamDetail(a.ssmParamsV.filtered[idx])
})
```

Each block's target (`a.openParamDetail`, `a.openSecretDetail`, etc.) is
itself an `(a *App)` method defined later in `app.go`, even though it's
entirely about one specific view (e.g. `openParamDetail` only ever touches
`a.paramDetailV`). Both halves — the wiring block and the trampoline it
calls — belong with the view they operate on (`paramdetail.go`,
`secretdetail.go`, etc.), not in the shared shell file.

## Solution

For each of the 8 source→detail pairs, move both halves into the file
that already owns the *target* view:

| Source table | Trampoline (moves to) |
|---|---|
| `queuesV.table` | `openMessages` → `messages.go` |
| `messagesV.table` | `openMessageDetail` → `message_detail.go` |
| `ssmParamsV.table` | `openParamDetail` → `paramdetail.go` |
| `secretsV.table` | `openSecretDetail` → `secretdetail.go` |
| `logsV.table` | `openLogSearch` → `logsearch.go` |
| `logSearchV.table` | `openLogEventDetail` → `logdetail.go` |
| `datadogLogsV.table` | `openDatadogLogDetail` → `datadoglogdetail.go` |
| `codePipelineListV.table` | `openCodePipelineDetail` → `codepipelinedetail.go` |

The `open*` trampolines move verbatim (same package, same receiver — a
pure file move, no signature change, so nothing outside `app.go` needs to
change). Each inline `SetSelectedFunc` block becomes a small
`wire<Source>Opens<Target>()` method defined next to its trampoline in the
same target file, called from `New()` as a single line at the exact point
the inline block used to sit — preserving the existing construction-order
comments ("done here because X must exist first") exactly, just as a
function call instead of an inline closure.

Net effect: `New()` shrinks by roughly 90 lines (8 blocks → 8 one-line
calls) and `app.go` overall shrinks by roughly 230 lines (that plus the 8
moved trampolines, ~150 lines). No behavior change — construction order,
closure bodies, and every reference stay identical, just relocated.

## Scope

### In scope

- The 8 pairs in the table above: trampoline + matching wiring, moved to
  their target view's existing file.
- `app.go`'s `New()`: 8 inline blocks replaced by 8 one-line calls, in the
  same relative order (preserving the ordering comments' intent).

### Out of scope

- `homeSections` (the static home-menu data, ~30 lines in `New()`) — pure
  data with no wiring-order dependency, and moving it doesn't reduce
  `app.go`'s size unless it also gets a new file, which feels like
  overkill for 30 lines of config. Left alone.
- `openHelp`/`closeHelp`, `switchTheme`, `switchConnection`, `switchTo`,
  `copyToClipboard`, `updateContextPanel`, `activeView`, `colorBordered`
  — these are genuinely cross-cutting (touch multiple views, `a.cfg`,
  `a.pages` routing itself) or don't have a single obvious target file.
  Not the same pattern as the 8 pairs above; not touched here.
- Any behavior change. Pure code motion, as with CR 59–61.

## Definition of done

1. `go build ./...` and `go test ./...` pass in `tui/`.
2. `app.go` is meaningfully shorter (~230 fewer lines).
3. No behavior change: pressing Enter on each of the 8 source tables still
   opens the right detail view with the right content — verified live
   (`verify-live` skill) for at least 2–3 of the 8 (the rest have existing
   unit test coverage exercising the same `open*` functions, per each
   view's own `_test.go`).
