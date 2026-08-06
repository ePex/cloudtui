package app

import (
	"os"
	"testing"

	"github.com/rivo/tview"

	"github.com/ePex/cloudtui/tui/internal/config"
)

func TestSettingsFormHasBorderAndTitle(t *testing.T) {
	a := New(config.Default())

	prim, ok := a.pages.GetPage("settings").(*tview.Form)
	if !ok {
		t.Fatalf("settings page = %T, want *tview.Form", a.pages.GetPage("settings"))
	}
	if got, want := prim.GetTitle(), " Settings "; got != want {
		t.Errorf("GetTitle() = %q, want %q", got, want)
	}
}

func TestSettingsFormHasThemeDropDown(t *testing.T) {
	a := New(config.Default())

	if a.settingsForm == nil {
		t.Fatal("a.settingsForm is nil")
	}
	if got := a.settingsForm.GetFormItemCount(); got < 1 {
		t.Errorf("form item count = %d, want >= 1", got)
	}

	dd, ok := a.settingsForm.GetFormItem(0).(*tview.DropDown)
	if !ok {
		t.Fatalf("form item 0 = %T, want *tview.DropDown", a.settingsForm.GetFormItem(0))
	}
	if got, want := dd.GetLabel(), "Theme"; got != want {
		t.Errorf("dropdown label = %q, want %q", got, want)
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
