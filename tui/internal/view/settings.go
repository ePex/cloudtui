package view

import (
	"fmt"

	"github.com/gdamore/tcell/v2"
	"github.com/rivo/tview"

	"github.com/ePex/cloudtui/tui/internal/config"
	"github.com/ePex/cloudtui/tui/internal/dialog"
	"github.com/ePex/cloudtui/tui/internal/ui"
)

// SettingsView is the Settings screen: a tview.List where each row is a
// configurable setting. Pressing Enter on a row opens the relevant picker
// overlay. Lives in internal/app (not internal/ui/views) because it needs
// live config read/write and runtime overlay control.
type SettingsView struct {
	list          *tview.List
	host          ui.Host
	themePicker   *dialog.ThemePicker
	connManager   *dialog.ConnManager
	awsProfiles   *dialog.AWSProfilesPicker
	datadogEditor *dialog.DatadogEditor
}

var _ ui.View = (*SettingsView)(nil)
var _ ui.Themeable = (*SettingsView)(nil)

func (s *SettingsView) Name() string               { return "settings" }
func (s *SettingsView) Title() string              { return "Settings" }
func (s *SettingsView) Primitive() tview.Primitive { return s.list }
func (s *SettingsView) List() *tview.List          { return s.list }

// NewSettingsView builds the Settings view as a tview.List. Each item opens
// a picker overlay when Enter is pressed: item 0 → theme picker, item 1 →
// connection manager, item 2 → AWS profiles (read-only), item 3 → Datadog
// editor.
func NewSettingsView(a ui.Host, themePicker *dialog.ThemePicker, connManager *dialog.ConnManager, awsProfiles *dialog.AWSProfilesPicker, datadogEditor *dialog.DatadogEditor) *SettingsView {
	l := tview.NewList().ShowSecondaryText(false)
	l.SetBorder(true).SetTitle(" Settings ")

	s := &SettingsView{list: l, host: a, themePicker: themePicker, connManager: connManager, awsProfiles: awsProfiles, datadogEditor: datadogEditor}

	// Items are populated by Refresh; add placeholders here so indices
	// are stable.
	l.AddItem("", "", 0, func() { s.themePicker.Show() })
	l.AddItem("", "", 0, func() { s.connManager.Show() })
	l.AddItem("", "", 0, func() { s.awsProfiles.Show() })
	l.AddItem("", "", 0, func() { s.datadogEditor.Show() })

	l.SetInputCapture(func(event *tcell.EventKey) *tcell.EventKey {
		switch event.Rune() {
		case 'j':
			return tcell.NewEventKey(tcell.KeyDown, 0, tcell.ModNone)
		case 'k':
			return tcell.NewEventKey(tcell.KeyUp, 0, tcell.ModNone)
		}
		return event
	})

	ui.StyleList(l, a.Config().Colors)
	s.Refresh()
	return s
}

// Refresh rebuilds the displayed text of all settings-list items to
// reflect the current config values (theme name, active connection
// name, active AWS profile, Datadog site). The AWS profile name comes
// straight from cfg.ActiveAWSProfile — no need to re-read ~/.aws here,
// unlike opening the overlay itself, which lists every discoverable
// profile.
func (s *SettingsView) Refresh() {
	cfg := s.host.Config()
	cur := s.list.GetCurrentItem()
	conn := cfg.ActiveConn()
	awsProfile := cfg.ActiveAWSProfile
	if awsProfile == "" {
		awsProfile = "(none)"
	}
	s.list.Clear()
	s.list.AddItem(fmt.Sprintf("Theme: %s", cfg.Theme), "", 0, func() { s.themePicker.Show() })
	s.list.AddItem(fmt.Sprintf("AMQ Connection: %s", conn.Name), "", 0, func() { s.connManager.Show() })
	s.list.AddItem(fmt.Sprintf("AWS Profile: %s", awsProfile), "", 0, func() { s.awsProfiles.Show() })
	s.list.AddItem(fmt.Sprintf("Datadog: %s", datadogSettingsLabel(cfg.Datadog)), "", 0, func() { s.datadogEditor.Show() })
	if cur >= 0 && cur < s.list.GetItemCount() {
		s.list.SetCurrentItem(cur)
	}
}

// ApplyPalette recolors the settings list for a live theme switch.
func (s *SettingsView) ApplyPalette(p config.Palette) {
	bg := tcell.GetColor(p.Background)
	s.list.SetBackgroundColor(bg)
	s.list.SetBorderColor(tcell.GetColor(p.ViewColor("settings")))
	s.list.SetTitleColor(tcell.GetColor(p.ViewColor("settings")))
	ui.StyleList(s.list, p)
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
