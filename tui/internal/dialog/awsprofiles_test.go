package dialog

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"testing"

	"github.com/gdamore/tcell/v2"
	"github.com/rivo/tview"

	"github.com/ePex/cloudtui/tui/internal/awsprofile"
)

func newTestAWSProfiles(t *testing.T) (*AWSProfilesPicker, *testHost) {
	t.Helper()
	host := newTestHost()
	return NewAWSProfilesPicker(host), host
}

func TestShowAWSProfilesPopulatesTableFromInjectedLister(t *testing.T) {
	ap, host := newTestAWSProfiles(t)
	host.listAWSProfiles = func(context.Context) ([]awsprofile.Profile, error) {
		return []awsprofile.Profile{
			{Name: "work", Region: "us-east-1", AuthType: awsprofile.AuthSSO},
			{Name: "personal", Region: "eu-west-1", AuthType: awsprofile.AuthStaticKeys},
		}, nil
	}

	ap.Show()

	if !ap.visible {
		t.Fatal("AWSProfilesPicker.visible = false after Show()")
	}
	if got := host.focused; got != ap.table {
		t.Errorf("focus after Show() = %v, want the profiles table", got)
	}
	if got := ap.table.GetRowCount(); got != 3 { // header + 2 profiles
		t.Fatalf("row count = %d, want 3 (header + 2 profiles)", got)
	}
	if got := ap.table.GetCell(1, 0).Text; got != "work" {
		t.Errorf("row 1 name = %q, want %q", got, "work")
	}
	if got := ap.table.GetCell(1, 2).Text; got != string(awsprofile.AuthSSO) {
		t.Errorf("row 1 auth = %q, want %q", got, awsprofile.AuthSSO)
	}
}

func TestShowAWSProfilesHandlesEmptyRegion(t *testing.T) {
	ap, host := newTestAWSProfiles(t)
	host.listAWSProfiles = func(context.Context) ([]awsprofile.Profile, error) {
		return []awsprofile.Profile{{Name: "no-region", Region: "", AuthType: awsprofile.AuthUnknown}}, nil
	}

	ap.Show()

	if got := ap.table.GetCell(1, 1).Text; got != "-" {
		t.Errorf("region cell for empty region = %q, want %q", got, "-")
	}
}

func TestShowAWSProfilesHandlesListerError(t *testing.T) {
	ap, host := newTestAWSProfiles(t)
	host.listAWSProfiles = func(context.Context) ([]awsprofile.Profile, error) {
		return nil, errors.New("boom")
	}

	ap.Show()

	if got := ap.table.GetCell(1, 0).Text; !strings.Contains(got, "boom") {
		t.Errorf("error cell = %q, want it to contain the error message", got)
	}
}

func TestAWSProfilesRefreshReinvokesLister(t *testing.T) {
	ap, host := newTestAWSProfiles(t)
	calls := 0
	host.listAWSProfiles = func(context.Context) ([]awsprofile.Profile, error) {
		calls++
		return nil, nil
	}

	ap.Show() // first call
	capture := ap.table.GetInputCapture()
	if capture == nil {
		t.Fatal("AWSProfilesPicker.table has no input capture set")
	}
	capture(tcell.NewEventKey(tcell.KeyRune, 'r', tcell.ModNone))

	if calls != 2 {
		t.Errorf("listAWSProfiles called %d times, want 2 (open + refresh)", calls)
	}
}

func TestAWSProfilesEscCloses(t *testing.T) {
	ap, host := newTestAWSProfiles(t)
	host.listAWSProfiles = func(context.Context) ([]awsprofile.Profile, error) { return nil, nil }
	ap.Show()

	capture := ap.table.GetInputCapture()
	if got := capture(tcell.NewEventKey(tcell.KeyEscape, 0, tcell.ModNone)); got != nil {
		t.Errorf("Esc capture returned %v, want nil (event consumed)", got)
	}
	if ap.visible {
		t.Error("Esc did not close the AWS Profiles overlay")
	}
}

func TestAWSProfilesSlashFocusesFilterInput(t *testing.T) {
	ap, host := newTestAWSProfiles(t)
	host.listAWSProfiles = func(context.Context) ([]awsprofile.Profile, error) { return nil, nil }
	ap.Show()

	capture := ap.table.GetInputCapture()
	capture(tcell.NewEventKey(tcell.KeyRune, '/', tcell.ModNone))

	if got := host.focused; got != ap.filterInput {
		t.Errorf("focus after '/' = %v, want the filter input", got)
	}
}

func TestApplyAWSProfilesFilterNarrowsRowsByName(t *testing.T) {
	ap, host := newTestAWSProfiles(t)
	host.listAWSProfiles = func(context.Context) ([]awsprofile.Profile, error) {
		return []awsprofile.Profile{
			{Name: "work-dev", Region: "eu-central-1", AuthType: awsprofile.AuthSSO},
			{Name: "work-prod", Region: "eu-central-1", AuthType: awsprofile.AuthSSO},
			{Name: "personal", Region: "us-east-1", AuthType: awsprofile.AuthStaticKeys},
		}, nil
	}
	ap.Show()

	ap.applyFilter("work")

	if got := ap.table.GetRowCount(); got != 3 { // header + 2 matches
		t.Fatalf("row count after filter = %d, want 3 (header + 2 matches)", got)
	}
	if got := ap.table.GetTitle(); got != " AWS Profiles (work) " {
		t.Errorf("title after filter = %q, want %q", got, " AWS Profiles (work) ")
	}

	rendered := renderedScreenText(t, ap.table, 60, 10)
	if !strings.Contains(rendered, "work") {
		t.Errorf("rendered screen = %q, want it to contain the filter text %q", rendered, "work")
	}
}

func TestApplyAWSProfilesFilterClearRestoresAll(t *testing.T) {
	ap, host := newTestAWSProfiles(t)
	host.listAWSProfiles = func(context.Context) ([]awsprofile.Profile, error) {
		return []awsprofile.Profile{
			{Name: "one", AuthType: awsprofile.AuthUnknown},
			{Name: "two", AuthType: awsprofile.AuthUnknown},
		}, nil
	}
	ap.Show()
	ap.applyFilter("one")

	ap.applyFilter("")

	if got := ap.table.GetRowCount(); got != 3 { // header + 2
		t.Errorf("row count after clearing filter = %d, want 3", got)
	}
}

func TestShowAWSProfilesResetsFilterFromPreviousVisit(t *testing.T) {
	ap, host := newTestAWSProfiles(t)
	host.listAWSProfiles = func(context.Context) ([]awsprofile.Profile, error) {
		return []awsprofile.Profile{{Name: "one"}, {Name: "two"}}, nil
	}
	ap.Show()
	ap.applyFilter("one")
	ap.close()

	ap.Show() // reopen

	if got := ap.table.GetRowCount(); got != 3 { // header + both, filter reset
		t.Errorf("row count on reopen = %d, want 3 (filter should reset)", got)
	}
	if got := ap.filterInput.GetText(); got != "" {
		t.Errorf("filter input text on reopen = %q, want empty", got)
	}
}

// TestAWSProfilesActivateCallsHostAndCloses confirms activate() hands
// the right name to host.SetActiveAWSProfile, reports it in the status
// bar, and closes — the disk-persistence/info-panel/settings-list half
// of what this test verified before the CR 78 move (App's real
// SetActiveAWSProfile wiring) is App's own responsibility now, covered
// by internal/app/host_test.go's
// TestSetActiveAWSProfilePersistsAndUpdatesUI instead: testHost
// deliberately only records this call, it doesn't persist or update
// any other UI.
func TestAWSProfilesActivateCallsHostAndCloses(t *testing.T) {
	ap, host := newTestAWSProfiles(t)
	host.listAWSProfiles = func(context.Context) ([]awsprofile.Profile, error) { return nil, nil }
	ap.Show()

	ap.activate("work")

	if got := host.activeAWSProfile; got != "work" {
		t.Errorf("SetActiveAWSProfile called with %q, want %q", got, "work")
	}
	if !strings.Contains(host.status, "work") {
		t.Errorf("status = %q, want it to contain %q", host.status, "work")
	}
	if ap.visible {
		t.Error("overlay should close after activating a profile")
	}
}

func TestAWSProfilesActiveProfileMarkedWithStar(t *testing.T) {
	ap, host := newTestAWSProfiles(t)
	host.cfg.ActiveAWSProfile = "work"
	host.listAWSProfiles = func(context.Context) ([]awsprofile.Profile, error) {
		return []awsprofile.Profile{{Name: "work"}, {Name: "other"}}, nil
	}

	ap.Show()

	if got := ap.table.GetCell(1, 0).Text; !strings.Contains(got, "⭐") {
		t.Errorf("active profile row = %q, want it marked with ⭐", got)
	}
	if got := ap.table.GetCell(2, 0).Text; strings.Contains(got, "⭐") {
		t.Errorf("inactive profile row = %q, want no ⭐", got)
	}
}

func TestAWSProfilesEnterActivatesRowRespectingFilter(t *testing.T) {
	ap, host := newTestAWSProfiles(t)
	host.listAWSProfiles = func(context.Context) ([]awsprofile.Profile, error) {
		return []awsprofile.Profile{
			{Name: "work-dev"},
			{Name: "personal"},
		}, nil
	}
	ap.Show()
	ap.applyFilter("personal") // only "personal" remains, at row 1

	ap.table.Select(1, 0)
	// Invoke the table's registered SetSelectedFunc handler directly.
	handler := ap.table.InputHandler()
	handler(tcell.NewEventKey(tcell.KeyEnter, 0, tcell.ModNone), func(tview.Primitive) {})

	if got := host.activeAWSProfile; got != "personal" {
		t.Errorf("activeAWSProfile = %q, want %q (the filtered row, not the unfiltered index)", got, "personal")
	}
}

// TestRepaintAWSProfilesScrollsToTopWithManyRows guards against the same
// bug fixed for queuesView (spec/11-bugfix-queues-scroll-to-top):
// tview.Table's "track end" auto-scroll latches on during the table's
// first, still-empty draw and stays latched through repaint, scrolling a
// long list to the bottom instead of the top.
func TestRepaintAWSProfilesScrollsToTopWithManyRows(t *testing.T) {
	ap, host := newTestAWSProfiles(t)
	profiles := make([]awsprofile.Profile, 50)
	for i := range profiles {
		profiles[i] = awsprofile.Profile{Name: fmt.Sprintf("profile-%02d", i)}
	}
	host.listAWSProfiles = func(context.Context) ([]awsprofile.Profile, error) {
		return profiles, nil
	}

	table := ap.table
	table.SetRect(0, 0, 60, 15) // fewer visible rows than profiles above

	screen := tcell.NewSimulationScreen("")
	if err := screen.Init(); err != nil {
		t.Fatalf("screen.Init: %v", err)
	}
	screen.SetSize(60, 15)

	// First draw while the table is still empty, mirroring the real
	// sequence: AWSProfilesPicker.Show draws the overlay before populating it.
	table.Draw(screen)

	ap.Show()

	// The redraw that follows population.
	table.Draw(screen)

	if row, _ := table.GetOffset(); row != 0 {
		t.Errorf("table scrolled away from top: rowOffset = %d, want 0", row)
	}
}
