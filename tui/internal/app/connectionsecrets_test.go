package app

import (
	"context"
	"errors"
	"testing"

	"github.com/ePex/cloudtui/tui/internal/config"
	"github.com/ePex/cloudtui/tui/internal/queue"
)

// newTestAppForSecrets builds a minimal *App sufficient for exercising
// resolvePassword/secretBackend directly — no tview widgets are touched by
// either, so a bare struct with the fields they actually read is enough.
func newTestAppForSecrets(profile string, reveal func(ctx context.Context, profile, name string) (string, bool, error)) *App {
	return &App{
		cfg:         config.Config{ActiveAWSProfile: profile},
		secretCache: newSecretCache(),
		revealSecret: func(ctx context.Context, profile, name string) (string, bool, error) {
			return reveal(ctx, profile, name)
		},
	}
}

func TestResolvePasswordNoProfileSelectedSkipsRevealCall(t *testing.T) {
	calls := 0
	a := newTestAppForSecrets("", func(context.Context, string, string) (string, bool, error) {
		calls++
		return "", false, nil
	})

	_, err := a.resolvePassword(context.Background(), a.cfg.ActiveAWSProfile, "my-secret")
	if err == nil {
		t.Fatal("resolvePassword() error = nil, want an error when no AWS profile is selected")
	}
	if calls != 0 {
		t.Errorf("revealSecret called %d times, want 0 (no API call without a profile)", calls)
	}
}

func TestResolvePasswordCachesAcrossCalls(t *testing.T) {
	calls := 0
	a := newTestAppForSecrets("prof", func(context.Context, string, string) (string, bool, error) {
		calls++
		return "s3cr3t", false, nil
	})

	for i := 0; i < 2; i++ {
		v, err := a.resolvePassword(context.Background(), "prof", "my-secret")
		if err != nil {
			t.Fatalf("resolvePassword() error = %v", err)
		}
		if v != "s3cr3t" {
			t.Errorf("resolvePassword() = %q, want %q", v, "s3cr3t")
		}
	}
	if calls != 1 {
		t.Errorf("revealSecret called %d times, want 1 (second call should hit the cache)", calls)
	}
}

func TestResolvePasswordRejectsBinarySecret(t *testing.T) {
	a := newTestAppForSecrets("prof", func(context.Context, string, string) (string, bool, error) {
		return "", true, nil
	})

	if _, err := a.resolvePassword(context.Background(), "prof", "my-secret"); err == nil {
		t.Fatal("resolvePassword() error = nil, want an error for a binary-valued secret")
	}
}

// secretsFakeBackend implements queue.Backend with per-call-name failure
// injection, so tests can drive secretBackend's retry/no-retry paths
// without a real jolokia/proxy client or HTTP server. Distinct from
// queues_test.go's fakeQueueBackend, which has no failure-injection support.
type secretsFakeBackend struct {
	listCalls   int
	listFailN   int // List fails for the first listFailN calls
	removeCalls int
	removeFails bool
}

func (f *secretsFakeBackend) List(ctx context.Context) ([]queue.Summary, error) {
	f.listCalls++
	if f.listCalls <= f.listFailN {
		return nil, errors.New("boom")
	}
	return []queue.Summary{{Name: "q1"}}, nil
}
func (f *secretsFakeBackend) BrowseMessages(ctx context.Context, queueName string, filter queue.MessageFilter) ([]queue.Message, error) {
	return nil, nil
}
func (f *secretsFakeBackend) PurgeQueue(ctx context.Context, queueName string) error { return nil }
func (f *secretsFakeBackend) RemoveMessage(ctx context.Context, queueName, messageID string) error {
	f.removeCalls++
	if f.removeFails {
		return errors.New("boom")
	}
	return nil
}
func (f *secretsFakeBackend) MoveMessage(ctx context.Context, sourceQueue, messageID, targetQueue string) error {
	return nil
}
func (f *secretsFakeBackend) MoveAllMessages(ctx context.Context, sourceQueue, targetQueue string) (int, error) {
	return 0, nil
}
func (f *secretsFakeBackend) SendMessage(ctx context.Context, queueName, body string) error {
	return nil
}
func (f *secretsFakeBackend) DeleteMessages(ctx context.Context, queueName string, filter queue.MessageFilter) (int, error) {
	return 0, nil
}
func (f *secretsFakeBackend) MoveMessages(ctx context.Context, sourceQueue, targetQueue string, filter queue.MessageFilter) (int, error) {
	return 0, nil
}

func newTestSecretBackend(a *App, profile string, fake *secretsFakeBackend) *secretBackend {
	return &secretBackend{
		app:        a,
		conn:       config.Connection{Name: "test", Backend: "jolokia", Queue: config.QueueConfig{PasswordSecret: "my-secret"}},
		secretName: "my-secret",
		profile:    profile,
		build:      func(config.Connection) queue.Backend { return fake },
	}
}

func TestSecretBackendListRetriesOnceOnFailure(t *testing.T) {
	revealCalls := 0
	a := newTestAppForSecrets("prof", func(context.Context, string, string) (string, bool, error) {
		revealCalls++
		return "pw", false, nil
	})
	fake := &secretsFakeBackend{listFailN: 1} // fails once, then succeeds
	b := newTestSecretBackend(a, "prof", fake)

	out, err := b.List(context.Background())
	if err != nil {
		t.Fatalf("List() error = %v, want nil after one retry", err)
	}
	if len(out) != 1 {
		t.Errorf("List() = %v, want one summary", out)
	}
	if fake.listCalls != 2 {
		t.Errorf("inner List called %d times, want 2 (initial failure + one retry)", fake.listCalls)
	}
	if revealCalls != 2 {
		t.Errorf("revealSecret called %d times, want 2 (initial resolve + refresh after failure)", revealCalls)
	}
}

func TestSecretBackendListSurfacesErrorAfterRetryExhausted(t *testing.T) {
	a := newTestAppForSecrets("prof", func(context.Context, string, string) (string, bool, error) {
		return "pw", false, nil
	})
	fake := &secretsFakeBackend{listFailN: 100} // always fails
	b := newTestSecretBackend(a, "prof", fake)

	if _, err := b.List(context.Background()); err == nil {
		t.Fatal("List() error = nil, want the error surfaced after the retry also fails")
	}
	if fake.listCalls != 2 {
		t.Errorf("inner List called %d times, want exactly 2 (initial + one retry, no more)", fake.listCalls)
	}
}

func TestSecretBackendRemoveMessageDoesNotRetryButInvalidatesCache(t *testing.T) {
	revealCalls := 0
	a := newTestAppForSecrets("prof", func(context.Context, string, string) (string, bool, error) {
		revealCalls++
		return "pw", false, nil
	})
	fake := &secretsFakeBackend{removeFails: true}
	b := newTestSecretBackend(a, "prof", fake)

	if err := b.RemoveMessage(context.Background(), "q1", "m1"); err == nil {
		t.Fatal("RemoveMessage() error = nil, want the injected failure surfaced")
	}
	if fake.removeCalls != 1 {
		t.Errorf("inner RemoveMessage called %d times, want exactly 1 (no silent retry of a mutating call)", fake.removeCalls)
	}
	if revealCalls != 1 {
		t.Fatalf("revealSecret called %d times before the next call, want 1 (initial resolve only so far)", revealCalls)
	}

	// The cache should have been invalidated by the failed write, so the
	// next call (via current()) re-resolves.
	if _, err := b.current(context.Background()); err != nil {
		t.Fatalf("current() error = %v", err)
	}
	if revealCalls != 2 {
		t.Errorf("revealSecret called %d times after a following current(), want 2 (cache invalidated by the failed write)", revealCalls)
	}
}

// TestSecretBackendCurrentUsesCapturedProfileNotLiveConfig is a
// regression test for the bug fixed in spec/88: current() must use the
// profile captured at construction, not whatever a.cfg.ActiveAWSProfile
// happens to be at call time.
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
