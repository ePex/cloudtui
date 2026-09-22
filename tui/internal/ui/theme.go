package ui

import (
	"github.com/gdamore/tcell/v2"
	"github.com/rivo/tview"

	"github.com/ePex/cloudtui/tui/internal/config"
)

// Themeable is implemented by views/overlays that need to recolor
// themselves when the active theme changes.
type Themeable interface {
	ApplyPalette(p config.Palette)
}

// ApplyTviewStyles sets tview's package-level default styles from p — the
// single palette → tview.Styles mapping. Widgets copy these at
// construction time, so the Style* helpers in style.go reapply the same
// mapping to already-built widgets on a live theme switch; keeping it in
// one place is what lets a live switch match a restart exactly.
func ApplyTviewStyles(p config.Palette) {
	bg := tcell.GetColor(p.Background)
	tview.Styles.PrimitiveBackgroundColor = bg
	tview.Styles.ContrastBackgroundColor = bg
	tview.Styles.MoreContrastBackgroundColor = bg
	tview.Styles.BorderColor = tcell.GetColor(p.Border)
	tview.Styles.TitleColor = tcell.GetColor(p.Border)
	tview.Styles.GraphicsColor = tcell.GetColor(p.Border)
	tview.Styles.PrimaryTextColor = tcell.GetColor(p.Text)
	tview.Styles.SecondaryTextColor = tcell.GetColor(p.Value)
	tview.Styles.TertiaryTextColor = tcell.GetColor(p.Label)
	tview.Styles.InverseTextColor = tcell.GetColor(p.SelectionText)
	tview.Styles.ContrastSecondaryTextColor = tcell.GetColor(p.Value)
}
