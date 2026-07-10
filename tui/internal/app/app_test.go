package app

import (
	"testing"

	"github.com/gdamore/tcell/v2"
)

func TestNewRegistersViewsWithSecretsDefault(t *testing.T) {
	a := New()

	wantNames := []string{"secrets", "params", "queues"}
	if len(a.views) != len(wantNames) {
		t.Fatalf("len(views) = %d, want %d", len(a.views), len(wantNames))
	}
	for i, v := range a.views {
		if got := v.Name(); got != wantNames[i] {
			t.Errorf("views[%d].Name() = %q, want %q", i, got, wantNames[i])
		}
	}

	if name, _ := a.pages.GetFrontPage(); name != "secrets" {
		t.Errorf("front page = %q, want %q", name, "secrets")
	}
}

func TestSwitchTo(t *testing.T) {
	a := New()

	a.switchTo("queues")
	if name, _ := a.pages.GetFrontPage(); name != "queues" {
		t.Fatalf("front page after switchTo(\"queues\") = %q, want %q", name, "queues")
	}

	a.switchTo("bogus")
	if name, _ := a.pages.GetFrontPage(); name != "queues" {
		t.Errorf("front page after switchTo(\"bogus\") = %q, want unchanged %q", name, "queues")
	}
}

func TestOnGlobalKeyFocusesPromptOnColon(t *testing.T) {
	a := New()
	a.tv.SetFocus(a.pages)

	event := tcell.NewEventKey(tcell.KeyRune, ':', tcell.ModNone)
	if got := a.onGlobalKey(event); got != nil {
		t.Errorf("onGlobalKey(':') returned %v, want nil", got)
	}
	if a.tv.GetFocus() != a.prompt {
		t.Errorf("focus after ':' = %v, want prompt", a.tv.GetFocus())
	}
	if name, _ := a.topLeft.GetFrontPage(); name != "prompt" {
		t.Errorf("topLeft front page after ':' = %q, want %q", name, "prompt")
	}
}

func TestOnGlobalKeyPassesThroughOtherKeys(t *testing.T) {
	a := New()
	a.tv.SetFocus(a.pages)

	event := tcell.NewEventKey(tcell.KeyRune, 'x', tcell.ModNone)
	if got := a.onGlobalKey(event); got != event {
		t.Errorf("onGlobalKey('x') = %v, want event passed through unchanged", got)
	}
	if name, _ := a.topLeft.GetFrontPage(); name != "info" {
		t.Errorf("topLeft front page after 'x' = %q, want unchanged %q", name, "info")
	}
}

func TestOnGlobalKeyPassesThroughWhenPromptFocused(t *testing.T) {
	a := New()
	a.tv.SetFocus(a.prompt)

	event := tcell.NewEventKey(tcell.KeyRune, ':', tcell.ModNone)
	if got := a.onGlobalKey(event); got != event {
		t.Errorf("onGlobalKey(':') while prompt focused = %v, want event passed through unchanged", got)
	}
}

func TestOnPromptDoneQuit(t *testing.T) {
	a := New()
	a.prompt.SetText("quit")

	a.onPromptDone(tcell.KeyEnter)

	if got := a.prompt.GetText(); got != "" {
		t.Errorf("prompt text after quit = %q, want empty", got)
	}
	if want := a.pages.GetPage("secrets"); a.tv.GetFocus() != want {
		t.Errorf("focus after quit = %v, want front page's primitive %v", a.tv.GetFocus(), want)
	}
	if name, _ := a.topLeft.GetFrontPage(); name != "info" {
		t.Errorf("topLeft front page after quit = %q, want %q", name, "info")
	}
}

func TestOnPromptDoneSwitchesToKnownView(t *testing.T) {
	a := New()
	a.prompt.SetText("params")

	a.onPromptDone(tcell.KeyEnter)

	if name, _ := a.pages.GetFrontPage(); name != "params" {
		t.Errorf("front page after command %q = %q, want %q", "params", name, "params")
	}
	if got := a.prompt.GetText(); got != "" {
		t.Errorf("prompt text after Enter = %q, want empty", got)
	}
	if want := a.pages.GetPage("params"); a.tv.GetFocus() != want {
		t.Errorf("focus after Enter = %v, want front page's primitive %v", a.tv.GetFocus(), want)
	}
	if name, _ := a.topLeft.GetFrontPage(); name != "info" {
		t.Errorf("topLeft front page after Enter = %q, want %q", name, "info")
	}
}

func TestOnPromptDoneUnknownCommandLeavesViewUnchanged(t *testing.T) {
	a := New()
	a.switchTo("queues")
	a.prompt.SetText("bogus")

	a.onPromptDone(tcell.KeyEnter)

	if name, _ := a.pages.GetFrontPage(); name != "queues" {
		t.Errorf("front page after unknown command = %q, want unchanged %q", name, "queues")
	}
	if name, _ := a.topLeft.GetFrontPage(); name != "info" {
		t.Errorf("topLeft front page after unknown command = %q, want %q", name, "info")
	}
}

func TestOnPromptDoneNonEnterReturnsFocusWithoutSwitching(t *testing.T) {
	a := New()
	a.switchTo("queues")
	a.prompt.SetText("params")
	a.topLeft.SwitchToPage("prompt")
	a.tv.SetFocus(a.prompt)

	a.onPromptDone(tcell.KeyEscape)

	if name, _ := a.pages.GetFrontPage(); name != "queues" {
		t.Errorf("front page after Escape = %q, want unchanged %q", name, "queues")
	}
	if got := a.prompt.GetText(); got != "" {
		t.Errorf("prompt text after Escape = %q, want empty", got)
	}
	if want := a.pages.GetPage("queues"); a.tv.GetFocus() != want {
		t.Errorf("focus after Escape = %v, want front page's primitive %v", a.tv.GetFocus(), want)
	}
	if name, _ := a.topLeft.GetFrontPage(); name != "info" {
		t.Errorf("topLeft front page after Escape = %q, want %q", name, "info")
	}
}
