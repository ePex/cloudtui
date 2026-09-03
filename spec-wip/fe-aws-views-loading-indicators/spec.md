# FE: loading indicators + SSO re-auth status for the remaining AWS views

Date: 2026-09-03

## Purpose

`QueuesView` shows an immediate "Loading queues…" placeholder the
moment `Load()` starts, and — if the fetch is blocked on an AWS SSO
browser login (`ui.ReauthStatusShower`) — swaps that placeholder for a
"AWS SSO session expired…" message, then reverts once login completes.
`SSMParamsView`, `SecretsView`, `LogsView` (CloudWatch Logs),
`CodePipelineListView`, and `CodePipelineDetailView` have none of the
first half: their `load()` starts a goroutine with no visible change
at all until it resolves — a real AWS API call with normal network
latency, so a fetch that takes a second or two looks identical to a
frozen screen. This was already flagged as a gap and queued in
`BACKLOG.md`.

## Scope

- **Immediate loading placeholder**, mirroring `QueuesView.Load()`:
  each of the 5 views' `load()` (`CodePipelineDetailView`'s equivalent
  is called from `Open()`) calls its own existing `showStatus(...)`
  with a `"Loading <thing>…"` message *synchronously, before* the
  fetch goroutine starts — not just reactively inside the reauth
  callbacks, which is all that happens today.
- **`loadSeq` guard**, mirroring `QueuesView`: a call counter
  incremented at the top of `load()`, checked inside the
  `QueueUpdateDraw` callback before repainting, so a slow, superseded
  fetch (e.g. `r` pressed twice, or the view re-activated before the
  first call resolves) can't clobber a newer one with stale data.
  None of the 5 views currently guard against this.
- **`ui.ReauthStatusShower` implemented on all 5 views** — extracting
  each view's existing inline `onReauth`/`onCode` closures (currently
  calling `showStatus` directly) into named `ShowReauthWaiting(msg
  string)`/`ShowReauthDone()` methods matching `QueuesView`'s shape,
  called from within the view's own `load()` (these views resolve
  AWS SSO re-auth via `awsauth.WithReauth`'s direct per-call-site
  callbacks, not `secretbackend.SecretResolver`'s app.go-dispatched
  mechanism `QueuesView` uses — so this is a structural/interface
  consistency change, not a change to *how* re-auth is triggered).
  `ShowReauthDone()` reverts to the view's own loading-placeholder
  text (its "something to revert to" — this is what `BACKLOG.md`
  flagged as missing before the loading placeholder existed).

## Out of scope

- No change to `QueuesView` itself — already has all of this.
- No change to the underlying re-auth *mechanism*
  (`awsauth.WithReauth`'s direct callbacks vs. `secretbackend`'s
  app.go-dispatched one) — out of scope, and not something `BACKLOG.md`
  asked for.
- No change to `logSearchView` (per-log-group search, opened from
  `LogsView`) or Datadog Logs — `BACKLOG.md` names CloudWatch Logs
  specifically; Datadog Logs already has its own reauth-independent
  flow and wasn't flagged.
- No change to polling/watch behavior (`CodePipelineDetailView`'s
  `w`-triggered background watch, `App.handlePipelinePoll`) — a poll
  landing while `load()`'s own placeholder is showing is an existing,
  unrelated interaction, untouched here.

## Data & config

No new files — this is small, mechanical, near-identical edits to 5
existing view files (`tui/internal/view/{ssmparams,secrets,logs,
codepipelinelist,codepipelinedetail}.go`) plus each view's `_test.go`.
