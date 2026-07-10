package app

import (
	"fmt"
	"os"

	"github.com/rivo/tview"

	"github.com/ePex/cloudtui/tui/internal/awsprofile"
	"github.com/ePex/cloudtui/tui/internal/config"
	"github.com/ePex/cloudtui/tui/internal/ui"
)

const (
	profilePickerWidth  = 40
	profilePickerHeight = 12
)

// settingsView is the Settings screen: a list of configurable settings.
// Unlike the other (stateless placeholder) views, it needs live config
// read/write and to trigger the profile-picker overlay, so it lives here
// instead of internal/ui/views.
type settingsView struct {
	list *tview.List
}

func (s *settingsView) Name() string               { return "settings" }
func (s *settingsView) Title() string              { return "Settings" }
func (s *settingsView) Primitive() tview.Primitive { return s.list }

// newSettingsView builds the Settings view and wires its "AWS Profile"
// row to a's profile picker.
func newSettingsView(a *App) ui.View {
	list := tview.NewList().ShowSecondaryText(true)
	list.AddItem("AWS Profile", profileSecondaryText(a.cfg), 0, a.openProfilePicker)
	// Read-only for now: unlike AWS profiles, a proxy URL isn't a
	// discoverable list to pick from, so there's no picker here — edit
	// config.yaml by hand. An interactive editor is a reasonable later
	// increment, not this one.
	list.AddItem("Queue Connection", queueConnectionSecondaryText(a.cfg), 0, nil)
	list.SetBorder(true).SetTitle(" Settings ")

	a.settingsList = list
	return &settingsView{list: list}
}

// profileSecondaryText is the settings list's "AWS Profile" row value.
func profileSecondaryText(cfg config.Config) string {
	if cfg.AWS.Profile == "" {
		return "not set"
	}
	return cfg.AWS.Profile
}

// queueConnectionSecondaryText is the settings list's "Queue Connection" row value.
func queueConnectionSecondaryText(cfg config.Config) string {
	if cfg.Queue.ProxyURL == "" {
		return "not set"
	}
	return cfg.Queue.ProxyURL
}

// openProfilePicker shows a modal listing the AWS profiles discovered on
// this machine, pre-selecting AWS_PROFILE/AWS_DEFAULT_PROFILE (or
// "default") if present among them.
func (a *App) openProfilePicker() {
	names, err := awsprofile.List()

	picker := tview.NewList().ShowSecondaryText(false)
	picker.SetBorder(true).SetTitle(" AWS Profile ")
	picker.SetDoneFunc(a.closeProfilePicker)

	switch {
	case err != nil:
		picker.AddItem(fmt.Sprintf("error: %v", err), "", 0, nil)
	case len(names) == 0:
		picker.AddItem("no profiles found", "", 0, nil)
	default:
		preferred := os.Getenv("AWS_PROFILE")
		if preferred == "" {
			preferred = os.Getenv("AWS_DEFAULT_PROFILE")
		}
		if preferred == "" {
			preferred = "default"
		}
		selectIndex := 0
		for i, name := range names {
			picker.AddItem(name, "", 0, func() { a.selectProfile(name) })
			if name == preferred {
				selectIndex = i
			}
		}
		picker.SetCurrentItem(selectIndex)
	}

	a.rootPages.AddPage("profile-picker", centered(picker, profilePickerWidth, profilePickerHeight), true, false)
	a.rootPages.ShowPage("profile-picker")
	a.tv.SetFocus(picker)
}

// selectProfile persists name as the chosen AWS profile, updates the
// settings list and the top bar, and closes the picker.
func (a *App) selectProfile(name string) {
	a.cfg.AWS.Profile = name
	if err := config.SaveDefault(a.cfg); err != nil {
		fmt.Fprintf(os.Stderr, "cloudtui: saving config: %v\n", err)
	}
	a.settingsList.SetItemText(0, "AWS Profile", profileSecondaryText(a.cfg))
	a.refreshInfoPanel()
	a.closeProfilePicker()
}

// closeProfilePicker hides and discards the picker overlay and returns
// focus to the main pages area.
func (a *App) closeProfilePicker() {
	a.rootPages.HidePage("profile-picker")
	a.rootPages.RemovePage("profile-picker")
	a.tv.SetFocus(a.pages)
}
