package app

import (
	"os"
	"strings"
	"testing"

	"github.com/rivo/tview"

	"github.com/ePex/cloudtui/tui/internal/config"
)

func TestSettingsListHasBorderAndTitle(t *testing.T) {
	a := New(config.Default())

	prim, ok := a.pages.GetPage("settings").(*tview.List)
	if !ok {
		t.Fatalf("settings page = %T, want *tview.List", a.pages.GetPage("settings"))
	}
	if got, want := prim.GetTitle(), " Settings "; got != want {
		t.Errorf("GetTitle() = %q, want %q", got, want)
	}
}

func TestSettingsListHasThreeItems(t *testing.T) {
	a := New(config.Default())

	if a.settingsList == nil {
		t.Fatal("a.settingsList is nil")
	}
	if got := a.settingsList.GetItemCount(); got != 3 {
		t.Errorf("settings list item count = %d, want 3", got)
	}
}

func TestSettingsListItemTwoIsAWSProfiles(t *testing.T) {
	a := New(config.Default())

	main2, _ := a.settingsList.GetItemText(2)
	if !strings.Contains(main2, "AWS Profiles") {
		t.Errorf("item 2 = %q, want it to contain 'AWS Profiles'", main2)
	}
}

func TestSettingsListItemsShowCurrentThemeAndConnection(t *testing.T) {
	a := New(config.Default())

	main0, _ := a.settingsList.GetItemText(0)
	if !strings.Contains(main0, "Theme") || !strings.Contains(main0, "dark") {
		t.Errorf("item 0 = %q, want it to contain 'Theme' and 'dark'", main0)
	}
	main1, _ := a.settingsList.GetItemText(1)
	if !strings.Contains(main1, "Connection") || !strings.Contains(main1, "def") {
		t.Errorf("item 1 = %q, want it to contain 'Connection' and 'def'", main1)
	}
}

func TestRefreshSettingsListUpdatesTheme(t *testing.T) {
	t.Chdir(t.TempDir())
	a := New(config.Default())
	t.Cleanup(func() { applyTheme(config.Default().Colors) })

	a.switchTheme("cyberpunk")

	main0, _ := a.settingsList.GetItemText(0)
	if !strings.Contains(main0, "cyberpunk") {
		t.Errorf("item 0 after switchTheme = %q, want it to contain 'cyberpunk'", main0)
	}
}

func TestSwitchThemeAppliesPalette(t *testing.T) {
	t.Chdir(t.TempDir())
	a := New(config.Default())
	t.Cleanup(func() { applyTheme(config.Default().Colors) })

	a.switchTheme("cyberpunk")

	if got := a.cfg.Theme; got != "cyberpunk" {
		t.Errorf("cfg.Theme after switchTheme(\"cyberpunk\") = %q, want %q", got, "cyberpunk")
	}
	want, _ := config.PaletteForTheme("cyberpunk")
	if got := a.cfg.Colors.Background; got != want.Background {
		t.Errorf("cfg.Colors.Background = %q, want %q", got, want.Background)
	}
}

func TestSwitchThemeUnknownIsNoOp(t *testing.T) {
	t.Chdir(t.TempDir())
	a := New(config.Default())
	original := a.cfg.Theme

	a.switchTheme("nosuchtheme")

	if got := a.cfg.Theme; got != original {
		t.Errorf("cfg.Theme after unknown switchTheme = %q, want unchanged %q", got, original)
	}
}

func TestSwitchThemePersistsConfig(t *testing.T) {
	t.Chdir(t.TempDir())
	a := New(config.Default())
	t.Cleanup(func() { applyTheme(config.Default().Colors) })

	a.switchTheme("cyberpunk")

	if _, err := os.Stat("config.yaml"); err != nil {
		t.Errorf("config.yaml not written after switchTheme: %v", err)
	}
}
