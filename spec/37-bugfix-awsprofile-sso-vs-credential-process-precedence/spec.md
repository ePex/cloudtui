# Spec — Bugfix 37: awsprofile.classify() SSO-vs-credential_process precedence

Date: 2026-08-10

## Background

Discovered during FE 36 (spec/36-fe-aws-sso-reauth) manual verification.
`aws-sso-util`'s "populate profiles" mode writes profiles with *both*
native `sso_*` fields and a `credential_process = aws-sso-util
credential-process --profile <name>` fallback line, for compatibility
with tools that predate native SSO config support — e.g. this real
profile from this machine's `~/.aws/config`:

```
[profile mlf-preprod]
sso_start_url = https://mlf.awsapps.com/start/#/
sso_account_id = 393809552481
sso_role_name = AdministratorAccess_T12H
credential_process = aws-sso-util credential-process --profile mlf-preprod
sso_auto_populated = true
```

`awsprofile.classify()` (spec/28-fe-aws-profile-discovery) checks
`CredentialProcess` first, so it labels this profile `credential-process`.
But `aws-sdk-go-v2/config`'s actual runtime resolution
(`resolveCredsFromProfile` in `resolve_credentials.go`) checks SSO
configuration *before* falling back to `credential_process` — confirmed
live against this profile: with its SSO cache deleted, the real error
returned by an SDK call is genuinely `*ssocreds.InvalidTokenError`
(`errors.As` matches), proving the SDK used the native SSO provider, not
the credential_process command, despite both being present.

## Problem

1. The `:ap` AWS Profiles screen mislabels any profile with this shape as
   `credential-process` when the SDK actually authenticates it via native
   SSO.
2. FE 36's `NeedsReauth` gates re-auth on `AuthType == AuthSSO`; the
   wrong label meant it never fired for exactly this profile shape,
   reproducing the "commands just fail" bug FE 36 was meant to fix.
3. `TestListMixedSSOAndCredentialProcessPrefersCredentialProcess`
   (FE 28) asserts the wrong outcome, based on the same incorrect
   assumption about SDK precedence stated in `classify()`'s own comment.

## Decision (confirmed)

Reorder `classify()`'s switch so the SSO check comes before the
`credential_process` check, matching `resolveCredsFromProfile`'s real
order. `RoleARN` and static-keys positions are unchanged — no evidence
either is similarly wrong, and this stays a minimal, targeted fix rather
than a full precedence rewrite.

## Scope

- `tui/internal/awsprofile/list.go`: reorder `classify()`.
- `tui/internal/awsprofile/list_test.go`: flip
  `TestListMixedSSOAndCredentialProcessPrefersCredentialProcess` to
  assert SSO wins (rename accordingly), update its comment.
- No change needed to FE 36's `awsauth`/`app` code — once classification
  is correct, the existing `NeedsReauth`/`Login` (native `aws sso login`)
  already handle this profile shape correctly.

## Out of scope

- Reconciling `classify()`'s remaining categories (`RoleARN`,
  static-keys) against the SDK's full precedence order — no evidence of
  a mismatch there, not touching without cause.
