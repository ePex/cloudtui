package view

import (
	"context"
	"strings"
	"testing"

	"github.com/gdamore/tcell/v2"

	"github.com/ePex/cloudtui/tui/internal/awssecrets"
)

func newTestSecretDetailView(t *testing.T) (*fakeViewHost, *SecretDetailView) {
	t.Helper()
	host := newFakeViewHost()
	return host, NewSecretDetailView(host, func() {})
}

func TestSecretDetailViewRenderShowsMaskedBeforeReveal(t *testing.T) {
	_, dv := newTestSecretDetailView(t)

	dv.Render(awssecrets.Secret{Name: "/app/db", ARN: "arn:aws:secretsmanager:...:secret:/app/db"})

	text := dv.textView.GetText(true)
	for _, want := range []string{"/app/db", "reveal"} {
		if !strings.Contains(text, want) {
			t.Errorf("detail text = %q, want it to contain %q", text, want)
		}
	}
	if got, want := dv.textView.GetTitle(), " Secret — /app/db "; got != want {
		t.Errorf("title = %q, want %q", got, want)
	}
}

func TestSecretDetailViewRenderShowsValueAfterReveal(t *testing.T) {
	_, dv := newTestSecretDetailView(t)
	dv.Render(awssecrets.Secret{Name: "/app/db"})

	dv.revealed = true
	dv.displayValue = "hello"
	dv.renderBody()

	text := dv.textView.GetText(true)
	if !strings.Contains(text, "hello") {
		t.Errorf("detail text = %q, want it to contain the revealed value", text)
	}
}

func TestSecretDetailViewRenderShowsBinaryMessageAfterReveal(t *testing.T) {
	_, dv := newTestSecretDetailView(t)
	dv.Render(awssecrets.Secret{Name: "/app/blob"})

	dv.revealed = true
	dv.isBinary = true
	dv.renderBody()

	text := dv.textView.GetText(true)
	if !strings.Contains(text, "binary secret") {
		t.Errorf("detail text = %q, want it to mention a binary secret", text)
	}
}

func TestSecretDetailViewShortcutsIncludeRevealOnlyBeforeReveal(t *testing.T) {
	_, dv := newTestSecretDetailView(t)
	dv.secret = awssecrets.Secret{Name: "/app/db"}

	found := false
	for _, sc := range dv.Shortcuts() {
		if sc.Key == "r" {
			found = true
		}
	}
	if !found {
		t.Error("Shortcuts() missing \"r\" (reveal) before reveal")
	}

	dv.revealed = true
	for _, sc := range dv.Shortcuts() {
		if sc.Key == "r" {
			t.Error("Shortcuts() still lists \"r\" (reveal) after reveal")
		}
	}
}

// TestSecretDetailViewShortcutsIncludeCopyImmediately locks in the
// behavior the user asked for: 'c' must be available the moment the
// detail view opens, before any reveal — copying a secret shouldn't
// require displaying it first.
func TestSecretDetailViewShortcutsIncludeCopyImmediately(t *testing.T) {
	_, dv := newTestSecretDetailView(t)
	dv.secret = awssecrets.Secret{Name: "/app/db"}

	found := false
	for _, sc := range dv.Shortcuts() {
		if sc.Key == "c" {
			found = true
		}
	}
	if !found {
		t.Error("Shortcuts() missing \"c\" (copy) before any reveal has happened")
	}
}

func TestSecretDetailViewShortcutsHideCopyOnceKnownBinary(t *testing.T) {
	_, dv := newTestSecretDetailView(t)
	dv.secret = awssecrets.Secret{Name: "/app/blob"}
	dv.fetched = true
	dv.isBinary = true

	for _, sc := range dv.Shortcuts() {
		if sc.Key == "c" {
			t.Error("Shortcuts() lists \"c\" (copy) once a secret is known to be binary — nothing to copy, ever")
		}
	}
}

// TestSecretDetailViewCopyWritesDisplayValueToClipboard exercises the
// "already fetched" fast path (e.g. after a prior 'r'): pressing 'c'
// copies straight from the cache, no fetch needed.
func TestSecretDetailViewCopyWritesDisplayValueToClipboard(t *testing.T) {
	host, dv := newTestSecretDetailView(t)
	dv.Render(awssecrets.Secret{Name: "/app/db"})
	dv.fetched = true
	dv.revealed = true
	dv.displayValue = "hello"

	capture := dv.textView.GetInputCapture()
	capture(tcell.NewEventKey(tcell.KeyRune, 'c', tcell.ModNone))

	if got := host.copiedData; got != "hello" {
		t.Errorf("copied data = %q, want %q", got, "hello")
	}
	if got := host.status; !strings.Contains(got, "/app/db") {
		t.Errorf("status = %q, want it to mention the secret name", got)
	}
	if strings.Contains(host.status, "hello") {
		t.Error("status must never show the copied value itself")
	}
}

// TestSecretDetailViewCopyFetchedValueRejectsBinary exercises the
// "already fetched, turned out binary" fast path directly (skipping the
// fetch goroutine, per this file's established testing constraint — see
// TestHandleFetchResult for the async plumbing itself).
func TestSecretDetailViewCopyFetchedValueRejectsBinary(t *testing.T) {
	host, dv := newTestSecretDetailView(t)
	dv.Render(awssecrets.Secret{Name: "/app/blob"})
	dv.fetched = true
	dv.isBinary = true

	dv.copyFetchedValue()

	if got := host.copiedData; got != "" {
		t.Errorf("copied data = %q, want untouched (empty) for a binary secret", got)
	}
	if got := host.status; !strings.Contains(got, "/app/blob") {
		t.Errorf("status = %q, want it to mention the secret name", got)
	}
}

// TestHandleFetchResult exercises the fetch outcome logic directly,
// bypassing the goroutine + QueueUpdateDraw in fetchThen (which would
// otherwise block forever without a running tview event loop — see
// ssmParamsView/queuesView's tests for the same established constraint).
// This is what actually proves the "'c' before reveal fetches, then
// copies, without displaying" behavior end to end.
func TestHandleFetchResult(t *testing.T) {
	t.Run("success copies without revealing", func(t *testing.T) {
		host, dv := newTestSecretDetailView(t)
		dv.Render(awssecrets.Secret{Name: "/app/db"})

		dv.handleFetchResult("hello", false, nil, dv.copyFetchedValue)

		if got := host.copiedData; got != "hello" {
			t.Errorf("copied data = %q, want %q", got, "hello")
		}
		if dv.revealed {
			t.Error("revealed = true, want false: copying must not display the value on screen")
		}
		text := dv.textView.GetText(true)
		if strings.Contains(text, "hello") {
			t.Error("detail text must not contain the value after a copy-only fetch")
		}
		if !strings.Contains(text, "reveal") {
			t.Error("detail text should still show the masked prompt after a copy-only fetch")
		}
	})

	t.Run("success then reveal uses the cached value, no re-fetch", func(t *testing.T) {
		host, dv := newTestSecretDetailView(t)
		dv.Render(awssecrets.Secret{Name: "/app/db"})
		dv.handleFetchResult(`{"a":1}`, false, nil, dv.copyFetchedValue)

		calls := 0
		host.revealSecretFn = func(context.Context, string, string) (string, bool, error) {
			calls++
			return "", false, nil
		}
		dv.reveal()

		if calls != 0 {
			t.Error("reveal() re-fetched despite the value already being cached from a prior copy")
		}
		if !dv.revealed {
			t.Error("revealed = false, want true after reveal() on an already-fetched secret")
		}
		if got := dv.textView.GetText(true); !strings.Contains(got, `"a": 1`) {
			t.Errorf("detail text = %q, want it to show the cached (pretty-printed) value", got)
		}
	})

	t.Run("binary secret is cached masked, and Shortcuts drops copy", func(t *testing.T) {
		_, dv := newTestSecretDetailView(t)
		dv.Render(awssecrets.Secret{Name: "/app/blob"})

		called := false
		dv.handleFetchResult("", true, nil, func() { called = true })

		if !called {
			t.Error("onSuccess was not invoked for a successful binary fetch")
		}
		if !dv.fetched || !dv.isBinary {
			t.Error("fetched/isBinary not set correctly for a binary secret")
		}
		for _, sc := range dv.Shortcuts() {
			if sc.Key == "c" {
				t.Error("Shortcuts() still lists \"c\" after discovering the secret is binary")
			}
		}
	})

	t.Run("error logs and shows status, does not call onSuccess", func(t *testing.T) {
		host, dv := newTestSecretDetailView(t)
		dv.Render(awssecrets.Secret{Name: "/app/db"})

		called := false
		dv.handleFetchResult("", false, context.DeadlineExceeded, func() { called = true })

		if called {
			t.Error("onSuccess was invoked despite an error")
		}
		if dv.fetched {
			t.Error("fetched = true, want false after an error")
		}
		if got := host.status; !strings.Contains(got, "deadline exceeded") {
			t.Errorf("status = %q, want it to contain the error", got)
		}
	})
}

func TestPrettyPrintJSON(t *testing.T) {
	cases := []struct {
		name string
		in   string
		want string
	}{
		{
			name: "object",
			in:   `{"username":"x","password":"y"}`,
			want: "{\n  \"password\": \"y\",\n  \"username\": \"x\"\n}",
		},
		{
			name: "array",
			in:   `[1,2,3]`,
			want: "[\n  1,\n  2,\n  3\n]",
		},
		{
			name: "plain string, not JSON",
			in:   "just a plain secret value",
			want: "just a plain secret value",
		},
		{
			name: "invalid JSON",
			in:   `{"unterminated":`,
			want: `{"unterminated":`,
		},
		{
			name: "empty string",
			in:   "",
			want: "",
		},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			if got := prettyPrintJSON(tc.in); got != tc.want {
				t.Errorf("prettyPrintJSON(%q) = %q, want %q", tc.in, got, tc.want)
			}
		})
	}
}
