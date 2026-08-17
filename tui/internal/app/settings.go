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
// connection manager, item 2 → AWS profiles (read-only), item 3 → Datadog
// editor.
func newSettingsView(a *App) ui.View {
	l := tview.NewList().ShowSecondaryText(false)
	l.SetBorder(true).SetTitle(" Settings ")

	// Items are populated by refreshSettingsList; add placeholders here so
	// indices are stable.
	l.AddItem("", "", 0, func() { a.themePicker.show() })
	l.AddItem("", "", 0, func() { a.connManager.show() })
	l.AddItem("", "", 0, func() { a.awsProfiles.show() })
	l.AddItem("", "", 0, func() { a.datadogEditor.show() })

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
// to reflect the current config values (theme name, active connection
// name, active AWS profile, Datadog site). The AWS profile name comes
// straight from cfg.ActiveAWSProfile — no need to re-read ~/.aws here,
// unlike opening the overlay itself, which lists every discoverable
// profile.
func (a *App) refreshSettingsList() {
	if a.settingsList == nil {
		return
	}
	cur := a.settingsList.GetCurrentItem()
	conn := a.cfg.ActiveConn()
	awsProfile := a.cfg.ActiveAWSProfile
	if awsProfile == "" {
		awsProfile = "(none)"
	}
	a.settingsList.Clear()
	a.settingsList.AddItem(fmt.Sprintf("Theme: %s", a.cfg.Theme), "", 0, func() { a.themePicker.show() })
	a.settingsList.AddItem(fmt.Sprintf("AMQ Connection: %s", conn.Name), "", 0, func() { a.connManager.show() })
	a.settingsList.AddItem(fmt.Sprintf("AWS Profile: %s", awsProfile), "", 0, func() { a.awsProfiles.show() })
	a.settingsList.AddItem(fmt.Sprintf("Datadog: %s", datadogSettingsLabel(a.cfg.Datadog)), "", 0, func() { a.datadogEditor.show() })
	if cur >= 0 && cur < a.settingsList.GetItemCount() {
		a.settingsList.SetCurrentItem(cur)
	}
}

// datadogSettingsLabel summarizes cfg for the settings list row — "(none)"
// when no access token is configured (the Site alone doesn't mean
// anything's usable yet), otherwise the site that will actually be used
// (falling back to the same default internal/datadoglogs.Search applies).
func datadogSettingsLabel(cfg config.DatadogConfig) string {
	if cfg.AccessToken == "" {
		return "(none)"
	}
	if cfg.Site == "" {
		return "datadoghq.com"
	}
	return cfg.Site
}

// themePicker is the theme-picker overlay: a list of every embedded theme,
// with the active one marked ⭐.
type themePicker struct {
	host    ui.Host
	flex    *tview.Flex
	list    *tview.List
	visible bool
}

// newThemePicker builds the theme-picker overlay's widgets.
func newThemePicker(host ui.Host) *themePicker {
	tp := &themePicker{host: host}
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

// show opens the theme-picker overlay, pre-selecting the current theme.
func (tp *themePicker) show() {
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
func (tp *themePicker) close() {
	tp.host.HidePage("theme-picker")
	tp.visible = false
	tp.host.FocusMain()
}

// ApplyPalette recolors the theme-picker overlay for a live theme switch.
func (tp *themePicker) ApplyPalette(p config.Palette) {
	bg := tcell.GetColor(p.Background)
	tp.flex.SetBackgroundColor(bg)
	tp.flex.SetBorderColor(tcell.GetColor(p.Border))
	tp.flex.SetTitleColor(tcell.GetColor(p.Border))
	styleList(tp.list, p)
	tp.list.SetBackgroundColor(bg)
}

var _ ui.Themeable = (*themePicker)(nil)
