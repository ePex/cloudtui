package app

import (
	"fmt"

	"github.com/gdamore/tcell/v2"
	"github.com/rivo/tview"

	"github.com/ePex/cloudtui/tui/internal/config"
	"github.com/ePex/cloudtui/tui/internal/ui"
)

// settingsView is the Settings screen: a tview.List where each row is a
// configurable setting. Pressing Enter on a row opens the relevant picker
// overlay. Lives in internal/app (not internal/ui/views) because it needs
// live config read/write and runtime overlay control.
type settingsView struct {
	list *tview.List
}

func (s *settingsView) Name() string               { return "settings" }
func (s *settingsView) Title() string              { return "Settings" }
func (s *settingsView) Primitive() tview.Primitive { return s.list }

// styleDropDown applies palette colors to the dropdown's popup list so
// unselected items are readable against the theme background.
func styleDropDown(dd *tview.DropDown, p config.Palette) {
	dd.SetListStyles(
		tcell.StyleDefault.
			Foreground(tcell.GetColor(p.Text)).
			Background(tcell.GetColor(p.Background)),
		tcell.StyleDefault.
			Foreground(tcell.GetColor(p.SelectionText)).
			Background(tcell.GetColor(p.SelectionBg)),
	)
}

// newSettingsView builds the Settings view as a tview.List. Each item opens
// a picker overlay when Enter is pressed: item 0 → theme picker, item 1 →
// connection manager, item 2 → AWS profiles (read-only).
func newSettingsView(a *App) ui.View {
	l := tview.NewList().ShowSecondaryText(false)
	l.SetBorder(true).SetTitle(" Settings ")

	// Items are populated by refreshSettingsList; add placeholders here so
	// indices are stable.
	l.AddItem("", "", 0, func() { a.showThemePicker() })
	l.AddItem("", "", 0, func() { a.showConnectionManager() })
	l.AddItem("", "", 0, func() { a.showAWSProfiles() })

	l.SetInputCapture(func(event *tcell.EventKey) *tcell.EventKey {
		switch event.Rune() {
		case 'j':
			return tcell.NewEventKey(tcell.KeyDown, 0, tcell.ModNone)
		case 'k':
			return tcell.NewEventKey(tcell.KeyUp, 0, tcell.ModNone)
		}
		return event
	})

	styleList(l, a.cfg.Colors)
	a.settingsList = l
	a.refreshSettingsList()
	return &settingsView{list: l}
}

// refreshSettingsList rebuilds the displayed text of all settings-list items
// to reflect the current config values (theme name, active connection alias).
func (a *App) refreshSettingsList() {
	if a.settingsList == nil {
		return
	}
	cur := a.settingsList.GetCurrentItem()
	conn := a.cfg.ActiveConn()
	a.settingsList.Clear()
	a.settingsList.AddItem(fmt.Sprintf("Theme: %s", a.cfg.Theme), "", 0, func() { a.showThemePicker() })
	a.settingsList.AddItem(fmt.Sprintf("Connection: %s", conn.Alias), "", 0, func() { a.showConnectionManager() })
	a.settingsList.AddItem("AWS Profiles", "", 0, func() { a.showAWSProfiles() })
	if cur >= 0 && cur < a.settingsList.GetItemCount() {
		a.settingsList.SetCurrentItem(cur)
	}
}

// showThemePicker opens the theme-picker overlay, pre-selecting the current theme.
func (a *App) showThemePicker() {
	themes := config.AvailableThemes()
	a.themePickerList.Clear()
	for i, name := range themes {
		n := name
		prefix := "   "
		if n == a.cfg.Theme {
			prefix = "⭐ "
			a.themePickerList.SetCurrentItem(i)
		}
		a.themePickerList.AddItem(prefix+n, "", 0, func() {
			a.closeThemePicker()
			a.switchTheme(n)
		})
	}
	// SetCurrentItem must be called after all items are added.
	for i, name := range themes {
		if name == a.cfg.Theme {
			a.themePickerList.SetCurrentItem(i)
			break
		}
	}
	a.rootPages.ShowPage("theme-picker")
	a.tv.SetFocus(a.themePickerList)
	a.themePickerVisible = true
}

// closeThemePicker hides the theme-picker overlay and restores focus.
func (a *App) closeThemePicker() {
	a.rootPages.HidePage("theme-picker")
	a.themePickerVisible = false
	a.tv.SetFocus(a.pages)
}
