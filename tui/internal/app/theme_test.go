package app

import (
	"testing"

	"github.com/gdamore/tcell/v2"
	"github.com/rivo/tview"

	"github.com/ePex/cloudtui/tui/internal/config"
)

func mustTheme(t *testing.T, name string) config.Palette {
	t.Helper()
	p, ok := config.PaletteForTheme(name)
	if !ok {
		t.Fatalf("config.PaletteForTheme(%q) = (_, false)", name)
	}
	return p
}

func TestApplyThemeSetsBoxDefaults(t *testing.T) {
	p := config.Palette{
		Background:    "#111111",
		Border:        "#222222",
		Label:         "#333333",
		Text:          "#444444",
		Value:         "#555555",
		SelectionText: "#666666",
	}
	applyTheme(p)
	t.Cleanup(func() { applyTheme(config.Default().Colors) })

	box := tview.NewBox()
	if got, want := box.GetBackgroundColor(), tcell.GetColor(p.Background); got != want {
		t.Errorf("GetBackgroundColor() = %v, want %v", got, want)
	}
	if got, want := box.GetBorderColor(), tcell.GetColor(p.Border); got != want {
		t.Errorf("GetBorderColor() = %v, want %v", got, want)
	}
}

func TestReapplyThemeUpdatesStatusBarColors(t *testing.T) {
	a := New(config.Default())
	t.Cleanup(func() { applyTheme(config.Default().Colors) })

	p := mustTheme(t, "cyberpunk")
	a.cfg.Colors = p
	reapplyTheme(a, p)

	if got, want := a.statusBar.GetBackgroundColor(), tcell.GetColor(p.StatusBarBg); got != want {
		t.Errorf("statusBar background after reapplyTheme = %v, want %v", got, want)
	}
}

func TestReapplyThemeUpdatesInfoPanelText(t *testing.T) {
	a := New(config.Default())
	t.Cleanup(func() { applyTheme(config.Default().Colors) })

	a.cfg.Theme = "cyberpunk"
	a.cfg.Colors = mustTheme(t, "cyberpunk")
	reapplyTheme(a, a.cfg.Colors)

	text := a.infoPanel.GetText(true)
	if len(text) == 0 {
		t.Error("infoPanel text is empty after reapplyTheme")
	}
}

func TestReapplyThemeUpdatesGlobalStyles(t *testing.T) {
	a := New(config.Default())
	t.Cleanup(func() { applyTheme(config.Default().Colors) })

	p := mustTheme(t, "cyberpunk")
	a.cfg.Colors = p
	reapplyTheme(a, p)

	if got, want := tview.Styles.PrimitiveBackgroundColor, tcell.GetColor(p.Background); got != want {
		t.Errorf("tview.Styles.PrimitiveBackgroundColor after reapplyTheme = %v, want %v", got, want)
	}
}
