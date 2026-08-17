# Spec — CR 89: move `secretBackend` out of `internal/app` into `internal/queue/secretbackend`

Date: 2026-08-17

## Background

Phase 5 (`spec/64`), the last item left after phases 1-4's app-package
split (CR 82-87) and bugfix 88's correctness fix: `connectionsecrets.go`'s
`secretBackend`. Spec/64 flagged it as data-layer-ish, not fitting
"view" or "dialog," and left its destination as "TBD... likely belongs
nearer `internal/queue` or stays in `internal/app`" — explicitly
optional, lowest priority of everything in the split.

Re-reading the file fresh, post-bugfix-88 (which added a captured
`profile` field and is the current shape of the file):

**1. Nothing here is view/dialog-shaped at all** — unlike every file
in phases 1-4, there's no `ui.View`/`ui.Themeable` to implement, no
UI surface whatsoever. It's pure backend-construction plumbing:
`secretCache` (an in-memory map), `resolvePassword` (cache-or-reveal
logic), `secretBackend` (a `queue.Backend` decorator that resolves its
password lazily), and 3 pure helper functions
(`passwordSecretName`/`connWithPassword`/`buildBackend`).

**2. What's left touching `*App` is narrow and mostly incidental.**
Grepping every `a.X`/`b.app.X` reference in the file:

| Reference | What it actually needs |
|---|---|
| `a.revealSecret` (via `resolvePassword`) | One func value, already `awssecrets.Reveal` — also independently used by `viewhost.go`'s `RevealSecret` (the Secrets-Manager browser), so this stays a shared `*App` field, just also passed to whatever owns `resolvePassword` post-move |
| `a.secretCache` | Fully self-contained already — zero `*App` dependency, just needs a fixed home |
| `b.profile` (post-bugfix-88) | Already just a captured string, no `*App` reach-in left at all |

Nothing here needs `ui.ViewHost`'s wide surface, or even `ui.Host`'s —
this file was never a `ui.View`/dialog-shaped consumer of `*App` to
begin with, just a data-layer helper that happened to be written
inside `internal/app` because that's where `queue.Backend`
construction already lived.

**3. Only 2 symbols are referenced from outside the file** (grep
confirms): `newBackendForConn` (4 call sites: `app.go:202`,
`app.go:563`, `host.go:108`, `host.go:135`) and the `secretCache`
field (declared + constructed once in `app.go`, never read directly
elsewhere). `resolvePassword`/`passwordSecretName`/`connWithPassword`/
`buildBackend`/`secretBackend` itself are all used only within
`connectionsecrets.go`.

## Problem

None of `secretCache`/`resolvePassword`/`secretBackend`/
`buildBackend` need `internal/app`'s concrete `*App` type or its
`ui.ViewHost`/`ui.Host` interfaces — they need exactly one func value
(`RevealSecret`-shaped) and `internal/queue`'s `Backend` interface
plus its 2 concrete implementations (`jolokia`, `proxy`) to build the
inner backend. Leaving this in `internal/app` ties genuinely
UI-independent, reusable backend-construction logic to the
application-shell package for no structural reason — exactly what
spec/64 flagged, now confirmed by a fresh read rather than a guess.

## Solution

**New package `internal/queue/secretbackend`**, a sibling of
`internal/queue/jolokia`/`internal/queue/proxy` (both already
implementations `secretbackend.New` needs to build the inner backend
from) rather than a new top-level package — matches spec/64's own
guess and keeps every `queue.Backend`-shaped piece of this codebase
under `internal/queue`.

1. **`SecretResolver`** absorbs `secretCache` + `resolvePassword`'s
   logic into one exported type, constructed with the one func value
   it needs:
   ```go
   type SecretResolver struct {
       cache  *secretCache // unexported, absorbed unchanged
       reveal func(ctx context.Context, profile, name string) (string, bool, error)
   }
   func NewSecretResolver(reveal func(ctx context.Context, profile, name string) (string, bool, error)) *SecretResolver
   func (r *SecretResolver) Resolve(ctx context.Context, profile, secretName string) (string, error)
   func (r *SecretResolver) Invalidate(profile, secretName string)
   ```
2. **`Backend`** is `secretBackend` relocated, taking `*SecretResolver`
   instead of `*App`; all 7 mutating methods + `List`/`BrowseMessages`
   unchanged in logic (`b.app.resolvePassword(...)` →
   `b.resolver.Resolve(...)`, `b.app.secretCache.invalidate(...)` →
   `b.resolver.Invalidate(...)`).
3. **`New(resolver *SecretResolver, profile string, conn
   config.Connection) queue.Backend`** replaces `newBackendForConn`,
   dropping the `*App` parameter entirely — `passwordSecretName`/
   `connWithPassword`/`buildBackend` move in unchanged (already
   `*App`-free).
4. **`*App`** gains `secretResolver *secretbackend.SecretResolver`
   (replacing the `secretCache *secretCache` field), constructed once:
   `a.secretResolver = secretbackend.NewSecretResolver(a.revealSecret)`.
   Its 4 call sites become
   `secretbackend.New(a.secretResolver, a.cfg.ActiveAWSProfile, conn)`.
5. **`internal/app/connectionsecrets.go` is deleted entirely** — unlike
   every phase 1-4 CR, nothing here was ever part of `ui.ViewHost`/
   `ui.Host`, so there's no interface-conformance requirement forcing
   `*App` to keep a trampoline. Callers invoke `secretbackend.New(...)`
   directly.

## Scope

### In scope

- New `internal/queue/secretbackend` package: `SecretResolver`,
  `Backend` (unexported concrete struct backing `queue.Backend`,
  matching `jolokia`/`proxy`'s existing style of returning a concrete
  pointer from `New`), `New`, plus the 3 unchanged helper functions.
- `internal/app/connectionsecrets.go` deleted; its logic fully
  relocated, not duplicated.
- `app.go`: `secretCache` field → `secretResolver
  *secretbackend.SecretResolver`; construction line updated; 2
  `newBackendForConn` call sites updated.
- `host.go`: 2 `newBackendForConn` call sites updated.
- Existing `internal/app/connectionsecrets_test.go` (7 tests: 6
  original + 1 added by bugfix 88): all of them exercise
  `resolvePassword`/`secretBackend` behavior directly and move to
  `internal/queue/secretbackend`, adapted to construct a
  `SecretResolver` instead of a bare `*App`. Bugfix 88's other
  addition, `TestSetActiveAWSProfileRebuildsSecretBackedBackend` (in
  `host_test.go`, needs a real `*App`), stays in `internal/app`,
  adapted for the new field/call shape.
- `gofmt`/`go vet`/`go build`/`go test` clean; zero import cycle.

### Out of scope

- Any change to `secretBackend`'s read/write retry behavior, the
  cache's `(profile, secretName)` keying, or bugfix 88's
  captured-`profile` fix — all relocated verbatim, not revisited.
- Any change to `ui.ViewHost`/`ui.Host` — this file was never part of
  either interface; nothing there changes.
- Live verification — same reasoning as bugfix 88: this is pure
  backend-construction plumbing with no UI surface, fully covered by
  unit tests. (Existing manual coverage of the AWS-Secrets-Manager
  connection-password feature itself is `spec/56`'s, unaffected by
  where the code lives.)

## Definition of done

1. `internal/queue/secretbackend` holds `SecretResolver`/`Backend`/
   `New`; `internal/app/connectionsecrets.go` no longer exists.
2. `*App` has no `secretCache` field; `secretResolver
   *secretbackend.SecretResolver` replaces it, constructed once.
3. `go build`/`go test`/`go vet` clean, `gofmt -l` clean, zero import
   cycle.
4. All 8 existing tests pass, relocated/adapted per Scope.
5. No behavior change.
