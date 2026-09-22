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

// StyleForm applies p to every style f copies at construction: the label
// color, the field style, and the three button styles — the same colors
// ApplyTviewStyles gives a freshly built form. Form.Draw hands the label
// color and field style down to every item via SetFormAttributes, so this
// also recolors the form's input fields, text areas, and dropdowns.
// The form's own background, border, and title stay with each dialog's
// ApplyPalette.
//
// Each item's and button's own Box background is reset too: an item like
// InputField wraps its field in a separate inner widget, and
// SetFormAttributes only reaches that inner one — the outer Box keeps its
// construction-time background and paints it wherever the field doesn't
// cover (the same trap reapplyTheme works around for the command prompt).
func StyleForm(f *tview.Form, p config.Palette) *tview.Form {
	bg := tcell.GetColor(p.Background)
	text := tcell.GetColor(p.Text)
	value := tcell.GetColor(p.Value)
	for i := 0; i < f.GetFormItemCount(); i++ {
		if b, ok := f.GetFormItem(i).(interface {
			SetBackgroundColor(tcell.Color) *tview.Box
		}); ok {
			b.SetBackgroundColor(bg)
		}
	}
	for i := 0; i < f.GetButtonCount(); i++ {
		f.GetButton(i).SetBackgroundColor(bg)
	}
	return f.
		SetLabelColor(value).
		SetFieldStyle(tcell.StyleDefault.Foreground(text).Background(bg)).
		SetButtonStyle(tcell.StyleDefault.Foreground(text).Background(bg)).
		SetButtonActivatedStyle(tcell.StyleDefault.Foreground(bg).Background(text)).
		SetButtonDisabledStyle(tcell.StyleDefault.Foreground(value).Background(bg))
}

// StyleFilterInput applies p to a stand-alone input field (a view's "/"
// filter, a picker's search box): label in Label, the field itself in
// SelectionText on SelectionBg, and — to match what a freshly built field
// gets — the placeholder in Value on Background.
//
// The colors go through SetFormAttributes rather than SetLabelColor /
// SetFieldBackgroundColor / SetFieldTextColor: InputField wraps a private
// TextArea with its own embedded Box, and that inner Box's background is
// what the label is drawn on. SetFormAttributes is the only exported
// InputField method that reaches it (the same trap reapplyTheme works
// around for the command prompt); without it the label keeps the
// construction-time theme's background after a live switch. Label width 0
// means "fit the label", which is what every filter input uses (none
// calls SetLabelWidth). The outer Box's background is reset as well.
func StyleFilterInput(i *tview.InputField, p config.Palette) *tview.InputField {
	bg := tcell.GetColor(p.Background)
	i.SetBackgroundColor(bg)
	i.SetFormAttributes(0, tcell.GetColor(p.Label), bg, tcell.GetColor(p.SelectionText), tcell.GetColor(p.SelectionBg))
	return i.SetPlaceholderStyle(tcell.StyleDefault.Foreground(tcell.GetColor(p.Value)).Background(bg))
}

// StyleFormDropDown applies p to the parts of dd a tview.Form doesn't
// restyle itself — for a dropdown inside a form (whose label, field, and
// background Form.Draw hands down via SetFormAttributes). That's the popup
// list (unselected rows in Text on Background, the selected row in the
// palette's selection colors — without this, unselected popup items are
// unreadable) plus the focused, prefix, and disabled styles, which tview
// copies at construction and would otherwise keep the previous theme's
// colors after a live switch. Those three get the same colors
// ApplyTviewStyles gives a freshly built dropdown.
func StyleFormDropDown(dd *tview.DropDown, p config.Palette) *tview.DropDown {
	bg := tcell.GetColor(p.Background)
	text := tcell.GetColor(p.Text)
	inverted := tcell.StyleDefault.Foreground(bg).Background(text)
	return dd.
		SetListStyles(
			tcell.StyleDefault.Foreground(text).Background(bg),
			tcell.StyleDefault.
				Foreground(tcell.GetColor(p.SelectionText)).
				Background(tcell.GetColor(p.SelectionBg)),
		).
		SetFocusedStyle(inverted).
		SetPrefixStyle(inverted).
		SetDisabledStyle(tcell.StyleDefault.Foreground(tcell.GetColor(p.Value)).Background(bg))
}

// StyleDropDown applies p to a stand-alone dropdown (not inside a form):
// everything StyleFormDropDown does, plus what a form would otherwise hand
// down — its own Box background (which the label is drawn on), the label
// in Label, and the field in SelectionText on SelectionBg, matching the
// stand-alone filter inputs (see StyleFilterInput). Label width 0 means
// "fit the label"; no stand-alone dropdown calls SetLabelWidth.
func StyleDropDown(dd *tview.DropDown, p config.Palette) *tview.DropDown {
	bg := tcell.GetColor(p.Background)
	dd.SetFormAttributes(0, tcell.GetColor(p.Label), bg, tcell.GetColor(p.SelectionText), tcell.GetColor(p.SelectionBg))
	return StyleFormDropDown(dd, p)
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
