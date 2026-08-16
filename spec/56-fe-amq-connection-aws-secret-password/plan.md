# Plan — FE 56: AWS-Secrets-Manager-backed connection passwords

> **Update:** after the first implementation pass shipped both a Password
> and a Password Secret (AWS) field always visible together, that read as
> "which one actually wins?" in practice. Revised to a "Password Source"
> dropdown (`Plain` / `AWS Secret`) that dynamically swaps a single field
> between the two — see the `app.go`/`connections.go` sections below. The
> saved connection now only ever has one of `password`/`passwordSecret` set,
> by construction, not just by documented precedence.

## Approach

Add `PasswordSecret` to `QueueConfig`/`ProxyConfig`. When set, a new
`secretBackend` wrapper — implementing `queue.Backend` itself — sits between
the UI and the real `jolokia.Client`/`proxy.Client`, resolving the password
from Secrets Manager (via the existing `a.revealSecret` hook, already used
by the Secrets Manager browser view) and caching it in memory. Every other
call site keeps using `a.backend`/`qv.backend` exactly as today — the
wrapper is invisible to `queues.go`, `messages.go`, `message_detail.go`, and
the settings/connection UI, so none of them change.

### Key technical decision: piggyback on the existing async-call pattern instead of adding a bespoke one

spec.md describes resolution as an explicit step wired into activation
(startup / `switchConnection` / `saveConnEditor`), off the tview goroutine,
with a transient status-bar message while in flight.

Having read the actual call sites, every single `queue.Backend` method call
in this codebase — reads and writes alike — is *already* dispatched as
`go func() { ...; app.tv.QueueUpdateDraw(...) }()` by its caller (e.g.
`queuesView.load()`, `messagesView.deleteMarked()`). That means a
`secretBackend` method is *never* called directly on the tview goroutine —
whichever view invoked it has already hopped off it. So `secretBackend` can
simply resolve (and block on the network) synchronously inside its own
methods, with zero new goroutine/`QueueUpdateDraw` wiring at the three
activation sites, and the UI is still never blocked — for the exact same
reason none of the existing read/write calls block it today.

Concretely: `secretBackend.current(ctx)` resolves (cache-or-fetch) lazily on
first use rather than eagerly at activation. Practically this still
resolves "on activation" for the common case, because activating a
connection immediately triggers `queuesView.load()` → `List()`, which is the
first real call. The difference from spec.md's literal wording:

- No separate "Resolving secret…" status-bar message — the existing
  queue-list loading state covers it (same as any other slow first load).
- A resolution failure (including "no AWS profile selected") surfaces
  through the existing `queuesView.showError()` path (an `Error: …` row in
  the queue table), not a status-bar message. This is the same place every
  other backend error already surfaces, jolokia and proxy alike.
- The "no profile selected → fail immediately without attempting a call"
  requirement is still met exactly: `resolvePassword` checks for an empty
  profile before ever calling `a.revealSecret`.

Net effect: identical behavior to what spec.md describes, fewer moving
parts, no new call sites to keep in sync with the three places a connection
can become active. Flagging this explicitly since it's a real implementation
simplification, not just a rename — happy to go the literal route (add
explicit resolve-and-swap steps with their own status message at each of
the three activation sites) if you'd rather have the more visible "resolving
secret" feedback.

## Files touched

### `tui/internal/config/config.go`

- `PasswordSecret string \`yaml:"passwordSecret,omitempty"\`` added to both
  `QueueConfig` and `ProxyConfig`. No other change — `Load()`'s
  `MQPROXY_CLIENT_PASSWORD` injection is untouched; it's harmless to still
  fill `Password` even when `PasswordSecret` is set, because `secretBackend`
  never reads `Password` for a secret-backed connection (see below) — it
  always overwrites it with the resolved value. Precedence falls out of the
  architecture rather than needing explicit logic in `config.go`.

### `tui/internal/app/connectionsecrets.go` (new)

- `secretCache`: an in-memory `map[string]string` keyed by `profile + "\x00"
  + secretName`, guarded by a mutex. `get`/`set`/`invalidate`.
- `(a *App) resolvePassword(ctx, profile, secretName) (string, error)`:
  returns an immediate error if `profile == ""` (no AWS call attempted);
  otherwise cache hit, or calls `a.revealSecret` (the existing hook), errors
  if the secret is binary-valued (out of scope per spec.md), caches and
  returns the string value on success.
- `passwordSecretName(conn config.Connection) string` / `connWithPassword(conn
  config.Connection, password string) config.Connection`: small helpers
  mirroring the existing `conn.Backend == "proxy"` switch already used in
  `newBackendForConn` and `connections.go`.
- `buildBackend(conn config.Connection) queue.Backend`: the old body of
  `newBackendForConn` (jolokia vs. proxy client construction), unchanged,
  just renamed and no longer exported outside the package boundary it
  already lived in.
- `newBackendForConn(a *App, conn config.Connection) queue.Backend`: if
  `passwordSecretName(conn) == ""`, calls `buildBackend(conn)` exactly as
  today (zero behavior change for connections without a secret). Otherwise
  returns a `*secretBackend`.
- `secretBackend`: holds `app *App`, the secret-bearing `conn
  config.Connection`, `secretName string`, a mutex, and a lazily-built
  `inner queue.Backend`, plus a `build func(config.Connection) queue.Backend`
  field (defaults to `buildBackend`; overridable in tests — same pattern as
  `a.revealSecret` being a swappable field). Implements all 9
  `queue.Backend` methods:
  - `current(ctx) (queue.Backend, error)`: returns `inner` if already built;
    otherwise resolves the password via `app.resolvePassword`, builds
    `inner` via `build(connWithPassword(conn, pw))`, and caches it in the
    struct (not re-resolved again until `refresh()` clears it).
  - `refresh()`: invalidates the cache entry for `(activeAWSProfile,
    secretName)` and nils out `inner`, forcing the next `current()` call to
    re-resolve and rebuild.
  - `List` / `BrowseMessages` (read): `current` → call → on error, `refresh()`
    then `current` again → retry the call once. A `current()` failure after
    refresh (e.g. still no profile) surfaces the *original* call error, not
    the refresh error.
  - `PurgeQueue` / `RemoveMessage` / `MoveMessage` / `MoveAllMessages` /
    `SendMessage` / `DeleteMessages` / `MoveMessages` (write): `current` →
    call → on error, `refresh()` for next time, but return the original
    error without retrying the call itself.

### `tui/internal/app/app.go`

- The three `newBackendForConn(conn)` call sites (init at ~line 233,
  `switchConnection` at ~line 1165) become `newBackendForConn(a, conn)`.
- `New()`: initialize `a.secretCache` before the init-time backend
  construction (before line 233).
- Connection editor form: replace the always-visible Password Secret field
  with an `AddDropDown("Password Source", []string{"Plain", "AWS Secret"},
  0, nil)` at item 5, followed by `AddPasswordField("Password", ...)` as
  item 6 (the default). The dropdown's `selected` callback isn't passed to
  `AddDropDown` itself — that would fire during construction, before item 6
  exists — it's wired via `SetSelectedFunc` right after the whole chain is
  built, calling `setConnEditorPasswordField(sourceIdx)` (new,
  `connections.go`), which does `RemoveFormItem(6)` then adds either
  `AddPasswordField("Password", ...)` or `AddInputField("Password Secret
  (AWS)", ...)` back — always landing at index 6 again since it's the last
  form item (`AddButton` items aren't counted by `GetFormItem`, so Save/
  Cancel are unaffected by the swap). Item count stays 7, so the overlay
  height is unchanged from the first pass (20).

### `tui/internal/app/connections.go`

- `newBackendForConn(conn)` call in `saveConnEditor` → `newBackendForConn(a,
  conn)`.
- New `setConnEditorPasswordField(sourceIdx int)` (see above).
- `showConnEditor`: compute `sourceIdx` from whether
  `conn.Queue.PasswordSecret`/`conn.Proxy.PasswordSecret` (per backend) is
  set, call `GetFormItem(5).(*tview.DropDown).SetCurrentOption(sourceIdx)` —
  which fires the callback above, swapping item 6 to the right field type —
  then set item 6's text to whichever of password/passwordSecret applies.
- `saveConnEditor`: read item 5's current option to get `sourceIdx`, then
  read item 6 into `password` *or* `passwordSecret` (never both — the other
  stays `""`) depending on `sourceIdx`; set whichever field on the saved
  `config.Connection`.

### `tui/config.example.yaml`

- Document `passwordSecret` with a comment on the example connection block,
  noting it takes precedence over `password`/`MQPROXY_CLIENT_PASSWORD` and
  requires an AWS profile selected in Settings.

## Testing

- `connectionsecrets_test.go` (new): unit tests against `secretBackend`
  directly, with `app.revealSecret` and `secretBackend.build` swapped for
  fakes — no real AWS or HTTP calls:
  - Cache hit: activating twice / calling `current()` twice issues exactly
    one `revealSecret` call (definition-of-done item 3).
  - No profile selected: `resolvePassword` returns an error without calling
    `revealSecret` at all.
  - Read retry: a fake inner backend whose `List` fails once then succeeds
    → `secretBackend.List` invalidates, re-resolves, and returns the
    second, successful result.
  - Read retry exhausted: fake inner backend that always fails → error
    surfaces after exactly one retry (not an infinite loop).
  - Write no-retry: a fake inner backend whose `RemoveMessage` fails →
    `secretBackend.RemoveMessage` returns the error without a second call to
    the fake backend, but the cache is invalidated (checked via a following
    `current()` call re-triggering `revealSecret`).
- `config_test.go`: round-trip test that `passwordSecret` survives
  `Save`/`Load` on both `queue` and `proxy` blocks.
- `connections_test.go` (or wherever the editor's save/prefill logic is
  covered, if it is today): confirm the new field round-trips through
  show→save.
- Manual (`verify-live` skill, per `tui/CLAUDE.md` — this touches connection
  behavior): create a connection with `passwordSecret` pointed at a real
  test secret, with an AWS profile selected, and confirm it authenticates
  and lists queues; confirm a wrong/nonexistent secret name surfaces a
  clear `Error: …` row instead of a silent failure.
