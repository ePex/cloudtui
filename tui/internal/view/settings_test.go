package view

import (
	"strings"
	"testing"

	"github.com/ePex/cloudtui/tui/internal/config"
	"github.com/ePex/cloudtui/tui/internal/dialog"
)

func newTestSettingsView(t *testing.T) (*fakeViewHost, *SettingsView) {
	t.Helper()
	host := newFakeViewHost()
	confirm := dialog.NewConfirmDialog(host)
	connManager := dialog.NewConnManager(host, confirm)
	awsProfiles := dialog.NewAWSProfilesPicker(host)
	datadogEditor := dialog.NewDatadogEditor(host)
	themePicker := dialog.NewThemePicker(host)
	return host, NewSettingsView(host, themePicker, connManager, awsProfiles, datadogEditor)
}

func TestSettingsViewNameAndTitle(t *testing.T) {
	_, sv := newTestSettingsView(t)
	if got, want := sv.Name(), "settings"; got != want {
		t.Errorf("Name() = %q, want %q", got, want)
	}
	if got, want := sv.Title(), "Settings"; got != want {
		t.Errorf("Title() = %q, want %q", got, want)
	}
	if got, want := sv.List().GetTitle(), " Settings "; got != want {
		t.Errorf("list title = %q, want %q", got, want)
	}
}

func TestSettingsListHasFourItems(t *testing.T) {
	_, sv := newTestSettingsView(t)

	if got := sv.List().GetItemCount(); got != 4 {
		t.Errorf("settings list item count = %d, want 4", got)
	}
}

func TestSettingsListItemThreeIsDatadog(t *testing.T) {
	_, sv := newTestSettingsView(t)

	main3, _ := sv.List().GetItemText(3)
	if !strings.Contains(main3, "Datadog") || !strings.Contains(main3, "(none)") {
		t.Errorf("item 3 = %q, want it to contain 'Datadog' and '(none)'", main3)
	}
}

func TestSettingsListItemThreeShowsConfiguredDatadogSite(t *testing.T) {
	host, sv := newTestSettingsView(t)
	host.cfg.Datadog.Site = "datadoghq.eu"
	host.cfg.Datadog.AccessToken = "tok"

	sv.Refresh()

	main3, _ := sv.List().GetItemText(3)
	if !strings.Contains(main3, "datadoghq.eu") {
		t.Errorf("item 3 = %q, want it to contain %q", main3, "datadoghq.eu")
	}
}

func TestSettingsListItemTwoIsAWSProfile(t *testing.T) {
	_, sv := newTestSettingsView(t)

	main2, _ := sv.List().GetItemText(2)
	if !strings.Contains(main2, "AWS Profile") || !strings.Contains(main2, "(none)") {
		t.Errorf("item 2 = %q, want it to contain 'AWS Profile' and '(none)'", main2)
	}
}

func TestSettingsListItemTwoShowsActiveAWSProfile(t *testing.T) {
	host, sv := newTestSettingsView(t)
	host.cfg.ActiveAWSProfile = "work"

	sv.Refresh()

	main2, _ := sv.List().GetItemText(2)
	if !strings.Contains(main2, "work") {
		t.Errorf("item 2 = %q, want it to contain %q", main2, "work")
	}
}

func TestSettingsListItemsShowCurrentThemeAndConnection(t *testing.T) {
	_, sv := newTestSettingsView(t)

	main0, _ := sv.List().GetItemText(0)
	if !strings.Contains(main0, "Theme") || !strings.Contains(main0, "dark") {
		t.Errorf("item 0 = %q, want it to contain 'Theme' and 'dark'", main0)
	}
	main1, _ := sv.List().GetItemText(1)
	if !strings.Contains(main1, "Connection") || !strings.Contains(main1, "def") {
		t.Errorf("item 1 = %q, want it to contain 'Connection' and 'def'", main1)
	}
}

// TestSettingsViewRefreshPreservesCursorPosition covers the one piece
// of Refresh()'s own logic none of the other tests here happen to
// exercise: rebuilding the list (Clear + re-AddItem) must not reset
// the user's current selection.
func TestSettingsViewRefreshPreservesCursorPosition(t *testing.T) {
	host, sv := newTestSettingsView(t)
	sv.List().SetCurrentItem(2)

	host.cfg.ActiveAWSProfile = "work"
	sv.Refresh()

	if got := sv.List().GetCurrentItem(); got != 2 {
		t.Errorf("current item after Refresh() = %d, want 2 (preserved)", got)
	}
}

func TestDatadogSettingsLabel(t *testing.T) {
	cases := []struct {
		name string
		cfg  config.DatadogConfig
		want string
	}{
		{"unconfigured", config.DatadogConfig{}, "(none)"},
		{"token without site defaults label", config.DatadogConfig{AccessToken: "tok"}, "datadoghq.com"},
		{"token with site shows site", config.DatadogConfig{Site: "datadoghq.eu", AccessToken: "tok"}, "datadoghq.eu"},
		{"site without token still (none)", config.DatadogConfig{Site: "datadoghq.eu"}, "(none)"},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			if got := datadogSettingsLabel(c.cfg); got != c.want {
				t.Errorf("datadogSettingsLabel(%+v) = %q, want %q", c.cfg, got, c.want)
			}
		})
	}
}
