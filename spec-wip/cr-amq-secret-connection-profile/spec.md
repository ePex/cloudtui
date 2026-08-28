# Per-connection AWS profile for AWS-Secrets-Manager-backed passwords

Date: 2026-08-28

## What

A connection whose password comes from AWS Secrets Manager
(`passwordSecret`) currently resolves that secret via whatever AWS
profile happens to be **globally active** (`cfg.ActiveAWSProfile`,
Settings → AWS Profiles) — a connection never states which profile its
own secret actually lives in. This change adds a second, required field
alongside `passwordSecret`: `passwordSecretAWSProfile`, naming the exact
AWS profile to use for *that connection's* secret, independent of
whatever profile is globally active for SSM Parameters/Secrets
Manager/CloudWatch Logs/CodePipeline browsing.

## Why

Reported directly: it's not obvious, looking at a connection using
`passwordSecret`, which AWS profile its password actually depends on —
that information lives entirely in a separate, global setting the user
has to remember to keep pointed at the right account. Worse, switching
the global AWS profile (e.g. to browse a different account's SSM
parameters) silently changes which profile an *already-configured*
connection's secret resolves against too, even though the user's intent
was just "look at some other account's parameters," not "change which
account this broker's password comes from." Making the profile an
explicit, required part of the connection itself removes this ambiguity
entirely — a connection is now fully self-describing.

This directly reverses a deliberate prior decision (spec/12's
"per-connection AWS profile" was explicitly out of scope) — that's the
point: the user asked for it, once actually living with the global-only
behavior made the ambiguity concrete.

## Scope

- `config.QueueConfig`/`config.ProxyConfig` gain
  `PasswordSecretAWSProfile string` (yaml `passwordSecretAWSProfile`),
  alongside the existing `PasswordSecret`. **Required** whenever
  `PasswordSecret` is set — not optional-with-fallback-to-the-global-
  profile. This is a breaking config change for any existing connection
  using `passwordSecret`: it needs this field added by hand (or via the
  connection editor) to keep working.
- Connection editor: a new "Secret AWS Profile" text field, shown
  alongside "Password Secret (AWS)" only when Password Source = AWS
  Secret (same conditional-visibility mechanism), and validated as
  required on save when that source is selected (same pattern as the
  existing "Name is required" check).
- `secretbackend.New` drops its separate `profile string` parameter
  entirely — the profile now comes from the connection itself
  (`conn.Queue.PasswordSecretAWSProfile` /
  `conn.Proxy.PasswordSecretAWSProfile`), so there's nothing left for a
  caller to pass. All 4 call sites simplify accordingly.
- `App.SetActiveAWSProfile` no longer rebuilds `a.backend` — secret
  resolution no longer depends on the globally active profile at all,
  so switching it has zero effect on an already-configured connection's
  password. This is the direct, deliberate reversal of spec/88's fix
  (which existed *specifically because* secret resolution used to
  depend on the global profile); `TestSetActiveAWSProfileRebuildsSecretBackedBackend`
  is deleted, not just updated, since the scenario it guards against can
  no longer occur.
- `SecretResolver.Resolve`'s "no AWS profile selected" error message
  (for an empty profile) gets reworded — it's no longer about the global
  Settings picker; it now means the connection's own
  `passwordSecretAWSProfile` is unset (a hand-edited config bypassing
  the editor's now-required validation).
- `tui/config.example.yaml` updated to document the new field and show
  it as required alongside `passwordSecret`.

## Out of scope

- Any change to how the *globally* active AWS profile works for SSM
  Parameters/Secrets Manager/CloudWatch Logs/CodePipeline — those keep
  using `cfg.ActiveAWSProfile` exactly as today. This change only
  affects where a connection's *own* password-secret profile comes
  from.
- Structured/JSON secrets, sourcing username/broker-name/URL from
  Secrets Manager, a manual "refresh secret" action, editing/rotating
  the secret's value from within cloudtui — all still out of scope per
  spec/12, unchanged.
- A profile-picker dropdown (reusing `awsprofile.List()`'s discovery)
  for the new field — a plain text input, matching every other field in
  this form (including the existing "Password Secret (AWS)" field,
  which is also freeform text with no autocomplete).
