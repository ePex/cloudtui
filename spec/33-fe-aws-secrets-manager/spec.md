# Spec — FE 33: AWS Secrets Manager integration

Date: 2026-08-08

## Background

FE 32 added a Parameter Store browser (`ssm-parameters`) using
`aws-sdk-go-v2/service/ssm` and the profile selected via FE 29
(`cfg.ActiveAWSProfile`). Secrets Manager is a related but distinct AWS
service — separate SDK module (`aws-sdk-go-v2/service/secretsmanager`),
separate API, and a meaningfully different value-retrieval shape (see
below) — deliberately deferred out of FE 32's scope to its own feature.

## Problem

No way to look at Secrets Manager entries from cloudtui — same
motivation as FE 32 (e.g. confirming a DB credential or API key stored
there without leaving the app or reaching for the AWS CLI/console).

## Decisions (confirmed)

1. **New top-level app**, not folded into the SSM Parameters view. Same
   pattern as `queues`/`ssm-parameters`: a `secrets-manager` entry in
   Home's "Apps" section, its own `ui.View`.
2. **Uses `cfg.ActiveAWSProfile`** for credentials, same as FE 32 —
   errors clearly if none is selected.
3. **Every secret's value is masked on screen until explicitly revealed.**
   Unlike Parameter Store, this isn't a design choice with an
   alternative: Secrets Manager's `ListSecrets` structurally never
   returns a value (only metadata — name, ARN, last-changed date,
   rotation status); the value only ever comes back from a separate
   `GetSecretValue` call. So the list view is metadata-only by
   construction, and the detail view's `r`-to-reveal action is what
   triggers that fetch when the user wants to *see* the value on
   screen — same UX shape as FE 32's `SecureString`, arrived at for a
   different structural reason.
4. **Copying does not require revealing.** `c` is available the moment
   the detail view opens, not only after `r`. Pressing it fetches the
   value if not already fetched (the same `GetSecretValue` call `r`
   would trigger) and copies it straight to the clipboard — the screen
   stays masked throughout. This matches how password managers let you
   copy a credential without ever displaying it. Fetching is cached
   either way: pressing the other key afterward (`r` after a silent
   `c`, or `c` after an `r`) reuses the value already fetched rather
   than calling `GetSecretValue` again.
5. **A revealed value is pretty-printed if it parses as JSON**, since
   Secrets Manager secrets are very commonly JSON key/value blobs
   (e.g. `{"username": "...", "password": "..."}`); otherwise shown as
   the raw string. A `SecretBinary` value (no `SecretString`) is shown
   as "(binary secret — cannot display)" rather than attempting to
   render arbitrary bytes, and `c` has nothing to copy for one — it's
   the only case where the copy shortcut disappears, once the fetch
   reveals the secret is binary.
6. **Read-only, browse-scoped**: list + reveal only. No create/update/
   delete/rotate, no integration with `config.Connection`.
7. **Reuses the `c`-to-copy-to-clipboard shortcut** from FE 32's
   parameter detail view (same `App.copyToClipboard`). For a
   pretty-printed JSON value, copies the pretty-printed text (what
   would be shown if revealed), not the original compact string.
   **FE 32's own detail view was updated to match** (decision 4's
   copy-without-reveal behavior applies to a `SecureString` parameter
   too) — see that spec's `tasks.md` for the pointer.

## Proposed scope for this slice

- `tui/internal/awssecrets` (or similar): thin wrapper over
  `secretsmanager.Client` — `List(ctx, profile) ([]Secret, error)`
  (paginated `ListSecrets`) and `Reveal(ctx, profile, name) (Secret,
  error)` (`GetSecretValue`, `AWSCURRENT` only — no version/stage
  selection in this slice).
- New view (table, same shape as `ssmParamsView`): lists secrets
  (name/last-changed/rotation-enabled), filterable by substring on name.
  `Enter` opens a detail view showing metadata + "(encrypted — press
  `r` to reveal)"; `r` reveals (async `GetSecretValue`), `c` copies the
  shown value once present.
- Registered as a real `ui.View`/`ui.Shortcuttable`, listed under Home's
  "Apps" section next to `queues` and `ssm-parameters`.

## Out of scope (this slice)

- Writing/managing secrets (create, update, delete, rotate).
- Selecting a specific version/stage (`VersionId`/`VersionStage`) —
  always fetches `AWSCURRENT`.
- Using a secret's value to populate a connection.
- Any region other than the active profile's configured region.
- Secret metadata beyond what's shown in the list/detail (tags, rotation
  Lambda ARN, etc.) — only what's directly useful for browsing.
