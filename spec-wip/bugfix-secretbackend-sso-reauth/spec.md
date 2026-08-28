# SSO re-auth for AWS-Secrets-Manager-backed connections

Date: 2026-08-28

## What

A proxy (or Jolokia) connection whose password comes from AWS Secrets
Manager (`passwordSecret`, resolved by `secretbackend.SecretResolver`)
currently just logs an error and shows the generic error row when the
active AWS profile's SSO session has expired. Every other AWS-backed
view (SSM Parameters, Secrets Manager, CloudWatch Logs, CodePipeline)
already handles this via `awsauth.WithReauth`: show a status message,
open the browser to `aws sso login`, retry once. This fix wires the
exact same mechanism into `secretbackend.SecretResolver.Resolve` — the
one place that actually calls out to AWS Secrets Manager — so it applies
automatically to every queue/message operation on a secret-backed
connection (list queues, browse/send/delete/move messages, purge), not
just one call site.

## Why

Reported directly: using a proxy connection with an AWS-secret password,
once the SSO session expires, produces only a logged error and a dead
end — the user has to know to go run `aws sso login` themselves (or use
`:ap` to reselect the profile, which doesn't itself trigger a fresh
login) before anything works again. Every other part of the app that
depends on the same SSO session already recovers from this
automatically and visibly (spec/36-fe-aws-sso-reauth). The
`secretbackend` package was the one AWS-touching code path that predates
that mechanism and was never wired into it.

## Scope

- `secretbackend.SecretResolver` gains `authTypeFor` and `login`
  function fields (injected at construction, same pattern as the
  existing `reveal` field) plus an `onReauth func()` callback, and wraps
  its `reveal` call with `awsauth.WithReauth`.
- `secretbackend.NewSecretResolver`'s signature changes to accept these
  three new parameters. `internal/app`'s one production call site is
  updated to pass `a.AWSAuthTypeFor`, `a.AWSSSOLogin`, and an `onReauth`
  closure that shows a status message via the bottom status bar
  (`host.SetStatus`) — not a per-view table row, since
  `SecretResolver`/`Backend` are shared across every view that might
  call into a secret-backed `queue.Backend` (QueuesView, MessagesView,
  purge/move-all, send-message), unlike the existing `WithReauth` call
  sites which are each owned by one specific view's table.
- Existing `secretbackend` package tests updated for the new
  constructor signature (non-SSO stub `authTypeFor`, so their behavior
  is completely unchanged — `NeedsReauth` short-circuits false and
  `login`/`onReauth` are never invoked).
- New test(s) proving the actual re-auth path: an SSO-shaped error from
  `reveal` triggers `onReauth`, then `login`, then a successful retry
  (mirroring the "reauth succeeds" and "reauth fails" cases the
  `awsauth` package itself already covers generically — this is about
  proving `SecretResolver` wires `WithReauth` correctly, not
  re-testing `WithReauth` itself).

## Out of scope

- Any change to `awsauth.WithReauth`, `NeedsReauth`, or `Login`
  themselves — reused exactly as they already exist.
- Non-SSO auth types (static keys, assume-role, credential-process) —
  `WithReauth`/`NeedsReauth` already only act on `AuthSSO`, unchanged
  here.
- Retrying the *queue.Backend* call itself beyond what `secretbackend.Backend`
  already does (its existing refresh-and-retry-once-on-read logic,
  never-retry-on-write) — this fix is purely about the secret-resolution
  step underneath that, which is what actually fails when SSO expires.
