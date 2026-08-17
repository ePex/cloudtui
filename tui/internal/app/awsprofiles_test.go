package app

import (
	"context"
	"errors"
	"fmt"
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

	a.awsProfiles.Show()

	if !a.awsProfiles.visible {
		t.Fatal("awsProfiles.visible = false after awsProfiles.Show()")
	}
	if got := a.tv.GetFocus(); got != a.awsProfiles.table {
		t.Errorf("focus after awsProfiles.Show() = %v, want the profiles table", got)
	}
	if got := a.awsProfiles.table.GetRowCount(); got != 3 { // header + 2 profiles
		t.Fatalf("row count = %d, want 3 (header + 2 profiles)", got)
	}
	if got := a.awsProfiles.table.GetCell(1, 0).Text; got != "work" {
		t.Errorf("row 1 name = %q, want %q", got, "work")
	}
	if got := a.awsProfiles.table.GetCell(1, 2).Text; got != string(awsprofile.AuthSSO) {
		t.Errorf("row 1 auth = %q, want %q", got, awsprofile.AuthSSO)
	}
}

func TestShowAWSProfilesHandlesEmptyRegion(t *testing.T) {
	a := New(config.Default())
	a.listAWSProfiles = func(context.Context) ([]awsprofile.Profile, error) {
		return []awsprofile.Profile{{Name: "no-region", Region: "", AuthType: awsprofile.AuthUnknown}}, nil
	}

	a.awsProfiles.Show()

	if got := a.awsProfiles.table.GetCell(1, 1).Text; got != "-" {
		t.Errorf("region cell for empty region = %q, want %q", got, "-")
	}
}

func TestShowAWSProfilesHandlesListerError(t *testing.T) {
	a := New(config.Default())
	a.listAWSProfiles = func(context.Context) ([]awsprofile.Profile, error) {
		return nil, errors.New("boom")
	}

	a.awsProfiles.Show()

	if got := a.awsProfiles.table.GetCell(1, 0).Text; !strings.Contains(got, "boom") {
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

	a.awsProfiles.Show() // first call
	capture := a.awsProfiles.table.GetInputCapture()
	if capture == nil {
		t.Fatal("awsProfiles.table has no input capture set")
	}
	capture(tcell.NewEventKey(tcell.KeyRune, 'r', tcell.ModNone))

	if calls != 2 {
		t.Errorf("listAWSProfiles called %d times, want 2 (open + refresh)", calls)
	}
}

func TestAWSProfilesEscCloses(t *testing.T) {
	a := New(config.Default())
	a.listAWSProfiles = func(context.Context) ([]awsprofile.Profile, error) { return nil, nil }
	a.awsProfiles.Show()

	capture := a.awsProfiles.table.GetInputCapture()
	if got := capture(tcell.NewEventKey(tcell.KeyEscape, 0, tcell.ModNone)); got != nil {
		t.Errorf("Esc capture returned %v, want nil (event consumed)", got)
	}
	if a.awsProfiles.visible {
		t.Error("Esc did not close the AWS Profiles overlay")
	}
}

func TestAWSProfilesSlashFocusesFilterInput(t *testing.T) {
	a := New(config.Default())
	a.listAWSProfiles = func(context.Context) ([]awsprofile.Profile, error) { return nil, nil }
	a.awsProfiles.Show()

	capture := a.awsProfiles.table.GetInputCapture()
	capture(tcell.NewEventKey(tcell.KeyRune, '/', tcell.ModNone))

	if got := a.tv.GetFocus(); got != a.awsProfiles.filterInput {
		t.Errorf("focus after '/' = %v, want the filter input", got)
	}
}

func TestApplyAWSProfilesFilterNarrowsRowsByName(t *testing.T) {
	a := New(config.Default())
	a.listAWSProfiles = func(context.Context) ([]awsprofile.Profile, error) {
		return []awsprofile.Profile{
			{Name: "work-dev", Region: "eu-central-1", AuthType: awsprofile.AuthSSO},
			{Name: "work-prod", Region: "eu-central-1", AuthType: awsprofile.AuthSSO},
			{Name: "personal", Region: "us-east-1", AuthType: awsprofile.AuthStaticKeys},
		}, nil
	}
	a.awsProfiles.Show()

	a.awsProfiles.applyFilter("work")

	if got := a.awsProfiles.table.GetRowCount(); got != 3 { // header + 2 matches
		t.Fatalf("row count after filter = %d, want 3 (header + 2 matches)", got)
	}
	if got := a.awsProfiles.table.GetTitle(); got != " AWS Profiles (work) " {
		t.Errorf("title after filter = %q, want %q", got, " AWS Profiles (work) ")
	}

	rendered := renderedScreenText(t, a.awsProfiles.table, 60, 10)
	if !strings.Contains(rendered, "work") {
		t.Errorf("rendered screen = %q, want it to contain the filter text %q", rendered, "work")
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
	a.awsProfiles.Show()
	a.awsProfiles.applyFilter("one")

	a.awsProfiles.applyFilter("")

	if got := a.awsProfiles.table.GetRowCount(); got != 3 { // header + 2
		t.Errorf("row count after clearing filter = %d, want 3", got)
	}
}

func TestShowAWSProfilesResetsFilterFromPreviousVisit(t *testing.T) {
	a := New(config.Default())
	a.listAWSProfiles = func(context.Context) ([]awsprofile.Profile, error) {
		return []awsprofile.Profile{{Name: "one"}, {Name: "two"}}, nil
	}
	a.awsProfiles.Show()
	a.awsProfiles.applyFilter("one")
	a.awsProfiles.close()

	a.awsProfiles.Show() // reopen

	if got := a.awsProfiles.table.GetRowCount(); got != 3 { // header + both, filter reset
		t.Errorf("row count on reopen = %d, want 3 (filter should reset)", got)
	}
	if got := a.awsProfiles.filterInput.GetText(); got != "" {
		t.Errorf("filter input text on reopen = %q, want empty", got)
	}
}

func TestActivateAWSProfilePersistsAndUpdatesUI(t *testing.T) {
	t.Chdir(t.TempDir())
	a := New(config.Default())
	a.listAWSProfiles = func(context.Context) ([]awsprofile.Profile, error) { return nil, nil }
	a.awsProfiles.Show()

	a.awsProfiles.activate("work")

	if got := a.cfg.ActiveAWSProfile; got != "work" {
		t.Errorf("cfg.ActiveAWSProfile = %q, want %q", got, "work")
	}
	if got := a.infoPanel.GetText(true); !strings.Contains(got, "work") {
		t.Errorf("info panel = %q, want it to contain %q", got, "work")
	}
	if main2, _ := a.settingsList.GetItemText(2); !strings.Contains(main2, "work") {
		t.Errorf("settings list item 2 = %q, want it to contain %q", main2, "work")
	}
	if a.awsProfiles.visible {
		t.Error("overlay should close after activating a profile")
	}
	if _, err := os.Stat("config.yaml"); err != nil {
		t.Errorf("config.yaml not written after awsProfiles.activate: %v", err)
	}
}

func TestAWSProfilesActiveProfileMarkedWithStar(t *testing.T) {
	a := New(config.Default())
	a.cfg.ActiveAWSProfile = "work"
	a.listAWSProfiles = func(context.Context) ([]awsprofile.Profile, error) {
		return []awsprofile.Profile{{Name: "work"}, {Name: "other"}}, nil
	}

	a.awsProfiles.Show()

	if got := a.awsProfiles.table.GetCell(1, 0).Text; !strings.Contains(got, "⭐") {
		t.Errorf("active profile row = %q, want it marked with ⭐", got)
	}
	if got := a.awsProfiles.table.GetCell(2, 0).Text; strings.Contains(got, "⭐") {
		t.Errorf("inactive profile row = %q, want no ⭐", got)
	}
}

func TestAWSProfilesEnterActivatesRowRespectingFilter(t *testing.T) {
	t.Chdir(t.TempDir())
	a := New(config.Default())
	a.listAWSProfiles = func(context.Context) ([]awsprofile.Profile, error) {
		return []awsprofile.Profile{
			{Name: "work-dev"},
			{Name: "personal"},
		}, nil
	}
	a.awsProfiles.Show()
	a.awsProfiles.applyFilter("personal") // only "personal" remains, at row 1

	a.awsProfiles.table.Select(1, 0)
	// Invoke the table's registered SetSelectedFunc handler directly.
	handler := a.awsProfiles.table.InputHandler()
	handler(tcell.NewEventKey(tcell.KeyEnter, 0, tcell.ModNone), func(tview.Primitive) {})

	if got := a.cfg.ActiveAWSProfile; got != "personal" {
		t.Errorf("cfg.ActiveAWSProfile = %q, want %q (the filtered row, not the unfiltered index)", got, "personal")
	}
}

// TestRepaintAWSProfilesScrollsToTopWithManyRows guards against the same
// bug fixed for queuesView (spec/11-bugfix-queues-scroll-to-top):
// tview.Table's "track end" auto-scroll latches on during the table's
// first, still-empty draw and stays latched through repaint, scrolling a
// long list to the bottom instead of the top.
func TestRepaintAWSProfilesScrollsToTopWithManyRows(t *testing.T) {
	a := New(config.Default())
	profiles := make([]awsprofile.Profile, 50)
	for i := range profiles {
		profiles[i] = awsprofile.Profile{Name: fmt.Sprintf("profile-%02d", i)}
	}
	a.listAWSProfiles = func(context.Context) ([]awsprofile.Profile, error) {
		return profiles, nil
	}

	table := a.awsProfiles.table
	table.SetRect(0, 0, 60, 15) // fewer visible rows than profiles above

	screen := tcell.NewSimulationScreen("")
	if err := screen.Init(); err != nil {
		t.Fatalf("screen.Init: %v", err)
	}
	screen.SetSize(60, 15)

	// First draw while the table is still empty, mirroring the real
	// sequence: awsProfiles.show draws the overlay before populating it.
	table.Draw(screen)

	a.awsProfiles.Show()

	// The redraw that follows population.
	table.Draw(screen)

	if row, _ := table.GetOffset(); row != 0 {
		t.Errorf("table scrolled away from top: rowOffset = %d, want 0", row)
	}
}
