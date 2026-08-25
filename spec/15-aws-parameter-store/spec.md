# AWS Systems Manager Parameter Store browser

_Condensed from spec/32 — see that folder for the incremental history._

## Purpose

Browse AWS Systems Manager Parameter Store values from inside cloudtui —
e.g. confirming a broker endpoint or credential stored there — without
leaving the app or reaching for the AWS CLI/console.

## Behavior / user flow

- A top-level app (`ssm-parameters`), registered as a real `ui.View` in
  Home's "AWS" section and switchable via the command prompt — not tucked
  under Settings, since browsing is a primary feature, not configuration.
- Uses `cfg.ActiveAWSProfile` (spec/14) for credentials; errors
  clearly if none is selected rather than falling back to the SDK's
  default credential chain.
- List view (table, same shape as `queuesView`): parameters fetched
  recursively from the root path (`/` via `GetParametersByPath`), showing
  a leftmost star column plus name/type/last-modified. `/` filters by
  substring on name, same convention as the queues list and AWS Profiles.
- `f` toggles the selected row's favorite status, scoped to the current
  AWS profile (see "Data & config" below) — favorited rows always sort
  above non-favorited ones (a stable partition on top of the loaded
  order, not a second column-sort mode), and show a `★` in the star
  column. Same mechanism, key, and display as Secrets Manager (spec/16)
  and CloudWatch Logs (spec/17).
- `Enter` on a `String`/`StringList` row opens a detail view showing the
  value immediately.
- `Enter` on a `SecureString` (KMS-encrypted) row opens a detail view
  showing only metadata, masked — `r` explicitly reveals it (a second
  `GetParameter` call with decryption). List/metadata calls never decrypt.
- `c` in the detail view copies the currently-shown value to the system
  clipboard — available as soon as a value is on screen (for a
  `SecureString`, only after `r` has revealed it).
- Read-only, browse-scoped: no put/delete/create, and no integration with
  `config.Connection` or the connection editor.

## Data & config

- `tui/internal/awsssm/` (or equivalent): `ListParameters(ctx, profile,
  path) ([]Parameter, error)` and `GetParameterValue(ctx, profile, name
  string, decrypt bool) (string, error)`.
- Credentials resolved via `config.LoadDefaultConfig(ctx,
  config.WithSharedConfigProfile(name))` — this genuinely authenticates
  (unlike `awsprofile`'s config-file-only reads), so SSO/credential_process
  resolution actually executes. An expired SSO token does not itself
  trigger a browser login — see spec/14's auto-reauth behavior,
  which wraps this view's `load()`.
- Always fetches the whole parameter tree from `/` and filters
  client-side (same approach as the AWS Profiles overlay) — no
  path-scoped server-side fetching in this slice.
- Favorited parameter names are stored in `config.Config.AWSFavorites`
  (`SSMParameters map[string][]string`, keyed by AWS profile name — see
  spec/16 for why favorites are per-profile and why the three item kinds
  are independent namespaces). Persisted immediately on toggle via
  `Host.ToggleFavorite`.

## Implementation notes

- Registered in Home's "AWS" section, alongside `secrets-manager`,
  `cloudwatch-logs`, and `codepipeline` — not alongside `queues`, which is
  its own "ActiveMQ" section (see spec/05 for the current section
  grouping).
- Detail view reuses the existing message-detail-view rendering pattern.

## Notable gotchas worth preserving

- Secrets Manager was deliberately **not** folded into this view despite
  the similar "browse a secret store" shape — it has a structurally
  different value-retrieval API (`ListSecrets` never returns a value at
  all) and ships as its own app; see spec/16.
