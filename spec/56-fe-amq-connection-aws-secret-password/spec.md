# Spec — FE 56: AWS-Secrets-Manager-backed connection passwords

Date: 2026-08-16

## Background

Connections (FE 22) store the broker password as plain text in
`config.yaml`, or pick it up from `MQPROXY_CLIENT_PASSWORD` if the config
field is empty. Separately, Settings → AWS Profiles (FE 28/`awsprofile`) lets
the user pick a single active AWS CLI profile (`cfg.ActiveAWSProfile`,
independent of connections), and `internal/awssecrets` already knows how to
fetch a decrypted secret value from AWS Secrets Manager
(`awssecrets.Reveal(ctx, profile, name)`), used today by the Secrets Manager
browser view.

## Problem

Storing broker passwords in plain text in `config.yaml` isn't acceptable for
some environments. The password should be able to live in AWS Secrets
Manager instead, resolved on demand rather than kept on disk.

## Solution

Add an optional `passwordSecret` field to a connection's backend config
(`queue` or `proxy`). When set, it names a Secrets Manager secret whose
current value is used as the password instead of the plain `password` field.
The secret is resolved using the globally selected AWS profile (Settings →
AWS Profiles / `cfg.ActiveAWSProfile`) — the same profile already shown in
the info panel — not a new per-connection profile field.

### Config shape

```yaml
connections:
  - name: aws-staging
    backend: proxy
    proxy:
      url: http://localhost:8080
      username: cloudtui
      passwordSecret: /cloudtui/aws-staging/mq-password   # resolved via Secrets Manager
      # password: ""                                       # ignored when passwordSecret is set
```

`passwordSecret` takes precedence over both `password` and the
`MQPROXY_CLIENT_PASSWORD` env-var injection when non-empty. The secret's
value is used verbatim as the password — no JSON key extraction (see Out of
scope). In practice the connection editor (see "In scope" below) makes the
two mutually exclusive by construction — a saved connection only ever has
one of `password`/`passwordSecret` set, never both.

### Resolution, caching, and failure handling

- Resolution is lazy: it happens inside whichever `queue.Backend` call first
  needs the password (not as a separate step at activation time). Every
  `queue.Backend` call in this codebase already runs off the tview goroutine
  at its call site (`go func() { ...; QueueUpdateDraw(...) }()` — e.g. the
  queue list's `load()`), so resolving there — rather than adding a bespoke
  async step wired into startup/`switchConnection`/`saveConnEditor` — never
  blocks the UI either, for the same reason none of those existing calls do.
  A resolution failure (including "no AWS profile selected") surfaces
  through whichever view made the call, the same place any other backend
  error already shows up (e.g. an `Error: …` row in the queue table) — not a
  dedicated status-bar message.
- If no AWS profile is selected, resolution fails immediately with a clear
  error ("no AWS profile selected — pick one in Settings → AWS Profiles")
  instead of attempting a call.
- Resolved values are cached in memory only, keyed by `(profile, secretName)`
  — never written to `config.yaml`, never persisted across process restarts.
  Reactivating the same connection (or switching back to it later in the same
  session) reuses the cached value instead of calling AWS again.
- **Failure and refetch** — this is the one place this spec is deliberately
  narrower than "retry any failed call":
  - Read-only backend calls (list queues, browse/detail a message) that fail
    while the active connection uses a cached secret: invalidate the cached
    value, refetch the secret, rebuild the backend client, and transparently
    retry the same call once. If the retry also fails, surface the error as
    usual.
  - Mutating calls (delete, move, send, purge) are **never** auto-retried —
    silently replaying a delete/move after a transient failure risks
    double-applying it if the first attempt actually succeeded server-side
    but returned an error. On failure, the cached secret is still invalidated
    (so the *next* call, of either kind, fetches fresh), but the mutating
    call itself just fails and is reported to the user as it is today.
  - This means a rotated secret is discovered either on the next read, or
    immediately if a mutating call happens to fail for some other reason
    first — not proactively.

## Scope

### In scope

- `PasswordSecret string` field (yaml `passwordSecret,omitempty`) on both
  `config.QueueConfig` and `config.ProxyConfig`.
- In-memory resolution cache keyed by `(profile, secretName)`.
- Lazy resolution, piggybacking on existing call-site async dispatch (see
  above) rather than adding new goroutine/`QueueUpdateDraw` wiring at the
  three points a connection can become active.
- Invalidate-and-retry-once behavior for read-only backend calls; invalidate-
  only for mutating calls, as described above.
- Connection editor: a "Password Source" dropdown (`Plain` / `AWS Secret`)
  that dynamically swaps a single field below it between Password and
  Password Secret (AWS) — only one is ever visible or saved at a time, for
  both jolokia and proxy backends. Not two always-visible fields: showing
  both invited exactly the "which one actually wins?" confusion the
  dropdown exists to remove.
- `config.example.yaml` documents the new field with a comment.

### Out of scope

- Per-connection AWS profile (uses the single global `ActiveAWSProfile`).
- JSON/structured secrets (e.g. `{"username": "...", "password": "..."}`) —
  only a plain-string secret value is supported. A JSON-valued secret is used
  verbatim (including braces), which will simply fail authentication.
- Sourcing the username, broker name, or URL from Secrets Manager — password
  only.
- A manual "reconnect now" / "refresh secret" UI action — refresh only
  happens as a side effect of activation or a failed read call.
- Editing or rotating the secret's value in AWS from within cloudtui.
- `devtool` CLI support for setting `passwordSecret`.

## Files touched

| File | Change |
|---|---|
| `tui/internal/config/config.go` | `PasswordSecret` on `QueueConfig`/`ProxyConfig` |
| `tui/internal/config/config_test.go` | round-trip / precedence tests |
| `tui/internal/app/*` (new: connection secret resolution) | in-memory cache, async resolve-and-swap, invalidate/retry wiring |
| `tui/internal/app/connections.go` | editor form: Password Source dropdown dynamically swapping a Password/Password Secret field |
| `tui/internal/queue/*` | backend wrapper (or equivalent) distinguishing read vs. mutating calls for retry purposes — exact mechanism decided in `plan.md` |
| `tui/config.example.yaml` | document `passwordSecret` |

## Definition of done

1. `go build ./...` and `go test ./...` pass in `tui/`.
2. A connection with `passwordSecret` set resolves its password via
   `awssecrets.Reveal` using the active AWS profile and connects successfully
   (verified live against a real secret — see `verify-live` skill).
3. A second activation of the same connection in the same session does not
   issue a second `GetSecretValue` call (cache hit) — covered by a unit test
   against a fake/counting resolver, not a real AWS call.
4. A failing read-only call triggers invalidate + refetch + one transparent
   retry; a still-failing retry surfaces the error normally.
5. A failing mutating call invalidates the cache but does not retry itself.
6. No AWS profile selected → clear, immediate error, no attempted API call.
7. `config.example.yaml` documents the field.
