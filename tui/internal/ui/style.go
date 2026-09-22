package ui

import (
	"github.com/gdamore/tcell/v2"
	"github.com/rivo/tview"

	"github.com/ePex/cloudtui/tui/internal/config"
)

// StyleList applies p to every item style of l. The main, secondary, and
// shortcut styles use the same colors ApplyTviewStyles gives a freshly
// built list — reapplied here because tview.List copies them at
// construction, so without this a live theme switch would leave every
// unselected row in the previous theme's colors. Selection is always
// wired explicitly: tview's computed default inverts body text, which
// doesn't produce the palette's highlight look.
func StyleList(l *tview.List, p config.Palette) *tview.List {
	bg := tcell.GetColor(p.Background)
	return l.
		SetMainTextStyle(tcell.StyleDefault.Foreground(tcell.GetColor(p.Text)).Background(bg)).
		SetSecondaryTextStyle(tcell.StyleDefault.Foreground(tcell.GetColor(p.Label)).Background(bg)).
		SetShortcutStyle(tcell.StyleDefault.Foreground(tcell.GetColor(p.Value)).Background(bg)).
		SetSelectedBackgroundColor(tcell.GetColor(p.SelectionBg)).
		SetSelectedTextColor(tcell.GetColor(p.SelectionText))
}

// StyleDropDown applies palette colors to the dropdown's popup list so
// unselected items are readable against the theme background.
func StyleDropDown(dd *tview.DropDown, p config.Palette) {
	dd.SetListStyles(
		tcell.StyleDefault.
			Foreground(tcell.GetColor(p.Text)).
			Background(tcell.GetColor(p.Background)),
		tcell.StyleDefault.
			Foreground(tcell.GetColor(p.SelectionText)).
			Background(tcell.GetColor(p.SelectionBg)),
	)
}

// AutocompletePanelBlend is how far StyleInputFieldAutocomplete tints the
// drop-down's background toward the palette's accent color. Exported so
// tests can compute the same expected color rather than duplicating the
// blend factor.
const AutocompletePanelBlend = 0.15

// StyleInputFieldAutocomplete applies palette colors to i's autocomplete
// drop-down. tview's InputField sizes the drop-down's popup to exactly fit
// its entries and gives no way to draw a real border around it (see
// tui/CLAUDE.md's tview gotchas), so unselected rows use a background tinted
// toward the palette's accent color rather than a flat copy of the screen
// background — otherwise the popup has no visible edge and reads as loose
// text floating over whatever else is on screen.
func StyleInputFieldAutocomplete(i *tview.InputField, p config.Palette) *tview.InputField {
	panelBg := BlendColors(tcell.GetColor(p.Background), tcell.GetColor(p.Accent), AutocompletePanelBlend)
	return i.SetAutocompleteStyles(
		panelBg,
		tcell.StyleDefault.
			Foreground(tcell.GetColor(p.Text)).
			Background(panelBg),
		tcell.StyleDefault.
			Foreground(tcell.GetColor(p.SelectionText)).
			Background(tcell.GetColor(p.SelectionBg)),
	)
}

// BlendColors linearly interpolates from a toward b by t (0 keeps a, 1
// yields b). If either color can't be broken into RGB components (e.g. an
// unset palette field), a is returned unchanged rather than blending toward
// garbage.
func BlendColors(a, b tcell.Color, t float64) tcell.Color {
	ar, ag, ab := a.RGB()
	br, bg, bb := b.RGB()
	if ar < 0 || br < 0 {
		return a
	}
	lerp := func(x, y int32) int32 {
		return x + int32(float64(y-x)*t)
	}
	return tcell.NewRGBColor(lerp(ar, br), lerp(ag, bg), lerp(ab, bb))
}
