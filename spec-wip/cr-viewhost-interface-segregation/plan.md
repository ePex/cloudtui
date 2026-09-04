# Plan

## Approach

### 1. New interfaces replace `viewhost.go`'s content entirely

`internal/ui/viewhost.go` currently declares one `ViewHost` interface.
Its content is replaced with 7 narrower interfaces (kept in the same
file — they're conceptually one cohesive group, "the view-host
interfaces," same role the file already plays):

```go
package ui

import (
	"context"
	"time"

	"github.com/ePex/cloudtui/tui/internal/awscodepipeline"
	"github.com/ePex/cloudtui/tui/internal/awslogs"
	"github.com/ePex/cloudtui/tui/internal/awsprofile"
	"github.com/ePex/cloudtui/tui/internal/awssecrets"
	"github.com/ePex/cloudtui/tui/internal/awsssm"
	"github.com/ePex/cloudtui/tui/internal/config"
	"github.com/ePex/cloudtui/tui/internal/datadoglogs"
)

// AWSAuthHost is the small subset every AWS-SSO-gated resource view's
// load() needs. Embedded by every per-resource host interface below
// whose load() goes through internal/view/awsload.go's shared
// runAWSLoad helper (SSMParamsHost, SecretsHost, CloudWatchLogsHost,
// CodePipelineHost) — not DatadogLogsHost, since Datadog's own
// API-key auth is unrelated to AWS SSO re-auth.
type AWSAuthHost interface {
	AWSAuthTypeFor(ctx context.Context, profile string) (awsprofile.AuthType, error)
	AWSSSOLogin(ctx context.Context, profile string, onCode func(code, url string)) error
}

// SSMParamsHost is what SSMParamsView and ParamDetailView need.
type SSMParamsHost interface {
	Host
	AWSAuthHost
	ListParameters(ctx context.Context, profile, path string) ([]awsssm.Parameter, error)
	RevealParameter(ctx context.Context, profile, name string) (string, error)
	CopyToClipboard(data string)
}

// SecretsHost is what SecretsView and SecretDetailView need.
type SecretsHost interface {
	Host
	AWSAuthHost
	ListSecrets(ctx context.Context, profile string) ([]awssecrets.Secret, error)
	RevealSecret(ctx context.Context, profile, name string) (value string, isBinary bool, err error)
	CopyToClipboard(data string)
}

// CloudWatchLogsHost is what LogsView, LogSearchView, and
// LogDetailView need.
type CloudWatchLogsHost interface {
	Host
	AWSAuthHost
	ListLogGroups(ctx context.Context, profile string) ([]awslogs.LogGroup, error)
	FilterLogEvents(ctx context.Context, profile, logGroupName string, start, end time.Time, pattern, nextToken string) (events []awslogs.LogEvent, next string, err error)
	CopyToClipboard(data string)
}

// DatadogLogsHost is what DatadogLogsView and DatadogLogDetailView need.
type DatadogLogsHost interface {
	Host
	SearchDatadogLogs(ctx context.Context, cfg config.DatadogConfig, query string, from, to time.Time) (events []datadoglogs.LogEvent, hasMore bool, err error)
	ListDatadogFacetValues(ctx context.Context, cfg config.DatadogConfig, facet string, from, to time.Time) ([]string, error)
	SetPendingCloudWatchPattern(pattern string, timestamp time.Time)
	SwitchTo(name string)
	CopyToClipboard(data string)
}

// CodePipelineHost is what CodePipelineListView, CodePipelineDetailView,
// and PipelineWatcher need. PipelineWatcher itself only calls
// Config/QueueUpdateDraw (via Host), GetPipelineState, AWSAuthTypeFor,
// and AWSSSOLogin on its host — not the 3 watch-toggle methods below,
// which it owns and implements itself — but reuses this same interface
// as its sibling views rather than needing a near-duplicate interface
// for one caller.
type CodePipelineHost interface {
	Host
	AWSAuthHost
	ListPipelines(ctx context.Context, profile string) ([]awscodepipeline.Pipeline, error)
	GetPipelineState(ctx context.Context, profile, pipelineName string) ([]awscodepipeline.StageStatus, error)
	IsWatchingPipeline(name string) bool
	StartWatchingPipeline(name string)
	StopWatchingPipeline(name string)
}

// MessagesHost is what MessagesView needs beyond plain Host: SwitchTo,
// to return to "queues" when Esc/Backspace is pressed.
type MessagesHost interface {
	Host
	SwitchTo(name string)
}
```

`queues.go`, `message_detail.go`, `settings.go`, `log.go` take plain
`ui.Host` — confirmed via direct grep that none of the four calls any
`ViewHost`-only method.

The 10 methods no longer declared on any `ui` interface
(`SwitchToPage`, `UpdateContextPanel`, and the 8 `OpenX` cross-view
trampolines) **stay exactly as they are on `*App`** — confirmed via
grep that every call site uses them as plain method values
(`view.NewQueuesView(a, ..., a.OpenMessages)`, no parentheses — a
`func(...)` value, not an interface call) or, for
`SwitchToPage`/`UpdateContextPanel`, calls them directly on the
concrete `a *App` receiver (`app.go`, `viewwiring.go`, `theme.go`).
Nothing dispatches through an interface for any of these 10, so
removing them from `ui`'s interfaces is a pure type-level cleanup with
zero behavior change and zero other code to touch.

### 2. `internal/view/awsload.go`'s `runAWSLoad` narrows its `host` param

```go
func runAWSLoad[T any](
	host interface {
		ui.Host
		ui.AWSAuthHost
	},
	...
)
```

(or a small unexported named type in that file, e.g. `type
awsLoadHost interface { ui.Host; ui.AWSAuthHost }`, if an inline
interface type reads worse at the call site — decided during
implementation, whichever compiles cleaner). Every one of its 5
callers' host types (`SSMParamsHost`, `SecretsHost`,
`CloudWatchLogsHost`, `CodePipelineHost`) already embeds both `Host`
and `AWSAuthHost`, so no caller needs a wider type just to call
`runAWSLoad`.

### 3. Per-view migration

Each view's struct field and constructor parameter change from `host
ui.ViewHost` / `a ui.ViewHost` to the narrow interface from step 1 —
purely a type change, no logic touched. `internal/app/app.go`'s
construction-site calls (`view.NewSSMParamsView(a, ...)` etc.) need no
change at all: `*App` already implements every method on every new
interface (it implements the old, wider `ViewHost`), so passing the
same `a` value type-checks against each narrower parameter type
automatically.

### 4. `internal/app/viewhost.go`'s assertion

```go
var _ ui.ViewHost = (*App)(nil)
```

becomes one assertion per new interface:

```go
var _ ui.SSMParamsHost = (*App)(nil)
var _ ui.SecretsHost = (*App)(nil)
var _ ui.CloudWatchLogsHost = (*App)(nil)
var _ ui.DatadogLogsHost = (*App)(nil)
var _ ui.CodePipelineHost = (*App)(nil)
var _ ui.MessagesHost = (*App)(nil)
```

(`ui.Host` already has its own assertion elsewhere per spec/03 — not
duplicated here.) This is *more* compile-time proof of completeness
than the single wide assertion, not less, per spec/03's existing note
that a full-interface assertion is "load-bearing proof that the
interfaces are complete."

## Files touched

- `internal/ui/viewhost.go` — rewritten per step 1.
- `internal/view/awsload.go` — `runAWSLoad`'s `host` param narrowed.
- `internal/view/{ssmparams,paramdetail}.go` → `SSMParamsHost`
- `internal/view/{secrets,secretdetail}.go` → `SecretsHost`
- `internal/view/{logs,logsearch,logdetail}.go` → `CloudWatchLogsHost`
- `internal/view/{datadoglogs,datadoglogdetail}.go` → `DatadogLogsHost`
- `internal/view/{codepipelinelist,codepipelinedetail,pipelinewatcher}.go` → `CodePipelineHost`
- `internal/view/messages.go` → `MessagesHost`
- `internal/view/{queues,message_detail,settings,log}.go` → `ui.Host`
- `internal/app/viewhost.go` — assertions per step 4.
- `spec/03-architecture-and-package-layout/spec.md` — merge-back
  (last task).
- No `_test.go` file is expected to need changes: `fakeViewHost`
  already implements every method on the old, wider `ViewHost`, so it
  automatically satisfies every new, narrower interface too (Go's
  structural typing) — see spec.md's "Out of scope" for why the fake
  itself isn't split up.

## Testing

- No new tests — this is a pure type-level refactor with no new
  behavior to cover. The acceptance bar is: every existing test in
  `internal/view/...` and `internal/app/...` keeps passing unchanged,
  proving no behavior moved.
- `go build ./...`/`go vet ./...`/`go test ./...` after every task;
  `gofmt` before every commit — standard bar per `tui/CLAUDE.md`. A
  compile failure at any point immediately reveals a missed call site
  or a wrong interface boundary (a real advantage of doing this in Go:
  the type system enforces completeness at build time — there's no way
  to "forget" a call a view actually makes, since the code simply
  won't compile if the narrowed interface is missing a method it
  calls).

## Key decisions / trade-offs

- **7 new interfaces, not more/fewer.** Considered going all the way
  down to one interface per view (e.g. separate `ParamDetailHost` vs.
  `SSMParamsHost`), but `ssmparams.go`/`paramdetail.go`'s actual method
  sets are close enough (both need `AWSAuthHost` + `CopyToClipboard`,
  differing only in which one AWS fetcher method each calls) that
  sharing one cluster interface per resource — matching how the
  originating architectural review framed the natural groupings — was
  judged the right granularity: narrow enough to fix the ISP violation,
  not so narrow it produces 17 near-identical one-off interfaces.
- **`PipelineWatcher` reuses `CodePipelineHost` rather than getting its
  own narrower interface**, even though it's a stricter subset (no
  watch-toggle methods) — see the interface's own doc comment. One
  extra unused method-set entry on one non-view type is a smaller cost
  than a second CodePipeline-flavored interface only one caller uses.
- **`ui.ViewHost` is deleted, not deprecated/kept as an alias.**
  Confirmed via grep it has no other use than view constructors + its
  own assertion, both of which are migrated in this same CR — keeping
  a now-pointless alias around would violate `CLAUDE.md`'s "no dead
  code" rule for no benefit.
- **The shared test fake stays as one `fakeViewHost` covering
  everything** — see spec.md's "Out of scope."
- **Task order**: interfaces first (step 1+2, compiles standalone,
  nothing depends on it yet), then one task per cluster (each is an
  independent, isolated, easily-reviewable diff — mirrors how the
  earlier 5-view loading-indicator CR was broken down), then the
  `ui.Host`-only views, then the `internal/app/viewhost.go` assertion
  cleanup, then merge-back — see `tasks.md`.
