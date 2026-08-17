package secretbackend

import (
	"context"
	"errors"
	"testing"

	"github.com/ePex/cloudtui/tui/internal/config"
	"github.com/ePex/cloudtui/tui/internal/queue"
)

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

func TestResolveCachesAcrossCalls(t *testing.T) {
	calls := 0
	r := NewSecretResolver(func(context.Context, string, string) (string, bool, error) {
		calls++
		return "s3cr3t", false, nil
	})

	for i := 0; i < 2; i++ {
		v, err := r.Resolve(context.Background(), "prof", "my-secret")
		if err != nil {
			t.Fatalf("Resolve() error = %v", err)
		}
		if v != "s3cr3t" {
			t.Errorf("Resolve() = %q, want %q", v, "s3cr3t")
		}
	}
	if calls != 1 {
		t.Errorf("reveal called %d times, want 1 (second call should hit the cache)", calls)
	}
}

func TestResolveRejectsBinarySecret(t *testing.T) {
	r := NewSecretResolver(func(context.Context, string, string) (string, bool, error) {
		return "", true, nil
	})

	if _, err := r.Resolve(context.Background(), "prof", "my-secret"); err == nil {
		t.Fatal("Resolve() error = nil, want an error for a binary-valued secret")
	}
}

// fakeBackend implements queue.Backend with per-call-name failure
// injection, so tests can drive Backend's retry/no-retry paths without a
// real jolokia/proxy client or HTTP server.
type fakeBackend struct {
	listCalls   int
	listFailN   int // List fails for the first listFailN calls
	removeCalls int
	removeFails bool
}

func (f *fakeBackend) List(ctx context.Context) ([]queue.Summary, error) {
	f.listCalls++
	if f.listCalls <= f.listFailN {
		return nil, errors.New("boom")
	}
	return []queue.Summary{{Name: "q1"}}, nil
}
func (f *fakeBackend) BrowseMessages(ctx context.Context, queueName string, filter queue.MessageFilter) ([]queue.Message, error) {
	return nil, nil
}
func (f *fakeBackend) PurgeQueue(ctx context.Context, queueName string) error { return nil }
func (f *fakeBackend) RemoveMessage(ctx context.Context, queueName, messageID string) error {
	f.removeCalls++
	if f.removeFails {
		return errors.New("boom")
	}
	return nil
}
func (f *fakeBackend) MoveMessage(ctx context.Context, sourceQueue, messageID, targetQueue string) error {
	return nil
}
func (f *fakeBackend) MoveAllMessages(ctx context.Context, sourceQueue, targetQueue string) (int, error) {
	return 0, nil
}
func (f *fakeBackend) SendMessage(ctx context.Context, queueName, body string) error {
	return nil
}
func (f *fakeBackend) DeleteMessages(ctx context.Context, queueName string, filter queue.MessageFilter) (int, error) {
	return 0, nil
}
func (f *fakeBackend) MoveMessages(ctx context.Context, sourceQueue, targetQueue string, filter queue.MessageFilter) (int, error) {
	return 0, nil
}

func newTestBackend(resolver *SecretResolver, profile string, fake *fakeBackend) *Backend {
	return &Backend{
		resolver:   resolver,
		conn:       config.Connection{Name: "test", Backend: "jolokia", Queue: config.QueueConfig{PasswordSecret: "my-secret"}},
		secretName: "my-secret",
		profile:    profile,
		build:      func(config.Connection) queue.Backend { return fake },
	}
}

func TestBackendListRetriesOnceOnFailure(t *testing.T) {
	revealCalls := 0
	r := NewSecretResolver(func(context.Context, string, string) (string, bool, error) {
		revealCalls++
		return "pw", false, nil
	})
	fake := &fakeBackend{listFailN: 1} // fails once, then succeeds
	b := newTestBackend(r, "prof", fake)

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
		t.Errorf("reveal called %d times, want 2 (initial resolve + refresh after failure)", revealCalls)
	}
}

func TestBackendListSurfacesErrorAfterRetryExhausted(t *testing.T) {
	r := NewSecretResolver(func(context.Context, string, string) (string, bool, error) {
		return "pw", false, nil
	})
	fake := &fakeBackend{listFailN: 100} // always fails
	b := newTestBackend(r, "prof", fake)

	if _, err := b.List(context.Background()); err == nil {
		t.Fatal("List() error = nil, want the error surfaced after the retry also fails")
	}
	if fake.listCalls != 2 {
		t.Errorf("inner List called %d times, want exactly 2 (initial + one retry, no more)", fake.listCalls)
	}
}

func TestBackendRemoveMessageDoesNotRetryButInvalidatesCache(t *testing.T) {
	revealCalls := 0
	r := NewSecretResolver(func(context.Context, string, string) (string, bool, error) {
		revealCalls++
		return "pw", false, nil
	})
	fake := &fakeBackend{removeFails: true}
	b := newTestBackend(r, "prof", fake)

	if err := b.RemoveMessage(context.Background(), "q1", "m1"); err == nil {
		t.Fatal("RemoveMessage() error = nil, want the injected failure surfaced")
	}
	if fake.removeCalls != 1 {
		t.Errorf("inner RemoveMessage called %d times, want exactly 1 (no silent retry of a mutating call)", fake.removeCalls)
	}
	if revealCalls != 1 {
		t.Fatalf("reveal called %d times before the next call, want 1 (initial resolve only so far)", revealCalls)
	}

	// The cache should have been invalidated by the failed write, so the
	// next call (via current()) re-resolves.
	if _, err := b.current(context.Background()); err != nil {
		t.Fatalf("current() error = %v", err)
	}
	if revealCalls != 2 {
		t.Errorf("reveal called %d times after a following current(), want 2 (cache invalidated by the failed write)", revealCalls)
	}
}
