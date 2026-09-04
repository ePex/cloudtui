# CR: split ui.ViewHost into narrow, per-resource interfaces

Date: 2026-09-04

## Purpose

`ui.ViewHost` (`internal/ui/viewhost.go`) is a single 28-method
interface (on top of `Host`'s ~23) that every one of the ~17 view
types in `internal/view/` depends on via its constructor's `host
ui.ViewHost` parameter — even though most views call only a handful of
those 28 methods. This was flagged in the 2026-09-04 architectural
review (`BACKLOG.md`) as the clearest ISP (interface segregation
principle) violation in the codebase, with k9s's narrow, per-resource
`dao.Accessor`/`Loggable`/`Scalable` interfaces cited as the pattern
to follow instead of one god interface.

A follow-up usage audit (grepping every view's actual `host.Xxx()`/
`a.Xxx()` calls) confirms the shape is worse than "a few unused
methods": **10 of ViewHost's 28 methods (the 8 `OpenX` cross-view
trampolines, plus `SwitchToPage`/`UpdateContextPanel`) are called by
zero views** — every view instead receives its specific navigation
callback as a plain `func(...)` constructor parameter (e.g. `a.OpenMessages`
passed into `NewQueuesView`), exactly as spec/03 already documents.
Those 10 exist on `ViewHost` purely so `*App` satisfies the interface
type as a whole; no view needs them. Beyond that, the remaining 18
AWS/Datadog/CodePipeline methods split cleanly into 5 non-overlapping
per-resource clusters (SSM, Secrets, CloudWatch Logs, Datadog Logs,
CodePipeline), and 3 views (`queues.go`, `message_detail.go`,
`settings.go`, `log.go`) call **no** `ViewHost`-only method at all —
plain `ui.Host` already covers them.

## Scope

- Define narrow interfaces in `internal/ui/` (file layout decided in
  `plan.md`) for each natural cluster identified by the usage audit:
  - `AWSAuthHost`: `AWSAuthTypeFor`, `AWSSSOLogin` — the two methods
    `internal/view/awsload.go`'s shared `runAWSLoad` helper needs,
    embedded by every cluster whose `load()` goes through it.
  - `SSMParamsHost`: `Host` + `AWSAuthHost` + `ListParameters` +
    `RevealParameter` + `CopyToClipboard`.
  - `SecretsHost`: `Host` + `AWSAuthHost` + `ListSecrets` +
    `RevealSecret` + `CopyToClipboard`.
  - `CloudWatchLogsHost`: `Host` + `AWSAuthHost` + `ListLogGroups` +
    `FilterLogEvents` + `CopyToClipboard`.
  - `DatadogLogsHost`: `Host` + `SearchDatadogLogs` +
    `ListDatadogFacetValues` + `SetPendingCloudWatchPattern` +
    `SwitchTo` + `CopyToClipboard` (no `AWSAuthHost` — Datadog's own
    API key auth doesn't go through AWS SSO re-auth).
  - `CodePipelineHost`: `Host` + `AWSAuthHost` + `ListPipelines` +
    `GetPipelineState` + `IsWatchingPipeline` + `StartWatchingPipeline`
    + `StopWatchingPipeline`.
  - `MessagesHost`: `Host` + `SwitchTo` (the one extra method
    `messages.go` needs beyond plain `Host`).
- Migrate every view constructor and struct field from `ui.ViewHost`
  to the narrowest interface that covers it:
  - `ssmparams.go`, `paramdetail.go` → `SSMParamsHost`
  - `secrets.go`, `secretdetail.go` → `SecretsHost`
  - `logs.go`, `logsearch.go`, `logdetail.go` → `CloudWatchLogsHost`
  - `datadoglogs.go`, `datadoglogdetail.go` → `DatadogLogsHost`
  - `codepipelinelist.go`, `codepipelinedetail.go`,
    `pipelinewatcher.go` → `CodePipelineHost` (`PipelineWatcher`
    technically only calls `Config`/`QueueUpdateDraw`/`GetPipelineState`/
    `AWSAuthTypeFor`/`AWSSSOLogin` on its host — not the 3 watch-toggle
    methods, which it owns itself — but reuses the same interface as
    its sibling views rather than introduce a near-duplicate one for a
    single caller; see `plan.md`)
  - `messages.go` → `MessagesHost`
  - `queues.go`, `message_detail.go`, `settings.go`, `log.go` → plain
    `ui.Host` (confirmed via direct grep: zero `ViewHost`-only calls
    in any of these four)
- `internal/view/awsload.go`'s `runAWSLoad[T any]`'s `host` parameter
  narrowed from `ui.ViewHost` to the minimal interface it actually
  calls (`Host` + `AWSAuthHost`), since every one of its 5 callers now
  passes a narrower type than `ui.ViewHost`.
- Remove `ui.ViewHost` itself once nothing references it — confirmed
  via grep that its only other use besides view constructors is its
  own `var _ ui.ViewHost = (*App)(nil)` compile-time assertion in
  `internal/app/viewhost.go`, which gets replaced by one assertion per
  new interface (`var _ ui.SSMParamsHost = (*App)(nil)`, etc.) — this
  is *more* assertions, not fewer, and each is more informative (spec/03
  already calls a full-interface assertion "load-bearing proof that the
  interfaces are complete").
- Update `spec/03-architecture-and-package-layout/spec.md`'s `ViewHost`
  section to reflect the new shape (merge-back, last task).

## Out of scope

- **`testfake_test.go`'s shared `fakeViewHost` is not split up.**
  Go's structural typing means a single wide fake struct already
  satisfies every one of the new narrower interfaces without any
  change — the review's complaint ("`testfake_test.go` has grown to
  200+ lines of mostly no-op stubs") is about *production* views
  depending on more than they use, not about the test fake's size.
  Splitting the fake into 6 per-cluster fakes would be substantial,
  separate churn (every existing test file constructs its view via
  `newFakeViewHost()`) for comparatively little further benefit, and
  is deliberately not attempted here. If a future change wants smaller
  fakes too, that's a separate, later CR.
- **`ui.Host` itself is untouched.** Its own shape (dialogs' contract)
  wasn't flagged by the review and isn't part of this audit.
- **No behavior change.** Purely a type-level refactor — every
  concrete value passed to every view constructor is still `*App` (or
  a test fake), unchanged; only the *declared* parameter/field types
  narrow.
- **`internal/dialog` is untouched** — dialogs already depend on the
  narrower `ui.Host`, not `ui.ViewHost`; this CR doesn't touch that
  package at all.

## Data & config

No new packages. New interface declarations land in `internal/ui/`
(exact file(s) decided in `plan.md`, replacing or supplementing the
current single-interface `viewhost.go`). Touches every file in
`internal/view/` listed above, `internal/app/viewhost.go` (the
assertions), and `spec/03-architecture-and-package-layout/spec.md`
(merge-back).
