package ui

import (
	"strings"
	"testing"

	"github.com/rivo/tview"

	"github.com/ePex/cloudtui/tui/internal/config"
)

func TestInfoPanelContainsTheme(t *testing.T) {
	cfg := config.Default() // theme = "dark"
	text := newInfoPanel(cfg).GetText(true)

	for _, want := range []string{"Theme:", "dark"} {
		if !strings.Contains(text, want) {
			t.Errorf("info panel text = %q, want it to contain %q", text, want)
		}
	}
}

func TestInfoPanelContainsConnectionName(t *testing.T) {
	cfg := config.Default() // name = "default"
	text := newInfoPanel(cfg).GetText(true)

	for _, want := range []string{"AMQ Connection:", "default"} {
		if !strings.Contains(text, want) {
			t.Errorf("info panel text = %q, want it to contain %q", text, want)
		}
	}
}

func TestInfoPanelTextShowsConnectionName(t *testing.T) {
	cfg := config.Default()
	cfg.Connections[0].Name = "staging"

	text := InfoPanelText(cfg)

	if !strings.Contains(text, "staging") {
		t.Errorf("InfoPanelText() = %q, want it to contain name %q", text, "staging")
	}
}

func TestInfoPanelTextShowsNoneWhenNoAWSProfileSelected(t *testing.T) {
	cfg := config.Default()

	text := InfoPanelText(cfg)

	if !strings.Contains(text, "AWS Profile:") || !strings.Contains(text, "(none)") {
		t.Errorf("InfoPanelText() = %q, want it to contain \"AWS Profile:\" and \"(none)\"", text)
	}
}

func TestInfoPanelTextShowsActiveAWSProfile(t *testing.T) {
	cfg := config.Default()
	cfg.ActiveAWSProfile = "work"

	text := InfoPanelText(cfg)

	if !strings.Contains(text, "AWS Profile:") || !strings.Contains(text, "work") {
		t.Errorf("InfoPanelText() = %q, want it to contain \"AWS Profile:\" and \"work\"", text)
	}
}

func TestInfoPanelTextShowsThemeName(t *testing.T) {
	cfg := config.Default()
	cfg.Theme = "cyberpunk"

	text := InfoPanelText(cfg)

	if !strings.Contains(text, "cyberpunk") {
		t.Errorf("InfoPanelText() = %q, want it to contain %q", text, "cyberpunk")
	}
}

func TestNewTopBarHasDividerBetweenInfoAndNav(t *testing.T) {
	tb := NewTopBar(config.Default(), tview.NewInputField())

	if got, want := tb.Root.GetItemCount(), 4; got != want {
		t.Errorf("root.GetItemCount() = %d, want %d (info, divider, context, logo)", got, want)
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

	// A logo taller than ShortcutPanelRows drives the height.
	tallLogo := []string{"1", "2", "3", "4", "5", "6", "7", "8", "9", "10", "11", "12"}
	tall := NewTopBar(config.Config{Logo: tallLogo}, prompt)
	if tall.Height != 12 {
		t.Errorf("height with 12-line logo = %d, want 12", tall.Height)
	}

	// A short logo: height is at least ShortcutPanelRows (currently 11).
	short := NewTopBar(config.Config{Logo: []string{"1"}}, prompt)
	if short.Height != 11 {
		t.Errorf("height with 1-line logo = %d, want 11 (ShortcutPanelRows)", short.Height)
	}
}

func TestNewTopBarLeftPagesDefaultsToInfo(t *testing.T) {
	tb := NewTopBar(config.Default(), tview.NewInputField())

	if name, _ := tb.Left.GetFrontPage(); name != "info" {
		t.Errorf("front page = %q, want %q", name, "info")
	}
}

func TestNewTopBarExposesInfoPanel(t *testing.T) {
	cfg := config.Default()
	tb := NewTopBar(cfg, tview.NewInputField())

	if tb.Info == nil {
		t.Fatal("TopBar.Info is nil")
	}
	if got, want := tb.Info.GetText(false), InfoPanelText(cfg); got != want {
		t.Errorf("tb.Info.GetText(false) = %q, want %q", got, want)
	}
}

func TestNewTopBarExposesDividerContextLogo(t *testing.T) {
	tb := NewTopBar(config.Default(), tview.NewInputField())

	if tb.Divider == nil {
		t.Error("TopBar.Divider is nil")
	}
	if tb.ContextPanel == nil {
		t.Error("TopBar.ContextPanel is nil")
	}
	if tb.Logo == nil {
		t.Error("TopBar.Logo is nil")
	}
}

func TestNewTopBarContextPanelEmptyByDefault(t *testing.T) {
	tb := NewTopBar(config.Default(), tview.NewInputField())

	if got := tb.ContextPanel.GetText(true); got != "" {
		t.Errorf("contextPanel initial text = %q, want empty", got)
	}
}
