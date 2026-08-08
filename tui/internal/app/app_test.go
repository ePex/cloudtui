package app

import (
	"context"
	"strings"
	"testing"

	"github.com/gdamore/tcell/v2"
	"github.com/rivo/tview"

	"github.com/ePex/cloudtui/tui/internal/awsprofile"
	"github.com/ePex/cloudtui/tui/internal/config"
	"github.com/ePex/cloudtui/tui/internal/queue"
	"github.com/ePex/cloudtui/tui/internal/ui"
)

func TestNewRegistersViewsWithHomeDefault(t *testing.T) {
	a := New(config.Default())

	wantNames := []string{"home", "settings", "log", "queues"}
	if len(a.views) != len(wantNames) {
		t.Fatalf("len(views) = %d, want %d", len(a.views), len(wantNames))
	}
	for i, v := range a.views {
		if got := v.Name(); got != wantNames[i] {
			t.Errorf("views[%d].Name() = %q, want %q", i, got, wantNames[i])
		}
	}

	if name, _ := a.pages.GetFrontPage(); name != "home" {
		t.Errorf("front page = %q, want %q", name, "home")
	}
}

func TestSwitchTo(t *testing.T) {
	a := New(config.Default())

	a.switchTo("settings")
	if name, _ := a.pages.GetFrontPage(); name != "settings" {
		t.Fatalf("front page after switchTo(\"settings\") = %q, want %q", name, "settings")
	}

	a.switchTo("bogus")
	if name, _ := a.pages.GetFrontPage(); name != "settings" {
		t.Errorf("front page after switchTo(\"bogus\") = %q, want unchanged %q", name, "settings")
	}
}

func TestOnGlobalKeyFocusesPromptOnColon(t *testing.T) {
	a := New(config.Default())
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
	a := New(config.Default())
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
	a := New(config.Default())
	a.tv.SetFocus(a.prompt)

	event := tcell.NewEventKey(tcell.KeyRune, ':', tcell.ModNone)
	if got := a.onGlobalKey(event); got != event {
		t.Errorf("onGlobalKey(':') while prompt focused = %v, want event passed through unchanged", got)
	}
}

func TestOnPromptDoneQuit(t *testing.T) {
	a := New(config.Default())
	a.prompt.SetText("quit")

	a.onPromptDone(tcell.KeyEnter)

	if got := a.prompt.GetText(); got != "" {
		t.Errorf("prompt text after quit = %q, want empty", got)
	}
	if name, _ := a.topLeft.GetFrontPage(); name != "info" {
		t.Errorf("topLeft front page after quit = %q, want %q", name, "info")
	}
}

func TestOnPromptDoneSwitchesToKnownView(t *testing.T) {
	a := New(config.Default())
	a.prompt.SetText("settings")

	a.onPromptDone(tcell.KeyEnter)

	if name, _ := a.pages.GetFrontPage(); name != "settings" {
		t.Errorf("front page after command %q = %q, want %q", "settings", name, "settings")
	}
	if got := a.prompt.GetText(); got != "" {
		t.Errorf("prompt text after Enter = %q, want empty", got)
	}
	if name, _ := a.topLeft.GetFrontPage(); name != "info" {
		t.Errorf("topLeft front page after Enter = %q, want %q", name, "info")
	}
}

func TestOnPromptDoneShortAliasH(t *testing.T) {
	a := New(config.Default())
	a.switchTo("settings")
	a.prompt.SetText("h")

	a.onPromptDone(tcell.KeyEnter)

	if name, _ := a.pages.GetFrontPage(); name != "home" {
		t.Errorf("front page after ':h' = %q, want %q", name, "home")
	}
}

func TestOnPromptDoneShortAliasS(t *testing.T) {
	a := New(config.Default())
	a.prompt.SetText("s")

	a.onPromptDone(tcell.KeyEnter)

	if name, _ := a.pages.GetFrontPage(); name != "settings" {
		t.Errorf("front page after ':s' = %q, want %q", name, "settings")
	}
}

func TestOnPromptDoneAqOpensConnectionManager(t *testing.T) {
	a := New(config.Default())
	a.prompt.SetText("aq")

	a.onPromptDone(tcell.KeyEnter)

	if !a.connManagerVisible {
		t.Error("connManagerVisible = false after ':aq', want true")
	}
	if got := a.tv.GetFocus(); got != a.connManagerList {
		t.Errorf("focus after ':aq' = %v, want the connection manager list", got)
	}
}

func TestOnPromptDoneConnectionsOpensConnectionManager(t *testing.T) {
	a := New(config.Default())
	a.prompt.SetText("connections")

	a.onPromptDone(tcell.KeyEnter)

	if !a.connManagerVisible {
		t.Error("connManagerVisible = false after ':connections', want true")
	}
}

func TestOnPromptDoneAqWorksFromAnyView(t *testing.T) {
	a := New(config.Default())
	a.switchTo("log") // not settings — proves ':aq' isn't tied to any specific starting view
	a.prompt.SetText("aq")

	a.onPromptDone(tcell.KeyEnter)

	if !a.connManagerVisible {
		t.Error("':aq' from the log view did not open the connection manager")
	}
}

func TestOnPromptDoneApOpensAWSProfiles(t *testing.T) {
	a := New(config.Default())
	a.listAWSProfiles = func(context.Context) ([]awsprofile.Profile, error) { return nil, nil }
	a.prompt.SetText("ap")

	a.onPromptDone(tcell.KeyEnter)

	if !a.awsProfilesVisible {
		t.Error("awsProfilesVisible = false after ':ap', want true")
	}
	if got := a.tv.GetFocus(); got != a.awsProfilesTable {
		t.Errorf("focus after ':ap' = %v, want the AWS profiles table", got)
	}
}

func TestOnPromptDoneAwsprofilesOpensAWSProfiles(t *testing.T) {
	a := New(config.Default())
	a.listAWSProfiles = func(context.Context) ([]awsprofile.Profile, error) { return nil, nil }
	a.prompt.SetText("awsprofiles")

	a.onPromptDone(tcell.KeyEnter)

	if !a.awsProfilesVisible {
		t.Error("awsProfilesVisible = false after ':awsprofiles', want true")
	}
}

func TestOnPromptDoneApWorksFromAnyView(t *testing.T) {
	a := New(config.Default())
	a.listAWSProfiles = func(context.Context) ([]awsprofile.Profile, error) { return nil, nil }
	a.switchTo("log") // not settings — proves ':ap' isn't tied to any specific starting view
	a.prompt.SetText("ap")

	a.onPromptDone(tcell.KeyEnter)

	if !a.awsProfilesVisible {
		t.Error("':ap' from the log view did not open the AWS profiles overlay")
	}
}

func TestOnPromptDoneShortAliasQ(t *testing.T) {
	a := New(config.Default())
	a.prompt.SetText("q")

	// Should not panic; tv.Stop() is a no-op without a screen.
	a.onPromptDone(tcell.KeyEnter)

	if got := a.prompt.GetText(); got != "" {
		t.Errorf("prompt text after 'q' command = %q, want empty", got)
	}
}

func TestOnPromptDoneThemeCommand(t *testing.T) {
	t.Chdir(t.TempDir())
	a := New(config.Default())
	a.prompt.SetText("theme cyberpunk")

	a.onPromptDone(tcell.KeyEnter)

	if got := a.cfg.Theme; got != "cyberpunk" {
		t.Errorf("cfg.Theme after ':theme cyberpunk' = %q, want %q", got, "cyberpunk")
	}
}

func TestOnPromptDoneThemeCommandUnknownIsNoOp(t *testing.T) {
	a := New(config.Default())
	original := a.cfg.Theme
	a.prompt.SetText("theme nosuchtheme")

	a.onPromptDone(tcell.KeyEnter)

	if got := a.cfg.Theme; got != original {
		t.Errorf("cfg.Theme after ':theme nosuchtheme' = %q, want unchanged %q", got, original)
	}
}

func TestOnPromptDoneUnknownCommandLeavesViewUnchanged(t *testing.T) {
	a := New(config.Default())
	a.switchTo("settings")
	a.prompt.SetText("bogus")

	a.onPromptDone(tcell.KeyEnter)

	if name, _ := a.pages.GetFrontPage(); name != "settings" {
		t.Errorf("front page after unknown command = %q, want unchanged %q", name, "settings")
	}
	if name, _ := a.topLeft.GetFrontPage(); name != "info" {
		t.Errorf("topLeft front page after unknown command = %q, want %q", name, "info")
	}
}

func TestOnPromptDoneNonEnterReturnsFocusWithoutSwitching(t *testing.T) {
	a := New(config.Default())
	a.switchTo("settings")
	a.prompt.SetText("settings")
	a.topLeft.SwitchToPage("prompt")
	a.tv.SetFocus(a.prompt)

	a.onPromptDone(tcell.KeyEscape)

	if name, _ := a.pages.GetFrontPage(); name != "settings" {
		t.Errorf("front page after Escape = %q, want unchanged %q", name, "settings")
	}
	if got := a.prompt.GetText(); got != "" {
		t.Errorf("prompt text after Escape = %q, want empty", got)
	}
	if name, _ := a.topLeft.GetFrontPage(); name != "info" {
		t.Errorf("topLeft front page after Escape = %q, want %q", name, "info")
	}
}

func TestOnGlobalKeySwitchesToHomeAndSettings(t *testing.T) {
	a := New(config.Default())
	a.tv.SetFocus(a.pages)

	event := tcell.NewEventKey(tcell.KeyRune, 's', tcell.ModNone)
	if got := a.onGlobalKey(event); got != nil {
		t.Errorf("onGlobalKey('s') returned %v, want nil", got)
	}
	if name, _ := a.pages.GetFrontPage(); name != "settings" {
		t.Errorf("front page after 's' = %q, want %q", name, "settings")
	}

	event = tcell.NewEventKey(tcell.KeyRune, 'h', tcell.ModNone)
	if got := a.onGlobalKey(event); got != nil {
		t.Errorf("onGlobalKey('h') returned %v, want nil", got)
	}
	if name, _ := a.pages.GetFrontPage(); name != "home" {
		t.Errorf("front page after 'h' = %q, want %q", name, "home")
	}
}

func TestOnGlobalKeyQuitConsumesEvent(t *testing.T) {
	a := New(config.Default())
	a.tv.SetFocus(a.pages)

	event := tcell.NewEventKey(tcell.KeyRune, 'q', tcell.ModNone)
	if got := a.onGlobalKey(event); got != nil {
		t.Errorf("onGlobalKey('q') returned %v, want nil", got)
	}
}

func TestOnGlobalKeyHelpTogglesAndSwallowsOtherKeys(t *testing.T) {
	a := New(config.Default())
	a.tv.SetFocus(a.pages)
	a.switchTo("settings")

	open := tcell.NewEventKey(tcell.KeyRune, '?', tcell.ModNone)
	if got := a.onGlobalKey(open); got != nil {
		t.Errorf("onGlobalKey('?') returned %v, want nil", got)
	}
	if !a.helpVisible {
		t.Fatal("helpVisible = false after '?', want true")
	}

	hEvent := tcell.NewEventKey(tcell.KeyRune, 'h', tcell.ModNone)
	if got := a.onGlobalKey(hEvent); got != nil {
		t.Errorf("onGlobalKey('h') while help open returned %v, want nil (swallowed)", got)
	}
	if name, _ := a.pages.GetFrontPage(); name != "settings" {
		t.Errorf("front page changed to %q while help open, want unchanged %q", name, "settings")
	}

	if got := a.onGlobalKey(open); got != nil {
		t.Errorf("onGlobalKey('?') to close returned %v, want nil", got)
	}
	if a.helpVisible {
		t.Error("helpVisible = true after closing '?', want false")
	}
}

func TestOnGlobalKeyHelpEscapeCloses(t *testing.T) {
	a := New(config.Default())
	a.tv.SetFocus(a.pages)
	a.openHelp()

	escape := tcell.NewEventKey(tcell.KeyEscape, 0, tcell.ModNone)
	if got := a.onGlobalKey(escape); got != nil {
		t.Errorf("onGlobalKey(Escape) while help open returned %v, want nil", got)
	}
	if a.helpVisible {
		t.Error("helpVisible = true after Escape, want false")
	}
}

func TestViewBorderColorMatchesConfiguredPerViewColor(t *testing.T) {
	a := New(config.Default())

	prim, ok := a.pages.GetPage("settings").(*tview.List)
	if !ok {
		t.Fatalf("settings page is %T, want *tview.List", a.pages.GetPage("settings"))
	}

	want := tcell.GetColor(a.cfg.Colors.ViewColor("settings"))
	if got := prim.GetBorderColor(); got != want {
		t.Errorf("settings border color = %v, want %v", got, want)
	}
}

// fakeShortcuttableView implements both ui.View and ui.Shortcuttable.
type fakeShortcuttableView struct {
	name string
}

var _ ui.View = (*fakeShortcuttableView)(nil)
var _ ui.Shortcuttable = (*fakeShortcuttableView)(nil)

func (f *fakeShortcuttableView) Name() string               { return f.name }
func (f *fakeShortcuttableView) Title() string              { return f.name }
func (f *fakeShortcuttableView) Primitive() tview.Primitive { return tview.NewBox() }
func (f *fakeShortcuttableView) Shortcuts() []ui.Shortcut {
	return []ui.Shortcut{
		{Key: "p", Description: "purge"},
		{Key: "m", Description: "move"},
	}
}

func TestSwitchToClearsContextPanelForNonShortcuttableView(t *testing.T) {
	a := New(config.Default())
	// settings doesn't implement Shortcuttable (home does, deliberately —
	// see TestSwitchToHomeShowsGlobalHotkeysInContextPanel).
	a.switchTo("settings")
	if got := a.contextPanel.GetText(true); got != "" {
		t.Errorf("contextPanel after switchTo(settings) = %q, want empty", got)
	}
}

func TestSwitchToHomeShowsGlobalHotkeysInContextPanel(t *testing.T) {
	a := New(config.Default())

	a.switchTo("home")

	text := a.contextPanel.GetText(true)
	for _, want := range []string{"?", "help", "l", "log", "s", "settings", "q", "quit", ":", "command"} {
		if !strings.Contains(text, want) {
			t.Errorf("contextPanel after switchTo(home) = %q, want it to contain %q", text, want)
		}
	}
	if strings.Contains(text, "home") {
		t.Errorf("contextPanel after switchTo(home) = %q, want it to omit the redundant \"home\" entry", text)
	}
}

func TestSwitchToRendersShortcutsForShortcuttableView(t *testing.T) {
	a := New(config.Default())
	fv := &fakeShortcuttableView{name: "shortcuttable"}
	a.views = append(a.views, fv)
	a.pages.AddPage(fv.Name(), fv.Primitive(), true, false)

	a.switchTo("shortcuttable")

	text := a.contextPanel.GetText(true)
	for _, want := range []string{"p", "purge", "m", "move"} {
		if !strings.Contains(text, want) {
			t.Errorf("contextPanel text = %q, want it to contain %q", text, want)
		}
	}
}

func TestViewBorderColorFallsBackForUnmappedView(t *testing.T) {
	a := New(config.Default())
	fv := &fakeShortcuttableView{name: "unmapped-view"}
	prim := fv.Primitive()

	a.colorBordered(fv, prim)

	box, ok := prim.(*tview.Box)
	if !ok {
		t.Fatalf("Primitive() = %T, want *tview.Box", prim)
	}
	want := tcell.GetColor(a.cfg.Colors.Border)
	if got := box.GetBorderColor(); got != want {
		t.Errorf("border color for unmapped view = %v, want fallback %v", got, want)
	}
}

func TestActivatableCalledBySwitchTo(t *testing.T) {
	a := New(config.Default())
	var activated bool
	fv := &fakeActivatableView{name: "act-test", activateFn: func() { activated = true }}
	a.views = append(a.views, fv)
	a.pages.AddPage(fv.Name(), fv.Primitive(), true, false)

	a.switchTo("act-test")

	if !activated {
		t.Error("Activate() not called by switchTo for activatable view")
	}
}

// fakeActivatableView implements ui.View and activatable.
type fakeActivatableView struct {
	name       string
	activateFn func()
}

var _ ui.View = (*fakeActivatableView)(nil)

func (f *fakeActivatableView) Name() string               { return f.name }
func (f *fakeActivatableView) Title() string              { return f.name }
func (f *fakeActivatableView) Primitive() tview.Primitive { return tview.NewBox() }
func (f *fakeActivatableView) Activate()                  { f.activateFn() }

func TestQueuesViewRegistered(t *testing.T) {
	a := New(config.Default())
	for _, v := range a.views {
		if v.Name() == "queues" {
			return
		}
	}
	t.Error("queues view not registered in app.views")
}

func TestPromptQueuesCommandSwitchesToQueuesView(t *testing.T) {
	a := New(config.Default())
	a.prompt.SetText("queues")

	a.onPromptDone(tcell.KeyEnter)

	if name, _ := a.pages.GetFrontPage(); name != "queues" {
		t.Errorf("front page after ':queues' = %q, want %q", name, "queues")
	}
}

func TestSortPickerQueues(t *testing.T) {
	cases := []struct {
		name        string
		sourceQueue string
		input       []string
		want        []string
	}{
		{
			name:        "DLQ source with matching non-DLQ queue pins it first",
			sourceQueue: "dlq.foo.bar",
			input:       []string{"alpha", "foo.bar", "zebra"},
			want:        []string{"foo.bar", "alpha", "zebra"},
		},
		{
			name:        "DLQ source case-insensitive prefix detection",
			sourceQueue: "DLQ.foo.bar",
			input:       []string{"foo.bar", "alpha"},
			want:        []string{"foo.bar", "alpha"},
		},
		{
			name:        "DLQ source candidate match is case-insensitive",
			sourceQueue: "dlq.Foo.Bar",
			input:       []string{"foo.bar", "alpha"},
			want:        []string{"foo.bar", "alpha"},
		},
		{
			name:        "DLQ source without matching candidate returns alphabetical",
			sourceQueue: "dlq.missing.queue",
			input:       []string{"zebra", "alpha", "mango"},
			want:        []string{"alpha", "mango", "zebra"},
		},
		{
			name:        "non-DLQ source returns alphabetical",
			sourceQueue: "foo.bar",
			input:       []string{"zebra", "alpha", "mango"},
			want:        []string{"alpha", "mango", "zebra"},
		},
		{
			name:        "empty list",
			sourceQueue: "dlq.foo",
			input:       []string{},
			want:        []string{},
		},
		{
			name:        "DLQ queues are de-prioritized after regular queues",
			sourceQueue: "foo.bar",
			input:       []string{"dlq.other", "regular", "another"},
			want:        []string{"another", "regular", "dlq.other"},
		},
		{
			name:        "system queues are de-prioritized last",
			sourceQueue: "foo.bar",
			input:       []string{"activemq.advisory", "regular", "statistics.foo"},
			want:        []string{"regular", "activemq.advisory", "statistics.foo"},
		},
		{
			name:        "full tier ordering: preferred, regular, DLQ, system",
			sourceQueue: "dlq.my.queue",
			input:       []string{"activemq.advisory", "dlq.other", "my.queue", "regular.a", "statistics.foo"},
			want:        []string{"my.queue", "regular.a", "dlq.other", "activemq.advisory", "statistics.foo"},
		},
		{
			name:        "IMQ source pins corresponding queue first",
			sourceQueue: "imq.foo.bar",
			input:       []string{"foo.bar", "alpha"},
			want:        []string{"foo.bar", "alpha"},
		},
		{
			name:        "IMQ queues are de-prioritized alongside DLQ queues",
			sourceQueue: "regular.queue",
			input:       []string{"imq.other", "regular.a", "dlq.other"},
			want:        []string{"regular.a", "dlq.other", "imq.other"},
		},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			got := sortPickerQueues(tc.sourceQueue, tc.input)
			if len(got) != len(tc.want) {
				t.Fatalf("sortPickerQueues() len = %d, want %d; got %v", len(got), len(tc.want), got)
			}
			for i := range got {
				if got[i] != tc.want[i] {
					t.Errorf("sortPickerQueues()[%d] = %q, want %q", i, got[i], tc.want[i])
				}
			}
		})
	}
}

func TestOpenMessageDetailSetsContextPanelShortcuts(t *testing.T) {
	a := New(config.Default())

	a.openMessageDetail("test-queue", queue.Message{ID: "id-1"})

	text := a.contextPanel.GetText(true)
	for _, want := range []string{"m", "move", "d", "delete", "Esc", "back"} {
		if !strings.Contains(text, want) {
			t.Errorf("contextPanel after openMessageDetail = %q, want it to contain %q", text, want)
		}
	}
}
