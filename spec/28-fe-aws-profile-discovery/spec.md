# Spec — FE 28: read AWS connection profiles from `~/.aws`

Date: 2026-08-08

## Background

cloudtui already supports AWS Amazon MQ as a target broker (FE 20/21/22:
`mq-proxy` exists specifically because AWS Amazon MQ doesn't expose
Jolokia), but only via a connection whose `url`/`username`/`password` the
user types in by hand — there is no integration with the AWS CLI's own
profile/credential system at all today.

"Add AWS support" is a larger effort than one spec. This is deliberately
scoped to its first, foundational slice: discovering what AWS profiles
exist locally, and nothing more.

## Problem

Someone who already has AWS CLI profiles configured (`~/.aws/config`,
`~/.aws/credentials`) has no way to reuse them in cloudtui — they'd have to
separately know and type broker connection details cloudtui doesn't yet
help them discover.

## Solution (this slice only)

A new internal package that reads `~/.aws/config` and `~/.aws/credentials`
(respecting the standard `AWS_CONFIG_FILE`/`AWS_SHARED_CREDENTIALS_FILE`
env var overrides) and returns the list of configured profile names, plus
enough metadata to tell them apart (region, and which kind of
authentication each uses — static keys, SSO, assume-role, or
`credential_process`).

**Read-only, no network calls, no credential resolution.** This slice
parses config files; it does not resolve, validate, or refresh actual
credentials (that would mean triggering SSO browser logins or STS calls as
a side effect of what should be inert discovery) and does not yet call any
AWS API (Amazon MQ broker listing is future work).

## Decisions (confirmed)

1. **Parsing: the official AWS SDK for Go v2's config loader**
   (`github.com/aws/aws-sdk-go-v2/config`), not a hand-rolled INI parser.
   New dependency, justified by: correct handling of `source_profile`
   inheritance, `sso-session` blocks, `[profile x]` vs `[x]` naming, and
   env var overrides — all things worth not reinventing — plus every later
   "AWS support" phase (calling the Amazon MQ API) needs this SDK anyway.
2. **Include a minimal UI surface this slice**: a read-only "AWS Profiles"
   view, reachable like the existing Theme/Connection settings entries.
   No wiring into `config.Connection` or the connection editor yet — this
   slice only *displays* what's discoverable; using a profile to build an
   actual connection is future work.

## Open question — doesn't block this slice

**Where does an AWS-sourced connection eventually fit the existing
model?** Is the long-term plan (a) a third `Connection.Backend` value
(`"aws"`) that maps to Jolokia or proxy underneath once a broker's
endpoint is known, or (b) AWS profiles are purely a *discovery aid* that
helps fill in a regular jolokia/proxy connection's fields (broker URL
found via the Amazon MQ API, credentials resolved via the chosen profile)
without being a new backend type itself? Worth a rough answer before the
*next* slice locks in a shape that doesn't fit, but doesn't need
resolving now.

## Scope

### In scope

- `tui/internal/awsprofile/`: `List() ([]Profile, error)` returning
  `{Name, Region, AuthType}` per discovered profile (`AuthType` one of
  static-keys / SSO / assume-role / credential-process / unknown),
  deduplicated across `config` and `credentials` files, via
  `aws-sdk-go-v2/config`'s shared-config loading.
- Unit tests using temp config/credentials files (`t.Setenv` for the
  `AWS_CONFIG_FILE`/`AWS_SHARED_CREDENTIALS_FILE` overrides — no
  dependency on this machine's real `~/.aws` in tests).
- Graceful handling of the common/expected case: no `~/.aws` directory at
  all (not everyone has the AWS CLI configured) → empty list, not an
  error.
- **UI**: a new read-only "AWS Profiles" entry in the Settings list
  (alongside "Theme" and "Connection"), opening an overlay listing
  discovered profiles — name, region, auth type. `r` refreshes (re-reads
  the files, in case the user edited them while cloudtui is running),
  `Esc` closes. No select/activate/edit actions — this view only informs;
  it does nothing to `config.yaml` or the active connection.

### Out of scope (this slice)

- Any actual AWS API calls (STS, SSO, Amazon MQ) — resolving credentials,
  listing brokers, anything network-touching.
- Any change to `config.Connection` or the connection editor.
- Using a selected profile for anything (no "activate", no pre-filling a
  new connection from it).
- SSO token cache reading/refresh.
- Windows path handling beyond what the AWS SDK already does (it resolves
  home-directory paths correctly cross-platform on its own).

## Definition of done

1. `go build ./...`, `go vet ./...`, `go test ./...` pass in `tui/`.
2. Unit tests cover: static-key profiles, SSO profiles, assume-role
   profiles (`source_profile` chains), a profile present in only one of
   the two files, and the no-`~/.aws`-directory case.
3. Manual verification against this machine's real `~/.aws` files (if
   any exist) or a temp fixture, per the `verify-live` skill's spirit —
   confirm the real output looks sane before calling it done.
4. The Settings → "AWS Profiles" overlay renders the same data live in a
   running instance, verified via tmux per `verify-live`.
