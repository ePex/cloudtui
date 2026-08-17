package dialog

import (
	"github.com/gdamore/tcell/v2"
	"github.com/rivo/tview"

	"github.com/ePex/cloudtui/tui/internal/config"
	"github.com/ePex/cloudtui/tui/internal/ui"
)

// ThemePicker is the theme-picker overlay: a list of every embedded theme,
// with the active one marked ⭐.
type ThemePicker struct {
	host    ui.Host
	flex    *tview.Flex
	list    *tview.List
	visible bool
}

// NewThemePicker builds the theme-picker overlay's widgets.
func NewThemePicker(host ui.Host) *ThemePicker {
	tp := &ThemePicker{host: host}
	tp.list = tview.NewList().ShowSecondaryText(false)
	tp.flex = tview.NewFlex().SetDirection(tview.FlexRow).
		AddItem(tp.list, 0, 1, true)
	tp.flex.SetBorder(true).SetTitle(" Theme ")
	tp.list.SetInputCapture(func(event *tcell.EventKey) *tcell.EventKey {
		switch {
		case event.Key() == tcell.KeyEscape:
			tp.close()
			return nil
		case event.Rune() == 'j':
			return tcell.NewEventKey(tcell.KeyDown, 0, tcell.ModNone)
		case event.Rune() == 'k':
			return tcell.NewEventKey(tcell.KeyUp, 0, tcell.ModNone)
		}
		return event
	})
	return tp
}

// Show opens the theme-picker overlay, pre-selecting the current theme.
func (tp *ThemePicker) Show() {
	host := tp.host
	currentTheme := host.Config().Theme
	themes := config.AvailableThemes()
	tp.list.Clear()
	for i, name := range themes {
		n := name
		prefix := "   "
		if n == currentTheme {
			prefix = "⭐ "
			tp.list.SetCurrentItem(i)
		}
		tp.list.AddItem(prefix+n, "", 0, func() {
			tp.close()
			host.SwitchTheme(n)
		})
	}
	// SetCurrentItem must be called after all items are added.
	for i, name := range themes {
		if name == currentTheme {
			tp.list.SetCurrentItem(i)
			break
		}
	}
	host.ShowPage("theme-picker")
	host.SetFocus(tp.list)
	tp.visible = true
}

// close hides the theme-picker overlay and restores focus.
func (tp *ThemePicker) close() {
	tp.host.HidePage("theme-picker")
	tp.visible = false
	tp.host.FocusMain()
}

// ApplyPalette recolors the theme-picker overlay for a live theme switch.
func (tp *ThemePicker) ApplyPalette(p config.Palette) {
	bg := tcell.GetColor(p.Background)
	tp.flex.SetBackgroundColor(bg)
	tp.flex.SetBorderColor(tcell.GetColor(p.Border))
	tp.flex.SetTitleColor(tcell.GetColor(p.Border))
	ui.StyleList(tp.list, p)
	tp.list.SetBackgroundColor(bg)
}

var _ ui.Themeable = (*ThemePicker)(nil)

// Primitive returns ThemePicker's root widget, for sizing/embedding.
func (tp *ThemePicker) Primitive() tview.Primitive { return tp.flex }

// Visible reports whether ThemePicker is currently shown.
func (tp *ThemePicker) Visible() bool { return tp.visible }
