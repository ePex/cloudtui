package app

import (
	"testing"

	"github.com/gdamore/tcell/v2"
	"github.com/rivo/tview"

	"github.com/ePex/cloudtui/tui/internal/ui"
)

// fakeFilterableView is a minimal ui.View + ui.Filterable implementation
// used to exercise the '/' filter overlay, since no real view implements
// Filterable yet.
type fakeFilterableView struct {
	name     string
	filtered string
}

var _ ui.View = (*fakeFilterableView)(nil)
var _ ui.Filterable = (*fakeFilterableView)(nil)

func (f *fakeFilterableView) Name() string               { return f.name }
func (f *fakeFilterableView) Title() string              { return f.name }
func (f *fakeFilterableView) Primitive() tview.Primitive { return tview.NewBox() }
func (f *fakeFilterableView) Filter(query string)        { f.filtered = query }

func TestNewRegistersViewsWithHomeDefault(t *testing.T) {
	a := New()

	wantNames := []string{"home", "secrets", "params", "queues", "settings"}
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
	a := New()

	a.switchTo("params")
	if name, _ := a.pages.GetFrontPage(); name != "params" {
		t.Fatalf("front page after switchTo(\"params\") = %q, want %q", name, "params")
	}

	a.switchTo("bogus")
	if name, _ := a.pages.GetFrontPage(); name != "params" {
		t.Errorf("front page after switchTo(\"bogus\") = %q, want unchanged %q", name, "params")
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
	if want := a.pages.GetPage("home"); a.tv.GetFocus() != want {
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
	a.switchTo("params")
	a.prompt.SetText("bogus")

	a.onPromptDone(tcell.KeyEnter)

	if name, _ := a.pages.GetFrontPage(); name != "params" {
		t.Errorf("front page after unknown command = %q, want unchanged %q", name, "params")
	}
	if name, _ := a.topLeft.GetFrontPage(); name != "info" {
		t.Errorf("topLeft front page after unknown command = %q, want %q", name, "info")
	}
}

func TestOnPromptDoneNonEnterReturnsFocusWithoutSwitching(t *testing.T) {
	a := New()
	a.switchTo("params")
	a.prompt.SetText("params")
	a.topLeft.SwitchToPage("prompt")
	a.tv.SetFocus(a.prompt)

	a.onPromptDone(tcell.KeyEscape)

	if name, _ := a.pages.GetFrontPage(); name != "params" {
		t.Errorf("front page after Escape = %q, want unchanged %q", name, "params")
	}
	if got := a.prompt.GetText(); got != "" {
		t.Errorf("prompt text after Escape = %q, want empty", got)
	}
	if want := a.pages.GetPage("params"); a.tv.GetFocus() != want {
		t.Errorf("focus after Escape = %v, want front page's primitive %v", a.tv.GetFocus(), want)
	}
	if name, _ := a.topLeft.GetFrontPage(); name != "info" {
		t.Errorf("topLeft front page after Escape = %q, want %q", name, "info")
	}
}

func TestOnGlobalKeySwitchesToHomeAndSettings(t *testing.T) {
	a := New()
	a.tv.SetFocus(a.pages)
	a.switchTo("params")

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
	a := New()
	a.tv.SetFocus(a.pages)

	event := tcell.NewEventKey(tcell.KeyRune, 'q', tcell.ModNone)
	if got := a.onGlobalKey(event); got != nil {
		t.Errorf("onGlobalKey('q') returned %v, want nil", got)
	}
	// Application.Stop() is a documented no-op without a real screen
	// (checked in the earlier prompt-quit tests' commit); nothing further
	// to assert here.
}

func TestOnGlobalKeyHelpTogglesAndSwallowsOtherKeys(t *testing.T) {
	a := New()
	a.tv.SetFocus(a.pages)
	a.switchTo("params")

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
	if name, _ := a.pages.GetFrontPage(); name != "params" {
		t.Errorf("front page changed to %q while help open, want unchanged %q", name, "params")
	}

	if got := a.onGlobalKey(open); got != nil {
		t.Errorf("onGlobalKey('?') to close returned %v, want nil", got)
	}
	if a.helpVisible {
		t.Error("helpVisible = true after closing '?', want false")
	}
}

func TestOnGlobalKeyHelpEscapeCloses(t *testing.T) {
	a := New()
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

func TestBeginFilterNoOpOnNonFilterableView(t *testing.T) {
	a := New() // active view is "home", which doesn't implement ui.Filterable
	a.tv.SetFocus(a.pages)

	a.beginFilter()

	if name, _ := a.topLeft.GetFrontPage(); name != "info" {
		t.Errorf("topLeft front page after beginFilter() on non-Filterable view = %q, want unchanged %q", name, "info")
	}
	if a.tv.GetFocus() == a.filterInput {
		t.Error("focus moved to filterInput for a non-Filterable view")
	}
}

func TestFilterAppliesToFilterableView(t *testing.T) {
	a := New()
	fv := &fakeFilterableView{name: "fake"}
	a.views = append(a.views, fv)
	a.pages.AddPage(fv.Name(), fv.Primitive(), true, false)
	a.switchTo("fake")
	a.tv.SetFocus(a.pages)

	a.beginFilter()

	if name, _ := a.topLeft.GetFrontPage(); name != "filter" {
		t.Fatalf("topLeft front page after beginFilter() = %q, want %q", name, "filter")
	}
	if a.tv.GetFocus() != a.filterInput {
		t.Errorf("focus after beginFilter() = %v, want filterInput", a.tv.GetFocus())
	}

	a.filterInput.SetText("abc")
	a.onFilterDone(tcell.KeyEnter)

	if fv.filtered != "abc" {
		t.Errorf("fv.filtered = %q, want %q", fv.filtered, "abc")
	}
	if got := a.filterInput.GetText(); got != "" {
		t.Errorf("filterInput text after Enter = %q, want empty", got)
	}
	if name, _ := a.topLeft.GetFrontPage(); name != "info" {
		t.Errorf("topLeft front page after Enter = %q, want %q", name, "info")
	}
	if want := a.pages.GetPage("fake"); a.tv.GetFocus() != want {
		t.Errorf("focus after Enter = %v, want front page's primitive %v", a.tv.GetFocus(), want)
	}
}

func TestViewBorderColorMatchesConfiguredPerViewColor(t *testing.T) {
	a := New()

	prim, ok := a.pages.GetPage("secrets").(*tview.TextView)
	if !ok {
		t.Fatalf("secrets page is %T, want *tview.TextView", a.pages.GetPage("secrets"))
	}

	want := tcell.GetColor(a.cfg.Colors.Views["secrets"])
	if got := prim.GetBorderColor(); got != want {
		t.Errorf("secrets border color = %v, want %v", got, want)
	}
}

func TestViewBorderColorFallsBackForUnmappedView(t *testing.T) {
	a := New()
	fv := &fakeFilterableView{name: "unmapped-view"}
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

func TestOnGlobalKeySlashRoutesToBeginFilter(t *testing.T) {
	a := New()
	fv := &fakeFilterableView{name: "fake2"}
	a.views = append(a.views, fv)
	a.pages.AddPage(fv.Name(), fv.Primitive(), true, false)
	a.switchTo("fake2")
	a.tv.SetFocus(a.pages)

	event := tcell.NewEventKey(tcell.KeyRune, '/', tcell.ModNone)
	if got := a.onGlobalKey(event); got != nil {
		t.Errorf("onGlobalKey('/') returned %v, want nil", got)
	}
	if name, _ := a.topLeft.GetFrontPage(); name != "filter" {
		t.Errorf("topLeft front page after '/' = %q, want %q", name, "filter")
	}
}
