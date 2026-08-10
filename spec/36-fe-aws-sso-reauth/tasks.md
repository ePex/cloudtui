# Tasks — FE 36: Automatic AWS SSO re-authentication

1. [x] `internal/awsprofile`: add `AuthTypeFor(ctx, name) (AuthType, error)`,
   factoring the per-profile `config.LoadSharedConfigProfile` call out of
   `List()`'s loop into a shared unexported helper. Unit tests: one case
   per `AuthType` plus "profile not found", via `t.TempDir()` fixtures
   matching `list_test.go`'s existing conventions.
2. [x] New package `internal/awsauth`: `NeedsReauth(err, authType) bool`
   and `Login(ctx, profile) error` (the `aws sso login --profile <name>`
   shell-out, with a clear error if `aws` isn't on `PATH`). Unit tests
   for `NeedsReauth` only (table-driven, per plan.md) — `Login` is not
   unit-tested since it shells out and opens a browser; covered instead
   by manual verification in task 8.
3. [x] `internal/awsauth`: add generic `WithReauth[T any](ctx, profile,
   authType, login, onReauth, call) (T, error)`. Unit tests with fake
   `login`/`call` closures covering: first call succeeds; first call
   fails needing reauth + login succeeds + retry succeeds; first call
   fails needing reauth + login fails; first call fails not needing
   reauth.
4. [x] `internal/app`: add `awsAuthTypeFor` and `awsSSOLogin` func
   fields to `App`, wired to `awsprofile.AuthTypeFor` and `awsauth.Login`
   in `New()`. `go mod tidy` to promote `aws-sdk-go-v2/credentials` from
   indirect to direct (no version change, already resolved
   transitively). No behavior change yet — nothing calls these fields
   until tasks 5–7.
5. [x] Wire SSM Parameters (`ssmparams.go`): `load()` routes through
   `awsauth.WithReauth`; add `showStatus(msg string)` (accent-colored,
   sibling to `showError`). Unit test covers `showStatus`'s rendering
   (text + accent color) — `load()`'s goroutine+`QueueUpdateDraw` path
   isn't directly testable without a running tview event loop (same
   constraint documented on `TestSSMParamsViewLoadErrorsWithoutActiveProfile`);
   the retry control flow itself is covered independently by
   `internal/awsauth`'s tests (task 3).
6. [x] Wire Secrets Manager (`secrets.go`) the same way as task 5, with
   its own `showStatus` unit test.
7. [x] Wire CloudWatch Logs (`logs.go`) the same way as task 5, with its
   own `showStatus` unit test.
8. [x] Manual verification against a real SSO profile: expire/delete its
   cached token, open each of SSM Parameters / Secrets Manager /
   CloudWatch Logs, confirm the status message appears, the browser
   opens, and the view loads data after completing login — no further
   action needed. Done by the user against the real `mlf-preprod`
   profile (`aws-sso-util`-managed, cached SSO token deleted): confirmed
   working after fixing a profile-classification bug uncovered along the
   way — see spec/37-bugfix-awsprofile-sso-vs-credential-process-precedence.
