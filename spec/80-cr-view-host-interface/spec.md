# Spec — CR 80: design `ui.ViewHost`, the resource-view equivalent of `ui.Host`

Date: 2026-08-17

## Background

With CR 79's trampoline split done, the 17 phase-4 files split cleanly
into: 12 view types (each still holding `app *App` directly, same
pre-decoupling shape the 10 dialogs had before CR 67) and
`internal/app/viewwiring.go` (the cross-view navigation layer, staying
in `internal/app` permanently). This CR designs the interface the 12
view types will depend on instead of `*App` — the same role `ui.Host`
plays for the 10 dialogs, grounded the same way CR 66/67's audit
grounded `ui.Host`: by reading exactly which `*App` members each
caller touches, not guessing from the type's shape.

Re-verified every candidate method's actual caller(s) directly (not
reused from the earlier scoping audit, since that predates CR 79's
split and one design question below changes the shape of the result):
grepped each of `updateContextPanel`, `copyToClipboard`,
`pendingCloudWatchPattern`, `switchTo`, `isWatchingPipeline`/
`startWatchingPipeline`/`stopWatchingPipeline`, and `.backend` across
all 17 files individually. Two corrections from the earlier estimate:

- **`a.notify(...)`** (used for CodePipeline watch notifications) has
  zero callers outside `codepipelinewatch.go` itself — which isn't
  moving (no view type of its own, stays whole in `internal/app` per
  CR 79's spec). Dropped from the interface entirely — no view needs
  it.
- **`pendingCloudWatchPattern`** is only ever *written* by a moving
  file (`datadoglogdetail.go`); every read/clear happens in
  `viewwiring.go`/`app.go`'s `switchTo`, both staying in
  `internal/app` with direct field access. Only a setter is needed on
  the interface, not a getter.

## Problem

Same problem `ui.Host` solved for dialogs: `internal/view` importing
back to `*App` would cycle with `internal/app` importing
`internal/view` (needed for `app.go`'s view construction, mirroring
`app.go`'s dialog construction today). The 12 view types need a
narrower contract instead.

## Design decision: dialogs are a direct dependency, not an interface concern

Unlike `spec/70`'s dialog-to-`*App` audit (dialogs had no other package
to depend on except `*App`), the 12 views' dialog interactions
(`a.confirm.Show(...)`, `a.movePicker.Show(...)`, etc.) can now depend
on `internal/dialog` **directly** — it's a fully independent,
importable package since CR 78, and `internal/dialog` doesn't import
anything that would cycle back. So: `internal/view` imports
`internal/dialog`; each view type takes the specific dialog instances
it needs as constructor parameters, the same pattern `ConnEditor`
already uses for its `*ConnManager` sibling reference. This keeps
~9 dialog-open methods off `ViewHost` entirely — smaller interface,
and it mirrors how dialogs themselves never needed to know about
overlay-to-overlay routing through `Host`.

## Solution

`ui.ViewHost` (new file `internal/ui/viewhost.go`), **embedding
`ui.Host`** rather than duplicating it — `*App` already implements
every `ui.Host` method, so embedding costs nothing and several are
directly reusable as-is by views (`SetFocus`, `FocusMain`,
`QueueUpdateDraw`, `SetStatus`, `Config`, `Backend`):

```go
type ViewHost interface {
	Host

	// Chrome
	SwitchToPage(name string)
	UpdateContextPanel(v View)
	SwitchTo(name string)
	CopyToClipboard(data string)

	// Cross-view navigation — implemented by viewwiring.go, calling
	// into the target view's own (still same-package) methods.
	OpenMessages(queueName string)
	OpenMessageDetail(queueName string, msg queue.Message)
	OpenParamDetail(param awsssm.Parameter)
	OpenSecretDetail(secret awssecrets.Secret)
	OpenLogSearch(logGroupName string)
	OpenLogEventDetail(event awslogs.LogEvent)
	OpenDatadogLogDetail(event datadoglogs.LogEvent)
	OpenCodePipelineDetail(pipelineName string)

	// FE 41's Datadog->CloudWatch jump (write-only — see Background)
	SetPendingCloudWatchPattern(pattern string)

	// CodePipeline background watcher (implemented by codepipelinewatch.go)
	IsWatchingPipeline(name string) bool
	StartWatchingPipeline(name string)
	StopWatchingPipeline(name string)

	// Injectable data-fetchers (mirrors the existing App func fields —
	// ListAWSProfiles/AWSAuthTypeFor's AWS-profile counterpart already
	// lives on Host)
	ListParameters(ctx context.Context, profile, path string) ([]awsssm.Parameter, error)
	RevealParameter(ctx context.Context, profile, name string) (string, error)
	ListSecrets(ctx context.Context, profile string) ([]awssecrets.Secret, error)
	RevealSecret(ctx context.Context, profile, name string) (value string, isBinary bool, err error)
	ListLogGroups(ctx context.Context, profile string) ([]awslogs.LogGroup, error)
	FilterLogEvents(ctx context.Context, profile, logGroupName string, start, end time.Time, pattern string) (events []awslogs.LogEvent, hasMore bool, err error)
	SearchDatadogLogs(ctx context.Context, cfg config.DatadogConfig, query string, from, to time.Time) (events []datadoglogs.LogEvent, hasMore bool, err error)
	ListDatadogFacetValues(ctx context.Context, cfg config.DatadogConfig, facet string, from, to time.Time) ([]string, error)
	ListPipelines(ctx context.Context, profile string) ([]awscodepipeline.Pipeline, error)
	GetPipelineState(ctx context.Context, profile, pipelineName string) ([]awscodepipeline.StageStatus, error)
	AWSAuthTypeFor(ctx context.Context, profile string) (awsprofile.AuthType, error)
	AWSSSOLogin(ctx context.Context, profile string) error
}
```

28 new methods + the 20 inherited from `ui.Host` = 48 total in the
interface's method set, but only 28 need `*App` work: 14 are
genuinely new (small wrappers around a previously-unexported field —
`SwitchToPage`, `SetPendingCloudWatchPattern`, and the 12 data-fetch
methods, each wrapping the like-named unexported func field) and 14
are renames of an existing unexported method to its exported form
(`switchTo`→`SwitchTo`, `updateContextPanel`→`UpdateContextPanel`,
`copyToClipboard`→`CopyToClipboard`, the 3 CodePipeline-watch methods,
and the 8 `viewwiring.go` `open*`→`Open*` trampolines) — see plan.md
for the exact breakdown and every call site each rename touches.

## Scope

### In scope

- `internal/ui/viewhost.go`: the `ViewHost` interface as above, with
  doc comments (mirrors `host.go`'s existing doc-comment density).
- `internal/app/viewhost.go` (new): the 14 genuinely-new wrapper
  methods, each delegating to the existing private field
  (`SwitchToPage` → `a.pages.SwitchToPage`, `ListParameters` →
  `a.listParameters`, etc.) — no new logic, pure exposure.
- `internal/app/app.go`, `codepipelinewatch.go`, `viewwiring.go`: the
  14 renames (capitalize an existing unexported method), plus fixing
  every call site across the files that reference them (`theme.go`,
  `messages.go`, `datadoglogdetail.go`, `logsearch.go`,
  `secretdetail.go`, `paramdetail.go`, `logdetail.go`,
  `codepipelinelist.go`, `codepipelinedetail.go`).
- `var _ ui.ViewHost = (*App)(nil)` compile-time assertion.
- `gofmt`/`go vet`/`go build`/`go test` clean across `tui/`.

### Out of scope

- Any view type actually adopting `ui.ViewHost` in place of `app *App`
  — separate CR(s) next, per-view or in small batches (mirroring how
  CR 68/69 split the 10 dialogs' adoption into two batches based on
  which needed sibling-reference handling).
- The physical move of any view file into `internal/view` — later,
  once every view type depends on `ViewHost` instead of `*App`.
- Any accessor-method pass on the 12 view types themselves (the
  `Primitive()`/`Visible()`-equivalent CR 73 did for dialogs) — needed
  before `viewwiring.go`'s `Open*` methods can stop reaching into raw
  view fields, but that's a different CR from designing the interface
  those `Open*` methods sit behind.
- `settings.go`'s deeper pre-existing wrinkle (flagged in CR 79's
  spec) — not addressed by this interface design.
- Any behavior change. This CR only adds new methods to `*App` that
  wrap existing private state — `go test ./...` passing is sufficient,
  no live verification needed (nothing new is reachable yet; no
  production code calls these new methods until the adoption CRs).

## Definition of done

1. `go build ./...` and `go test ./...` pass in `tui/`.
2. `ui.ViewHost` exists, embeds `ui.Host`, declares the 28 methods
   above; `*App` implements all of them (28 new + 20 inherited),
   verified by `var _ ui.ViewHost = (*App)(nil)`.
3. `gofmt -l` reports nothing; `go vet ./...` clean.
4. No behavior change — additive only.
