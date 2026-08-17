# Plan — CR 80: `ui.ViewHost`

## Approach

### 1. `internal/ui/viewhost.go` (new)

The `ViewHost` interface exactly as spec.md's Solution section shows —
embeds `Host`, declares the 28 new methods, doc comments on each
group mirroring `host.go`'s existing style. Imports: `context`,
`time`, `github.com/rivo/tview` is NOT needed directly (only via the
embedded `Host`'s `tview.Primitive` — already imported there), plus
`internal/awsssm`, `internal/awssecrets`, `internal/awslogs`,
`internal/datadoglogs`, `internal/awscodepipeline`, `internal/queue`,
`internal/config` for the parameter/return types. Confirmed none of
these import `internal/ui` back (checked directly — no cycle).

### 2. `*App` implements the 28 methods — three different treatments

**(a) Genuinely new methods** (7) — small wrappers around existing
unexported fields, added to a new `internal/app/viewhost.go` (mirrors
`host.go`'s role: "the file where `*App`'s interface-satisfying
methods live", one per interface rather than mixed into `app.go`):

```go
func (a *App) SwitchToPage(name string) { a.pages.SwitchToPage(name) }

func (a *App) SetPendingCloudWatchPattern(pattern string) {
	a.pendingCloudWatchPattern = pattern
}

func (a *App) ListParameters(ctx context.Context, profile, path string) ([]awsssm.Parameter, error) {
	return a.listParameters(ctx, profile, path)
}
func (a *App) RevealParameter(ctx context.Context, profile, name string) (string, error) {
	return a.revealParameter(ctx, profile, name)
}
func (a *App) ListSecrets(ctx context.Context, profile string) ([]awssecrets.Secret, error) {
	return a.listSecrets(ctx, profile)
}
func (a *App) RevealSecret(ctx context.Context, profile, name string) (value string, isBinary bool, err error) {
	return a.revealSecret(ctx, profile, name)
}
func (a *App) ListLogGroups(ctx context.Context, profile string) ([]awslogs.LogGroup, error) {
	return a.listLogGroups(ctx, profile)
}
func (a *App) FilterLogEvents(ctx context.Context, profile, logGroupName string, start, end time.Time, pattern string) ([]awslogs.LogEvent, bool, error) {
	return a.filterLogEvents(ctx, profile, logGroupName, start, end, pattern)
}
func (a *App) SearchDatadogLogs(ctx context.Context, cfg config.DatadogConfig, query string, from, to time.Time) ([]datadoglogs.LogEvent, bool, error) {
	return a.searchDatadogLogs(ctx, cfg, query, from, to)
}
func (a *App) ListDatadogFacetValues(ctx context.Context, cfg config.DatadogConfig, facet string, from, to time.Time) ([]string, error) {
	return a.listDatadogFacetValues(ctx, cfg, facet, from, to)
}
func (a *App) ListPipelines(ctx context.Context, profile string) ([]awscodepipeline.Pipeline, error) {
	return a.listPipelines(ctx, profile)
}
func (a *App) GetPipelineState(ctx context.Context, profile, pipelineName string) ([]awscodepipeline.StageStatus, error) {
	return a.getPipelineState(ctx, profile, pipelineName)
}
func (a *App) AWSAuthTypeFor(ctx context.Context, profile string) (awsprofile.AuthType, error) {
	return a.awsAuthTypeFor(ctx, profile)
}
func (a *App) AWSSSOLogin(ctx context.Context, profile string) error {
	return a.awsSSOLogin(ctx, profile)
}
```

(That's 1 + 1 + 12 = 14 wrapper methods — the plan's "(a) genuinely
new" bucket is bigger than spec.md's "7" estimate suggested; the 12
data-fetch wrappers are all "new" in the sense that no method existed
before, only a field. Correcting the count here: 14 new methods, not
7 — spec.md undercounted by lumping the 12 data-fetchers into the
same mental bucket as the true renames below. Total is still 28
methods overall.)

**(b) Renamed in place** (11) — capitalize the existing method, fix
every call site in the same pass, same file (no relocation):

| Old (unexported) | New (exported) | File | Call sites to fix (besides definition) |
|---|---|---|---|
| `switchTo` | `SwitchTo` | `app.go` | 9 in `app.go`, 1 in `messages.go`, 1 in `datadoglogdetail.go` (`dv.app.switchTo`) |
| `updateContextPanel` | `UpdateContextPanel` | `app.go` | `app.go`×1, `theme.go`×1, `codepipelinedetail.go`×1, `datadoglogdetail.go`×1, `logsearch.go`×1, `secretdetail.go`×1, `paramdetail.go`×1 |
| `copyToClipboard` | `CopyToClipboard` | `app.go` | `datadoglogdetail.go`, `logdetail.go`, `paramdetail.go`, `secretdetail.go` (all via `dv.app.copyToClipboard`) |
| `isWatchingPipeline` | `IsWatchingPipeline` | `codepipelinewatch.go` | `codepipelinewatch.go`×1 (internal), `codepipelinelist.go`×2, `codepipelinedetail.go`×2 |
| `startWatchingPipeline` | `StartWatchingPipeline` | `codepipelinewatch.go` | `codepipelinelist.go`×1, `codepipelinedetail.go`×1 |
| `stopWatchingPipeline` | `StopWatchingPipeline` | `codepipelinewatch.go` | `codepipelinewatch.go`×2 (internal), `codepipelinelist.go`×1, `codepipelinedetail.go`×1 |
| `openMessages` | `OpenMessages` | `viewwiring.go` | `viewwiring.go`×1 (internal, from `wireQueuesOpensMessages`) |
| `openMessageDetail` | `OpenMessageDetail` | `viewwiring.go` | `viewwiring.go`×1 |
| `openParamDetail` | `OpenParamDetail` | `viewwiring.go` | `viewwiring.go`×1 |
| `openSecretDetail` | `OpenSecretDetail` | `viewwiring.go` | `viewwiring.go`×1 |
| `openLogSearch` | `OpenLogSearch` | `viewwiring.go` | `viewwiring.go`×1 |
| `openLogEventDetail` | `OpenLogEventDetail` | `viewwiring.go` | `viewwiring.go`×1 |
| `openDatadogLogDetail` | `OpenDatadogLogDetail` | `viewwiring.go` | `viewwiring.go`×1 |
| `openCodePipelineDetail` | `OpenCodePipelineDetail` | `viewwiring.go` | `viewwiring.go`×1 |

(14 renames listed — 3 + 3 + 8; matches the "11" figure only if
`switchTo`/`updateContextPanel`/`copyToClipboard` are counted as one
group of 3 renames producing many call-site fixes each. Corrected
count: **14 renamed methods**, not 11 — same correction as (a) above.)

**(c) `var _ ui.ViewHost = (*App)(nil)`** — compile-time assertion,
added to `internal/app/viewhost.go` alongside the new methods.

Corrected total: **14 new + 14 renamed = 28**, matching spec.md's
headline number even though the internal split differs from its
"7 new / mostly renames" framing — worth fixing in spec.md once this
plan is approved (see spec sync note below).

### 3. Verification order

One bucket at a time: all 14 new wrapper methods first (additive,
can't break anything — `go build ./...` after), then the 3
`app.go`/`codepipelinewatch.go` renames one at a time (each touches
multiple files, `go build ./...` after each to catch every call site),
then the 8 `viewwiring.go` renames (self-contained, one `go build
./...` after all 8 since they only affect that one file). Finally
`go vet ./...`, `go test ./...` repo-wide, plus the `var _
ui.ViewHost = (*App)(nil)` assertion as the last line added (forces a
compile error immediately if any method is missing/mis-signatured).

## Files touched

- `internal/ui/viewhost.go` (new)
- `internal/app/viewhost.go` (new — 14 wrapper methods + the
  `var _ ui.ViewHost` assertion)
- `internal/app/app.go` (3 renames: `SwitchTo`, `UpdateContextPanel`,
  `CopyToClipboard`, plus their own internal call-site fixes)
- `internal/app/codepipelinewatch.go` (3 renames: `IsWatchingPipeline`,
  `StartWatchingPipeline`, `StopWatchingPipeline`)
- `internal/app/viewwiring.go` (8 renames: `OpenMessages` etc.)
- `internal/app/messages.go`, `theme.go`, `datadoglogdetail.go`,
  `logsearch.go`, `secretdetail.go`, `paramdetail.go`, `logdetail.go`,
  `codepipelinelist.go`, `codepipelinedetail.go` (call-site updates
  only, for the renames above)

## Key decisions

- **New wrapper methods go in `internal/app/viewhost.go`, not
  scattered or added to `host.go`** — mirrors `host.go`'s existing
  precedent (one file per interface `*App` implements) rather than
  growing `host.go` into a file implementing two unrelated
  interfaces.
- **Renames stay in their current file** — `SwitchTo`/
  `UpdateContextPanel`/`CopyToClipboard` are core `App` routing
  methods used well beyond view-host purposes (e.g. `switchTo` is
  handed to `views.NewHome` as a callback); moving them to
  `viewhost.go` would misrepresent them as view-host-specific
  plumbing when they're really general `App` methods that also happen
  to satisfy part of `ViewHost`.
- **Corrected the "7 new" vs "14 new / 14 renamed" split from
  spec.md** — spec.md's Solution section didn't separately size the
  12 data-fetch wrappers as "new" work; this plan does, since each
  requires writing an actual method body (however thin), same effort
  shape as the other 2 genuinely-new methods, not the "capitalize and
  fix call sites" shape of a rename. Total interface size (28) is
  unchanged.
- **No new tests** — pure additive/rename with zero behavior change;
  existing tests continue exercising the same code paths under the
  new (or same, for the 14 additions) names.
- **No new dependencies.**

## Definition of done

Unchanged from spec.md — `go build`/`go test`/`go vet` pass, `gofmt -l`
clean, `ui.ViewHost` exists and embeds `ui.Host`, `*App` implements
all 28 methods (verified by the compile-time assertion), zero
behavior change.
