# Plan — FE 36: Automatic AWS SSO re-authentication

## Approach

Add one new leaf package, `internal/awsauth`, that knows how to (a)
recognize an "SSO token missing/expired" error and (b) run `aws sso
login --profile <name>`. Wire it into the three existing AWS views
(`ssmparams.go`, `secrets.go`, `logs.go`) through the same
dependency-injection pattern the codebase already uses for every other
AWS/queue call (`App.listParameters`, `App.listSecrets`, ... — plain func
fields set to the real implementation in `New()`, overridden with fakes
in tests). No new UI framework code, no package-level mutable state.

## New package: `internal/awsauth`

### `awsauth.go`

```go
// NeedsReauth reports whether err indicates the active profile's cached
// AWS SSO token is missing or expired — the specific case `aws sso
// login` fixes. Only meaningful for SSO profiles; authType gates it so a
// coincidentally similar error from a non-SSO credential provider is
// never misread as this case.
func NeedsReauth(err error, authType awsprofile.AuthType) bool

// Login runs `aws sso login --profile <profile>`, which opens the
// user's default browser to complete the SSO device-authorization flow
// and writes the resulting token to ~/.aws/sso/cache (the same cache
// aws-sdk-go-v2's SSO credential provider reads). Blocks until the AWS
// CLI process exits or ctx is canceled. Returns a clear error if `aws`
// isn't on PATH, or if the CLI exits non-zero (output included).
func Login(ctx context.Context, profile string) error
```

`NeedsReauth` detection:
- `authType != awsprofile.AuthSSO` → always `false`.
- `errors.As(err, &(*ssocreds.InvalidTokenError))` → `true` (covers
  legacy `sso_start_url` profiles: missing cache file, unreadable cache,
  or expired `ExpiresAt`).
- Otherwise, `strings.Contains(err.Error(), "cached SSO token is
  expired, or not present")` → `true` (covers modern `sso-session`
  profiles, whose token-provider doesn't wrap a typed error — see
  `ssocreds/sso_token_provider.go` in aws-sdk-go-v2/credentials).
- Otherwise `false`.

### `retry.go`

```go
// WithReauth calls call once. If it fails with an error NeedsReauth
// recognizes for profile's authType, it invokes onReauth (so the caller
// can show a status message), then login, then retries call exactly
// once. Any other failure — including login itself failing — is
// returned as-is (login failure wraps the original error for context).
func WithReauth[T any](
    ctx context.Context,
    profile string,
    authType awsprofile.AuthType,
    login func(ctx context.Context, profile string) error,
    onReauth func(),
    call func(ctx context.Context) (T, error),
) (T, error)
```

`login` is passed in (not called as `awsauth.Login` directly) so it can
be faked in `internal/app` tests without shelling out — same reason
`App.listParameters` etc. are func fields rather than direct calls to
`awsssm.List`.

## Changes to `internal/awsprofile`

Add `AuthTypeFor(ctx context.Context, name string) (AuthType, error)` —
loads and classifies a single profile (reuses the existing `classify`
and `configFiles` helpers already in `list.go`), instead of scanning and
loading every profile via `List()` just to find one. Small refactor:
factor the per-profile `config.LoadSharedConfigProfile` call out of
`List()`'s loop into an unexported helper both `List` and `AuthTypeFor`
call, so the file-resolution logic isn't duplicated.

## Changes to `internal/app`

`app.go`:
- New func fields on `App`, wired in `New()` next to the existing AWS
  ones: `awsAuthTypeFor func(context.Context, string) (awsprofile.AuthType, error)` → `awsprofile.AuthTypeFor`,
  `awsSSOLogin func(context.Context, string) error` → `awsauth.Login`.

`ssmparams.go` / `secrets.go` / `logs.go` — each `load()` becomes:
```go
go func() {
    ctx := context.Background()
    authType, _ := pv.app.awsAuthTypeFor(ctx, profile) // error → AuthUnknown, gate just won't match
    result, err := awsauth.WithReauth(ctx, profile, authType, pv.app.awsSSOLogin,
        func() {
            pv.app.tv.QueueUpdateDraw(func() {
                pv.showStatus("AWS SSO session expired — opening browser to log in...")
            })
        },
        func(ctx context.Context) ([]awsssm.Parameter, error) {
            return pv.app.listParameters(ctx, profile, "/")
        },
    )
    pv.app.tv.QueueUpdateDraw(func() {
        if err != nil {
            slog.Error("ssm parameters: failed to list parameters", "error", err)
            pv.showError(err)
            return
        }
        pv.repaint(result)
    })
}()
```
(mirrored for `secretsView`/`logsView` with their own result/call types —
Go generics make `WithReauth` reusable as-is across all three without a
shared interface).

Each view gets a small `showStatus(msg string)`, sibling to its existing
`showError`: same "write into row 1, clear old rows" shape, but using
`p.Accent` for the text color instead of `tcell.ColorRed`, so a
Q in-progress a reauth reads as informational, not a failure.

## `go.mod`

`internal/awsauth` imports `github.com/aws/aws-sdk-go-v2/credentials/ssocreds`
directly (for `*ssocreds.InvalidTokenError`). That module is already
resolved transitively (`aws-sdk-go-v2/credentials v1.19.34 // indirect`,
pulled in by `config`) — this promotes it from indirect to direct via
`go mod tidy`, not a new dependency to justify from scratch.

## Testing

- `internal/awsauth/awsauth_test.go`: table-driven cases for
  `NeedsReauth` — wrapped `*ssocreds.InvalidTokenError` + `AuthSSO` →
  true; sso-session-style string error + `AuthSSO` → true; same errors
  with `AuthAssumeRole`/`AuthStaticKeys`/`AuthCredentialProcess`/
  `AuthUnknown` → false; unrelated error (e.g. `AccessDeniedException`)
  + `AuthSSO` → false; `nil` error → false.
- `internal/awsauth/retry_test.go`: `WithReauth` with fake `login`/`call`
  closures (call counts recorded) — first call succeeds (login/onReauth
  never invoked); first call fails needing reauth, login succeeds,
  second call succeeds (onReauth invoked once, call invoked twice);
  first call fails needing reauth, login fails (call invoked once,
  returned error reflects the login failure); first call fails not
  needing reauth (e.g. wrong authType) — login/onReauth never invoked,
  original error returned unchanged.
- `internal/awsprofile/authtype_test.go` (or added to `list_test.go`):
  `AuthTypeFor` against a `t.TempDir()` config file, mirroring
  `list_test.go`'s existing fixture conventions — one case per
  `AuthType`, plus "profile not found."
- `internal/app` view tests: existing `ssmparams_test.go` /
  `secrets_test.go` / `logs_test.go` fakes for `listParameters` etc.
  are unaffected (unchanged error path when `awsAuthTypeFor`/
  `awsSSOLogin` are left as zero-value funcs — tests that don't set them
  never hit `NeedsReauth`'s `true` branch since the fake list error
  isn't an `*ssocreds.InvalidTokenError`). Add one new test per view
  exercising the reauth path: fake `awsAuthTypeFor` → `AuthSSO`, fake
  first `listParameters` call → `*ssocreds.InvalidTokenError`-wrapped
  error, fake `awsSSOLogin` → success + second call succeeds; assert
  `showStatus` was reached (table shows the in-progress message before
  the final repaint) and the final repaint reflects the retried data.
- No test shells out to the real `aws` CLI or touches a browser —
  `Login` itself (the `exec.Command` call) is exercised only by manual
  verification below, not unit tests.
- Manual verification (per `tui/CLAUDE.md`): with a real SSO profile,
  expire/delete its cached token (`rm ~/.aws/sso/cache/*.json` or wait
  for natural expiry), open SSM Parameters (or Secrets Manager /
  CloudWatch Logs), confirm the status message appears, the browser
  opens, and after completing login in the browser the view loads data
  without any further action. Record this in `tasks.md`.

## Trade-offs / open questions for review

1. `AuthTypeFor` re-reads `~/.aws/config`/`credentials` from disk on
   every `load()` call (once per view activation), same cost class as
   `awsprofile.List()` which the AWS Profiles overlay already does
   synchronously on every open — expected to be negligible, but flagging
   since it's an extra file read that wasn't there before.
2. `Login` runs with `context.Background()` (via the view's existing
   unstarted-context goroutine pattern) — no in-app way to cancel a
   hung/abandoned browser login short of restarting cloudtui. Matches
   spec's "no settings toggle" scope decision; acceptable since it's
   backgrounded and doesn't block the rest of the UI.
