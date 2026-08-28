# Implementation plan

## `tui/internal/queue/secretbackend/secretbackend.go`

New imports: `awsauth`, `awsprofile`.

`SecretResolver` gains three fields, injected via `NewSecretResolver`
exactly like the existing `reveal` field:

```go
type SecretResolver struct {
	cache       *secretCache
	reveal      func(ctx context.Context, profile, name string) (string, bool, error)
	authTypeFor func(ctx context.Context, profile string) (awsprofile.AuthType, error)
	login       func(ctx context.Context, profile string) error
	onReauth    func()
}

func NewSecretResolver(
	reveal func(ctx context.Context, profile, name string) (string, bool, error),
	authTypeFor func(ctx context.Context, profile string) (awsprofile.AuthType, error),
	login func(ctx context.Context, profile string) error,
	onReauth func(),
) *SecretResolver {
	return &SecretResolver{cache: newSecretCache(), reveal: reveal, authTypeFor: authTypeFor, login: login, onReauth: onReauth}
}
```

`Resolve` wraps the `reveal` call with `awsauth.WithReauth`. `WithReauth`
is generic over a single `(T, error)` return, but `reveal` returns
`(string, bool, error)`, so a small local result type carries both
values through:

```go
type revealResult struct {
	value    string
	isBinary bool
}

func (r *SecretResolver) Resolve(ctx context.Context, profile, secretName string) (string, error) {
	if profile == "" {
		return "", fmt.Errorf("no AWS profile selected — pick one in Settings -> AWS Profiles")
	}
	if v, ok := r.cache.get(profile, secretName); ok {
		return v, nil
	}

	authType, _ := r.authTypeFor(ctx, profile)
	result, err := awsauth.WithReauth(ctx, profile, authType, r.login, r.onReauth,
		func(ctx context.Context) (revealResult, error) {
			value, isBinary, err := r.reveal(ctx, profile, secretName)
			return revealResult{value, isBinary}, err
		},
	)
	if err != nil {
		return "", fmt.Errorf("resolving password secret %q: %w", secretName, err)
	}
	if result.isBinary {
		return "", fmt.Errorf("password secret %q has a binary value, expected a string", secretName)
	}
	r.cache.set(profile, secretName, result.value)
	return result.value, nil
}
```

`authTypeFor` is always called (needed to decide whether `WithReauth`
should even consider reauth) — never nil in production. `login` and
`onReauth` are only ever invoked when `NeedsReauth` returns true (i.e.
`authType == awsprofile.AuthSSO` and the error shape matches), so tests
that stub `authTypeFor` to return a non-SSO type can safely pass `nil`
for both, exactly like `awsauth.WithReauth`'s own nil-safe `onReauth`
handling.

## `tui/internal/app/app.go`

The one production call site:

```go
a.secretResolver = secretbackend.NewSecretResolver(a.revealSecret)
```

becomes:

```go
a.secretResolver = secretbackend.NewSecretResolver(a.revealSecret, a.AWSAuthTypeFor, a.AWSSSOLogin, func() {
	a.QueueUpdateDraw(func() {
		a.SetStatus("AWS SSO session expired — opening browser to log in...")
	})
})
```

Uses the bottom status bar (`SetStatus`), not a per-view table row —
see spec.md for why (this code path isn't owned by any one view).

## `tui/internal/queue/secretbackend/secretbackend_test.go`

- New helper `newTestResolver(reveal) *SecretResolver` wrapping
  `NewSecretResolver` with a non-SSO `authTypeFor` stub
  (`func(context.Context, string) (awsprofile.AuthType, error) { return "", nil }`)
  and `nil` for `login`/`onReauth` — used by every existing test
  (`TestResolveNoProfileSelectedSkipsRevealCall`,
  `TestResolveCachesAcrossCalls`, `TestResolveRejectsBinarySecret`, and
  the three `newTestBackend`-based tests, which construct their own
  resolver inline) so none of their behavior or assertions change.
- New tests exercising the actual reauth wiring:
  - `TestResolveTriggersReauthOnSSOExpiredError` — `reveal` fails once
    with an SSO-expired-shaped error (matching what
    `awsauth.NeedsReauth` detects) then succeeds; `authTypeFor` returns
    `awsprofile.AuthSSO`; assert `onReauth` was called before `login`,
    `login` was called with the right profile, and `Resolve` ultimately
    returns the value from the successful retry.
  - `TestResolveSurfacesErrorWhenReauthLoginFails` — same setup but
    `login` itself returns an error; assert `Resolve` returns an error
    (wrapping the original, per `WithReauth`'s own documented
    behavior) rather than looping or panicking.
  - `TestResolveDoesNotReauthForNonSSOProfile` — `authTypeFor` returns
    `awsprofile.AuthStaticKeys`; `reveal` fails with the same
    SSO-shaped error text; assert `login`/`onReauth` are never called
    and the raw error is surfaced (proves this doesn't accidentally
    fire reauth for auth types where the whole flow, especially
    `login`'s real `aws sso login` shell-out, would just fail).

## Manual verification

Genuinely hard to trigger for real (needs an actually-expired SSO
session against a real AWS org) — `tasks.md`'s manual-check task
documents inspecting the code path live if a real expired session is
available, but falls back to the unit tests above as the primary
evidence, consistent with `CLAUDE.md`'s "say so explicitly" rule for
behavior that can't be fully driven by hand. The visible-effect half
(does `SetStatus` actually render as expected in a live terminal) is
already covered by the *existing* `WithReauth` call sites' own
established live behavior — this isn't new UI, just a new trigger path
into function that's already visibly proven to work.
