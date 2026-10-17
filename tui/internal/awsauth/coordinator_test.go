package awsauth

import (
	"context"
	"errors"
	"sync"
	"testing"
	"time"
)

// testTimeout bounds every wait in these tests, so a broken coordinator
// fails instead of hanging.
const testTimeout = 2 * time.Second

// fakeLogin is one call of fakeRuns.run: the test decides when and how
// it ends.
type fakeLogin struct {
	ctx     context.Context
	profile string
	onCode  func(code, url string)
	result  chan error
}

// fakeRuns stands in for awsauth.Login: each call shows up on started
// and returns what the test sends on its result channel, or ctx.Err()
// once its context is done. With hold set, a cancelled run doesn't
// return until hold is closed (a CLI slow to exit).
type fakeRuns struct {
	started chan *fakeLogin
	hold    chan struct{}
}

func newFakeRuns() *fakeRuns { return &fakeRuns{started: make(chan *fakeLogin, 10)} }

func (f *fakeRuns) run(ctx context.Context, profile string, onCode func(code, url string)) error {
	l := &fakeLogin{ctx: ctx, profile: profile, onCode: onCode, result: make(chan error, 1)}
	f.started <- l
	select {
	case err := <-l.result:
		return err
	case <-ctx.Done():
		if f.hold != nil {
			<-f.hold
		}
		return ctx.Err()
	}
}

// next returns the next run started, failing the test if none starts.
func (f *fakeRuns) next(t *testing.T) *fakeLogin {
	t.Helper()
	select {
	case l := <-f.started:
		return l
	case <-time.After(testTimeout):
		t.Fatal("no login run started")
		return nil
	}
}

// none fails the test if another run starts.
func (f *fakeRuns) none(t *testing.T) {
	t.Helper()
	select {
	case l := <-f.started:
		t.Fatalf("another login run started (profile %q)", l.profile)
	case <-time.After(50 * time.Millisecond):
	}
}

// login calls c.Login on a goroutine and returns its result channel.
func login(c *LoginCoordinator, ctx context.Context, profile string, onCode func(code, url string)) <-chan error {
	done := make(chan error, 1)
	go func() { done <- c.Login(ctx, profile, onCode) }()
	return done
}

// result waits for a Login's result, failing the test if it doesn't
// return.
func result(t *testing.T, done <-chan error) error {
	t.Helper()
	select {
	case err := <-done:
		return err
	case <-time.After(testTimeout):
		t.Fatal("Login didn't return")
		return nil
	}
}

// waiting fails the test unless Login is still waiting.
func waiting(t *testing.T, done <-chan error) {
	t.Helper()
	select {
	case err := <-done:
		t.Fatalf("Login returned %v, want it still waiting", err)
	case <-time.After(50 * time.Millisecond):
	}
}

// waitForCallers waits until n callers wait for profile's run's code,
// so a test knows a Login has joined before it ends the run.
func waitForCallers(t *testing.T, c *LoginCoordinator, profile string, n int) {
	t.Helper()
	deadline := time.Now().Add(testTimeout)
	for time.Now().Before(deadline) {
		c.mu.Lock()
		r := c.running[profile]
		got := 0
		if r != nil {
			got = len(r.waiters)
		}
		c.mu.Unlock()
		if got == n {
			return
		}
		time.Sleep(time.Millisecond)
	}
	t.Fatalf("never saw %d callers waiting for %q", n, profile)
}

// codes records onCode calls.
type codes struct {
	mu  sync.Mutex
	got []string
}

func (c *codes) onCode(code, url string) {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.got = append(c.got, code+" "+url)
}

func (c *codes) list() []string {
	c.mu.Lock()
	defer c.mu.Unlock()
	return append([]string(nil), c.got...)
}

// wantCodes waits briefly for c to hold exactly want.
func wantCodes(t *testing.T, who string, c *codes, want ...string) {
	t.Helper()
	deadline := time.Now().Add(testTimeout)
	for {
		got := c.list()
		if len(got) == len(want) {
			for i := range got {
				if got[i] != want[i] {
					t.Fatalf("%s got codes %q, want %q", who, got, want)
				}
			}
			return
		}
		if len(got) > len(want) || time.Now().After(deadline) {
			t.Fatalf("%s got codes %q, want %q", who, got, want)
		}
		time.Sleep(time.Millisecond)
	}
}

func newTestCoordinator(t *testing.T) (*LoginCoordinator, *fakeRuns) {
	t.Helper()
	f := newFakeRuns()
	c := NewLoginCoordinator(f.run)
	t.Cleanup(c.Close)
	return c, f
}

func TestLoginCoordinatorSharesOneRunPerProfile(t *testing.T) {
	c, f := newTestCoordinator(t)
	var a, b codes

	doneA := login(c, t.Context(), "p", a.onCode)
	run := f.next(t)
	doneB := login(c, t.Context(), "p", b.onCode)
	waitForCallers(t, c, "p", 2)
	f.none(t)

	run.result <- nil

	if err := result(t, doneA); err != nil {
		t.Errorf("first caller: %v, want nil", err)
	}
	if err := result(t, doneB); err != nil {
		t.Errorf("second caller: %v, want nil", err)
	}
	if run.profile != "p" {
		t.Errorf("run for %q, want p", run.profile)
	}
}

func TestLoginCoordinatorRunsEachProfileSeparately(t *testing.T) {
	c, f := newTestCoordinator(t)

	doneP := login(c, t.Context(), "p", nil)
	first := f.next(t)
	doneQ := login(c, t.Context(), "q", nil)
	second := f.next(t)
	runs := map[string]*fakeLogin{first.profile: first, second.profile: second}
	if runs["p"] == nil || runs["q"] == nil {
		t.Fatalf("runs for %q and %q, want p and q", first.profile, second.profile)
	}

	failQ := errors.New("q failed")
	runs["q"].result <- failQ
	if err := result(t, doneQ); !errors.Is(err, failQ) {
		t.Errorf("q: %v, want %v", err, failQ)
	}
	waiting(t, doneP)
	runs["p"].result <- nil
	if err := result(t, doneP); err != nil {
		t.Errorf("p: %v, want nil", err)
	}
}

// TestLoginCoordinatorGivesEveryCallerTheCode: callers waiting when the
// code is printed get it then, a caller joining afterwards gets it at
// once, and each gets it once even if the run reports it twice.
func TestLoginCoordinatorGivesEveryCallerTheCode(t *testing.T) {
	c, f := newTestCoordinator(t)
	var a, b, late codes

	doneA := login(c, t.Context(), "p", a.onCode)
	run := f.next(t)
	doneB := login(c, t.Context(), "p", b.onCode)
	waitForCallers(t, c, "p", 2)

	run.onCode("ABCD-EFGH", "https://example.invalid/device")
	run.onCode("ABCD-EFGH", "https://example.invalid/device")
	doneLate := login(c, t.Context(), "p", late.onCode)

	want := "ABCD-EFGH https://example.invalid/device"
	wantCodes(t, "first caller", &a, want)
	wantCodes(t, "second caller", &b, want)
	wantCodes(t, "late caller", &late, want)

	run.result <- nil
	for _, done := range []<-chan error{doneA, doneB, doneLate} {
		if err := result(t, done); err != nil {
			t.Errorf("caller: %v, want nil", err)
		}
	}
}

// TestLoginCoordinatorCancelledCallerLeavesTheRunGoing is the bug's
// case: Esc during the login, then a reload, must not start a second
// login (a second browser window).
func TestLoginCoordinatorCancelledCallerLeavesTheRunGoing(t *testing.T) {
	c, f := newTestCoordinator(t)
	var cancelled codes

	ctx, cancel := context.WithCancel(t.Context())
	doneA := login(c, ctx, "p", cancelled.onCode)
	run := f.next(t)
	waitForCallers(t, c, "p", 1)
	cancel()

	if err := result(t, doneA); !errors.Is(err, context.Canceled) {
		t.Fatalf("cancelled caller: %v, want context.Canceled", err)
	}
	if err := run.ctx.Err(); err != nil {
		t.Fatalf("run's context: %v, want it still running", err)
	}

	var b codes
	doneB := login(c, t.Context(), "p", b.onCode)
	waitForCallers(t, c, "p", 1) // the reload, not the cancelled caller
	f.none(t)
	run.onCode("ABCD-EFGH", "https://example.invalid/device")
	run.result <- nil
	if err := result(t, doneB); err != nil {
		t.Errorf("caller after the cancel: %v, want the run's nil", err)
	}
	wantCodes(t, "caller after the cancel", &b, "ABCD-EFGH https://example.invalid/device")
	if got := cancelled.list(); len(got) != 0 {
		t.Errorf("cancelled caller got codes %q, want none", got)
	}

	doneC := login(c, t.Context(), "p", nil)
	again := f.next(t)
	again.result <- nil
	if err := result(t, doneC); err != nil {
		t.Errorf("login after the run finished: %v, want nil", err)
	}
}

func TestLoginCoordinatorFailedRunFailsEveryCaller(t *testing.T) {
	c, f := newTestCoordinator(t)

	doneA := login(c, t.Context(), "p", func(string, string) {})
	run := f.next(t)
	doneB := login(c, t.Context(), "p", func(string, string) {})
	waitForCallers(t, c, "p", 2)

	failed := errors.New("aws sso login failed")
	run.result <- failed

	for _, done := range []<-chan error{doneA, doneB} {
		if err := result(t, done); !errors.Is(err, failed) {
			t.Errorf("caller: %v, want %v", err, failed)
		}
	}
	f.none(t)
}

// TestLoginCoordinatorCloseEndsARunningLogin: Close cancels the run's
// context (killing the CLI) and returns only once the run returned, so
// quitting leaves no `aws sso login` behind.
func TestLoginCoordinatorCloseEndsARunningLogin(t *testing.T) {
	f := newFakeRuns()
	f.hold = make(chan struct{})
	c := NewLoginCoordinator(f.run)

	done := login(c, t.Context(), "p", nil)
	run := f.next(t)
	closed := make(chan struct{})
	go func() {
		c.Close()
		close(closed)
	}()

	select {
	case <-run.ctx.Done():
	case <-time.After(testTimeout):
		t.Fatal("Close didn't cancel the run's context")
	}
	select {
	case <-closed:
		t.Fatal("Close returned before the run did")
	case <-time.After(50 * time.Millisecond):
	}

	close(f.hold)
	select {
	case <-closed:
	case <-time.After(testTimeout):
		t.Fatal("Close didn't return after the run did")
	}
	if err := result(t, done); !errors.Is(err, context.Canceled) {
		t.Errorf("caller: %v, want context.Canceled", err)
	}
}
