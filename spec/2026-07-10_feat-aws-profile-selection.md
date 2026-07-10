# 2026-07-10 — AWS profile selection

## Feature

Kick off the AWS integration with the piece everything else depends on:
discovering the AWS profiles available on the local machine and letting
the user pick one, persisted across restarts.

## What

- **Profile discovery.** A new package scans `~/.aws/config` (`[default]`,
  `[profile <name>]`) and `~/.aws/credentials` (`[<name>]`), respecting
  `AWS_CONFIG_FILE`/`AWS_SHARED_CREDENTIALS_FILE` overrides, and merges
  the profile names found in both files (de-duplicated). Missing files
  are not an error — same graceful-fallback precedent as
  `config.Load()`.
- **Settings becomes a real view.** The `settings` placeholder is
  replaced with an actual selectable list. This pass adds one row, `AWS
  Profile` (showing the current selection or "not set"), but the view is
  structured as a general settings list so a future row — e.g. "Queue
  Connection" for the local-broker/proxy setting mentioned during
  scoping — can be added later without redesigning the view. That row
  itself is out of scope now.
- **Profile picker.** Activating the `AWS Profile` row opens a modal
  (same centered-overlay pattern as the existing help modal) listing the
  discovered profile names, pre-selecting `AWS_PROFILE`/
  `AWS_DEFAULT_PROFILE` if set, else `default` if it's among the
  discovered names. Enter picks and persists; Escape cancels.
- **Persistence.** The chosen profile is saved to `config.yaml` (a new
  `aws: profile:` field) so it's remembered across restarts. Config
  gains a save capability alongside the existing load.
- **Top bar reflects the selection.** The connection-info panel's
  `Profile:` line, currently a static placeholder, shows the
  persisted/selected profile name once one has been chosen.

## Why

Every other AWS-backed feature (secrets, parameters, queues) needs to
know which local profile to use. Reading real profile names out of the
user's actual `~/.aws` files — rather than requiring a typed name —
avoids typos and matches how the AWS CLI and other AWS tools already
present profile choice.

## Scope

- New package for profile discovery (stdlib only — no `aws-sdk-go-v2`
  dependency yet, since nothing in this pass makes an actual AWS call;
  that dependency arrives when secrets/params/queues get real backends).
- `internal/config`: a new `AWS.Profile` field and a `Save` capability.
- `settings` view rebuilt as a real list (replacing its placeholder),
  plus the profile-picker modal.
- Top bar connection-info panel updated to show the selected profile.
- Unit tests: profile discovery (both file formats, merge/de-dup,
  missing files, env var overrides), config save/load round-trip,
  settings list and profile-picker behavior (select/cancel), top bar
  update.

## Out of scope

- Authenticating or validating the profile against AWS in any way (no
  STS call, no credential resolution) — this pass only reads and
  remembers a *name*.
- The "Queue Connection" / local-broker / mq-proxy settings row —
  explicitly deferred to a future feature; the settings view is only
  structured to accommodate it later.
- SSO login flows, credential-process execution, or anything beyond
  reading profile names from the shared config/credentials files.
- Creating, editing, or deleting AWS profiles from within cloudtui —
  selection only.
- Any change to the secrets/params/queues placeholder views themselves.
