# AWS profile discovery, selection, and SSO re-authentication

_Condensed from spec/28, spec/29, spec/36, spec/37 — see those folders for the incremental history._

## Purpose

Let cloudtui discover the AWS CLI profiles already configured on the
user's machine, select one as active for AWS-backed views, and
transparently recover from an expired/missing SSO token — without the
user dropping to a terminal.

## Behavior / user flow

- **Discovery**: reads `~/.aws/config` and `~/.aws/credentials`
  (respecting `AWS_CONFIG_FILE`/`AWS_SHARED_CREDENTIALS_FILE` env var
  overrides) via `aws-sdk-go-v2/config`'s shared-config loader — never a
  hand-rolled INI parser, so `source_profile` inheritance, `sso-session`
  blocks, `[profile x]` vs `[x]` naming, and env overrides all work
  correctly. No `~/.aws` directory at all → empty list, not an error. This
  is read-only, no network calls, no credential resolution.
- **Settings → "AWS Profiles"** overlay (also opened via `:ap` /
  `:awsprofiles` from anywhere) lists discovered profiles: name, region,
  auth type. `/` filters by name (case-insensitive substring, same
  convention as the queues list — needed in practice: real machines can
  have 60+ profiles). `r` refreshes (re-reads the files). `Enter` activates
  the row under the cursor: persists `cfg.ActiveAWSProfile` to
  `config.yaml`, updates the info panel and the Settings list, closes the
  overlay, shows a status-bar confirmation. The active profile is marked
  with a star.
- Info panel gains an `AWS Profile: <name>` line (`(none)` when unset),
  same treatment as the `Connection:` line. Settings list shows an `AWS
  Profile: <name>` row that reflects the same in-memory value.
- Activating an AWS profile is entirely independent of the active broker
  **connection** — the two never affect each other. A profile is (for now)
  purely a discovery aid / remembered selection: `cfg.ActiveAWSProfile` is
  not wired into `config.Connection` or any backend.
- **Automatic SSO re-authentication**: when an AWS-backed view (SSM
  Parameters, Secrets Manager, CloudWatch Logs) fails a load specifically
  because the active SSO profile's cached token is missing/expired, the
  view shows a status message ("AWS SSO session expired — opening browser
  to log in..."), shells out to `aws sso login --profile <name>` in the
  background (on the existing load goroutine — no extra async wiring), and
  on success silently retries the original call once. If the retry also
  fails, the second error is shown normally (`showError`) — no retry
  loops. Any other failure (network error, real `AccessDenied`, bad
  parameter path, no profile selected, or a non-SSO auth type) is shown as
  a plain error immediately, unchanged.

## Data & config

- `config.Config.ActiveAWSProfile string` (`activeAWSProfile` in
  `config.yaml`) — independent top-level field, not part of
  `Connections`/`ActiveConnection`.
- `awsprofile.Profile{Name, Region, AuthType}` — `AuthType` is one of
  static-keys / SSO / assume-role / credential-process / unknown,
  deduplicated across the config and credentials files.

### `classify()` precedence (final, correct rule)

`awsprofile.classify()` checks in this order: **SSO configuration first**,
then `RoleARN` (assume-role), then `CredentialProcess`, then static keys.
This matches `aws-sdk-go-v2/config`'s actual runtime resolution order
(`resolveCredsFromProfile`). It matters concretely for profiles written by
`aws-sso-util`'s "populate profiles" mode, which include *both* native
`sso_*` fields *and* a `credential_process = aws-sso-util
credential-process --profile <name>` fallback line for older-tool
compatibility — e.g.:
```
[profile example-preprod]
sso_start_url = https://example.awsapps.com/start/#/
sso_account_id = 393809552481
sso_role_name = AdministratorAccess_T12H
credential_process = aws-sso-util credential-process --profile example-preprod
sso_auto_populated = true
```
Confirmed live: with this profile's SSO cache deleted, the real error from
an SDK call is `*ssocreds.InvalidTokenError` — proof the SDK authenticates
via native SSO, not the credential_process command, despite both being
present. Classifying this as `credential-process` would silently disable
auto-reauth for it (reauth is gated on `AuthType == AuthSSO`).

## Implementation notes

- `tui/internal/awsprofile/` — `List()`, `classify()` (SSO-first
  precedence).
- `tui/internal/awsauth/` — `NeedsReauth(err error, auth
  awsprofile.AuthType) bool` and `Login(ctx, profile string) error` (the
  `aws sso login` shell-out). `NeedsReauth` is true only when both: the
  active profile's `AuthType` is SSO, and the error indicates an
  invalid/missing/expired cached token — `errors.As` against
  `*ssocreds.InvalidTokenError` (legacy `sso_start_url` profiles) plus a
  narrow string check for the `sso-session`-style expired-token error.
- The three AWS views' `load()` functions (SSM Parameters, Secrets
  Manager, CloudWatch Logs) all go through one shared retry helper rather
  than duplicating the reauth dance per view.
- `:ap`/`:awsprofiles` reuses the same `onPromptDone` focus-reset guard
  built for `:aq` (spec-origin/12).

## Notable gotchas worth preserving

- `aws-sdk-go-v2`'s SSO credentials provider **never opens a browser
  itself** — if the cached token in `~/.aws/sso/cache` is missing or
  expired, `Retrieve()` just returns an error. Only the AWS CLI's `aws sso
  login` performs the actual browser-based device-authorization flow and
  writes the token cache — which is why re-auth shells out to the CLI
  rather than reimplementing the SSO OIDC device-authorization flow with
  `service/ssooidc` (avoids hand-maintaining the undocumented
  `~/.aws/sso/cache` token file format, a fragile, security-sensitive
  thing to get subtly wrong). This requires the `aws` CLI to be on `PATH`;
  if it isn't, that's surfaced as a clear error, not a hang.
- Re-auth is scoped to SSO only: `credential-process` tools are assumed to
  already handle their own browser login; assume-role/static-keys failures
  aren't SSO-refreshable this way.
- `classify()`'s precedence bug (see above) is a reminder that a profile
  can carry multiple plausible auth mechanisms in its config stanza — only
  the SDK's actual resolution order (not which field looks most
  "specific") determines which one really authenticates it.
