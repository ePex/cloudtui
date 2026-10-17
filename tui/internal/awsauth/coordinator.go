package awsauth

import (
	"context"
	"sync"
)

// LoginCoordinator runs at most one `aws sso login` per profile at a
// time. Every caller that needs a login for a profile whose login is
// already running waits for that run instead of starting another one —
// each run opens its own browser window, and approving one leaves the
// other waiting for a code nobody enters (see spec/14).
//
// A run uses the coordinator's own context, not the caller's: a caller
// that stops waiting (e.g. Esc on "Loading queues") returns at once,
// but the login keeps running, so approving the browser tab that's
// already open still logs in. Close ends a run that's still going.
type LoginCoordinator struct {
	run    func(ctx context.Context, profile string, onCode func(code, url string)) error
	ctx    context.Context
	cancel context.CancelFunc
	wg     sync.WaitGroup

	mu      sync.Mutex
	running map[string]*loginRun
}

// loginRun is one run of the login for one profile.
type loginRun struct {
	done chan struct{}
	err  error // set before done is closed

	// Guarded by LoginCoordinator.mu.
	code, url string
	hasCode   bool
	waiters   map[int]func(code, url string) // callers waiting for the code
	nextID    int
}

// NewLoginCoordinator returns a coordinator that logs in with run
// (awsauth.Login in the app). run must honor its context's
// cancellation, as exec.CommandContext does.
func NewLoginCoordinator(run func(ctx context.Context, profile string, onCode func(code, url string)) error) *LoginCoordinator {
	ctx, cancel := context.WithCancel(context.Background())
	return &LoginCoordinator{run: run, ctx: ctx, cancel: cancel, running: make(map[string]*loginRun)}
}

// Login logs in to profile, with the same contract as awsauth.Login: it
// returns the login's result, and calls onCode (if non-nil) once with
// the device verification code and URL. If a login for profile is
// already running, it waits for that one; onCode then gets its code,
// at once if it was already printed. If ctx is done first, Login
// returns ctx.Err() and the run goes on.
func (c *LoginCoordinator) Login(ctx context.Context, profile string, onCode func(code, url string)) error {
	c.mu.Lock()
	r, ok := c.running[profile]
	if !ok {
		r = &loginRun{done: make(chan struct{}), waiters: make(map[int]func(code, url string))}
		c.running[profile] = r
		c.wg.Add(1)
		go c.start(profile, r)
	}
	id := -1
	var code, url string
	if onCode != nil {
		if r.hasCode {
			code, url = r.code, r.url
		} else {
			id = r.nextID
			r.nextID++
			r.waiters[id] = onCode
		}
	}
	c.mu.Unlock()
	if onCode != nil && id < 0 {
		onCode(code, url)
	}

	select {
	case <-r.done:
		return r.err
	case <-ctx.Done():
		if id >= 0 {
			c.mu.Lock()
			delete(r.waiters, id) // a caller that stopped waiting gets no code
			c.mu.Unlock()
		}
		return ctx.Err()
	}
}

// start runs the login for profile and hands its code and result to
// r's callers.
func (c *LoginCoordinator) start(profile string, r *loginRun) {
	defer c.wg.Done()
	err := c.run(c.ctx, profile, func(code, url string) {
		c.mu.Lock()
		r.code, r.url, r.hasCode = code, url, true
		waiters := make([]func(code, url string), 0, len(r.waiters))
		for _, w := range r.waiters {
			waiters = append(waiters, w)
		}
		r.waiters = nil // each caller gets the code once, even if it's reported again
		c.mu.Unlock()
		for _, w := range waiters {
			w(code, url)
		}
	})

	c.mu.Lock()
	delete(c.running, profile)
	r.err = err
	c.mu.Unlock()
	close(r.done)
}

// Close ends every login still running (cancelling its context, which
// kills the `aws sso login` process) and returns once they've all
// returned, so quitting leaves no login behind. Logins started after
// Close fail at once.
func (c *LoginCoordinator) Close() {
	c.cancel()
	c.wg.Wait()
}
