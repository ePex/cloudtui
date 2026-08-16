package app

import (
	"context"
	"fmt"
	"sync"

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

// resolvePassword resolves secretName via the active AWS profile, using the
// cache when possible. No AWS call is attempted when profile is empty — the
// caller (a connection with no AWS profile selected) gets an immediate,
// specific error instead of a doomed API call.
func (a *App) resolvePassword(ctx context.Context, profile, secretName string) (string, error) {
	if profile == "" {
		return "", fmt.Errorf("no AWS profile selected — pick one in Settings -> AWS Profiles")
	}
	if v, ok := a.secretCache.get(profile, secretName); ok {
		return v, nil
	}
	value, isBinary, err := a.revealSecret(ctx, profile, secretName)
	if err != nil {
		return "", fmt.Errorf("resolving password secret %q: %w", secretName, err)
	}
	if isBinary {
		return "", fmt.Errorf("password secret %q has a binary value, expected a string", secretName)
	}
	a.secretCache.set(profile, secretName, value)
	return value, nil
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

// newBackendForConn constructs the queue.Backend for conn. Connections
// without a passwordSecret behave exactly as before (buildBackend directly,
// no wrapping). A passwordSecret-bearing connection gets a *secretBackend
// that resolves the password from AWS Secrets Manager on first use and
// transparently recovers from a stale/rotated secret — see secretBackend.
func newBackendForConn(a *App, conn config.Connection) queue.Backend {
	secretName := passwordSecretName(conn)
	if secretName == "" {
		return buildBackend(conn)
	}
	return &secretBackend{app: a, conn: conn, secretName: secretName, build: buildBackend}
}

// secretBackend wraps a queue.Backend whose password comes from AWS
// Secrets Manager. It resolves lazily: the network call happens inside
// whichever queue.Backend method is called first, which — every call site
// in this codebase already dispatches queue.Backend calls off the tview
// goroutine (go func() { ...; QueueUpdateDraw(...) }()) — never blocks the
// UI. See spec/56-fe-amq-connection-aws-secret-password, "Key technical
// decision", for why this piggybacks on that existing pattern instead of
// adding a bespoke async-resolve step at each activation site.
//
// Read calls (List, BrowseMessages) that fail against a stale cached
// password invalidate the cache, re-resolve, and retry once. Write calls
// invalidate on failure too (so the next call — read or write — gets a
// fresh password) but are never retried themselves, since silently
// replaying a delete/move/send risks double-applying it if the original
// attempt actually succeeded server-side despite returning an error.
type secretBackend struct {
	app        *App
	conn       config.Connection
	secretName string
	build      func(config.Connection) queue.Backend

	mu    sync.Mutex
	inner queue.Backend
}

// current returns the resolved inner backend, building it on first use.
func (b *secretBackend) current(ctx context.Context) (queue.Backend, error) {
	b.mu.Lock()
	defer b.mu.Unlock()
	if b.inner != nil {
		return b.inner, nil
	}
	pw, err := b.app.resolvePassword(ctx, b.app.cfg.ActiveAWSProfile, b.secretName)
	if err != nil {
		return nil, err
	}
	b.inner = b.build(connWithPassword(b.conn, pw))
	return b.inner, nil
}

// refresh invalidates the cached secret value and forces the next current()
// call to re-resolve and rebuild the inner backend.
func (b *secretBackend) refresh() {
	b.app.secretCache.invalidate(b.app.cfg.ActiveAWSProfile, b.secretName)
	b.mu.Lock()
	b.inner = nil
	b.mu.Unlock()
}

func (b *secretBackend) List(ctx context.Context) ([]queue.Summary, error) {
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

func (b *secretBackend) BrowseMessages(ctx context.Context, queueName string, filter queue.MessageFilter) ([]queue.Message, error) {
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

func (b *secretBackend) PurgeQueue(ctx context.Context, queueName string) error {
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

func (b *secretBackend) RemoveMessage(ctx context.Context, queueName, messageID string) error {
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

func (b *secretBackend) MoveMessage(ctx context.Context, sourceQueue, messageID, targetQueue string) error {
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

func (b *secretBackend) MoveAllMessages(ctx context.Context, sourceQueue, targetQueue string) (int, error) {
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

func (b *secretBackend) SendMessage(ctx context.Context, queueName, body string) error {
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

func (b *secretBackend) DeleteMessages(ctx context.Context, queueName string, filter queue.MessageFilter) (int, error) {
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

func (b *secretBackend) MoveMessages(ctx context.Context, sourceQueue, targetQueue string, filter queue.MessageFilter) (int, error) {
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
