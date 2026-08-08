package app

import (
	"context"
	"errors"
	"strings"
	"testing"

	"github.com/gdamore/tcell/v2"

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
