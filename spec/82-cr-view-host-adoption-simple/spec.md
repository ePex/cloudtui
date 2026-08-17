# Spec — CR 82: adopt `ui.ViewHost` in the 9 dialog-free view types

Date: 2026-08-17

## Background

With CR 81 done, the next phase-4 step is switching view types from
`app *App` to `host ui.ViewHost` (CR 80's interface) — the same
adoption CR 68/69 did for dialogs against `ui.Host`. Audited all 12
view + detail-view files' direct `internal/dialog` calls first, since
that determines the split: 5 files (`queues.go`, `messages.go`,
`message_detail.go`, `logsearch.go`, `datadoglogs.go`) call a dialog's
`Show(...)` directly (`.confirm`, `.movePicker`, `.sendMessage`,
`.messageFilter`, `.timeRangeModal`) — per CR 80's design decision,
`ui.ViewHost` deliberately doesn't expose those, so these 5 need new
direct `*dialog.X` fields alongside the `ui.ViewHost` switch, not just
a mechanical rename. The other 9 (`ssmparams.go`, `paramdetail.go`,
`secrets.go`, `secretdetail.go`, `logs.go`, `logdetail.go`,
`datadoglogdetail.go`, `codepipelinelist.go`, `codepipelinedetail.go`)
have no dialog coupling at all. This CR is those 9; the 5 dialog-
coupled ones are next.

Audited every `.app.X` call site in the 9 files (not just counted
them) before assuming the rename is purely mechanical — glad it
wasn't assumed: **13 of the ~19 distinct symbols these files touch
reach a raw `*App` field or the old unexported func field directly**,
not the exported `ui.ViewHost` method CR 80 already added for exactly
this:

| Raw access today | Exported `ui.ViewHost` equivalent |
|---|---|
| `.app.tv.SetFocus(p)` | `.app.SetFocus(p)` |
| `.app.tv.QueueUpdateDraw(f)` | `.app.QueueUpdateDraw(f)` |
| `.app.cfg` (`.Colors`, `.ActiveAWSProfile`, ...) | `.app.Config()` |
| `.app.contextPanel.SetText(text)` | `.app.SetContextHint(text)` |
| `.app.statusBar.SetText(text)` | `.app.SetStatus(text)` |
| `.app.awsAuthTypeFor(ctx, profile)` | `.app.AWSAuthTypeFor(ctx, profile)` |
| `.app.awsSSOLogin` (passed as a bare func value) | `.app.AWSSSOLogin` (method value — same call shape) |
| `.app.listParameters(...)` | `.app.ListParameters(...)` |
| `.app.listSecrets(...)` | `.app.ListSecrets(...)` |
| `.app.listLogGroups(...)` | `.app.ListLogGroups(...)` |
| `.app.listPipelines(...)` | `.app.ListPipelines(...)` |
| `.app.revealParameter(...)` | `.app.RevealParameter(...)` |
| `.app.revealSecret(...)` | `.app.RevealSecret(...)` |
| `.app.getPipelineState(...)` | `.app.GetPipelineState(...)` |

The other 6 symbols (`CopyToClipboard`, `SetPendingCloudWatchPattern`,
`IsWatchingPipeline`, `StartWatchingPipeline`, `StopWatchingPipeline`,
`SwitchTo`) are already the exported method — those call sites only
need the `.app.` → `.host.` rename, no shape change.

Both today's raw-field calls and their `ui.ViewHost` replacements do
the exact same thing (`SetFocus`/`SetStatus`/etc. are thin wrappers
CR 80 already verified against the real fields) — this is a
same-behavior substitution, not a redesign.

**Second finding, reading each file in full (not just grepping
`.app.`)**: 5 of the 9 — the detail views (`paramdetail.go`,
`secretdetail.go`, `logdetail.go`, `datadoglogdetail.go`,
`codepipelinedetail.go`) — have an Esc/Backspace handler that reaches
directly into a *sibling* view to return there:

```go
// paramdetail.go, unchanged shape in the other 4 (logdetail.go
// additionally can't use UpdateContextPanel — see below)
case event.Key() == tcell.KeyEscape, ...:
	a.pages.SwitchToPage("ssm-parameters")
	a.tv.SetFocus(a.ssmParamsV.table)
	a.UpdateContextPanel(a.ssmParamsV)
```

This is the reverse of CR 79/81's forward "open" trampolines — going
*back* to the view that opened this detail view — and nothing built
so far covers it: `ui.ViewHost` has no way to reach a sibling view,
and `SwitchTo(name)` (already on `ViewHost`) isn't a safe substitute —
it also calls the target's `Activate()` if it implements
`activatable`, which for `ssmParamsView` means an unwanted `load()`
network reload on every Esc, not present in today's behavior.
`logdetail.go` is a further variant: `logSearchView` isn't a
registered `ui.View` (no `Name()`/`Title()`), so it can't use
`UpdateContextPanel` at all — it manually rebuilds the context panel
from `a.logSearchV.Shortcuts()` instead, same shape as `viewwiring.go`'s
`OpenLogSearch`.

Fixed with the same tool CR 81 already established, just pointed the
other direction: each of these 5 constructors gains an `onBack func()`
parameter; `app.go` supplies the exact closure shown above (verbatim,
including `logdetail.go`'s manual context-panel rebuild) — that
closure lives in `internal/app`, so it can freely reach
`a.ssmParamsV.table` etc., same reasoning as `viewwiring.go`'s own
methods.

## Problem

Once these 9 files depend on `ui.ViewHost` instead of `*App`, none of
`.tv`/`.cfg`/`.contextPanel`/`.statusBar`, the lowercase func fields,
or a sibling view's fields are reachable — they're unexported members
of a concrete type (or a different concrete type entirely) the
interface deliberately doesn't expose.

## Solution

For each of the 9 files:

1. Struct field `app *App` → `host ui.ViewHost` (matches
   `internal/dialog`'s existing field name for the same role — every
   dialog type already names this field `host`, not `app`).
2. Constructor parameter `a *App` → `a ui.ViewHost` (parameter name
   `a` kept as-is — only its type changes; `app.go`'s call sites need
   zero changes, since `*App` already satisfies `ui.ViewHost`
   structurally).
3. Every `.app.X`/`a.X` call site updated per the table above —
   either a straight `.app.` → `.host.` rename (6 symbols), or a
   rename plus switching to the exported method (13 symbols).
4. The 5 detail views' constructors gain an `onBack func()` parameter,
   replacing their Esc-handler's sibling-reaching body with a single
   `onBack()` call; `app.go`'s 5 construction call sites pass the
   matching closure.

## Scope

### In scope

- `ssmparams.go`, `paramdetail.go`, `secrets.go`, `secretdetail.go`,
  `logs.go`, `logdetail.go`, `datadoglogdetail.go`,
  `codepipelinelist.go`, `codepipelinedetail.go`: field/parameter type
  swap, all call sites updated per the table above.
- The 5 detail views' `onBack` callback addition (see Solution) and
  `app.go`'s 5 matching construction-call updates.
- `gofmt`/`go vet`/`go build`/`go test` clean across `tui/`.

### Out of scope

- `queues.go`, `messages.go`, `message_detail.go`, `logsearch.go`,
  `datadoglogs.go` — the 5 dialog-coupled views, next CR (need new
  `*dialog.X` constructor fields, not just this rename).
- The physical move of any view file into `internal/view` — later,
  once all 14 files depend on `ui.ViewHost`.
- Any behavior change — every substitution in the table above is a
  same-behavior swap (CR 80 already verified each `ui.ViewHost`
  method wraps the exact field being replaced here), and the 5
  `onBack` closures are the exact same code, just relocated to
  `app.go` and invoked through a callback instead of inline.

### Live verification

Unlike CR 79/81's own callback-injection changes (also relocations,
also live-verified), this one touches real navigation on the Esc key
for 5 detail views — worth a quick live check via `verify-live` that
Esc from each of the 5 (SSM param detail, secret detail, log event
detail, Datadog log detail, CodePipeline detail) returns to the right
list view with focus on the table and the context panel showing that
list's shortcuts, not just trusting `go test`.

## Definition of done

1. `go build ./...` and `go test ./...` pass in `tui/`.
2. All 9 files hold `host ui.ViewHost` instead of `app *App`; zero
   remaining raw-field or unexported-func-field access to anything
   `ui.ViewHost` already exposes; the 5 detail views' Esc handlers
   call `onBack()` instead of reaching into a sibling view.
3. `gofmt -l` reports nothing; `go vet ./...` clean.
4. Esc-back live-verified for all 5 detail views.
5. No behavior change.
