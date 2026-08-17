# Plan — CR 82: `ui.ViewHost` adoption, 9 dialog-free views

## Approach

### 1. Per-file mechanical steps (same shape for all 9)

1. Struct field: `app *App` → `host ui.ViewHost` (gofmt re-aligns the
   struct's other field columns automatically — don't hand-align).
2. Constructor: `func newXView(a *App, ...) *xView` → `func
   newXView(a ui.ViewHost, ...) *xView` — parameter name `a` unchanged,
   only its type changes. The struct literal inside
   (`&xView{..., app: a, ...}`) becomes `&xView{..., host: a, ...}`.
3. Every call site touching the receiver's `.app` field
   (`xv.app.X`) becomes `xv.host.X`; every call site touching the
   constructor's own `a` parameter directly (some files mix both for
   the same value, e.g. inside closures built before `xv` exists)
   stays `a.X` — only the *symbol name* on the right of the dot changes
   per the table below, not which of `a`/`xv.host` is used at each
   site (preserve today's mix, don't unify it — out of scope, no
   behavior reason to touch it).
4. `import "github.com/ePex/cloudtui/tui/internal/ui"` added if not
   already present (needed for the `ui.ViewHost` type in the
   constructor signature).

### 2. Exact rename table (applies within all 9 files, wherever the
   symbol appears)

| Old | New |
|---|---|
| `.tv.SetFocus(` | `.SetFocus(` |
| `.tv.QueueUpdateDraw(` | `.QueueUpdateDraw(` |
| `.cfg.Colors` | `.Config().Colors` |
| `.cfg.ActiveAWSProfile` | `.Config().ActiveAWSProfile` |
| `.contextPanel.SetText(` | `.SetContextHint(` |
| `.statusBar.SetText(` | `.SetStatus(` |
| `.awsAuthTypeFor(` | `.AWSAuthTypeFor(` |
| `.awsSSOLogin` (bare, no parens — passed as a func value to `awsauth.WithReauth`) | `.AWSSSOLogin` (same bare form — a method value has the identical call shape as the old field value) |
| `.listParameters(` | `.ListParameters(` |
| `.listSecrets(` | `.ListSecrets(` |
| `.listLogGroups(` | `.ListLogGroups(` |
| `.listPipelines(` | `.ListPipelines(` |
| `.revealParameter(` | `.RevealParameter(` |
| `.revealSecret(` | `.RevealSecret(` |
| `.getPipelineState(` | `.GetPipelineState(` |
| `.CopyToClipboard(` / `.SetPendingCloudWatchPattern(` / `.IsWatchingPipeline(` / `.StartWatchingPipeline(` / `.StopWatchingPipeline(` / `.SwitchTo(` | unchanged (already exported) — only the `.app`/`.host` prefix changes |

### 3. Per-file notes (anything not covered by the generic steps)

- **`ssmparams.go`** (12 sites): straightforward — `.tv`×3,
  `.cfg`×3, `.awsAuthTypeFor`×1, `.awsSSOLogin`×1 (bare),
  `.listParameters`×1, `.tv.QueueUpdateDraw`×2 (inside the `load()`
  goroutine's two callback closures).
- **`paramdetail.go`** (9 sites): `.cfg`×2, `.contextPanel`×1,
  `.statusBar`×2, `.revealParameter`×1, `.tv.QueueUpdateDraw`×1
  (`.cfg.ActiveAWSProfile`×1 counted in the `.cfg`×2 above — 2 total
  `.cfg` uses: `.Colors` once, `.ActiveAWSProfile` once).
- **`secrets.go`** (12 sites): same shape as `ssmparams.go`
  (`.listSecrets` instead of `.listParameters`).
- **`secretdetail.go`** (10 sites): same shape as `paramdetail.go`
  plus one extra `.statusBar` call (the binary-secret "cannot copy"
  status message) — 3 `.statusBar` total, `.revealSecret`×1.
- **`logs.go`** (12 sites): same shape as `ssmparams.go`/`secrets.go`
  (`.listLogGroups`).
- **`logdetail.go`** (5 sites): smallest of the 9 — `.statusBar`×1,
  `.cfg`×2, `.contextPanel`×1. No async data-fetch (renders an
  already-fetched event, like all 6 detail views).
- **`datadoglogdetail.go`** (8 sites): `.statusBar`×2 (copy
  confirmation + "no CorrelationID found" warning), `.cfg`×2,
  `.contextPanel`×1. No `.tv`/data-fetch calls (confirmed by CR 81's
  audit — this file's only other App reach,
  `pendingCloudWatchPattern`, already goes through the exported
  `SetPendingCloudWatchPattern` setter per CR 81's fix).
- **`codepipelinelist.go`** (16 sites): same shape as
  `ssmparams.go`/`secrets.go`/`logs.go` (`.listPipelines`) — largest
  of the "list view" group because of the extra `.cfg.Colors` reads in
  `repaint`'s per-row coloring.
- **`codepipelinedetail.go`** (13 sites): same shape as the other
  detail views but with the async `.getPipelineState` data-fetch (this
  is the one detail view that *does* fetch — pipeline stage status is
  polled/loaded after the page opens, unlike the other 5 which render
  an already-resolved value).

### 4. `onBack` callback for the 5 detail views

Each of the 5 detail-view constructors gains a trailing `onBack
func()` parameter; the Esc/Backspace case in each's `SetInputCapture`
becomes a single `onBack()` call, replacing the inline sibling-reach.
`app.go`'s 5 construction call sites pass the exact closure moved
verbatim from the file it came from:

```go
// paramdetail.go's constructor loses this body...
a.pages.SwitchToPage("ssm-parameters")
a.tv.SetFocus(a.ssmParamsV.table)
a.UpdateContextPanel(a.ssmParamsV)

// ...app.go gains it as the onBack argument:
a.paramDetailV = newParamDetailView(a, func() {
	a.pages.SwitchToPage("ssm-parameters")
	a.tv.SetFocus(a.ssmParamsV.table)
	a.UpdateContextPanel(a.ssmParamsV)
})
```

Same shape for `secretdetail.go`/`secretsV`/`"secrets-manager"`,
`datadoglogdetail.go`/`datadogLogsV`/`"datadog-logs"`,
`codepipelinedetail.go`/`codePipelineListV`/`"codepipeline"`.
`logdetail.go`'s closure additionally carries its manual
`Shortcuts()`-based context-panel rebuild (moved verbatim, not
simplified to `UpdateContextPanel` — `logSearchView` still isn't a
registered `ui.View`, unrelated to this CR):

```go
a.logDetailV = newLogDetailView(a, func() {
	a.pages.SwitchToPage("log-search")
	a.tv.SetFocus(a.logSearchV.table)
	lines := make([]string, 0, len(a.logSearchV.Shortcuts()))
	for _, sc := range a.logSearchV.Shortcuts() {
		lines = append(lines, fmt.Sprintf("[%s]<%s>[-] %s", a.Config().Colors.Accent, sc.Key, sc.Description))
	}
	a.SetContextHint(strings.Join(lines, "\n"))
})
```

(Note: this closure runs in `app.go`, where `a` is the real `*App`,
not `ui.ViewHost` — so it could use `a.cfg.Colors`/`a.contextPanel`
directly with no rename needed. Using `a.Config().Colors`/
`a.SetContextHint(...)` instead anyway, for consistency with the rest
of this CR's rename and because `a` already satisfies `ui.ViewHost`
so both forms compile identically — picking the exported form avoids
having two different access styles to the same data one file apart.)

### 5. Verification order

One file at a time, in the order listed above (smallest/simplest
first: `logdetail.go`, then the other detail views, then the 4 list
views) — `gofmt -l`, `go build ./...` after each. For the 5 detail
views, this includes updating their `app.go` construction call site
in the same step (unlike the 4 list views, which need no `app.go`
change at all). `app.go` needs no *type-level* changes for any of
these 9 (confirmed: `*App` already satisfies `ui.ViewHost`, so every
existing `newXView(a, ...)` call site keeps
compiling unchanged). Final `go vet ./...`, `go test ./...` repo-wide.

## Files touched

- `ssmparams.go`, `paramdetail.go`, `secrets.go`, `secretdetail.go`,
  `logs.go`, `logdetail.go`, `datadoglogdetail.go`,
  `codepipelinelist.go`, `codepipelinedetail.go`
- `app.go` (5 construction call sites gain an `onBack` closure
  argument — the only change this CR makes to `app.go`)

## Key decisions

- **`onBack` closures are inline at each `app.go` call site, not new
  named `viewwiring.go` methods** — each is used exactly once, tightly
  coupled to its one detail view's construction; `viewwiring.go`'s
  existing methods represent reusable "open this view" operations
  callable from anywhere, which these aren't. Mirrors how dialog
  `onApply`/`onClose` callbacks are already inline closures at their
  call sites (e.g. `movePicker.Show(srcQueue, func(target string)
  {...})` in `queues.go`) rather than named helper methods.
- **Field renamed `app` → `host`, matching `internal/dialog`'s
  existing convention** — every dialog type already names its
  `ui.Host` field `host`; matching it here means the eventual physical
  move (phase 4's last step) doesn't also carry a naming
  inconsistency between the two overlay families.
- **Don't unify the `a`-vs-`xv.host` access-pattern inconsistency
  some files have** (e.g. `messages.go`'s mix, though that file isn't
  in this CR — `paramdetail.go`/`secretdetail.go` and others may have
  a milder version) — out of scope; this CR changes *what* each
  existing access point resolves to, not *which* access point is used
  where.
- **No new tests** — every substitution is between two things CR 80
  already proved equivalent (the `ui.ViewHost` method *is* the thin
  wrapper around the field being replaced); existing tests continue
  exercising the same runtime behavior.
- **No new dependencies** — `internal/ui` is already an
  `internal/app` dependency elsewhere; adding the import to these 9
  files just extends existing usage.

## Definition of done

Unchanged from spec.md — `go build`/`go test`/`go vet` pass, `gofmt -l`
clean, all 9 files hold `host ui.ViewHost`, zero raw-field/unexported-
func-field access remaining, zero behavior change.
