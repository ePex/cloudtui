package view

import (
	"context"
	"strings"
	"testing"

	"github.com/gdamore/tcell/v2"

	"github.com/ePex/cloudtui/tui/internal/awsssm"
)

func newTestParamDetailView(t *testing.T) (*fakeViewHost, *ParamDetailView) {
	t.Helper()
	host := newFakeViewHost()
	return host, NewParamDetailView(host, func() {})
}

func TestParamDetailViewRenderShowsStringValueImmediately(t *testing.T) {
	_, dv := newTestParamDetailView(t)

	dv.Render(awsssm.Parameter{Name: "/app/name", Type: awsssm.TypeString, Value: "hello"})

	text := dv.textView.GetText(true)
	for _, want := range []string{"/app/name", "String", "hello"} {
		if !strings.Contains(text, want) {
			t.Errorf("detail text = %q, want it to contain %q", text, want)
		}
	}
	if got, want := dv.textView.GetTitle(), " Parameter — /app/name "; got != want {
		t.Errorf("title = %q, want %q", got, want)
	}
}

func TestParamDetailViewRenderMasksUnrevealedSecureString(t *testing.T) {
	_, dv := newTestParamDetailView(t)

	dv.Render(awsssm.Parameter{Name: "/app/secret", Type: awsssm.TypeSecureString, Value: ""})

	text := dv.textView.GetText(true)
	if !strings.Contains(text, "reveal") {
		t.Errorf("detail text = %q, want it to prompt to reveal", text)
	}
}

func TestParamDetailViewShortcutsIncludeRevealOnlyBeforeReveal(t *testing.T) {
	_, dv := newTestParamDetailView(t)
	dv.param = awsssm.Parameter{Name: "/app/secret", Type: awsssm.TypeSecureString, Value: ""}

	found := false
	for _, sc := range dv.Shortcuts() {
		if sc.Key == "r" {
			found = true
		}
	}
	if !found {
		t.Error("Shortcuts() missing \"r\" (reveal) for an unrevealed SecureString")
	}

	dv.param.Value = "revealed-value"
	dv.displayed = true
	for _, sc := range dv.Shortcuts() {
		if sc.Key == "r" {
			t.Error("Shortcuts() still lists \"r\" (reveal) after the value was displayed")
		}
	}
}

func TestParamDetailViewShortcutsExcludeRevealForStringType(t *testing.T) {
	_, dv := newTestParamDetailView(t)
	dv.param = awsssm.Parameter{Name: "/app/name", Type: awsssm.TypeString, Value: "hello"}

	for _, sc := range dv.Shortcuts() {
		if sc.Key == "r" {
			t.Error("Shortcuts() lists \"r\" (reveal) for a non-SecureString parameter")
		}
	}
}

// TestParamDetailViewShortcutsIncludeCopyImmediately locks in the
// behavior the user asked for: 'c' must be available the moment the
// detail view opens, before any reveal — copying a SecureString
// shouldn't require displaying it first.
func TestParamDetailViewShortcutsIncludeCopyImmediately(t *testing.T) {
	_, dv := newTestParamDetailView(t)
	dv.param = awsssm.Parameter{Name: "/app/secret", Type: awsssm.TypeSecureString, Value: ""}

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

func TestParamDetailViewCopyWritesValueToClipboard(t *testing.T) {
	host, dv := newTestParamDetailView(t)
	dv.Render(awsssm.Parameter{Name: "/app/name", Type: awsssm.TypeString, Value: "hello"})

	capture := dv.textView.GetInputCapture()
	capture(tcell.NewEventKey(tcell.KeyRune, 'c', tcell.ModNone))

	if got := host.copiedData; got != "hello" {
		t.Errorf("copied data = %q, want %q", got, "hello")
	}
	if got := host.status; !strings.Contains(got, "/app/name") {
		t.Errorf("status = %q, want it to mention the parameter name", got)
	}
	if strings.Contains(host.status, "hello") {
		t.Error("status must never show the copied value itself")
	}
}

// TestParamDetailViewHandleFetchResult exercises the fetch outcome logic
// directly, bypassing the goroutine + QueueUpdateDraw in fetchThen (which
// would otherwise block forever without a running tview event loop — see
// ssmParamsView/queuesView's tests for the same established constraint).
// This is what actually proves the "'c' before reveal fetches, then
// copies, without displaying" behavior end to end for a SecureString.
func TestParamDetailViewHandleFetchResult(t *testing.T) {
	t.Run("success copies without displaying", func(t *testing.T) {
		host, dv := newTestParamDetailView(t)
		dv.Render(awsssm.Parameter{Name: "/app/secret", Type: awsssm.TypeSecureString, Value: ""})

		dv.handleFetchResult("hello", nil, dv.copyFetchedValue)

		if got := host.copiedData; got != "hello" {
			t.Errorf("copied data = %q, want %q", got, "hello")
		}
		if dv.displayed {
			t.Error("displayed = true, want false: copying must not display the value on screen")
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
		host, dv := newTestParamDetailView(t)
		dv.Render(awsssm.Parameter{Name: "/app/secret", Type: awsssm.TypeSecureString, Value: ""})
		dv.handleFetchResult("hello", nil, dv.copyFetchedValue)

		calls := 0
		host.revealParameterFn = func(context.Context, string, string) (string, error) {
			calls++
			return "", nil
		}
		dv.reveal()

		if calls != 0 {
			t.Error("reveal() re-fetched despite the value already being cached from a prior copy")
		}
		if !dv.displayed {
			t.Error("displayed = false, want true after reveal() on an already-fetched parameter")
		}
		if got := dv.textView.GetText(true); !strings.Contains(got, "hello") {
			t.Errorf("detail text = %q, want it to show the cached value", got)
		}
	})

	t.Run("error logs and shows status, does not call onSuccess", func(t *testing.T) {
		host, dv := newTestParamDetailView(t)
		dv.Render(awsssm.Parameter{Name: "/app/secret", Type: awsssm.TypeSecureString, Value: ""})

		called := false
		dv.handleFetchResult("", context.DeadlineExceeded, func() { called = true })

		if called {
			t.Error("onSuccess was invoked despite an error")
		}
		if dv.param.Value != "" {
			t.Error("param.Value set despite an error")
		}
		if got := host.status; !strings.Contains(got, "deadline exceeded") {
			t.Errorf("status = %q, want it to contain the error", got)
		}
	})
}
