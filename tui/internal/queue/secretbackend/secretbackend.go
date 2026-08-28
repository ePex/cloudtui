package secretbackend

import (
	"context"
	"fmt"
	"sync"

	"github.com/ePex/cloudtui/tui/internal/awsauth"
	"github.com/ePex/cloudtui/tui/internal/awsprofile"
	"github.com/ePex/cloudtui/tui/internal/config"
	"github.com/ePex/cloudtui/tui/internal/queue"
	"github.com/ePex/cloudtui/tui/internal/queue/jolokia"
	"github.com/ePex/cloudtui/tui/internal/queue/proxy"
)

// secretCache holds resolved AWS Secrets Manager values in memory, keyed by
// (AWS profile, secret name). Never persisted — a fresh process always
// starts with an empty cache. See spec/56-fe-amq-connection-aws-secret-password.
type secretCache struct {
	mu     sync.Mutex
	values map[string]string
}

func newSecretCache() *secretCache {
	return &secretCache{values: make(map[string]string)}
}

func secretCacheKey(profile, secretName string) string {
	return profile + "\x00" + secretName
}

func (c *secretCache) get(profile, secretName string) (string, bool) {
	c.mu.Lock()
	defer c.mu.Unlock()
	v, ok := c.values[secretCacheKey(profile, secretName)]
	return v, ok
}

func (c *secretCache) set(profile, secretName, value string) {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.values[secretCacheKey(profile, secretName)] = value
}

func (c *secretCache) invalidate(profile, secretName string) {
	c.mu.Lock()
	defer c.mu.Unlock()
	delete(c.values, secretCacheKey(profile, secretName))
}

// SecretResolver resolves AWS-Secrets-Manager-backed connection
// passwords, caching resolved values in memory. See
// spec/56-fe-amq-connection-aws-secret-password.
type SecretResolver struct {
	cache        *secretCache
	reveal       func(ctx context.Context, profile, name string) (string, bool, error)
	authTypeFor  func(ctx context.Context, profile string) (awsprofile.AuthType, error)
	login        func(ctx context.Context, profile string, onCode func(code, url string)) error
	onReauth     func()
	onReauthDone func()
	onCode       func(code, url string)
}

// NewSecretResolver constructs a SecretResolver that resolves secrets via
// reveal (e.g. awssecrets.Reveal). If reveal fails because profile's
// cached SSO token is missing/expired (per authTypeFor and
// awsauth.NeedsReauth), onReauth is called, then login (which calls
// onCode once it has the device verification code/URL to show — see
// awsauth.Login), then onReauthDone (regardless of whether login
// succeeded), then reveal is retried once — the same recovery every
// other AWS-backed view already gets via awsauth.WithReauth (see
// spec/14-aws-profiles), plus onReauthDone so a caller showing its own
// "reauth in progress" message (e.g. a table's loading placeholder) can
// revert it right when login finishes, rather than it sitting there
// through the retry too. login, onReauth, onReauthDone, and onCode are
// only ever invoked for an AuthSSO profile, so callers that never expect
// SSO profiles may pass nil for any of them.
func NewSecretResolver(
	reveal func(ctx context.Context, profile, name string) (string, bool, error),
	authTypeFor func(ctx context.Context, profile string) (awsprofile.AuthType, error),
	login func(ctx context.Context, profile string, onCode func(code, url string)) error,
	onReauth func(),
	onReauthDone func(),
	onCode func(code, url string),
) *SecretResolver {
	return &SecretResolver{cache: newSecretCache(), reveal: reveal, authTypeFor: authTypeFor, login: login, onReauth: onReauth, onReauthDone: onReauthDone, onCode: onCode}
}

// revealResult carries reveal's two success values through
// awsauth.WithReauth, which is generic over a single (T, error) return.
type revealResult struct {
	value    string
	isBinary bool
}

// Resolve resolves secretName via profile, using the cache when possible.
// No reveal call is attempted when profile is empty — the caller (a
// connection with no AWS profile selected) gets an immediate, specific
// error instead of a doomed API call.
func (r *SecretResolver) Resolve(ctx context.Context, profile, secretName string) (string, error) {
	if profile == "" {
		return "", fmt.Errorf("no AWS profile selected — pick one in Settings -> AWS Profiles")
	}
	if v, ok := r.cache.get(profile, secretName); ok {
		return v, nil
	}
	authType, _ := r.authTypeFor(ctx, profile)
	loginThenNotifyDone := func(ctx context.Context, profile string, onCode func(code, url string)) error {
		err := r.login(ctx, profile, onCode)
		if r.onReauthDone != nil {
			r.onReauthDone()
		}
		return err
	}
	result, err := awsauth.WithReauth(ctx, profile, authType, loginThenNotifyDone, r.onReauth, r.onCode,
		func(ctx context.Context) (revealResult, error) {
			value, isBinary, err := r.reveal(ctx, profile, secretName)
			return revealResult{value, isBinary}, err
		},
	)
	if err != nil {
		return "", fmt.Errorf("resolving password secret %q: %w", secretName, err)
	}
	value, isBinary := result.value, result.isBinary
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

// passwordSecretName returns the AWS Secrets Manager secret name configured
// for conn's backend, or "" if the connection uses a plain password.
func passwordSecretName(conn config.Connection) string {
	if conn.Backend == "proxy" {
		return conn.Proxy.PasswordSecret
	}
	return conn.Queue.PasswordSecret
}

// connWithPassword returns a copy of conn with its backend-appropriate
// password field set to password, overriding whatever was there before
// (including any plain-text password field a passwordSecret-bearing
// connection might still have on disk).
func connWithPassword(conn config.Connection, password string) config.Connection {
	if conn.Backend == "proxy" {
		conn.Proxy.Password = password
	} else {
		conn.Queue.Password = password
	}
	return conn
}

// buildBackend constructs the appropriate queue.Backend for conn's backend
// type and connection fields, as-is (no secret resolution).
func buildBackend(conn config.Connection) queue.Backend {
	if conn.Backend == "proxy" {
		return proxy.NewClient(conn.Proxy)
	}
	return jolokia.NewClient(conn.Queue)
}

// New constructs the queue.Backend for conn. Connections without a
// passwordSecret behave exactly as buildBackend directly, no wrapping. A
// passwordSecret-bearing connection gets a *Backend that resolves the
// password from AWS Secrets Manager on first use and transparently
// recovers from a stale/rotated secret — see Backend.
func New(resolver *SecretResolver, profile string, conn config.Connection) queue.Backend {
	secretName := passwordSecretName(conn)
	if secretName == "" {
		return buildBackend(conn)
	}
	return &Backend{resolver: resolver, conn: conn, secretName: secretName, profile: profile, build: buildBackend}
}

// Backend wraps a queue.Backend whose password comes from AWS Secrets
// Manager. It resolves lazily: the network call happens inside whichever
// queue.Backend method is called first, which — every call site in this
// codebase already dispatches queue.Backend calls off the tview goroutine
// (go func() { ...; QueueUpdateDraw(...) }()) — never blocks the UI. See
// spec/56-fe-amq-connection-aws-secret-password, "Key technical
// decision", for why this piggybacks on that existing pattern instead of
// adding a bespoke async-resolve step at each activation site.
//
// Read calls (List, BrowseMessages) that fail against a stale cached
// password invalidate the cache, re-resolve, and retry once. Write calls
// invalidate on failure too (so the next call — read or write — gets a
// fresh password) but are never retried themselves, since silently
// replaying a delete/move/send risks double-applying it if the original
// attempt actually succeeded server-side despite returning an error.
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

// current returns the resolved inner backend, building it on first use.
func (b *Backend) current(ctx context.Context) (queue.Backend, error) {
	b.mu.Lock()
	defer b.mu.Unlock()
	if b.inner != nil {
		return b.inner, nil
	}
	pw, err := b.resolver.Resolve(ctx, b.profile, b.secretName)
	if err != nil {
		return nil, err
	}
	b.inner = b.build(connWithPassword(b.conn, pw))
	return b.inner, nil
}

// refresh invalidates the cached secret value and forces the next current()
// call to re-resolve and rebuild the inner backend.
func (b *Backend) refresh() {
	b.resolver.Invalidate(b.profile, b.secretName)
	b.mu.Lock()
	b.inner = nil
	b.mu.Unlock()
}

func (b *Backend) List(ctx context.Context) ([]queue.Summary, error) {
	cur, err := b.current(ctx)
	if err != nil {
		return nil, err
	}
	out, err := cur.List(ctx)
	if err == nil {
		return out, nil
	}
	b.refresh()
	cur, rerr := b.current(ctx)
	if rerr != nil {
		return out, err
	}
	return cur.List(ctx)
}

func (b *Backend) BrowseMessages(ctx context.Context, queueName string, filter queue.MessageFilter) ([]queue.Message, error) {
	cur, err := b.current(ctx)
	if err != nil {
		return nil, err
	}
	out, err := cur.BrowseMessages(ctx, queueName, filter)
	if err == nil {
		return out, nil
	}
	b.refresh()
	cur, rerr := b.current(ctx)
	if rerr != nil {
		return out, err
	}
	return cur.BrowseMessages(ctx, queueName, filter)
}

func (b *Backend) PurgeQueue(ctx context.Context, queueName string) error {
	cur, err := b.current(ctx)
	if err != nil {
		return err
	}
	if err := cur.PurgeQueue(ctx, queueName); err != nil {
		b.refresh()
		return err
	}
	return nil
}

func (b *Backend) RemoveMessage(ctx context.Context, queueName, messageID string) error {
	cur, err := b.current(ctx)
	if err != nil {
		return err
	}
	if err := cur.RemoveMessage(ctx, queueName, messageID); err != nil {
		b.refresh()
		return err
	}
	return nil
}

func (b *Backend) MoveMessage(ctx context.Context, sourceQueue, messageID, targetQueue string) error {
	cur, err := b.current(ctx)
	if err != nil {
		return err
	}
	if err := cur.MoveMessage(ctx, sourceQueue, messageID, targetQueue); err != nil {
		b.refresh()
		return err
	}
	return nil
}

func (b *Backend) MoveAllMessages(ctx context.Context, sourceQueue, targetQueue string) (int, error) {
	cur, err := b.current(ctx)
	if err != nil {
		return 0, err
	}
	n, err := cur.MoveAllMessages(ctx, sourceQueue, targetQueue)
	if err != nil {
		b.refresh()
		return n, err
	}
	return n, nil
}

func (b *Backend) SendMessage(ctx context.Context, queueName, body string) error {
	cur, err := b.current(ctx)
	if err != nil {
		return err
	}
	if err := cur.SendMessage(ctx, queueName, body); err != nil {
		b.refresh()
		return err
	}
	return nil
}

func (b *Backend) DeleteMessages(ctx context.Context, queueName string, filter queue.MessageFilter) (int, error) {
	cur, err := b.current(ctx)
	if err != nil {
		return 0, err
	}
	n, err := cur.DeleteMessages(ctx, queueName, filter)
	if err != nil {
		b.refresh()
		return n, err
	}
	return n, nil
}

func (b *Backend) MoveMessages(ctx context.Context, sourceQueue, targetQueue string, filter queue.MessageFilter) (int, error) {
	cur, err := b.current(ctx)
	if err != nil {
		return 0, err
	}
	n, err := cur.MoveMessages(ctx, sourceQueue, targetQueue, filter)
	if err != nil {
		b.refresh()
		return n, err
	}
	return n, nil
}
