package app

import (
	"context"
	"errors"
	"os"
	"strings"
	"testing"

	"github.com/gdamore/tcell/v2"
	"github.com/rivo/tview"

	"github.com/ePex/cloudtui/tui/internal/awsprofile"
	"github.com/ePex/cloudtui/tui/internal/config"
)

func TestShowAWSProfilesPopulatesTableFromInjectedLister(t *testing.T) {
	a := New(config.Default())
	a.listAWSProfiles = func(context.Context) ([]awsprofile.Profile, error) {
		return []awsprofile.Profile{
			{Name: "work", Region: "us-east-1", AuthType: awsprofile.AuthSSO},
			{Name: "personal", Region: "eu-west-1", AuthType: awsprofile.AuthStaticKeys},
		}, nil
	}

	a.showAWSProfiles()

	if !a.awsProfilesVisible {
		t.Fatal("awsProfilesVisible = false after showAWSProfiles()")
	}
	if got := a.tv.GetFocus(); got != a.awsProfilesTable {
		t.Errorf("focus after showAWSProfiles() = %v, want the profiles table", got)
	}
	if got := a.awsProfilesTable.GetRowCount(); got != 3 { // header + 2 profiles
		t.Fatalf("row count = %d, want 3 (header + 2 profiles)", got)
	}
	if got := a.awsProfilesTable.GetCell(1, 0).Text; got != "work" {
		t.Errorf("row 1 name = %q, want %q", got, "work")
	}
	if got := a.awsProfilesTable.GetCell(1, 2).Text; got != string(awsprofile.AuthSSO) {
		t.Errorf("row 1 auth = %q, want %q", got, awsprofile.AuthSSO)
	}
}

func TestShowAWSProfilesHandlesEmptyRegion(t *testing.T) {
	a := New(config.Default())
	a.listAWSProfiles = func(context.Context) ([]awsprofile.Profile, error) {
		return []awsprofile.Profile{{Name: "no-region", Region: "", AuthType: awsprofile.AuthUnknown}}, nil
	}

	a.showAWSProfiles()

	if got := a.awsProfilesTable.GetCell(1, 1).Text; got != "-" {
		t.Errorf("region cell for empty region = %q, want %q", got, "-")
	}
}

func TestShowAWSProfilesHandlesListerError(t *testing.T) {
	a := New(config.Default())
	a.listAWSProfiles = func(context.Context) ([]awsprofile.Profile, error) {
		return nil, errors.New("boom")
	}

	a.showAWSProfiles()

	if got := a.awsProfilesTable.GetCell(1, 0).Text; !strings.Contains(got, "boom") {
		t.Errorf("error cell = %q, want it to contain the error message", got)
	}
}

func TestAWSProfilesRefreshReinvokesLister(t *testing.T) {
	a := New(config.Default())
	calls := 0
	a.listAWSProfiles = func(context.Context) ([]awsprofile.Profile, error) {
		calls++
		return nil, nil
	}

	a.showAWSProfiles() // first call
	capture := a.awsProfilesTable.GetInputCapture()
	if capture == nil {
		t.Fatal("awsProfilesTable has no input capture set")
	}
	capture(tcell.NewEventKey(tcell.KeyRune, 'r', tcell.ModNone))

	if calls != 2 {
		t.Errorf("listAWSProfiles called %d times, want 2 (open + refresh)", calls)
	}
}

func TestAWSProfilesEscCloses(t *testing.T) {
	a := New(config.Default())
	a.listAWSProfiles = func(context.Context) ([]awsprofile.Profile, error) { return nil, nil }
	a.showAWSProfiles()

	capture := a.awsProfilesTable.GetInputCapture()
	if got := capture(tcell.NewEventKey(tcell.KeyEscape, 0, tcell.ModNone)); got != nil {
		t.Errorf("Esc capture returned %v, want nil (event consumed)", got)
	}
	if a.awsProfilesVisible {
		t.Error("Esc did not close the AWS Profiles overlay")
	}
}

func TestAWSProfilesSlashFocusesFilterInput(t *testing.T) {
	a := New(config.Default())
	a.listAWSProfiles = func(context.Context) ([]awsprofile.Profile, error) { return nil, nil }
	a.showAWSProfiles()

	capture := a.awsProfilesTable.GetInputCapture()
	capture(tcell.NewEventKey(tcell.KeyRune, '/', tcell.ModNone))

	if got := a.tv.GetFocus(); got != a.awsProfilesFilterInput {
		t.Errorf("focus after '/' = %v, want the filter input", got)
	}
}

func TestApplyAWSProfilesFilterNarrowsRowsByName(t *testing.T) {
	a := New(config.Default())
	a.listAWSProfiles = func(context.Context) ([]awsprofile.Profile, error) {
		return []awsprofile.Profile{
			{Name: "example-dev", Region: "eu-central-1", AuthType: awsprofile.AuthSSO},
			{Name: "example-prod", Region: "eu-central-1", AuthType: awsprofile.AuthSSO},
			{Name: "personal", Region: "us-east-1", AuthType: awsprofile.AuthStaticKeys},
		}, nil
	}
	a.showAWSProfiles()

	a.applyAWSProfilesFilter("example")

	if got := a.awsProfilesTable.GetRowCount(); got != 3 { // header + 2 matches
		t.Fatalf("row count after filter = %d, want 3 (header + 2 matches)", got)
	}
	if got := a.awsProfilesTable.GetTitle(); got != " AWS Profiles [example] " {
		t.Errorf("title after filter = %q, want %q", got, " AWS Profiles [example] ")
	}
}

func TestApplyAWSProfilesFilterClearRestoresAll(t *testing.T) {
	a := New(config.Default())
	a.listAWSProfiles = func(context.Context) ([]awsprofile.Profile, error) {
		return []awsprofile.Profile{
			{Name: "one", AuthType: awsprofile.AuthUnknown},
			{Name: "two", AuthType: awsprofile.AuthUnknown},
		}, nil
	}
	a.showAWSProfiles()
	a.applyAWSProfilesFilter("one")

	a.applyAWSProfilesFilter("")

	if got := a.awsProfilesTable.GetRowCount(); got != 3 { // header + 2
		t.Errorf("row count after clearing filter = %d, want 3", got)
	}
}

func TestShowAWSProfilesResetsFilterFromPreviousVisit(t *testing.T) {
	a := New(config.Default())
	a.listAWSProfiles = func(context.Context) ([]awsprofile.Profile, error) {
		return []awsprofile.Profile{{Name: "one"}, {Name: "two"}}, nil
	}
	a.showAWSProfiles()
	a.applyAWSProfilesFilter("one")
	a.closeAWSProfiles()

	a.showAWSProfiles() // reopen

	if got := a.awsProfilesTable.GetRowCount(); got != 3 { // header + both, filter reset
		t.Errorf("row count on reopen = %d, want 3 (filter should reset)", got)
	}
	if got := a.awsProfilesFilterInput.GetText(); got != "" {
		t.Errorf("filter input text on reopen = %q, want empty", got)
	}
}

func TestActivateAWSProfilePersistsAndUpdatesUI(t *testing.T) {
	t.Chdir(t.TempDir())
	a := New(config.Default())
	a.listAWSProfiles = func(context.Context) ([]awsprofile.Profile, error) { return nil, nil }
	a.showAWSProfiles()

	a.activateAWSProfile("work")

	if got := a.cfg.ActiveAWSProfile; got != "work" {
		t.Errorf("cfg.ActiveAWSProfile = %q, want %q", got, "work")
	}
	if got := a.infoPanel.GetText(true); !strings.Contains(got, "work") {
		t.Errorf("info panel = %q, want it to contain %q", got, "work")
	}
	if a.awsProfilesVisible {
		t.Error("overlay should close after activating a profile")
	}
	if _, err := os.Stat("config.yaml"); err != nil {
		t.Errorf("config.yaml not written after activateAWSProfile: %v", err)
	}
}

func TestAWSProfilesActiveProfileMarkedWithStar(t *testing.T) {
	a := New(config.Default())
	a.cfg.ActiveAWSProfile = "work"
	a.listAWSProfiles = func(context.Context) ([]awsprofile.Profile, error) {
		return []awsprofile.Profile{{Name: "work"}, {Name: "other"}}, nil
	}

	a.showAWSProfiles()

	if got := a.awsProfilesTable.GetCell(1, 0).Text; !strings.Contains(got, "⭐") {
		t.Errorf("active profile row = %q, want it marked with ⭐", got)
	}
	if got := a.awsProfilesTable.GetCell(2, 0).Text; strings.Contains(got, "⭐") {
		t.Errorf("inactive profile row = %q, want no ⭐", got)
	}
}

func TestAWSProfilesEnterActivatesRowRespectingFilter(t *testing.T) {
	t.Chdir(t.TempDir())
	a := New(config.Default())
	a.listAWSProfiles = func(context.Context) ([]awsprofile.Profile, error) {
		return []awsprofile.Profile{
			{Name: "example-dev"},
			{Name: "personal"},
		}, nil
	}
	a.showAWSProfiles()
	a.applyAWSProfilesFilter("personal") // only "personal" remains, at row 1

	a.awsProfilesTable.Select(1, 0)
	// Invoke the table's registered SetSelectedFunc handler directly.
	handler := a.awsProfilesTable.InputHandler()
	handler(tcell.NewEventKey(tcell.KeyEnter, 0, tcell.ModNone), func(tview.Primitive) {})

	if got := a.cfg.ActiveAWSProfile; got != "personal" {
		t.Errorf("cfg.ActiveAWSProfile = %q, want %q (the filtered row, not the unfiltered index)", got, "personal")
	}
}
