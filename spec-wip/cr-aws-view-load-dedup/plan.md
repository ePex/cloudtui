# Plan

## Approach

Two new pieces, then five mechanical call-site rewrites:

1. **`awsauth.Do[T any]`** (`tui/internal/awsauth/retry.go`) — thin
   wrapper over the existing `WithReauth[T any]` that resolves
   `AuthType` internally instead of making every caller do it:

   ```go
   // Do is WithReauth with AuthType resolution folded in — every
   // current call site repeats "look up AuthType, then call
   // WithReauth" by hand; Do does both in one call.
   func Do[T any](
       ctx context.Context,
       profile string,
       authTypeFor func(ctx context.Context, profile string) (awsprofile.AuthType, error),
       login func(ctx context.Context, profile string, onCode func(code, url string)) error,
       onReauth func(),
       onCode func(code, url string),
       call func(ctx context.Context) (T, error),
   ) (T, error) {
       authType, _ := authTypeFor(ctx, profile)
       return WithReauth(ctx, profile, authType, login, onReauth, onCode, call)
   }
   ```

   `authTypeFor`/`login` stay as plain func parameters (not a
   `ui.ViewHost`) so `awsauth` doesn't gain a dependency on `internal/ui`
   — callers pass `host.AWSAuthTypeFor`/`host.AWSSSOLogin` as method
   values, same as they already pass `host.AWSSSOLogin` to `WithReauth`
   today. The `authType` lookup error is discarded (`_`) because that's
   the existing behavior at every current call site
   (`authType, _ := pv.host.AWSAuthTypeFor(ctx, profile)`) — not a
   regression introduced here.

2. **`runAWSLoad[T any]`** (new file `tui/internal/view/awsload.go`) —
   the shared shape of all 5 views' `load()`:

   ```go
   // runAWSLoad is the shared shape behind SSMParamsView, SecretsView,
   // LogsView, CodePipelineListView, and CodePipelineDetailView's
   // load(): guard on an empty AWS profile, bump *loadSeq, show a
   // loading placeholder, fetch in a goroutine with SSO-reauth retry
   // (awsauth.Do), then dispatch the result back on the UI goroutine —
   // discarding it if a newer load() has since started.
   func runAWSLoad[T any](
       host ui.ViewHost,
       loadSeq *int,
       showStatus func(string),
       showError func(error),
       loadingMsg string,
       fetch func(ctx context.Context, profile string) (T, error),
       onSuccess func(T),
   ) {
       profile := host.Config().ActiveAWSProfile
       if profile == "" {
           showError(fmt.Errorf("no AWS profile selected — use :ap to select one"))
           return
       }
       *loadSeq++
       seq := *loadSeq
       showStatus(loadingMsg)
       const reauthWaitingMsg = "AWS SSO session expired — opening browser to log in..."
       go func() {
           ctx := context.Background()
           result, err := awsauth.Do(ctx, profile, host.AWSAuthTypeFor, host.AWSSSOLogin,
               func() {
                   host.QueueUpdateDraw(func() { showStatus(reauthWaitingMsg) })
               },
               func(code, url string) {
                   host.QueueUpdateDraw(func() {
                       showStatus(fmt.Sprintf("%s Verify code %s at %s", reauthWaitingMsg, code, url))
                   })
               },
               func(ctx context.Context) (T, error) { return fetch(ctx, profile) },
           )
           host.QueueUpdateDraw(func() {
               if seq != *loadSeq {
                   return // superseded by a newer load()
               }
               if err != nil {
                   showError(err)
                   return
               }
               onSuccess(result)
           })
       }()
   }
   ```

   Takes `showStatus`/`showError` as plain funcs (each view's existing
   private methods), not the `ui.ReauthStatusShower` interface — the
   interface is what *external* callers (currently none, for these 5
   views; it exists for structural consistency with `QueuesView`) use
   to reach a view's status display, but internally `ShowReauthWaiting`/
   `ShowReauthDone` are already just one-line wrappers around
   `showStatus`. Routing the helper through the interface instead would
   mean calling `ShowReauthDone()` to display something that isn't
   really "reauth done" (it's the *initial* load), which reads wrong;
   passing `showStatus` directly avoids that mismatch and needs no new
   type.

3. **Each view's `load()` rewritten** to call `runAWSLoad`, e.g.
   `ssmparams.go`:

   ```go
   func (pv *SSMParamsView) load() {
       runAWSLoad(pv.host, &pv.loadSeq, pv.showStatus, pv.showError, loadingParametersStatus,
           func(ctx context.Context, profile string) ([]awsssm.Parameter, error) {
               return pv.host.ListParameters(ctx, profile, "/")
           },
           pv.repaint,
       )
   }

   func (pv *SSMParamsView) ShowReauthWaiting(msg string) { pv.showStatus(msg) }
   func (pv *SSMParamsView) ShowReauthDone()              { pv.showStatus(loadingParametersStatus) }
   ```

   `ShowReauthWaiting`/`ShowReauthDone` stay on each view unchanged —
   they still satisfy `ui.ReauthStatusShower`, just no longer called
   from inside `load()` itself (the helper calls `showStatus` directly
   for the same effect). `CodePipelineDetailView`'s `loadingMsg` stays
   computed per-call (`fmt.Sprintf("Loading %s…", dv.pipelineName)`),
   passed as the `loadingMsg` argument same as the other 4 pass a
   constant.

## Files touched

- `tui/internal/awsauth/retry.go` — add `Do`.
- `tui/internal/awsauth/retry_test.go` — add `Do` tests (below).
- `tui/internal/view/awsload.go` (new) — add `runAWSLoad`.
- `tui/internal/view/{ssmparams,secrets,logs,codepipelinelist,
  codepipelinedetail}.go` — `load()` rewritten to call it; no other
  method signatures change.
- Each of those 5 `_test.go` files — expected to need **no changes**:
  they exercise `load()`'s observable behavior (table contents, status
  text, timing of the loading placeholder, stale-response discarding)
  through the view's public/package-private surface, not the removed
  internals, so a correct refactor should leave every existing
  assertion passing unchanged. If any test turns out to have
  been accidentally coupled to the old internal shape, fixing that
  coupling is part of this CR, not a follow-up.

## Testing

- New `TestDoXxx` tests in `retry_test.go`, mirroring the existing
  `TestWithReauthXxx` style (plain func doubles, no mocking
  framework): success-first-try, retries-after-login,
  authTypeFor-error-is-ignored (call still proceeds — matches today's
  `_ :=` discard), and that `authTypeFor`'s returned `AuthType` is
  what's actually passed through to `NeedsReauth` (e.g. via
  `AuthStaticKeys` never triggering reauth, same as
  `TestWithReauthNotNeeded`).
- New `awsload_test.go` in `internal/view/` testing `runAWSLoad`
  directly against `fakeViewHost` (same fakes the 5 views' tests
  already use): empty-profile guard, loading placeholder shown
  synchronously, stale-response discarded via `loadSeq`, reauth
  waiting/done messages shown in order. This gives the shared logic
  its own direct coverage instead of relying solely on 5 views'
  tests to exercise it indirectly.
- Full existing suite for the 5 views must keep passing unchanged
  (see "Files touched" above) — that's the acceptance bar for "this
  refactor didn't change behavior."
- `go build`/`go vet`/`go test ./...` after each task, `gofmt` before
  each commit — standard bar per `tui/CLAUDE.md`.

## Key decisions / trade-offs

- **`showStatus`/`showError` as plain funcs, not an interface.**
  Considered defining a small `interface { showStatus(string);
  showError(error) }`, but Go doesn't allow an unexported
  interface method set to be satisfied across files as cleanly as
  just passing the two funcs directly, and two funcs is simpler to
  read at each of the 5 call sites than a one-off interface with two
  unexported methods. `ui.ReauthStatusShower` stays as-is for its
  actual purpose (external dispatch, `QueuesView`'s pattern), not
  reused here.
- **`loadSeq` passed as `*int`, not owned by the helper.** The counter
  has to live on the view (it's read in `Activate()`/`Open()`-adjacent
  code paths conceptually, and is part of each view's own state,
  inspectable in tests via `pv.loadSeq`), so the helper takes a
  pointer into it rather than trying to own/return it — keeps each
  view's struct shape unchanged.
- **Bundling `Do` into this CR rather than filing it separately**: per
  `spec.md`, `runAWSLoad` is `Do`'s only caller, so introducing either
  half without the other would be incomplete.
- **Not touching `QueuesView`**: confirmed again while reading
  `queues.go` — its re-auth path goes through
  `secretbackend.SecretResolver`/`app.go`'s dispatch, never calls
  `awsauth.WithReauth` directly, so there's no `runAWSLoad`-shaped
  call site there to convert.
