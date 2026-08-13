package app

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

func TestInfoPanelContainsConnectionAlias(t *testing.T) {
	cfg := config.Default() // alias = "def"
	text := newInfoPanel(cfg).GetText(true)

	for _, want := range []string{"Connection:", "def"} {
		if !strings.Contains(text, want) {
			t.Errorf("info panel text = %q, want it to contain %q", text, want)
		}
	}
}

func TestInfoPanelTextShowsConnectionAlias(t *testing.T) {
	cfg := config.Default()
	cfg.Connections[0].Alias = "stg"

	text := infoPanelText(cfg)

	if !strings.Contains(text, "stg") {
		t.Errorf("infoPanelText() = %q, want it to contain alias %q", text, "stg")
	}
}

func TestInfoPanelTextShowsNoneWhenNoAWSProfileSelected(t *testing.T) {
	cfg := config.Default()

	text := infoPanelText(cfg)

	if !strings.Contains(text, "AWS Profile:") || !strings.Contains(text, "(none)") {
		t.Errorf("infoPanelText() = %q, want it to contain \"AWS Profile:\" and \"(none)\"", text)
	}
}

func TestInfoPanelTextShowsActiveAWSProfile(t *testing.T) {
	cfg := config.Default()
	cfg.ActiveAWSProfile = "work"

	text := infoPanelText(cfg)

	if !strings.Contains(text, "AWS Profile:") || !strings.Contains(text, "work") {
		t.Errorf("infoPanelText() = %q, want it to contain \"AWS Profile:\" and \"work\"", text)
	}
}

func TestInfoPanelTextShowsThemeName(t *testing.T) {
	cfg := config.Default()
	cfg.Theme = "cyberpunk"

	text := infoPanelText(cfg)

	if !strings.Contains(text, "cyberpunk") {
		t.Errorf("infoPanelText() = %q, want it to contain %q", text, "cyberpunk")
	}
}

func TestNewTopBarHasDividerBetweenInfoAndNav(t *testing.T) {
	tb := newTopBar(config.Default(), tview.NewInputField())

	if got, want := tb.root.GetItemCount(), 4; got != want {
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

	// A logo taller than shortcutPanelRows drives the height.
	tallLogo := []string{"1", "2", "3", "4", "5", "6", "7", "8", "9", "10", "11", "12"}
	tall := newTopBar(config.Config{Logo: tallLogo}, prompt)
	if tall.height != 12 {
		t.Errorf("height with 12-line logo = %d, want 12", tall.height)
	}

	// A short logo: height is at least shortcutPanelRows (currently 11).
	short := newTopBar(config.Config{Logo: []string{"1"}}, prompt)
	if short.height != 11 {
		t.Errorf("height with 1-line logo = %d, want 11 (shortcutPanelRows)", short.height)
	}
}

func TestNewTopBarLeftPagesDefaultsToInfo(t *testing.T) {
	tb := newTopBar(config.Default(), tview.NewInputField())

	if name, _ := tb.left.GetFrontPage(); name != "info" {
		t.Errorf("front page = %q, want %q", name, "info")
	}
}

func TestNewTopBarExposesInfoPanel(t *testing.T) {
	cfg := config.Default()
	tb := newTopBar(cfg, tview.NewInputField())

	if tb.info == nil {
		t.Fatal("topBar.info is nil")
	}
	if got, want := tb.info.GetText(false), infoPanelText(cfg); got != want {
		t.Errorf("tb.info.GetText(false) = %q, want %q", got, want)
	}
}

func TestNewTopBarExposesDividerContextLogo(t *testing.T) {
	tb := newTopBar(config.Default(), tview.NewInputField())

	if tb.divider == nil {
		t.Error("topBar.divider is nil")
	}
	if tb.contextPanel == nil {
		t.Error("topBar.contextPanel is nil")
	}
	if tb.logo == nil {
		t.Error("topBar.logo is nil")
	}
}

func TestNewTopBarContextPanelEmptyByDefault(t *testing.T) {
	tb := newTopBar(config.Default(), tview.NewInputField())

	if got := tb.contextPanel.GetText(true); got != "" {
		t.Errorf("contextPanel initial text = %q, want empty", got)
	}
}
