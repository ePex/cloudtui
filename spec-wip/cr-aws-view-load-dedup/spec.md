# CR: dedup the AWS view load/reauth pattern

Date: 2026-09-04

## Purpose

`ssmparams.go`, `secrets.go`, `logs.go`, `codepipelinelist.go`, and
`codepipelinedetail.go` each hand-roll the same ~30-line shape inside
their `load()` (from the just-shipped
spec-wip/fe-aws-views-loading-indicators, now spec/15-17/20): guard on
an empty AWS profile, bump a `loadSeq` counter, show a loading
placeholder synchronously, spawn a goroutine that calls
`awsauth.WithReauth` (itself looking up `AWSAuthTypeFor` and wiring
`AWSSSOLogin` at every call site), then a `QueueUpdateDraw` callback
that checks the `loadSeq` guard before dispatching to
`showError`/the view's own repaint method.

This was flagged in a 2026-09-04 architectural review (see
`BACKLOG.md`'s "From the 2026-09-04 architectural review" section) as
the clearest duplication win in the codebase: `awsauth.WithReauth` is
already generic (`[T any]`), and the codebase already has precedent
for a small generics-based shared helper extracted from duplicated
per-view logic (`favorites.go`'s `sortFavoritesFirst[T any]`). The
review also cross-checked the independent `cloudtui-go`
reimplementation, which hit the identical duplication and did not
solve it either — and, notably, dropped the `loadSeq` guard entirely
in its two equivalent views, a live example of the staleness bug this
guard exists to prevent. That's read as a reason to keep the guard
central and hard to accidentally omit, not a reason to drop it.

Bundled with this: `cloudtui-go`'s `awsauth.Do(ctx, profile, setStatus,
call)` convenience wrapper, which resolves `authType` and wires
`Login` internally so call sites don't repeat
`host.AWSAuthTypeFor`/`host.AWSSSOLogin` themselves. It's bundled here
rather than filed separately because the new shared load helper is the
one and only caller of it — introducing `Do` without a caller, or the
helper without `Do`, would each be half a change.

## Scope

- A new shared helper (exact name/shape decided in `plan.md`) in
  `internal/view/` implementing the "guard on empty profile → bump
  `loadSeq` → show loading placeholder → goroutine → reauth-aware
  fetch → `QueueUpdateDraw` with staleness check → dispatch to
  success/error" shape once, generic over the fetched result type.
- A new `awsauth.Do(ctx, profile, setStatus, call)` wrapper in
  `internal/awsauth/` that resolves `AuthType` and supplies
  `awsprofile`'s real login internally, used by the shared helper
  above instead of the raw `awsauth.WithReauth` + explicit
  `AWSAuthTypeFor`/`AWSSSOLogin` combination every current call site
  repeats.
- All 5 views' `load()` methods rewritten to call the shared helper
  instead of hand-rolling the pattern.
- Existing per-view tests (`TestXShowReauthWaitingThenDone`,
  `TestXLoadShowsLoadingStatusImmediately`,
  `TestXLoadDiscardsStaleResponse`) kept — they exercise
  view-observable behavior, not the removed internals, so they should
  keep passing largely unchanged and continue to prove the refactor
  didn't change behavior. Any newly-shared logic (e.g. the guard
  itself) may also get its own direct unit test if that turns out
  cleaner than only exercising it through 5 views.

## Out of scope

- `ui.ViewHost`'s interface-segregation problem (also flagged in the
  review) — separate, larger, riskier refactor; not touched here.
- The `showError`/`showStatus`/table-clear-loop duplication (also
  flagged) — same shape of problem, but a distinct piece of code from
  the load/reauth pattern this CR targets; left in `BACKLOG.md` as a
  follow-up.
- `QueuesView` — untouched; it doesn't use `awsauth.WithReauth`
  directly (its re-auth is dispatched via
  `secretbackend.SecretResolver`/`app.go`, a different mechanism), so
  it's not part of this duplication and has no call site for the new
  helper.
- No behavior change from a user's perspective — this is a pure
  internal refactor. If anything, it removes one accidental
  discrepancy (see the `loadSeq` note above) rather than introducing
  new behavior.

## Data & config

No new files beyond the shared helper and `awsauth.Do`. Touches:
`tui/internal/view/{ssmparams,secrets,logs,codepipelinelist,
codepipelinedetail}.go` + their `_test.go`, plus
`tui/internal/awsauth/retry.go` (or a new file in that package) for
`Do`.
