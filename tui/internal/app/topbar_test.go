package app

import (
	"strings"
	"testing"

	"github.com/rivo/tview"

	"github.com/ePex/cloudtui/tui/internal/config"
)

func TestInfoPanelContainsPlaceholders(t *testing.T) {
	text := newInfoPanel(config.Default()).GetText(true)

	for _, want := range []string{"Profile:", "Queue Broker:", "(not configured)"} {
		if !strings.Contains(text, want) {
			t.Errorf("info panel text = %q, want it to contain %q", text, want)
		}
	}
}

func TestInfoPanelTextShowsConfiguredProfile(t *testing.T) {
	cfg := config.Default()
	cfg.AWS.Profile = "my-profile"

	text := infoPanelText(cfg)

	if !strings.Contains(text, "my-profile") {
		t.Errorf("info panel text = %q, want it to contain %q", text, "my-profile")
	}
	lines := strings.Split(text, "\n")
	if len(lines) == 0 || strings.Contains(lines[0], "(not configured)") {
		t.Errorf("Profile line = %q, want no placeholder once a profile is configured", lines[0])
	}
}

func TestShortcutsPanelContainsBindings(t *testing.T) {
	text := newShortcutsPanel(config.Default()).GetText(true)

	for _, want := range []string{":", "command", "q", "quit", "esc", "cancel"} {
		if !strings.Contains(text, want) {
			t.Errorf("shortcuts panel text = %q, want it to contain %q", text, want)
		}
	}
}

func TestLogoPanelMatchesConfig(t *testing.T) {
	cfg := config.Config{Logo: []string{"AAA", "BBB"}}

	if got, want := newLogoPanel(cfg).GetText(true), "AAA\nBBB"; got != want {
		t.Errorf("logo panel text = %q, want %q", got, want)
	}
}

func TestLogoWidth(t *testing.T) {
	if got, want := logoWidth([]string{"a", "abc", "ab"}), 3; got != want {
		t.Errorf("logoWidth() = %d, want %d", got, want)
	}
}

func TestNewTopBarHeightGrowsWithLogo(t *testing.T) {
	prompt := tview.NewInputField()
	filterInput := tview.NewInputField()

	tall := newTopBar(config.Config{Logo: []string{"1", "2", "3", "4", "5"}}, prompt, filterInput)
	if tall.height != 5 {
		t.Errorf("height with 5-line logo = %d, want 5", tall.height)
	}

	short := newTopBar(config.Config{Logo: []string{"1"}}, prompt, filterInput)
	if short.height != 3 {
		t.Errorf("height with 1-line logo = %d, want 3 (shortcuts line count floor)", short.height)
	}
}

func TestNewTopBarLeftPagesDefaultsToInfo(t *testing.T) {
	tb := newTopBar(config.Default(), tview.NewInputField(), tview.NewInputField())

	if name, _ := tb.left.GetFrontPage(); name != "info" {
		t.Errorf("front page = %q, want %q", name, "info")
	}
}

func TestNewTopBarHasFilterPage(t *testing.T) {
	tb := newTopBar(config.Default(), tview.NewInputField(), tview.NewInputField())

	if !tb.left.HasPage("filter") {
		t.Error("topLeft has no \"filter\" page")
	}
}

func TestNewTopBarExposesInfoPanel(t *testing.T) {
	cfg := config.Default()
	tb := newTopBar(cfg, tview.NewInputField(), tview.NewInputField())

	if tb.info == nil {
		t.Fatal("topBar.info is nil")
	}
	if got, want := tb.info.GetText(false), infoPanelText(cfg); got != want {
		t.Errorf("tb.info.GetText(false) = %q, want %q", got, want)
	}
}
