package app

import (
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
	for _, sc := range a.paramDetailV.Shortcuts() {
		if sc.Key == "r" {
			t.Error("Shortcuts() still lists \"r\" (reveal) after the value was revealed")
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

func TestParamDetailViewShortcutsIncludeCopyOnlyWhenValuePresent(t *testing.T) {
	a := New(config.Default())
	a.paramDetailV.param = awsssm.Parameter{Name: "/app/secret", Type: awsssm.TypeSecureString, Value: ""}

	for _, sc := range a.paramDetailV.Shortcuts() {
		if sc.Key == "c" {
			t.Error("Shortcuts() lists \"c\" (copy) for an unrevealed SecureString with no value to copy")
		}
	}

	a.paramDetailV.param.Value = "revealed-value"
	found := false
	for _, sc := range a.paramDetailV.Shortcuts() {
		if sc.Key == "c" {
			found = true
		}
	}
	if !found {
		t.Error("Shortcuts() missing \"c\" (copy) once a value is present")
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

func TestParamDetailViewCopyNoOpWithoutValue(t *testing.T) {
	a := New(config.Default())
	screen := tcell.NewSimulationScreen("")
	if err := screen.Init(); err != nil {
		t.Fatalf("screen.Init: %v", err)
	}
	a.screen = screen
	a.openParamDetail(awsssm.Parameter{Name: "/app/secret", Type: awsssm.TypeSecureString, Value: ""})

	capture := a.paramDetailV.textView.GetInputCapture()
	capture(tcell.NewEventKey(tcell.KeyRune, 'c', tcell.ModNone))

	if got := screen.GetClipboardData(); got != nil {
		t.Errorf("clipboard = %q, want untouched (nil) for an unrevealed SecureString", got)
	}
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
