package app

import (
	"context"
	"strings"
	"testing"

	"github.com/gdamore/tcell/v2"

	"github.com/ePex/cloudtui/tui/internal/awssecrets"
	"github.com/ePex/cloudtui/tui/internal/config"
)

func TestSecretDetailViewRenderShowsMaskedBeforeReveal(t *testing.T) {
	a := New(config.Default())

	a.secretDetailV.render(awssecrets.Secret{Name: "/app/db", ARN: "arn:aws:secretsmanager:...:secret:/app/db"})

	text := a.secretDetailV.textView.GetText(true)
	for _, want := range []string{"/app/db", "reveal"} {
		if !strings.Contains(text, want) {
			t.Errorf("detail text = %q, want it to contain %q", text, want)
		}
	}
}

func TestSecretDetailViewRenderShowsValueAfterReveal(t *testing.T) {
	a := New(config.Default())
	a.secretDetailV.render(awssecrets.Secret{Name: "/app/db"})

	a.secretDetailV.revealed = true
	a.secretDetailV.displayValue = "hello"
	a.secretDetailV.renderBody()

	text := a.secretDetailV.textView.GetText(true)
	if !strings.Contains(text, "hello") {
		t.Errorf("detail text = %q, want it to contain the revealed value", text)
	}
}

func TestSecretDetailViewRenderShowsBinaryMessageAfterReveal(t *testing.T) {
	a := New(config.Default())
	a.secretDetailV.render(awssecrets.Secret{Name: "/app/blob"})

	a.secretDetailV.revealed = true
	a.secretDetailV.isBinary = true
	a.secretDetailV.renderBody()

	text := a.secretDetailV.textView.GetText(true)
	if !strings.Contains(text, "binary secret") {
		t.Errorf("detail text = %q, want it to mention a binary secret", text)
	}
}

func TestSecretDetailViewShortcutsIncludeRevealOnlyBeforeReveal(t *testing.T) {
	a := New(config.Default())
	a.secretDetailV.secret = awssecrets.Secret{Name: "/app/db"}

	found := false
	for _, sc := range a.secretDetailV.Shortcuts() {
		if sc.Key == "r" {
			found = true
		}
	}
	if !found {
		t.Error("Shortcuts() missing \"r\" (reveal) before reveal")
	}

	a.secretDetailV.revealed = true
	for _, sc := range a.secretDetailV.Shortcuts() {
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
	a := New(config.Default())
	a.secretDetailV.secret = awssecrets.Secret{Name: "/app/db"}

	found := false
	for _, sc := range a.secretDetailV.Shortcuts() {
		if sc.Key == "c" {
			found = true
		}
	}
	if !found {
		t.Error("Shortcuts() missing \"c\" (copy) before any reveal has happened")
	}
}

func TestSecretDetailViewShortcutsHideCopyOnceKnownBinary(t *testing.T) {
	a := New(config.Default())
	a.secretDetailV.secret = awssecrets.Secret{Name: "/app/blob"}
	a.secretDetailV.fetched = true
	a.secretDetailV.isBinary = true

	for _, sc := range a.secretDetailV.Shortcuts() {
		if sc.Key == "c" {
			t.Error("Shortcuts() lists \"c\" (copy) once a secret is known to be binary — nothing to copy, ever")
		}
	}
}

func TestOpenSecretDetailSwitchesPageAndSetsTitle(t *testing.T) {
	a := New(config.Default())

	a.OpenSecretDetail(awssecrets.Secret{Name: "/app/db"})

	if name, _ := a.pages.GetFrontPage(); name != "secret-detail" {
		t.Errorf("front page = %q, want %q", name, "secret-detail")
	}
	if got, want := a.secretDetailV.textView.GetTitle(), " Secret — /app/db "; got != want {
		t.Errorf("title = %q, want %q", got, want)
	}
}

// TestSecretDetailViewCopyWritesDisplayValueToClipboard exercises the
// "already fetched" fast path (e.g. after a prior 'r'): pressing 'c'
// copies straight from the cache, no fetch needed.
func TestSecretDetailViewCopyWritesDisplayValueToClipboard(t *testing.T) {
	a := New(config.Default())
	screen := tcell.NewSimulationScreen("")
	if err := screen.Init(); err != nil {
		t.Fatalf("screen.Init: %v", err)
	}
	a.screen = screen
	a.OpenSecretDetail(awssecrets.Secret{Name: "/app/db"})
	a.secretDetailV.fetched = true
	a.secretDetailV.revealed = true
	a.secretDetailV.displayValue = "hello"

	capture := a.secretDetailV.textView.GetInputCapture()
	capture(tcell.NewEventKey(tcell.KeyRune, 'c', tcell.ModNone))

	if got := string(screen.GetClipboardData()); got != "hello" {
		t.Errorf("clipboard = %q, want %q", got, "hello")
	}
	if got := a.statusBar.GetText(true); !strings.Contains(got, "/app/db") {
		t.Errorf("status bar = %q, want it to mention the secret name", got)
	}
	if strings.Contains(a.statusBar.GetText(true), "hello") {
		t.Error("status bar must never show the copied value itself")
	}
}

// TestSecretDetailViewCopyFetchedValueRejectsBinary exercises the
// "already fetched, turned out binary" fast path directly (skipping the
// fetch goroutine, per this file's established testing constraint — see
// TestHandleFetchResult for the async plumbing itself).
func TestSecretDetailViewCopyFetchedValueRejectsBinary(t *testing.T) {
	a := New(config.Default())
	screen := tcell.NewSimulationScreen("")
	if err := screen.Init(); err != nil {
		t.Fatalf("screen.Init: %v", err)
	}
	a.screen = screen
	a.OpenSecretDetail(awssecrets.Secret{Name: "/app/blob"})
	a.secretDetailV.fetched = true
	a.secretDetailV.isBinary = true

	a.secretDetailV.copyFetchedValue()

	if got := screen.GetClipboardData(); got != nil {
		t.Errorf("clipboard = %q, want untouched (nil) for a binary secret", got)
	}
	if got := a.statusBar.GetText(true); !strings.Contains(got, "/app/blob") {
		t.Errorf("status bar = %q, want it to mention the secret name", got)
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
		a := New(config.Default())
		screen := tcell.NewSimulationScreen("")
		if err := screen.Init(); err != nil {
			t.Fatalf("screen.Init: %v", err)
		}
		a.screen = screen
		a.OpenSecretDetail(awssecrets.Secret{Name: "/app/db"})

		a.secretDetailV.handleFetchResult("hello", false, nil, a.secretDetailV.copyFetchedValue)

		if got := string(screen.GetClipboardData()); got != "hello" {
			t.Errorf("clipboard = %q, want %q", got, "hello")
		}
		if a.secretDetailV.revealed {
			t.Error("revealed = true, want false: copying must not display the value on screen")
		}
		text := a.secretDetailV.textView.GetText(true)
		if strings.Contains(text, "hello") {
			t.Error("detail text must not contain the value after a copy-only fetch")
		}
		if !strings.Contains(text, "reveal") {
			t.Error("detail text should still show the masked prompt after a copy-only fetch")
		}
	})

	t.Run("success then reveal uses the cached value, no re-fetch", func(t *testing.T) {
		a := New(config.Default())
		a.OpenSecretDetail(awssecrets.Secret{Name: "/app/db"})
		a.secretDetailV.handleFetchResult(`{"a":1}`, false, nil, a.secretDetailV.copyFetchedValue)

		calls := 0
		a.revealSecret = func(context.Context, string, string) (string, bool, error) {
			calls++
			return "", false, nil
		}
		a.secretDetailV.reveal()

		if calls != 0 {
			t.Error("reveal() re-fetched despite the value already being cached from a prior copy")
		}
		if !a.secretDetailV.revealed {
			t.Error("revealed = false, want true after reveal() on an already-fetched secret")
		}
		if got := a.secretDetailV.textView.GetText(true); !strings.Contains(got, `"a": 1`) {
			t.Errorf("detail text = %q, want it to show the cached (pretty-printed) value", got)
		}
	})

	t.Run("binary secret is cached masked, and Shortcuts drops copy", func(t *testing.T) {
		a := New(config.Default())
		a.OpenSecretDetail(awssecrets.Secret{Name: "/app/blob"})

		called := false
		a.secretDetailV.handleFetchResult("", true, nil, func() { called = true })

		if !called {
			t.Error("onSuccess was not invoked for a successful binary fetch")
		}
		if !a.secretDetailV.fetched || !a.secretDetailV.isBinary {
			t.Error("fetched/isBinary not set correctly for a binary secret")
		}
		for _, sc := range a.secretDetailV.Shortcuts() {
			if sc.Key == "c" {
				t.Error("Shortcuts() still lists \"c\" after discovering the secret is binary")
			}
		}
	})

	t.Run("error logs and shows status, does not call onSuccess", func(t *testing.T) {
		a := New(config.Default())
		a.OpenSecretDetail(awssecrets.Secret{Name: "/app/db"})

		called := false
		a.secretDetailV.handleFetchResult("", false, context.DeadlineExceeded, func() { called = true })

		if called {
			t.Error("onSuccess was invoked despite an error")
		}
		if a.secretDetailV.fetched {
			t.Error("fetched = true, want false after an error")
		}
		if got := a.statusBar.GetText(true); !strings.Contains(got, "deadline exceeded") {
			t.Errorf("status bar = %q, want it to contain the error", got)
		}
	})
}

func TestSecretDetailViewEscReturnsToSecretsManager(t *testing.T) {
	a := New(config.Default())
	a.OpenSecretDetail(awssecrets.Secret{Name: "/app/db"})

	capture := a.secretDetailV.textView.GetInputCapture()
	if capture == nil {
		t.Fatal("secretDetailV.textView has no input capture set")
	}
	capture(tcell.NewEventKey(tcell.KeyEscape, 0, tcell.ModNone))

	if name, _ := a.pages.GetFrontPage(); name != "secrets-manager" {
		t.Errorf("front page after Esc = %q, want %q", name, "secrets-manager")
	}
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
