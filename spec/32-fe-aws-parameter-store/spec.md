# Spec — FE 32: AWS Systems Manager Parameter Store integration

Date: 2026-08-08

## Background

FE 28/29 added AWS profile discovery and selection but explicitly stopped
short of any real AWS API call — everything was local file parsing. This
is the first feature to actually call AWS with those credentials.

Good news on feasibility: `aws-sdk-go-v2/config` (already a dependency)
transitively pulls in `service/sso`, `service/ssooidc`, and `service/sts`,
which is exactly what's needed to resolve credentials for this machine's
real profiles (mostly SSO-based, some `credential_process`-wrapped) via
`config.LoadDefaultConfig(ctx, config.WithSharedConfigProfile(name))`. The
only new dependency needed is `aws-sdk-go-v2/service/ssm` itself.

## Problem

No way to look at Parameter Store values from cloudtui — useful for e.g.
confirming a broker endpoint or credential stored there without leaving
the app or reaching for the AWS CLI/console.

## Decisions (confirmed)

1. **This is a new top-level app/view, not a Settings entry.** Registered
   like `queues` (`ui.View`, added to Home's "Apps" section, switchable
   via the command prompt), not tucked under Settings — Parameter Store
   browsing is a primary feature, not a configuration screen.
2. **Uses `cfg.ActiveAWSProfile`** (FE 29's selection) for credentials —
   errors clearly if none is selected, rather than silently falling back
   to whatever the SDK's default chain would pick.
3. **`SecureString` parameters (KMS-encrypted) are masked until
   explicitly revealed.** List/metadata calls never decrypt;
   `String`/`StringList` values show immediately on selection,
   `SecureString` requires an explicit second action to actually call
   `GetParameter` with decryption. Matches the connection editor's
   existing password-field convention.
4. **Read-only, browse-scoped**: list + view values only. No put/delete/
   create, and no integration with `config.Connection` or the connection
   editor — this is purely a Parameter Store browser, not a source for
   connection credentials (that would be a separate, later spec if wanted).
5. **The detail view has a `c` shortcut to copy the shown value to the
   system clipboard** (available once a value is actually on screen — for
   a `SecureString` that means only after `r` has revealed it), so a
   parameter value can be pasted elsewhere without hand-retyping it.

## Proposed scope for this slice

- `tui/internal/awsssm` (or similar): thin wrapper over `ssm.Client` —
  `ListParameters(ctx, profile, path) ([]Parameter, error)` and
  `GetParameterValue(ctx, profile, name string, decrypt bool) (string, error)`.
  Credentials resolved from the given profile name via
  `config.LoadDefaultConfig` + `WithSharedConfigProfile` — same pattern as
  `awsprofile`'s `LoadSharedConfigProfile`, but this one actually needs to
  *authenticate*, not just read config fields, so SSO/credential_process
  resolution genuinely executes. Correction (see FE 36): a cached SSO
  token being expired does *not* trigger a browser login by itself — the
  AWS SDK just errors, and cloudtui drives `aws sso login` in response
  (see spec/36-fe-aws-sso-reauth).
- New view (table, same shape as `queuesView`): lists parameters
  (name/type/last-modified) fetched recursively from a root path
  (`/`, via `GetParametersByPath`), filterable by substring on name (same
  `/`-filter convention as queues/AWS Profiles). `Enter` on a
  `String`/`StringList` row shows its value in a detail view (reusing the
  existing message-detail-view pattern); `Enter` on a `SecureString` row
  shows metadata and requires a second explicit key to decrypt-and-show.
- Registered as a real `ui.View`/`ui.Shortcuttable`, listed under Home's
  "Apps" section next to `queues`.

## Out of scope (this slice)

- Writing/managing parameters (put, delete, create).
- Using a parameter's value to populate a connection.
- Any region other than the active profile's configured region.
- Parameter Store's "advanced" tier features (parameter policies, etc.).
- Path-scoped fetching (v1 always fetches the whole tree from `/` and
  filters client-side, same as AWS Profiles) — could be slow on a large
  parameter store; a future improvement, not blocking this slice.
- **AWS Secrets Manager — deliberately deferred to its own later feature**,
  not folded into this one even though it's a similar "browse a secret
  store" shape. Confirmed explicitly: this slice is Parameter Store only.
