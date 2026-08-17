# Plan — Bugfix 88: switching AWS profile doesn't refresh a Secrets-Manager-backed connection

## Approach

Two small, related changes in `connectionsecrets.go` and `host.go`,
per spec.md's Solution — capture `profile` once at construction, and
make `SetActiveAWSProfile` rebuild the backend like its 2 siblings
already do.

### 1. `secretBackend` gains a fixed `profile` field

```go
type secretBackend struct {
	app        *App
	conn       config.Connection
	secretName string
	// profile is captured once, at construction, on the main
	// goroutine — never re-read from a.cfg here, since every
	// queue.Backend method (List, BrowseMessages, ...) runs on a
	// background goroutine that would otherwise race the main
	// goroutine's own writes to a.cfg (profile switch, connection
	// switch). Same discipline as spec/87's PipelineWatcher capturing
	// its profile once in StartWatchingPipeline.
	profile string
	build   func(config.Connection) queue.Backend

	mu    sync.Mutex
	inner queue.Backend
}
```

`newBackendForConn` captures it:

```go
func newBackendForConn(a *App, conn config.Connection) queue.Backend {
	secretName := passwordSecretName(conn)
	if secretName == "" {
		return buildBackend(conn)
	}
	return &secretBackend{
		app:        a,
		conn:       conn,
		secretName: secretName,
		profile:    a.cfg.ActiveAWSProfile,
		build:      buildBackend,
	}
}
```

`current()`/`refresh()` read `b.profile` instead of
`b.app.cfg.ActiveAWSProfile`:

```go
func (b *secretBackend) current(ctx context.Context) (queue.Backend, error) {
	b.mu.Lock()
	defer b.mu.Unlock()
	if b.inner != nil {
		return b.inner, nil
	}
	pw, err := b.app.resolvePassword(ctx, b.profile, b.secretName)
	if err != nil {
		return nil, err
	}
	b.inner = b.build(connWithPassword(b.conn, pw))
	return b.inner, nil
}

func (b *secretBackend) refresh() {
	b.app.secretCache.invalidate(b.profile, b.secretName)
	b.mu.Lock()
	b.inner = nil
	b.mu.Unlock()
}
```

`b.app` is still needed (for `resolvePassword`/`secretCache`, both
genuinely App-wide), just no longer for `.cfg.ActiveAWSProfile`.

### 2. `SetActiveAWSProfile` rebuilds the backend

```go
func (a *App) SetActiveAWSProfile(name string) {
	a.cfg.ActiveAWSProfile = name
	a.backend = newBackendForConn(a, a.cfg.ActiveConn())
	a.queuesV.SetBackend(a.backend)
	a.infoPanel.SetText(ui.InfoPanelText(a.cfg))
	a.settingsV.Refresh()
	if err := config.SaveDefault(a.cfg); err != nil {
		slog.Error("SetActiveAWSProfile: save failed", "error", err)
	}
}
```

Identical 2-line addition (`a.backend = newBackendForConn(...)` +
`a.queuesV.SetBackend(a.backend)`) to `switchConnection`/
`SaveConnection`'s existing shape — inserted right after the field
write that makes `a.cfg.ActiveConn()`'s dependency (`ActiveAWSProfile`)
current, same ordering those 2 already use for `ActiveConnection`.

### 3. Tests

**`newTestSecretBackend` helper gains a `profile` parameter**, since
`secretBackend` now requires one:

```go
func newTestSecretBackend(a *App, profile string, fake *secretsFakeBackend) *secretBackend {
	return &secretBackend{
		app:        a,
		conn:       config.Connection{Name: "test", Backend: "jolokia", Queue: config.QueueConfig{PasswordSecret: "my-secret"}},
		secretName: "my-secret",
		profile:    profile,
		build:      func(config.Connection) queue.Backend { return fake },
	}
}
```

The 3 existing call sites
(`TestSecretBackendListRetriesOnceOnFailure`,
`TestSecretBackendListSurfacesErrorAfterRetryExhausted`,
`TestSecretBackendRemoveMessageDoesNotRetryButInvalidatesCache`) pass
`"prof"`, matching each test's existing `newTestAppForSecrets("prof",
...)` call — no behavioral change to any of them, since they already
implicitly assumed `b`'s profile was `"prof"` via the old live-`a.cfg`
read.

**New regression test, proving finding 2 is fixed** — `current()` uses
the captured field, not whatever `a.cfg.ActiveAWSProfile` happens to
be at call time:

```go
func TestSecretBackendCurrentUsesCapturedProfileNotLiveConfig(t *testing.T) {
	var seenProfiles []string
	a := newTestAppForSecrets("prof-a", func(_ context.Context, profile, _ string) (string, bool, error) {
		seenProfiles = append(seenProfiles, profile)
		return "pw", false, nil
	})
	b := newTestSecretBackend(a, "prof-a", &secretsFakeBackend{})

	// Simulate a.cfg changing without going through
	// SetActiveAWSProfile/newBackendForConn (e.g. a background
	// goroutine racing the main goroutine's write, or simply a call
	// site that forgot to rebuild) — current() must be unaffected.
	a.cfg.ActiveAWSProfile = "prof-b"

	if _, err := b.current(context.Background()); err != nil {
		t.Fatalf("current() error = %v", err)
	}
	if len(seenProfiles) != 1 || seenProfiles[0] != "prof-a" {
		t.Errorf("revealSecret saw profiles %v, want [%q] (captured at construction, not live a.cfg)", seenProfiles, "prof-a")
	}
}
```

**New wiring test, proving finding 1 is fixed** — in
`internal/app/host_test.go`, alongside the existing
`TestSetActiveAWSProfilePersistsAndUpdatesUI`:

```go
func TestSetActiveAWSProfileRebuildsSecretBackedBackend(t *testing.T) {
	t.Chdir(t.TempDir())
	a := New(config.Default())
	a.cfg.Connections = append(a.cfg.Connections, config.Connection{
		Name: "secret-conn", Backend: "jolokia",
		Queue: config.QueueConfig{PasswordSecret: "my-secret"},
	})
	a.switchConnection("secret-conn")

	before, ok := a.backend.(*secretBackend)
	if !ok {
		t.Fatalf("a.backend = %T, want *secretBackend", a.backend)
	}
	if before.profile != "" {
		t.Errorf("secretBackend.profile before SetActiveAWSProfile = %q, want empty", before.profile)
	}

	a.SetActiveAWSProfile("work")

	after, ok := a.backend.(*secretBackend)
	if !ok {
		t.Fatalf("a.backend after SetActiveAWSProfile = %T, want *secretBackend", a.backend)
	}
	if after.profile != "work" {
		t.Errorf("secretBackend.profile after SetActiveAWSProfile = %q, want %q", after.profile, "work")
	}
	if after == before {
		t.Error("a.backend is the same *secretBackend instance after SetActiveAWSProfile, want a rebuilt one")
	}
}
```

`switchConnection` is already exercised as an unexported method from
this package's own tests (`viewwiring_test.go`'s
`TestSwitchConnectionRefreshesSettingsList`), so calling it directly
here to get a `secretBackend` onto `a.backend` matches existing
precedent.

### 4. Verification order

Step 1 (`secretBackend`'s new field) → step 2 (`SetActiveAWSProfile`)
→ step 3 (test helper + regression + wiring tests). `gofmt -l`/
`go build ./...`/`go vet ./...`/`go test ./...` after each step. Final
repo-wide pass.

## Files touched

- `tui/internal/app/connectionsecrets.go` (`secretBackend` struct,
  `newBackendForConn`, `current`, `refresh`).
- `tui/internal/app/host.go` (`SetActiveAWSProfile`).
- `tui/internal/app/connectionsecrets_test.go` (`newTestSecretBackend`
  helper + its 3 call sites, 1 new regression test).
- `tui/internal/app/host_test.go` (1 new wiring test).

## Key decisions

- **`profile` captured at construction, not passed as a
  `newBackendForConn` parameter** — `newBackendForConn(a, conn)` can
  already read `a.cfg.ActiveAWSProfile` itself at the point it's
  called (main goroutine, `a.cfg` current); adding a third parameter
  everywhere `newBackendForConn` is called would just relocate the
  same read one level up for no benefit, since all 3 call sites are
  already on the main goroutine.
- **No signature change to `newBackendForConn`** — all 3 existing call
  sites (`app.go:202`, `app.go:563`, `host.go:108`) are untouched;
  only the 4th call site this bugfix adds, in `SetActiveAWSProfile`,
  is new.
- **`SetActiveAWSProfile` rebuilds unconditionally**, even for
  connections without a `PasswordSecret` — matches
  `switchConnection`/`SaveConnection`'s existing unconditional
  rebuild; `newBackendForConn` already short-circuits to a cheap
  `buildBackend(conn)` call (no network I/O) in that case, so there's
  no meaningful cost to conditionalizing away.
- **Not touching the read/write retry asymmetry** in `current()`/the 7
  mutating methods — unrelated to where `profile` comes from, already
  covered by existing tests, out of scope per spec.md.

## Definition of done

Unchanged from spec.md — `secretBackend` never reads `a.cfg` directly;
`SetActiveAWSProfile` rebuilds `a.backend` like its 2 siblings;
`go build`/`go test`/`go vet` clean, `gofmt -l` clean; new regression
+ wiring tests pass; no behavior change for connections without a
`PasswordSecret`.
