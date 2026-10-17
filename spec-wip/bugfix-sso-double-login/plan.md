# Plan: AWS SSO login opens the browser twice

Implements `spec.md` in this folder.

## Where logins happen today

Every caller that recovers from an expired SSO session goes through
`awsauth.WithReauth` (directly or via `awsauth.Do`), which calls the
`login` function it's given:

- `secretbackend.SecretResolver.Resolve` (the queue connection's
  Secrets Manager password, spec/12),
- `view/awsload.go` (Parameter Store, Secrets Manager, CloudWatch Logs),
- `view.PipelineWatcher.pollPipeline` (each watched pipeline).

All three get that function from the app: `App.AWSSSOLogin`, which
calls `a.awsSSOLogin`, set to `awsauth.Login` in `app.New`.
`awsauth.Login` runs `aws sso login` with `exec.CommandContext` on the
caller's context, so cancelling the caller (Esc on "Loading queues")
kills the CLI.

## Approach: a login coordinator behind `AWSSSOLogin`

A new `awsauth.LoginCoordinator` (`internal/awsauth/coordinator.go`)
with the same `Login(ctx, profile, onCode) error` signature as
`awsauth.Login`, so it drops in as `a.awsSSOLogin` and every caller is
covered without changing any of them.

- **One run per profile.** It keeps a map from profile to the login in
  flight. The first caller starts one: the real `awsauth.Login` (the
  coordinator's injected `run` func), on a goroutine, with the
  coordinator's own context, not the caller's. Later callers for the
  same profile join it. The entry is removed when the run returns, so
  the next login after that starts a new run.
- **Every caller sees the code.** The run's `onCode` records the code
  and URL and passes them to each caller's `onCode`, including callers
  that join after the code was printed (they get it at once). Callers'
  `onCode` funcs are called outside the coordinator's lock; they
  already hand off to the UI goroutine themselves (`QueueUpdateDraw`).
- **Waiting is cancellable, the run is not.** Each caller waits for the
  run's result or its own `ctx.Done()`, whichever comes first, and
  returns `ctx.Err()` on cancellation. `WithReauth` then reports the
  failed re-auth as today ("…re-auth attempt failed: context
  canceled"), and the queue view shows its cancelled state. The run
  keeps going, so approving the open tab still logs in; the next load
  either joins it or, once it succeeded, needs no login.
- **A failed run's error goes to every caller** waiting on it. None of
  them starts another login: `WithReauth` only logs in once per call.
- **Quitting.** `Close()` cancels the coordinator's context, which
  makes `exec.CommandContext` kill a running `aws sso login`, and waits
  for the run goroutines to return, so the process isn't left behind
  when cloudtui exits. `App.Run` calls it after `tv.Run()` returns.

What's not covered: a caller whose own call fails just *after* a run
finished starts a new login. The token check that fails is a local file
read at the start of the call, so the window is tiny; a "recent
success" grace period would add state for no observed case.

## Missing token file

`awsauth.NeedsReauth` (`internal/awsauth/awsauth.go`) gains a second
narrow string check next to the existing `sso-session` one:
`"failed to read cached SSO token file"`, the SDK's text when an
`sso-session` profile's cache file can't be read
(`ssocreds/sso_cached_token.go`, `loadCachedToken`). Legacy
`sso_start_url` profiles already wrap that case in
`*ssocreds.InvalidTokenError`, which the check already handles. Still
gated on the profile's auth type being SSO.

A string check rather than `errors.Is(err, fs.ErrNotExist)`: the error
passes through several SDK wrappers between the credential provider and
the Secrets Manager call, and the existing check already matches by
text for the same reason. The test uses the real SDK's error (below),
so a changed wording shows up there.

## Files

- `tui/internal/awsauth/coordinator.go` (+ `coordinator_test.go`): new.
- `tui/internal/awsauth/awsauth.go` (+ test): the missing-file check.
- `tui/internal/app/app.go`: `a.awsSSOLogin = coordinator.Login`, keep
  the coordinator on `App`, `Close()` it in `Run`.
- Merge-back: spec/14 (the login flow: one per profile, cancelling,
  missing token file) and spec/12 (the secret-backed connection's
  re-auth, which refers to it).

## Testing

- **Coordinator, with a fake `run`** controlled by channels (no real
  CLI):
  - two concurrent logins for one profile: `run` called once, both
    return its result;
  - two profiles: two runs, independent;
  - a joiner gets the code whether it joins before or after the code
    was printed; each caller gets it once;
  - a caller whose context is cancelled returns `context.Canceled` at
    once while the run continues; a login started meanwhile joins that
    run; after it finishes, a new login starts a new run;
  - a failed run returns the same error to every waiting caller;
  - `Close()` cancels a running login's context and returns only after
    the run returned.
  - `go test -race` covers the locking.
- **`NeedsReauth`:** the existing table gains the missing-file case,
  built from the real SDK: load an `sso-session` profile from a temp
  `~/.aws/config` with no token cache and ask it for credentials, which
  fails locally with the SDK's own error (no network). Plus: the same
  error for a non-SSO auth type is not a re-auth case.
- **Mutation checks:** without the shared run (one run per caller), the
  "run called once" test fails; with the caller's context passed to
  `run`, the cancellation test fails; without the new string, the
  missing-file case fails.
- The app wiring (one line plus `Close` in `Run`) has no logic of its
  own; it's covered by the live check.
- **Live (fake `aws` on `PATH`, temp HOME, local broker),** the setup
  used to reproduce the bug: a connection with a Secrets Manager
  password for a fake `sso-session` profile, and a fake `aws` that logs
  each call and waits a few seconds. Then:
  - open the queue list, Esc during the login, reload: one `aws sso
    login` in the log, still running, then finishing;
  - with the token file removed: the login is offered (one call)
    instead of a plain error;
  - quit during a login: the fake `aws` process is gone afterwards.
  Plus `task test`, `task test:tui:race` and the smoke test.
