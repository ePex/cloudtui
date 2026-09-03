# Plan

## Pattern (identical shape across all 5 views)

Using `SSMParamsView` as the representative example — the same edit
applies verbatim (substituting the type name, the fetch call, and the
loading-message text) to `SecretsView`, `LogsView`,
`CodePipelineListView`, and `CodePipelineDetailView`.

1. **A named loading-message constant**, mirroring `queues.go`'s
   `loadingQueuesStatus`:
   ```go
   const loadingParametersStatus = "Loading parameters…"
   ```
   Per view: `"Loading secrets…"` (`SecretsView`), `"Loading log
   groups…"` (`LogsView`), `"Loading pipelines…"`
   (`CodePipelineListView`). `CodePipelineDetailView` is per-pipeline
   and already has `pipelineName` in scope by the time `load()` runs,
   so its message is built inline —
   `fmt.Sprintf("Loading %s…", dv.pipelineName)` — not a constant.

2. **`loadSeq int` field** added to the view's struct, mirroring
   `QueuesView.loadSeq`.

3. **`load()` restructured** to show the placeholder synchronously
   before launching the fetch goroutine, and guard the eventual
   `QueueUpdateDraw` callback with the sequence check:
   ```go
   func (pv *SSMParamsView) load() {
       profile := pv.host.Config().ActiveAWSProfile
       if profile == "" {
           pv.showError(fmt.Errorf("no AWS profile selected — use :ap to select one"))
           return
       }
       pv.loadSeq++
       seq := pv.loadSeq
       pv.showStatus(loadingParametersStatus)
       go func() {
           ctx := context.Background()
           authType, _ := pv.host.AWSAuthTypeFor(ctx, profile)
           params, err := awsauth.WithReauth(ctx, profile, authType, pv.host.AWSSSOLogin,
               func() {
                   pv.host.QueueUpdateDraw(func() { pv.ShowReauthWaiting(reauthWaitingMsg) })
               },
               func(code, url string) {
                   pv.host.QueueUpdateDraw(func() {
                       pv.ShowReauthWaiting(fmt.Sprintf("%s Verify code %s at %s", reauthWaitingMsg, code, url))
                   })
               },
               func(ctx context.Context) ([]awsssm.Parameter, error) {
                   return pv.host.ListParameters(ctx, profile, "/")
               },
           )
           pv.host.QueueUpdateDraw(func() {
               if seq != pv.loadSeq {
                   return // superseded by a newer load()
               }
               if err != nil {
                   slog.Error("ssm parameters: failed to list parameters", "error", err)
                   pv.showError(err)
                   return
               }
               pv.repaint(params)
           })
       }()
   }
   ```
   The profile-empty early return stays exactly as-is (before `loadSeq`
   increments or anything is shown) — showing a loading placeholder
   right before immediately replacing it with that error would just
   flicker.

4. **`ShowReauthWaiting`/`ShowReauthDone` extracted as named methods**,
   implementing `ui.ReauthStatusShower`:
   ```go
   var _ ui.ReauthStatusShower = (*SSMParamsView)(nil)

   func (pv *SSMParamsView) ShowReauthWaiting(msg string) {
       pv.showStatus(msg)
   }

   func (pv *SSMParamsView) ShowReauthDone() {
       pv.showStatus(loadingParametersStatus)
   }
   ```
   `load()`'s `onReauth`/`onCode` closures call `pv.ShowReauthWaiting(...)`
   instead of `pv.showStatus(...)` directly (shown above) — cosmetic
   from `awsauth.WithReauth`'s point of view (still direct callbacks,
   nothing routed through `app.go`), but gives these views the same
   named-method shape as `QueuesView`. Nothing currently calls
   `ShowReauthDone()` (there's no case where reauth starts, then needs
   reverting *without* the fetch completing right after — the retried
   `call(ctx)` inside `WithReauth` always runs next), so the method
   exists for interface completeness and future use, matching how
   `QueuesView.ShowReauthDone` is itself only reached today via the
   unrelated `secretbackend` path. Confirmed intentional, not dead
   code that should be deleted — the interface contract requires both
   methods, and a future caller (e.g. if `awsauth.WithReauth` grows a
   "reauth started but call still pending" distinct phase) would need
   it.

## Files touched

- `tui/internal/view/ssmparams.go` + `ssmparams_test.go`
- `tui/internal/view/secrets.go` + `secrets_test.go`
- `tui/internal/view/logs.go` + `logs_test.go`
- `tui/internal/view/codepipelinelist.go` + `codepipelinelist_test.go`
- `tui/internal/view/codepipelinedetail.go` + `codepipelinedetail_test.go`

`CodePipelineDetailView.load()` is called from `Open()`, not
`Activate()`/a registered `ui.View` — same edit, `dv.pipelineName` is
already set by the time `load()` runs so the per-pipeline message
works immediately.

## Testing

Each view already has a `newTestXView(t) (*fakeViewHost, *XView)`
helper (`tui/internal/view/testfake_test.go`'s shared `fakeViewHost`,
whose `QueueUpdateDraw` runs its callback inline — no real event
loop). Mirror `queues_test.go`'s 3 new-behavior tests per view,
substituting the relevant fake-host fetch field
(`listParametersFn`/`getSecretsFn`/etc. — exact field names per
`testfake_test.go`) for `fakeQueueBackend.listFn`:

- `Test<View>ShowReauthWaitingThenDone` — direct calls to
  `ShowReauthWaiting`/`ShowReauthDone`, asserting the table cell text.
- `Test<View>LoadShowsLoadingStatusImmediately` — a blocking fetch
  function (channel-gated, matching `TestQueuesViewLoadShowsLoadingStatusImmediately`),
  asserting the placeholder appears before the fetch resolves.
- `Test<View>LoadDiscardsStaleResponse` — two overlapping `load()`
  calls with controllable resolution order (matching
  `TestQueuesViewLoadDiscardsStaleResponse`'s `firstCalled`/
  `releaseFirst` channel pair), asserting the slower/first call's
  result never overwrites the second's.

No manual/live verification needed beyond the normal `go test` run —
this is pure UI-state logic with no broker/AWS-API behavior change,
fully exercisable through the existing fake-host test doubles (unlike
queue/message/connection changes, which `tui/CLAUDE.md` requires
live-verifying).
