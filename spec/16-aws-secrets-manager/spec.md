# AWS Secrets Manager browser

_Condensed from spec/33 — see that folder for the incremental history._

## Purpose

Browse AWS Secrets Manager entries from inside cloudtui — e.g. confirming a
DB credential or API key without leaving the app or reaching for the AWS
CLI/console.

## Behavior / user flow

- A top-level app (`secrets-manager`), its own `ui.View`, listed in Home's
  "AWS" section next to `ssm-parameters`, `cloudwatch-logs`, and
  `codepipeline` (not `queues`, which is its own "ActiveMQ" section).
- Uses `cfg.ActiveAWSProfile` for credentials; errors clearly if none is
  selected.
- List view (table): secrets fetched via paginated `ListSecrets`, showing
  a leftmost star column plus name/last-changed/rotation-enabled —
  **metadata only**, since `ListSecrets` structurally never returns a
  value. `/` filters by substring on name.
- `f` toggles the selected row's favorite status, scoped to the current
  AWS profile — favoriting is per profile because a secret name is only
  meaningful within the account a profile points at, and each AWS CLI
  profile already pins its own region (`~/.aws/config`), so profile alone
  is sufficient scoping with no separate region axis needed. Favorited
  rows always sort above non-favorited ones (a stable partition, not a
  second column-sort mode) and show a `★` in the star column. Same
  mechanism, key, and display as SSM Parameters (spec/15) and CloudWatch
  Logs (spec/17) — each of the three item kinds is its own independent
  namespace (favoriting a secret named `x` has no bearing on a parameter
  or log group also named `x`).
- `Enter` opens a detail view showing metadata plus "(encrypted — press `r`
  to reveal)".
- `r` reveals the value: an async `GetSecretValue` call (`AWSCURRENT` only
  — no version/stage selection). A value that parses as JSON is
  pretty-printed; otherwise shown as the raw string. A `SecretBinary`
  value (no `SecretString`) is shown as "(binary secret — cannot
  display)" and has no copy action.
- `c` copies the value to the clipboard **without requiring reveal
  first** — available the moment the detail view opens. Pressing it
  fetches the value if not already fetched (the same `GetSecretValue`
  call `r` triggers) and copies it while the screen stays masked — same
  UX as a password manager's copy-without-display. For a pretty-printed
  JSON value, copies the pretty-printed text. Whichever action (`r` or
  `c`) fetches first, the other reuses the cached value rather than
  calling `GetSecretValue` again.
- Read-only, browse-scoped: no create/update/delete/rotate, no
  integration with `config.Connection`.

## Data & config

- `tui/internal/awssecrets/`: `List(ctx, profile) ([]Secret, error)`
  (paginated `ListSecrets`) and `Reveal(ctx, profile, name) (value string,
  isBinary bool, err error)` (`GetSecretValue`, `AWSCURRENT` only —
  `isBinary` is true, and `value` empty, for a `SecretBinary`-only
  secret; there is no `Secret`-typed return here).
- Also used by spec/12 (named connections) to resolve a
  Secrets-Manager-backed connection password via `Reveal`.
- Favorited secret names are stored in `config.Config.AWSFavorites`
  (`Secrets map[string][]string`, keyed by AWS profile name; `FavoriteKind
  = FavoriteSecret`). `AWSFavorites` also holds the equivalent maps for
  SSM Parameters and CloudWatch Logs (spec/15, spec/17) — one map per
  item kind, since favorites don't cross namespaces. Persisted
  immediately on toggle via `Host.ToggleFavorite(kind, profile, name)`; a
  view reads its own favorites straight off `Host.Config().AWSFavorites`
  rather than through a dedicated getter, consistent with how every
  other view already reads config fields directly.

## Notable gotchas worth preserving

- The reveal-gating shape here (metadata-only list, explicit `r` to
  decrypt) mirrors Parameter Store's `SecureString` handling
  (spec/15) but for a structurally different reason: Parameter
  Store *chooses* to mask; Secrets Manager's list API is *incapable* of
  returning a value at all.
- The `c`-copies-without-revealing / cached-fetch behavior here is shared
  with the Parameter Store detail view (`App.copyToClipboard` + fetch
  caching) — both should stay in sync if either changes.
