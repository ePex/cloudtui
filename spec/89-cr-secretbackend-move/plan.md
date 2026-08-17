# Plan — CR 89: move `secretBackend` out of `internal/app` into `internal/queue/secretbackend`

## Approach

Unlike CR 82-87, there's no `ui.ViewHost`/`ui.Host` interface to keep
satisfying and no trampoline to leave behind — this file was never
part of either interface. Build the new package directly, verified in
isolation, then delete `internal/app/connectionsecrets.go` and
`connectionsecrets_test.go` outright once every caller is updated.

### 1. New package `internal/queue/secretbackend`

`secretCache` moves unchanged (still unexported — nothing outside
`SecretResolver` needs it):

```go
package secretbackend

type secretCache struct {
	mu     sync.Mutex
	values map[string]string
}

func newSecretCache() *secretCache { return &secretCache{values: make(map[string]string)} }
func secretCacheKey(profile, secretName string) string { return profile + "\x00" + secretName }
func (c *secretCache) get(profile, secretName string) (string, bool) { ... }
func (c *secretCache) set(profile, secretName, value string) { ... }
func (c *secretCache) invalidate(profile, secretName string) { ... }
```

`resolvePassword`'s logic becomes `SecretResolver.Resolve`, taking the
one func value it needs instead of `*App`:

```go
// SecretResolver resolves AWS-Secrets-Manager-backed connection
// passwords, caching resolved values in memory. See
// spec/56-fe-amq-connection-aws-secret-password.
type SecretResolver struct {
	cache  *secretCache
	reveal func(ctx context.Context, profile, name string) (string, bool, error)
}

// NewSecretResolver constructs a SecretResolver that resolves secrets
// via reveal (e.g. awssecrets.Reveal).
func NewSecretResolver(reveal func(ctx context.Context, profile, name string) (string, bool, error)) *SecretResolver {
	return &SecretResolver{cache: newSecretCache(), reveal: reveal}
}

// Resolve resolves secretName via profile, using the cache when
// possible. No reveal call is attempted when profile is empty — the
// caller (a connection with no AWS profile selected) gets an
// immediate, specific error instead of a doomed API call.
func (r *SecretResolver) Resolve(ctx context.Context, profile, secretName string) (string, error) {
	if profile == "" {
		return "", fmt.Errorf("no AWS profile selected — pick one in Settings -> AWS Profiles")
	}
	if v, ok := r.cache.get(profile, secretName); ok {
		return v, nil
	}
	value, isBinary, err := r.reveal(ctx, profile, secretName)
	if err != nil {
		return "", fmt.Errorf("resolving password secret %q: %w", secretName, err)
	}
	if isBinary {
		return "", fmt.Errorf("password secret %q has a binary value, expected a string", secretName)
	}
	r.cache.set(profile, secretName, value)
	return value, nil
}

// Invalidate forgets a previously-resolved value, forcing the next
// Resolve call for (profile, secretName) to re-resolve.
func (r *SecretResolver) Invalidate(profile, secretName string) {
	r.cache.invalidate(profile, secretName)
}
```

The 3 pure helpers move unchanged (already `*App`-free):
`passwordSecretName`, `connWithPassword`, `buildBackend` (this last one
now importing `internal/queue/jolokia`/`internal/queue/proxy` from its
new home instead of `internal/app` — same 2 imports, different
package).

`newBackendForConn` becomes `New`, dropping the `*App` parameter for
an already-resolved `*SecretResolver` + explicit `profile`:

```go
// New constructs the queue.Backend for conn. Connections without a
// passwordSecret behave exactly as buildBackend directly, no wrapping.
// A passwordSecret-bearing connection gets a *Backend that resolves
// the password from AWS Secrets Manager on first use and transparently
// recovers from a stale/rotated secret — see Backend.
func New(resolver *SecretResolver, profile string, conn config.Connection) queue.Backend {
	secretName := passwordSecretName(conn)
	if secretName == "" {
		return buildBackend(conn)
	}
	return &Backend{resolver: resolver, conn: conn, secretName: secretName, profile: profile, build: buildBackend}
}
```

`secretBackend` becomes `Backend`, `b.app.resolvePassword(...)` →
`b.resolver.Resolve(...)`, `b.app.secretCache.invalidate(...)` →
`b.resolver.Invalidate(...)`; every mutating/read method's retry logic
is otherwise byte-for-byte:

```go
// Backend wraps a queue.Backend whose password comes from AWS Secrets
// Manager. [doc comment carried over from secretBackend, updated to
// reference SecretResolver instead of *App — retry/refresh semantics
// unchanged, see spec.md]
type Backend struct {
	resolver   *SecretResolver
	conn       config.Connection
	secretName string
	// profile is captured once, by the caller of New — never re-read
	// from any live config here, since every queue.Backend method
	// (List, BrowseMessages, ...) runs on a background goroutine that
	// would otherwise race the caller's own config writes. See
	// spec/88, which introduced this same discipline in the
	// pre-move secretBackend.
	profile string
	build   func(config.Connection) queue.Backend

	mu    sync.Mutex
	inner queue.Backend
}

// Profile returns the AWS profile this Backend was constructed for.
// Exported for internal/app's own tests, which need to confirm
// SetActiveAWSProfile actually rebuilds the backend — mirrors CR 84's
// Table()/CR 85's List() accessors added for the identical reason.
func (b *Backend) Profile() string { return b.profile }

func (b *Backend) current(ctx context.Context) (queue.Backend, error) { ... } // b.resolver.Resolve(ctx, b.profile, b.secretName)
func (b *Backend) refresh() { ... }                                          // b.resolver.Invalidate(b.profile, b.secretName)
func (b *Backend) List(ctx context.Context) ([]queue.Summary, error) { ... }  // unchanged
// ...BrowseMessages/PurgeQueue/RemoveMessage/MoveMessage/MoveAllMessages/SendMessage/DeleteMessages/MoveMessages: unchanged
```

### 2. `internal/app` changes

**`app.go`**: struct field `secretCache *secretCache` →
`secretResolver *secretbackend.SecretResolver`; construction
(right after `a.revealSecret = awssecrets.Reveal`, so the value it
needs already exists):

```go
a.revealSecret = awssecrets.Reveal
// ...
a.secretResolver = secretbackend.NewSecretResolver(a.revealSecret)
```

The 2 `app.go` call sites:

```go
// New(), was: a.backend = newBackendForConn(a, cfg.ActiveConn())
a.backend = secretbackend.New(a.secretResolver, cfg.ActiveAWSProfile, cfg.ActiveConn())

// switchConnection(), was: a.backend = newBackendForConn(a, conn)
a.backend = secretbackend.New(a.secretResolver, a.cfg.ActiveAWSProfile, conn)
```

**`host.go`**: the 2 call sites (`SaveConnection`,
`SetActiveAWSProfile`) become:

```go
a.backend = secretbackend.New(a.secretResolver, a.cfg.ActiveAWSProfile, conn)             // SaveConnection
a.backend = secretbackend.New(a.secretResolver, a.cfg.ActiveAWSProfile, a.cfg.ActiveConn()) // SetActiveAWSProfile
```

Both files gain the
`"github.com/ePex/cloudtui/tui/internal/queue/secretbackend"` import.

**`connectionsecrets.go` deleted entirely** — nothing left in it once
(1) and the above land; every symbol it exported (only ever
internally, to `app.go`/`host.go`) has a `secretbackend`-qualified
replacement.

### 3. Tests

**New `internal/queue/secretbackend/secretbackend_test.go`** — the 7
existing tests in `connectionsecrets_test.go`
(`TestResolvePasswordNoProfileSelectedSkipsRevealCall`,
`TestResolvePasswordCachesAcrossCalls`,
`TestResolvePasswordRejectsBinarySecret`,
`TestSecretBackendListRetriesOnceOnFailure`,
`TestSecretBackendListSurfacesErrorAfterRetryExhausted`,
`TestSecretBackendRemoveMessageDoesNotRetryButInvalidatesCache`,
`TestSecretBackendCurrentUsesCapturedProfileNotLiveConfig`) move here,
renamed to drop the now-redundant `SecretBackend`/`Password` prefixes
(package name already says that) and adapted to construct a
`*SecretResolver` directly instead of a bare `*App`:

```go
func TestResolveNoProfileSelectedSkipsRevealCall(t *testing.T) {
	calls := 0
	r := NewSecretResolver(func(context.Context, string, string) (string, bool, error) {
		calls++
		return "", false, nil
	})

	_, err := r.Resolve(context.Background(), "", "my-secret")
	if err == nil {
		t.Fatal("Resolve() error = nil, want an error when no AWS profile is selected")
	}
	if calls != 0 {
		t.Errorf("reveal called %d times, want 0 (no API call without a profile)", calls)
	}
}
```

`secretsFakeBackend` moves as `fakeBackend` (the doc comment
distinguishing it from `queues_test.go`'s `fakeQueueBackend` is no
longer needed — different packages now, no naming collision to call
out).

`newTestSecretBackend(a *App, profile string, fake)` becomes
`newTestBackend(resolver *SecretResolver, profile string, fake *fakeBackend) *Backend`:

```go
func newTestBackend(resolver *SecretResolver, profile string, fake *fakeBackend) *Backend {
	return &Backend{
		resolver:   resolver,
		conn:       config.Connection{Name: "test", Backend: "jolokia", Queue: config.QueueConfig{PasswordSecret: "my-secret"}},
		secretName: "my-secret",
		profile:    profile,
		build:      func(config.Connection) queue.Backend { return fake },
	}
}
```

`TestSecretBackendListRetriesOnceOnFailure` →
`TestBackendListRetriesOnceOnFailure`,
`TestSecretBackendListSurfacesErrorAfterRetryExhausted` →
`TestBackendListSurfacesErrorAfterRetryExhausted`,
`TestSecretBackendRemoveMessageDoesNotRetryButInvalidatesCache` →
`TestBackendRemoveMessageDoesNotRetryButInvalidatesCache` — same
bodies, `a := newTestAppForSecrets(...)` → `r := NewSecretResolver(...)`,
`b := newTestSecretBackend(a, "prof", fake)` →
`b := newTestBackend(r, "prof", fake)`.

**`TestSecretBackendCurrentUsesCapturedProfileNotLiveConfig` is
dropped, not ported.** It existed to regression-test that `current()`
doesn't read `b.app.cfg` — a real risk pre-move, since `app *App` sat
right there on the struct alongside the new `profile` field. Post-move,
`Backend` has no `app`/`cfg`-shaped field at all; there's nothing left
to accidentally read. The property it guarded is now structural
(enforced by the type not compiling any other way), not a runtime
behavior worth a dedicated test — same reasoning as dropping a nil
check once its precondition becomes a compile-time guarantee (CR 85's
`ApplyPalette`).

**`internal/app/host_test.go`'s
`TestSetActiveAWSProfileRebuildsSecretBackedBackend`** stays (it needs
a real, fully-wired `*App` — `switchConnection`, `a.queuesV`,
`a.settingsV`, etc.), adapted for the new type and the new `Profile()`
accessor:

```go
func TestSetActiveAWSProfileRebuildsSecretBackedBackend(t *testing.T) {
	t.Chdir(t.TempDir())
	a := New(config.Default())
	a.cfg.Connections = append(a.cfg.Connections, config.Connection{
		Name: "secret-conn", Backend: "jolokia",
		Queue: config.QueueConfig{PasswordSecret: "my-secret"},
	})
	a.switchConnection("secret-conn")

	before, ok := a.backend.(*secretbackend.Backend)
	if !ok {
		t.Fatalf("a.backend = %T, want *secretbackend.Backend", a.backend)
	}
	if before.Profile() != "" {
		t.Errorf("Backend.Profile() before SetActiveAWSProfile = %q, want empty", before.Profile())
	}

	a.SetActiveAWSProfile("work")

	after, ok := a.backend.(*secretbackend.Backend)
	if !ok {
		t.Fatalf("a.backend after SetActiveAWSProfile = %T, want *secretbackend.Backend", a.backend)
	}
	if after.Profile() != "work" {
		t.Errorf("Backend.Profile() after SetActiveAWSProfile = %q, want %q", after.Profile(), "work")
	}
	if after == before {
		t.Error("a.backend is the same *secretbackend.Backend instance after SetActiveAWSProfile, want a rebuilt one")
	}
}
```

`host_test.go` gains the `secretbackend` import.

### 4. Verification order

Step 1 (new package, standalone) → step 3's new test file (verify the
package fully in isolation, `go test ./internal/queue/secretbackend/...`)
→ step 2 (`internal/app` call sites updated, old file deleted) →
`host_test.go`'s adapted test. `gofmt -l`/`go build ./...`/
`go vet ./...`/`go test ./...` after each step. Final repo-wide pass.

## Files touched

- New `internal/queue/secretbackend/secretbackend.go`.
- New `internal/queue/secretbackend/secretbackend_test.go`.
- `internal/app/connectionsecrets.go` deleted.
- `internal/app/connectionsecrets_test.go` deleted (content relocated).
- `internal/app/app.go` (field, construction, 2 call sites, new import).
- `internal/app/host.go` (2 call sites, new import).
- `internal/app/host_test.go` (1 test adapted, new import).

## Key decisions

- **`internal/queue/secretbackend`, not `internal/queue` itself** —
  keeps the parent `internal/queue` package (just the `Backend`
  interface + `Summary` type today) free of a dependency on its own
  children (`jolokia`/`proxy`), matching `jolokia`/`proxy`'s existing
  sibling-package shape rather than introducing an asymmetric
  parent-imports-child pattern.
- **`reveal` stays an injected func, not a new package dependency on
  `internal/awssecrets`** — `SecretResolver` doesn't need to know
  `awssecrets.Reveal` exists; `internal/app` wires the concrete
  implementation, matching every other injectable-func-field
  convention already used throughout `*App`.
- **`Backend`'s fields (`resolver`, `conn`, `secretName`, `profile`,
  `build`, `mu`, `inner`) stay unexported** — only `Profile()` is
  exported, and only because `internal/app`'s own wiring test needs
  it; nothing else outside the package needs to construct or inspect
  a `Backend` directly (`New` always returns the `queue.Backend`
  interface).
- **One test dropped, not ported** — see step 3's note on
  `TestSecretBackendCurrentUsesCapturedProfileNotLiveConfig`; the
  regression it guarded against becomes structurally impossible once
  `Backend` has no `app`/`cfg` reference to accidentally reach.
- **No behavior change anywhere** — every retry/cache/invalidate rule
  relocates verbatim; this is a pure package move plus the mechanical
  renames needed to drop the `*App` dependency.

## Definition of done

Unchanged from spec.md — `internal/queue/secretbackend` holds
`SecretResolver`/`Backend`/`New`; `internal/app/connectionsecrets.go`
no longer exists; `*App` has `secretResolver
*secretbackend.SecretResolver` instead of `secretCache`; `go build`/
`go test`/`go vet` clean, `gofmt -l` clean, zero import cycle; all
surviving tests pass (7 relocated + 1 dropped + 1 adapted); no
behavior change.
