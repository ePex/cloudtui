package ui

import (
	"testing"

	"github.com/gdamore/tcell/v2"
	"github.com/rivo/tview"

	"github.com/ePex/cloudtui/tui/internal/config"
)

// withTviewStyles sets tview.Styles from p for the rest of the test, as
// the app does at startup, and restores the previous global styles
// afterwards so tests don't leak state into each other.
func withTviewStyles(t *testing.T, p config.Palette) {
	t.Helper()
	saved := tview.Styles
	t.Cleanup(func() { tview.Styles = saved })
	ApplyTviewStyles(p)
}

// mustPalette returns the embedded theme name's palette.
func mustPalette(t *testing.T, name string) config.Palette {
	t.Helper()
	p, ok := config.PaletteForTheme(name)
	if !ok {
		t.Fatalf("no embedded theme %q", name)
	}
	return p
}

func TestApplyTviewStyles(t *testing.T) {
	p := mustPalette(t, "cyberpunk")
	withTviewStyles(t, p)

	checks := []struct {
		name string
		got  tcell.Color
		want string
	}{
		{"PrimitiveBackgroundColor", tview.Styles.PrimitiveBackgroundColor, p.Background},
		{"ContrastBackgroundColor", tview.Styles.ContrastBackgroundColor, p.Background},
		{"BorderColor", tview.Styles.BorderColor, p.Border},
		{"PrimaryTextColor", tview.Styles.PrimaryTextColor, p.Text},
		{"SecondaryTextColor", tview.Styles.SecondaryTextColor, p.Value},
		{"TertiaryTextColor", tview.Styles.TertiaryTextColor, p.Label},
		{"InverseTextColor", tview.Styles.InverseTextColor, p.SelectionText},
		{"ContrastSecondaryTextColor", tview.Styles.ContrastSecondaryTextColor, p.Value},
	}
	for _, c := range checks {
		if c.got != tcell.GetColor(c.want) {
			t.Errorf("%s = %v, want %s", c.name, c.got, c.want)
		}
	}
}
