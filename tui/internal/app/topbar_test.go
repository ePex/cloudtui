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
