package app

import (
	"os"
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

// setHomeDir isolates os.UserHomeDir() (and therefore config.SaveDefault's
// write target) to dir for the duration of the test — without this, any
// test exercising a config-persisting App method would write to the real
// developer's ~/.cloudtui/config.yaml. Sets both HOME (Unix) and
// USERPROFILE (Windows) unconditionally; the one os.UserHomeDir() doesn't
// consult on the current OS is simply ignored.
func setHomeDir(t *testing.T, dir string) {
	t.Helper()
	t.Setenv("HOME", dir)
	t.Setenv("USERPROFILE", dir)
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

// TestReapplyThemeUpdatesPromptRenderedBackground guards against a
// regression where the ':' prompt's visible background stayed on the
// startup theme after a live switch. InputField.GetBackgroundColor()
// (the embedded *Box) is NOT sufficient to catch this: InputField wraps a
// private *TextArea with its own separate embedded *Box, and
// TextArea.Draw() repaints its rect from that private Box's background on
// every frame — overwriting whatever InputField's own SetBackgroundColor
// painted moments earlier. The only exported InputField method that
// reaches the private TextArea's actual background is
// SetFormAttributes. Since that inner Box isn't reachable from this
// package (its field is unexported), the fix can only be verified by
// rendering to a SimulationScreen and reading cell styles back, the same
// technique TestPromptAutocompleteFirstOpenIsReadable already uses.
func TestReapplyThemeUpdatesPromptRenderedBackground(t *testing.T) {
	a := New(config.Default())
	t.Cleanup(func() { applyTheme(config.Default().Colors) })

	p := mustTheme(t, "cyberpunk")
	a.cfg.Colors = p
	reapplyTheme(a, p)

	screen := tcell.NewSimulationScreen("")
	if err := screen.Init(); err != nil {
		t.Fatalf("screen.Init: %v", err)
	}
	screen.SetSize(80, 20)

	a.prompt.SetRect(0, 0, 40, 1)
	a.prompt.Draw(screen)
	screen.Show()

	contents, _, _ := screen.GetContents()
	wantBg := tcell.GetColor(p.Background)
	wantFg := tcell.GetColor(p.Value)

	// Column 0 is the ':' label's leading space — the one part of the
	// prompt that's always painted (unlike the field area beyond it,
	// which intentionally stays transparent/ColorDefault).
	fg, bg, _ := contents[0].Style.Decompose()
	if bg != wantBg {
		t.Errorf("prompt label background after reapplyTheme = %v, want %v (palette Background)", bg.Hex(), wantBg.Hex())
	}
	if fg != wantFg {
		t.Errorf("prompt label foreground after reapplyTheme = %v, want %v (palette Value)", fg.Hex(), wantFg.Hex())
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

// TestSwitchThemeAppliesPalette, TestSwitchThemeUnknownIsNoOp, and
// TestSwitchThemePersistsConfig moved here from settings_test.go (now
// internal/view/settings_test.go) — they exercise switchTheme's own
// config mutation, not anything Settings-specific.

func TestSwitchThemeAppliesPalette(t *testing.T) {
	setHomeDir(t, t.TempDir())
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
	setHomeDir(t, t.TempDir())
	a := New(config.Default())
	original := a.cfg.Theme

	a.switchTheme("nosuchtheme")

	if got := a.cfg.Theme; got != original {
		t.Errorf("cfg.Theme after unknown switchTheme = %q, want unchanged %q", got, original)
	}
}

func TestSwitchThemePersistsConfig(t *testing.T) {
	setHomeDir(t, t.TempDir())
	a := New(config.Default())
	t.Cleanup(func() { applyTheme(config.Default().Colors) })

	a.switchTheme("cyberpunk")

	path, err := config.DefaultPath()
	if err != nil {
		t.Fatalf("config.DefaultPath() error = %v", err)
	}
	if _, err := os.Stat(path); err != nil {
		t.Errorf("config.yaml not written after switchTheme: %v", err)
	}
}
