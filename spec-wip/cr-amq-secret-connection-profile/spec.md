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
- Connection editor: a new "AWS Profile" text field, shown directly
  above "Secret Name" (renamed from "Password Secret (AWS)", then again
  from "Password Secret Name") only when Authentication Mode (renamed
  from "Password Source") = AWS Secret, and validated as required on
  save when that source is selected (same pattern as the existing "Name
  is required" check). Whichever field(s) Authentication Mode currently
  shows below it ("Password" for Plain, or "AWS Profile"/"Secret Name"
  for AWS Secret) render with a 2-space label indent, reading as
  visually nested under Authentication Mode rather than a peer of
  Name/Backend/URL. The field offers autocomplete against the same
  discovered-profile source as Settings → AWS Profiles
  (`host.ListAWSProfiles`, i.e. `awsprofile.List()`), filtered by
  prefix — same mechanism and degrade-on-error behavior as the existing
  JMS Type field's autocomplete (`MessageFilter.jmsTypeSuggestions`). It
  remains a plain, freeform `InputField` underneath: autocomplete only
  offers suggestions, it never restricts input to known profiles.
- Connection editor form reorganized into three visually-separated,
  non-interactive section headers (`── General ──`, `── Destination ──`,
  `── Auth ──`, added via `tview.Form.AddTextView` with
  `scrollable=false`, which makes a Form-embedded `TextView`
  non-focusable — Tab skips straight over it): General holds Name;
  Destination holds Backend and whatever it implies (Broker Name for
  jolokia, URL for both); Auth holds Username, Authentication Mode, and
  the indented field(s) described above. Save/Cancel remain unindented,
  last, outside every section. Prompted by the same conversation that
  named/positioned the AWS Profile field — the editor's fixed overlay
  height (`app.go`'s `connEditorOverlay`) was widened from 20 to 28 rows
  to fit the now-taller worst case (jolokia + AWS Secret, 11 form
  items) without clipping.
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
- Top-left info panel: the "AMQ Connection" line appends
  `(AWS: <profile>)` when the active connection authenticates via AWS
  Secret, so which account a connection's password depends on is
  visible at a glance without opening the connection editor — and
  visibly distinct from the separate "AWS Profile:" line below it
  (the *global* active profile), reinforcing that the two are
  independent.

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
- A hard profile-*picker* dropdown (a `DropDown` restricted to known
  profiles) for the new field — it stays a freeform `InputField` with
  autocomplete (added per user feedback after the initial cut), not a
  constrained selector; an unlisted or not-yet-configured profile name
  must still be typeable.
- Any further reorganization of the connection editor beyond the three
  sections above (e.g. a genuinely collapsible/foldable section, or
  sections that persist a per-user open/closed state) — these are
  always-expanded, purely visual groupings.
