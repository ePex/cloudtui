# Spec — FE 36: Automatic AWS SSO re-authentication

Date: 2026-08-10

## Background

FE 32/33/34 (SSM Parameters, Secrets Manager, CloudWatch Logs) all follow
the same shape: `load()` calls a thin AWS SDK wrapper with
`cfg.ActiveAWSProfile`, and on error calls the view's `showError(err)`,
which prints the raw error (`fmt.Sprintf("Error: %v", err)`) into the
table. Those specs assumed an expired cached SSO token would "prompt for
SSO login in a browser ... real AWS SDK behavior, not something cloudtui
controls." That assumption is wrong: `aws-sdk-go-v2`'s SSO credentials
provider never opens a browser itself — if the cached token in
`~/.aws/sso/cache` is missing or expired, `Retrieve()` just returns
`*ssocreds.InvalidTokenError` (legacy `sso_start_url` profiles) or a plain
wrapped error (modern `sso-session` profiles). Only the AWS CLI's `aws sso
login` command actually performs the browser-based device-authorization
flow and writes the token cache.

## Problem

When not authorized in AWS (no cached SSO token yet, or it's expired),
every AWS-backed view — SSM Parameters, Secrets Manager, CloudWatch Logs —
just fails: the table shows a raw SDK error string
(`the SSO session has expired or is invalid: ...`) and the user has to
drop out to a terminal, run `aws sso login --profile <name>` by hand, then
come back and reload. Reported as: "when i'm not authorized in aws the
commands fail."

## Decisions (proposed — confirm before I move to plan.md)

1. **Detect "needs SSO login" specifically**, not any AWS error. Only
   trigger re-auth when both are true: the active profile's `AuthType`
   (from `internal/awsprofile`) is `AuthSSO`, and the returned error
   indicates an invalid/missing/expired cached token (`errors.As` against
   `*ssocreds.InvalidTokenError` for legacy profiles, plus a narrow check
   for the `sso-session`-style "cached SSO token is expired, or not
   present" error). Any other failure (network error, real
   `AccessDenied`, bad parameter path, no profile selected, static-keys /
   assume-role / credential-process profiles) is shown as today —
   unchanged.
2. **Re-auth by shelling out to `aws sso login --profile <name>`**, not
   by reimplementing the SSO OIDC device-authorization flow with
   `service/ssooidc` ourselves. Trade-off: requires the AWS CLI to be
   installed and on `PATH` (reasonable — these are the same
   `~/.aws/config` profiles the AWS CLI itself manages, and SSO profiles
   are normally created via `aws configure sso`), but avoids
   reimplementing and hand-maintaining the undocumented `~/.aws/sso/cache`
   token file format ourselves, which is a fragile, security-sensitive
   thing to get subtly wrong. If `aws` isn't on `PATH`, that's surfaced as
   a clear error instead of a mysterious hang.
3. **Automatic, not confirmation-gated.** When re-auth is needed, the view
   shows a status message (e.g. "AWS SSO session expired — opening
   browser to log in..."), runs `aws sso login` in the background
   (already inside the existing load goroutine), and on success silently
   retries the original call once. No modal/confirm dialog first — the
   action is a browser tab opening, not anything destructive, and this is
   exactly what the user asked for happening on every unauthorized
   command is the entire point.
4. **Retry exactly once per failed load.** If the retried call still
   fails (e.g. login succeeded but the role genuinely lacks permissions),
   fall through to today's `showError` with that second error — no retry
   loops.
5. **Shared across all three AWS views** (SSM Parameters, Secrets
   Manager, CloudWatch Logs) via one small helper rather than duplicated
   per view — exact shape (a generic `WithReauth[T]`-style wrapper vs.
   something else) is a `plan.md` decision, not a spec decision.

## Proposed scope for this slice

- New package (e.g. `internal/awsauth`): `NeedsReauth(err error, auth
  awsprofile.AuthType) bool` and `Login(ctx context.Context, profile
  string) error` (the `aws sso login` shell-out).
- Wire `ssmparams.go`, `secrets.go`, `logs.go`'s `load()` functions
  through the shared retry helper.
- Status message shown in the view while `aws sso login` is running.
- Unit tests for `NeedsReauth`'s classification (SSO + expired token →
  true; SSO + other errors → false; non-SSO auth types → always false)
  and for the retry helper's control flow (success first try / success
  after reauth / failure after reauth), with `Login` faked/injected so
  tests never shell out or touch a browser.
- Manual verification: exercise against a real expired/absent SSO token
  (per `tui/CLAUDE.md`'s manual-testing convention for behavior unit
  tests can't cover) — actually watching the browser open and the view
  recover.

## Out of scope (this slice)

- Reimplementing the SSO OIDC device-authorization flow in Go (no new
  `service/ssooidc` usage) — see decision 2.
- Re-auth for `credential-process`, `assume-role`, or static-keys
  profiles. `credential-process` tools are assumed to already handle
  their own browser login per the existing package comments;
  assume-role/static-keys failures aren't SSO-refreshable this way.
- Any change to profile *selection* (FE 29) or *discovery* (FE 28).
- A settings toggle to disable auto-re-auth.
