package ui

import (
	"github.com/gdamore/tcell/v2"
	"github.com/rivo/tview"

	"github.com/ePex/cloudtui/tui/internal/config"
)

// StyleList applies p's selection colors to l. tview.List's own computed
// default selection style inverts body text (background/text swapped), which
// doesn't produce the palette's highlight look — so selection is wired
// explicitly here rather than riding on applyTheme.
//
// Note: tview.List exposes no getter for its selected-item style, so the
// result cannot be unit-tested directly. Verified manually instead.
func StyleList(l *tview.List, p config.Palette) *tview.List {
	return l.
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

// StyleInputFieldAutocomplete applies palette colors to i's autocomplete
// drop-down. tview's InputField sizes the drop-down's popup to exactly fit
// its entries and gives no way to draw a real border around it (see
// tui/CLAUDE.md's tview gotchas), so unselected rows use a background tinted
// toward the palette's accent color rather than a flat copy of the screen
// background — otherwise the popup has no visible edge and reads as loose
// text floating over whatever else is on screen.
func StyleInputFieldAutocomplete(i *tview.InputField, p config.Palette) *tview.InputField {
	panelBg := blendColors(tcell.GetColor(p.Background), tcell.GetColor(p.Accent), 0.15)
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

// blendColors linearly interpolates from a toward b by t (0 keeps a, 1
// yields b). If either color can't be broken into RGB components (e.g. an
// unset palette field), a is returned unchanged rather than blending toward
// garbage.
func blendColors(a, b tcell.Color, t float64) tcell.Color {
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
