package app

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/rivo/tview"
)

// pickerList digs the *tview.List out of the nested Flex built by
// centered(), the same structure used for the help modal.
func pickerList(t *testing.T, prim tview.Primitive) *tview.List {
	t.Helper()
	outer, ok := prim.(*tview.Flex)
	if !ok {
		t.Fatalf("picker primitive = %T, want *tview.Flex", prim)
	}
	inner, ok := outer.GetItem(1).(*tview.Flex)
	if !ok {
		t.Fatalf("picker outer.GetItem(1) = %T, want *tview.Flex", outer.GetItem(1))
	}
	list, ok := inner.GetItem(1).(*tview.List)
	if !ok {
		t.Fatalf("picker inner.GetItem(1) = %T, want *tview.List", inner.GetItem(1))
	}
	return list
}

func writeAWSFile(t *testing.T, path, content string) {
	t.Helper()
	if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
		t.Fatal(err)
	}
}

func TestSettingsListReflectsCurrentProfile(t *testing.T) {
	a := New()

	main, secondary := a.settingsList.GetItemText(0)
	if main != "AWS Profile" {
		t.Errorf("item main text = %q, want %q", main, "AWS Profile")
	}
	if want := profileSecondaryText(a.cfg); secondary != want {
		t.Errorf("item secondary text = %q, want %q", secondary, want)
	}
}

func TestOpenProfilePickerPopulatesAndPreselects(t *testing.T) {
	dir := t.TempDir()
	configPath := filepath.Join(dir, "config")
	credsPath := filepath.Join(dir, "credentials")
	writeAWSFile(t, configPath, "[default]\n[profile foo]\n")
	writeAWSFile(t, credsPath, "[bar]\n")

	t.Setenv("AWS_CONFIG_FILE", configPath)
	t.Setenv("AWS_SHARED_CREDENTIALS_FILE", credsPath)
	t.Setenv("AWS_PROFILE", "foo")

	a := New()
	a.openProfilePicker()

	if !a.rootPages.HasPage("profile-picker") {
		t.Fatal("rootPages has no \"profile-picker\" page after openProfilePicker()")
	}

	list := pickerList(t, a.rootPages.GetPage("profile-picker"))
	if got, want := list.GetItemCount(), 3; got != want {
		t.Fatalf("picker item count = %d, want %d", got, want)
	}

	wantNames := []string{"bar", "default", "foo"}
	for i, want := range wantNames {
		main, _ := list.GetItemText(i)
		if main != want {
			t.Errorf("picker item %d = %q, want %q", i, main, want)
		}
	}

	if got, want := list.GetCurrentItem(), 2; got != want { // "foo" is index 2 in the sorted list
		t.Errorf("preselected item = %d, want %d (%q)", got, want, "foo")
	}
}

func TestOpenProfilePickerShowsMessageWhenNoneFound(t *testing.T) {
	dir := t.TempDir()
	t.Setenv("AWS_CONFIG_FILE", filepath.Join(dir, "missing-config"))
	t.Setenv("AWS_SHARED_CREDENTIALS_FILE", filepath.Join(dir, "missing-credentials"))

	a := New()
	a.openProfilePicker()

	list := pickerList(t, a.rootPages.GetPage("profile-picker"))
	if got, want := list.GetItemCount(), 1; got != want {
		t.Fatalf("picker item count = %d, want %d", got, want)
	}
	if main, _ := list.GetItemText(0); main != "no profiles found" {
		t.Errorf("picker item = %q, want %q", main, "no profiles found")
	}
}

func TestSelectProfilePersistsAndUpdatesUI(t *testing.T) {
	t.Chdir(t.TempDir())

	a := New()
	a.openProfilePicker()

	a.selectProfile("my-profile")

	if got, want := a.cfg.AWS.Profile, "my-profile"; got != want {
		t.Errorf("a.cfg.AWS.Profile = %q, want %q", got, want)
	}
	if _, secondary := a.settingsList.GetItemText(0); secondary != "my-profile" {
		t.Errorf("settings list secondary text = %q, want %q", secondary, "my-profile")
	}
	if got := a.infoPanel.GetText(true); !strings.Contains(got, "my-profile") {
		t.Errorf("info panel text = %q, want it to contain %q", got, "my-profile")
	}
	if a.rootPages.HasPage("profile-picker") {
		t.Error("profile-picker page still present after selectProfile()")
	}

	if _, err := os.Stat("config.yaml"); err != nil {
		t.Errorf("config.yaml not written: %v", err)
	}
}

func TestClosePickerLeavesConfigUnchanged(t *testing.T) {
	t.Chdir(t.TempDir())

	a := New()
	a.openProfilePicker()
	a.closeProfilePicker()

	if got := a.cfg.AWS.Profile; got != "" {
		t.Errorf("a.cfg.AWS.Profile = %q, want empty after cancel", got)
	}
	if a.rootPages.HasPage("profile-picker") {
		t.Error("profile-picker page still present after closeProfilePicker()")
	}
}
