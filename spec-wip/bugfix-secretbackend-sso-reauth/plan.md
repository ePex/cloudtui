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

### Addendum, found live: also revert the "waiting" message

Live-testing this (see "Manual verification" below) surfaced a real gap
the plan above didn't cover: the bottom status bar shows the SSO-wait
message, but nothing ever clears it back — unlike every *other*
`WithReauth` call site, where the message lives in a table row that
naturally gets overwritten once real data renders, the bottom status bar
is a separate, persistent UI element with nothing to overwrite it. Worse,
the table's own "Loading queues…" placeholder (from the queues-loading
indicator bugfix, already shipped) just sits there unchanged through the
whole reauth wait, so the user sees two disconnected messages at once
with no indication they're related.

Fix: `SecretResolver` gains a fourth field, `onReauthDone func()`,
invoked immediately after `login` returns — success or failure — before
the retry fires:

```go
loginThenNotifyDone := func(ctx context.Context, profile string) error {
	err := r.login(ctx, profile)
	if r.onReauthDone != nil {
		r.onReauthDone()
	}
	return err
}
result, err := awsauth.WithReauth(ctx, profile, authType, loginThenNotifyDone, r.onReauth, /* ... */)
```

Firing `onReauthDone` regardless of `login`'s outcome matters: a caller
reverting its own "reauth in progress" message must not get stuck
showing it forever just because the login attempt failed.

New `ui.ReauthStatusShower` interface (`tui/internal/ui/reauth.go`,
mirroring the existing `Themeable`/`Shortcuttable` optional-interface
pattern — dispatched via the existing `a.activeView()` helper):

```go
type ReauthStatusShower interface {
	ShowReauthWaiting()
	ShowReauthDone()
}
```

`QueuesView` implements it (`tui/internal/view/queues.go`): a new
`loadingQueuesStatus` const holds `"Loading queues…"` (previously an
inline literal in `Load()`) so both `Load()` and `ShowReauthDone()` stay
in sync; `ShowReauthWaiting()` replaces the table's placeholder with the
SSO-wait message; `ShowReauthDone()` reverts it back to
`loadingQueuesStatus`.

## `tui/internal/app/app.go`

The one production call site:

```go
a.secretResolver = secretbackend.NewSecretResolver(a.revealSecret)
```

becomes:

```go
a.secretResolver = secretbackend.NewSecretResolver(a.revealSecret, a.AWSAuthTypeFor, a.AWSSSOLogin,
	func() {
		a.QueueUpdateDraw(func() {
			a.SetStatus("AWS SSO session expired — opening browser to log in...")
			if av, ok := a.activeView().(ui.ReauthStatusShower); ok {
				av.ShowReauthWaiting()
			}
		})
	},
	func() {
		a.QueueUpdateDraw(func() {
			a.SetStatus("")
			if av, ok := a.activeView().(ui.ReauthStatusShower); ok {
				av.ShowReauthDone()
			}
		})
	},
)
```

Uses the bottom status bar (`SetStatus`) *and*, if the currently active
view implements `ui.ReauthStatusShower`, its own table-level message too
— covers both UI regions the live test found showing stale/disconnected
text. `onReauthDone` also clears the status bar back to `""`, so it
doesn't linger after login completes the way it did before this
addendum. Not unit-tested at this exact call-site level (it's a thin,
direct composition of already-tested primitives — `SetStatus`,
`QueueUpdateDraw`, `activeView`, the type-assert dispatch pattern already
used for `Themeable` — with no independent logic of its own); covered
indirectly by `SecretResolver`'s wiring tests plus `QueuesView`'s own
`ShowReauthWaiting`/`ShowReauthDone` tests.

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
  - `TestResolveSurfacesErrorWhenReauthLoginFails` additionally asserts
    `onReauthDone` still fires when `login` fails (addendum above).

## `tui/internal/view/queues_test.go`

- `TestQueuesViewShowReauthWaitingThenDone` — after some prior table
  state (so there's something to overwrite), `ShowReauthWaiting()`
  shows the SSO-wait message and `ShowReauthDone()` reverts to
  `loadingQueuesStatus`.

## Manual verification

Turned out *not* to be as hard to trigger as initially assumed: the user
tested this live against a real expired SSO session and found the
addendum's gap directly (status bar message never clearing, table stuck
on a generic "Loading queues…" throughout) — which is exactly the kind
of thing `CLAUDE.md`'s "drive the real binary" guidance exists for. The
fix above was verified against that same live report, not re-tested
live independently in this session (no real expired SSO session
available here) — the unit tests are the evidence of record for this
implementation, with the live report as the origin of the addendum
itself.
