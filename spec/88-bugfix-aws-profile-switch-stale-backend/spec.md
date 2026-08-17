# Spec — Bugfix 88: switching AWS profile doesn't refresh a Secrets-Manager-backed connection

Date: 2026-08-17

## Background

`connectionsecrets.go`'s `secretBackend` wraps a `queue.Backend` whose
password comes from AWS Secrets Manager (see
`spec/56-fe-amq-connection-aws-secret-password`). It resolves and
caches the password lazily, on first use, via `current()`; a failed
call invalidates the cache and rebuilds via `refresh()`.

Surfaced while auditing phase 5's backlog item
(`spec/64-cr-app-package-split`'s optional `secretBackend` relocation)
— not the relocation itself, a correctness gap found while reading the
file fresh.

## Problem

`secretBackend.current()`/`refresh()` read `b.app.cfg.ActiveAWSProfile`
directly, at call time. Two consequences:

**1. Switching AWS profile has no visible effect on an
already-resolved connection.** `SetActiveAWSProfile` (`host.go:130`)
only updates `a.cfg.ActiveAWSProfile`, the info panel, and the
Settings list — unlike `switchConnection`/`SaveConnection`, it never
rebuilds `a.backend`. A `secretBackend` that already cached its
`inner` backend (i.e. any connection the user has used at least once
since it became active) keeps using the *old* profile's resolved
password indefinitely — not just until the next poll or redraw, but
until some unrelated API call happens to fail and trigger `refresh()`.
From the user's point of view: switch AWS profile in Settings, go back
to a connection using a Secrets-Manager password, and messages/queues
keep resolving against the previous profile's secret with no error
and no indication anything is stale.

**2. Even `refresh()`'s eventual re-resolution reads `a.cfg` from the
wrong goroutine.** Every `queue.Backend` method (`List`,
`BrowseMessages`, ...) is invoked from a background goroutine — every
view's `load()` spawns one, per this codebase's established
single-writer-via-QueueUpdateDraw discipline. `current()`/`refresh()`
reading `b.app.cfg.ActiveAWSProfile` there races the main goroutine's
own writes to `a.cfg` (profile switch, connection switch) with no
synchronization — the same category of bug `spec/87`'s
`PipelineWatcher` was explicitly designed to avoid, by capturing
`profile` once on the main goroutine at watch-start time rather than
re-reading `Config()` from its poll goroutine. `secretBackend` predates
that discipline and never got the equivalent treatment.

Both stem from the same root cause: the active AWS profile is read
lazily, from the wrong place, instead of being captured once when the
backend is (re)built.

## Solution

**Capture `profile` once, at construction, on the main goroutine** —
`newBackendForConn(a, conn)` already runs synchronously on the main
goroutine every time it's called (all 3 existing call sites are key
handlers / `ui.Host` methods). Read `a.cfg.ActiveAWSProfile` there and
store it on `secretBackend` as a fixed field; `current()`/`refresh()`
use that field instead of reaching back into `a.cfg`. This alone fixes
finding 2 (the race) and makes a `secretBackend`'s profile explicit and
immutable for its lifetime — consistent with how it already treats
`conn` (also captured once, not re-read from `a.cfg.Connections`).

**Rebuild `a.backend` in `SetActiveAWSProfile`**, the same 3-line
pattern `switchConnection`/`SaveConnection` already use
(`a.backend = newBackendForConn(a, a.cfg.ActiveConn())` +
`a.queuesV.SetBackend(a.backend)`) — this fixes finding 1. For a
connection with no `PasswordSecret`, this rebuilds a plain
jolokia/proxy client (cheap, no network call, no behavior change) —
same harmless-when-irrelevant shape the other 2 call sites already
have. Once every profile-affecting change goes through a fresh
`newBackendForConn` call, "how stale can a `secretBackend`'s profile
be" reduces to "as stale as the connection it's paired with," which is
already the accepted, existing behavior for connection switches.

## Scope

### In scope

- `connectionsecrets.go`: `secretBackend` gains a `profile string`
  field; `newBackendForConn` captures `a.cfg.ActiveAWSProfile` into it
  at construction; `current()`/`refresh()` read `b.profile` instead of
  `b.app.cfg.ActiveAWSProfile`.
- `host.go`'s `SetActiveAWSProfile`: rebuilds `a.backend` and calls
  `a.queuesV.SetBackend(a.backend)`, matching
  `switchConnection`/`SaveConnection`'s existing shape.
- Unit tests: switching AWS profile rebuilds a
  `PasswordSecret`-backed connection's backend with the new profile;
  an existing `secretBackend`'s resolution is unaffected by `a.cfg`
  mutating without going through `SetActiveAWSProfile` (regression
  coverage for finding 2 — proves the field, not live `a.cfg`, is what
  `current()`/`refresh()` read).
- `gofmt`/`go vet`/`go build`/`go test` clean.

### Out of scope

- **The phase 5 relocation** (`secretBackend` moving out of
  `internal/app`) — a separate, still-optional decision per spec/64;
  this bugfix applies to the file wherever it ends up living, and
  doesn't block or require that move.
- **Any change to the retry/refresh-on-failure behavior itself** —
  `current()`/`refresh()`'s read/write retry asymmetry (documented in
  `secretBackend`'s doc comment) is unrelated to where `profile` comes
  from and isn't touched.
- **Proactively invalidating the AWS-side secret cache
  (`secretCache`) on a profile switch** — not necessary: `secretCache`
  is already keyed by `(profile, secretName)`, so a new profile
  naturally misses the cache and resolves fresh; the old profile's
  cached value simply becomes unreferenced, not stale-and-served.

## Definition of done

1. `go build ./...` and `go test ./...` pass in `tui/`.
2. Switching AWS profile via Settings → AWS Profiles immediately
   causes a `PasswordSecret`-backed active connection to re-resolve
   against the new profile on its next use, with no dependency on a
   prior call failing first.
3. `secretBackend.current()`/`refresh()` no longer read `a.cfg` at
   all — confirmed by the regression test (finding 2).
4. `gofmt -l` clean, `go vet ./...` clean.
5. No behavior change for connections without a `PasswordSecret`.
