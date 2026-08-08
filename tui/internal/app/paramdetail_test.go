package app

import (
	"context"
	"strings"
	"testing"

	"github.com/gdamore/tcell/v2"

	"github.com/ePex/cloudtui/tui/internal/awsssm"
	"github.com/ePex/cloudtui/tui/internal/config"
)

func TestParamDetailViewRenderShowsStringValueImmediately(t *testing.T) {
	a := New(config.Default())

	a.paramDetailV.render(awsssm.Parameter{Name: "/app/name", Type: awsssm.TypeString, Value: "hello"})

	text := a.paramDetailV.textView.GetText(true)
	for _, want := range []string{"/app/name", "String", "hello"} {
		if !strings.Contains(text, want) {
			t.Errorf("detail text = %q, want it to contain %q", text, want)
		}
	}
}

func TestParamDetailViewRenderMasksUnrevealedSecureString(t *testing.T) {
	a := New(config.Default())

	a.paramDetailV.render(awsssm.Parameter{Name: "/app/secret", Type: awsssm.TypeSecureString, Value: ""})

	text := a.paramDetailV.textView.GetText(true)
	if !strings.Contains(text, "reveal") {
		t.Errorf("detail text = %q, want it to prompt to reveal", text)
	}
}

func TestParamDetailViewShortcutsIncludeRevealOnlyBeforeReveal(t *testing.T) {
	a := New(config.Default())
	a.paramDetailV.param = awsssm.Parameter{Name: "/app/secret", Type: awsssm.TypeSecureString, Value: ""}

	found := false
	for _, sc := range a.paramDetailV.Shortcuts() {
		if sc.Key == "r" {
			found = true
		}
	}
	if !found {
		t.Error("Shortcuts() missing \"r\" (reveal) for an unrevealed SecureString")
	}

	a.paramDetailV.param.Value = "revealed-value"
	a.paramDetailV.displayed = true
	for _, sc := range a.paramDetailV.Shortcuts() {
		if sc.Key == "r" {
			t.Error("Shortcuts() still lists \"r\" (reveal) after the value was displayed")
		}
	}
}

func TestParamDetailViewShortcutsExcludeRevealForStringType(t *testing.T) {
	a := New(config.Default())
	a.paramDetailV.param = awsssm.Parameter{Name: "/app/name", Type: awsssm.TypeString, Value: "hello"}

	for _, sc := range a.paramDetailV.Shortcuts() {
		if sc.Key == "r" {
			t.Error("Shortcuts() lists \"r\" (reveal) for a non-SecureString parameter")
		}
	}
}

func TestOpenParamDetailSwitchesPageAndSetsTitle(t *testing.T) {
	a := New(config.Default())

	a.openParamDetail(awsssm.Parameter{Name: "/app/name", Type: awsssm.TypeString, Value: "hello"})

	if name, _ := a.pages.GetFrontPage(); name != "ssm-param-detail" {
		t.Errorf("front page = %q, want %q", name, "ssm-param-detail")
	}
	if got, want := a.paramDetailV.textView.GetTitle(), " Parameter — /app/name "; got != want {
		t.Errorf("title = %q, want %q", got, want)
	}
}

// TestParamDetailViewShortcutsIncludeCopyImmediately locks in the
// behavior the user asked for: 'c' must be available the moment the
// detail view opens, before any reveal — copying a SecureString
// shouldn't require displaying it first.
func TestParamDetailViewShortcutsIncludeCopyImmediately(t *testing.T) {
	a := New(config.Default())
	a.paramDetailV.param = awsssm.Parameter{Name: "/app/secret", Type: awsssm.TypeSecureString, Value: ""}

	found := false
	for _, sc := range a.paramDetailV.Shortcuts() {
		if sc.Key == "c" {
			found = true
		}
	}
	if !found {
		t.Error("Shortcuts() missing \"c\" (copy) before any reveal has happened")
	}
}

func TestParamDetailViewCopyWritesValueToClipboard(t *testing.T) {
	a := New(config.Default())
	screen := tcell.NewSimulationScreen("")
	if err := screen.Init(); err != nil {
		t.Fatalf("screen.Init: %v", err)
	}
	a.screen = screen
	a.openParamDetail(awsssm.Parameter{Name: "/app/name", Type: awsssm.TypeString, Value: "hello"})

	capture := a.paramDetailV.textView.GetInputCapture()
	capture(tcell.NewEventKey(tcell.KeyRune, 'c', tcell.ModNone))

	if got := string(screen.GetClipboardData()); got != "hello" {
		t.Errorf("clipboard = %q, want %q", got, "hello")
	}
	if got := a.statusBar.GetText(true); !strings.Contains(got, "/app/name") {
		t.Errorf("status bar = %q, want it to mention the parameter name", got)
	}
	if strings.Contains(a.statusBar.GetText(true), "hello") {
		t.Error("status bar must never show the copied value itself")
	}
}

// TestHandleFetchResult exercises the fetch outcome logic directly,
// bypassing the goroutine + QueueUpdateDraw in fetchThen (which would
// otherwise block forever without a running tview event loop — see
// ssmParamsView/queuesView's tests for the same established constraint).
// This is what actually proves the "'c' before reveal fetches, then
// copies, without displaying" behavior end to end for a SecureString.
func TestParamDetailViewHandleFetchResult(t *testing.T) {
	t.Run("success copies without displaying", func(t *testing.T) {
		a := New(config.Default())
		screen := tcell.NewSimulationScreen("")
		if err := screen.Init(); err != nil {
			t.Fatalf("screen.Init: %v", err)
		}
		a.screen = screen
		a.openParamDetail(awsssm.Parameter{Name: "/app/secret", Type: awsssm.TypeSecureString, Value: ""})

		a.paramDetailV.handleFetchResult("hello", nil, a.paramDetailV.copyFetchedValue)

		if got := string(screen.GetClipboardData()); got != "hello" {
			t.Errorf("clipboard = %q, want %q", got, "hello")
		}
		if a.paramDetailV.displayed {
			t.Error("displayed = true, want false: copying must not display the value on screen")
		}
		text := a.paramDetailV.textView.GetText(true)
		if strings.Contains(text, "hello") {
			t.Error("detail text must not contain the value after a copy-only fetch")
		}
		if !strings.Contains(text, "reveal") {
			t.Error("detail text should still show the masked prompt after a copy-only fetch")
		}
	})

	t.Run("success then reveal uses the cached value, no re-fetch", func(t *testing.T) {
		a := New(config.Default())
		a.openParamDetail(awsssm.Parameter{Name: "/app/secret", Type: awsssm.TypeSecureString, Value: ""})
		a.paramDetailV.handleFetchResult("hello", nil, a.paramDetailV.copyFetchedValue)

		calls := 0
		a.revealParameter = func(context.Context, string, string) (string, error) {
			calls++
			return "", nil
		}
		a.paramDetailV.reveal()

		if calls != 0 {
			t.Error("reveal() re-fetched despite the value already being cached from a prior copy")
		}
		if !a.paramDetailV.displayed {
			t.Error("displayed = false, want true after reveal() on an already-fetched parameter")
		}
		if got := a.paramDetailV.textView.GetText(true); !strings.Contains(got, "hello") {
			t.Errorf("detail text = %q, want it to show the cached value", got)
		}
	})

	t.Run("error logs and shows status, does not call onSuccess", func(t *testing.T) {
		a := New(config.Default())
		a.openParamDetail(awsssm.Parameter{Name: "/app/secret", Type: awsssm.TypeSecureString, Value: ""})

		called := false
		a.paramDetailV.handleFetchResult("", context.DeadlineExceeded, func() { called = true })

		if called {
			t.Error("onSuccess was invoked despite an error")
		}
		if a.paramDetailV.param.Value != "" {
			t.Error("param.Value set despite an error")
		}
		if got := a.statusBar.GetText(true); !strings.Contains(got, "deadline exceeded") {
			t.Errorf("status bar = %q, want it to contain the error", got)
		}
	})
}

func TestParamDetailViewEscReturnsToSSMParameters(t *testing.T) {
	a := New(config.Default())
	a.openParamDetail(awsssm.Parameter{Name: "/app/name", Type: awsssm.TypeString, Value: "hello"})

	capture := a.paramDetailV.textView.GetInputCapture()
	if capture == nil {
		t.Fatal("paramDetailV.textView has no input capture set")
	}
	capture(tcell.NewEventKey(tcell.KeyEscape, 0, tcell.ModNone))

	if name, _ := a.pages.GetFrontPage(); name != "ssm-parameters" {
		t.Errorf("front page after Esc = %q, want %q", name, "ssm-parameters")
	}
}
