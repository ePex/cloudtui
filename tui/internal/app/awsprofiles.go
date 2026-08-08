package app

import (
	"context"
	"fmt"

	"github.com/gdamore/tcell/v2"
	"github.com/rivo/tview"
)

// setAWSProfilesHeader draws the overlay's column header row.
func (a *App) setAWSProfilesHeader() {
	p := a.cfg.Colors
	bg := tcell.GetColor(p.Label)
	fg := tcell.GetColor(p.Background)
	for i, label := range []string{"NAME", "REGION", "AUTH"} {
		a.awsProfilesTable.SetCell(0, i,
			tview.NewTableCell(label).
				SetTextColor(fg).
				SetBackgroundColor(bg).
				SetSelectable(false).
				SetExpansion(1).
				SetAlign(tview.AlignCenter))
	}
}

// showAWSProfiles opens the read-only AWS Profiles overlay. Discovery is
// local file I/O (~/.aws/config, ~/.aws/credentials), fast enough to run
// synchronously — unlike the broker-touching views, which use a
// goroutine + QueueUpdateDraw specifically because they're network calls.
func (a *App) showAWSProfiles() {
	a.populateAWSProfilesTable()
	a.rootPages.ShowPage("aws-profiles")
	a.tv.SetFocus(a.awsProfilesTable)
	a.awsProfilesVisible = true
}

// closeAWSProfiles hides the AWS Profiles overlay and restores focus.
func (a *App) closeAWSProfiles() {
	a.rootPages.HidePage("aws-profiles")
	a.awsProfilesVisible = false
	a.tv.SetFocus(a.pages)
}

// populateAWSProfilesTable re-runs discovery via a.listAWSProfiles and
// redraws the table. Called on open and on 'r' (the files may have changed
// since cloudtui started).
func (a *App) populateAWSProfilesTable() {
	t := a.awsProfilesTable
	for t.GetRowCount() > 1 {
		t.RemoveRow(t.GetRowCount() - 1)
	}

	profiles, err := a.listAWSProfiles(context.Background())
	if err != nil {
		t.SetCell(1, 0,
			tview.NewTableCell(fmt.Sprintf("Error: %v", err)).
				SetTextColor(tcell.ColorRed).
				SetExpansion(3),
		)
		t.SetTitle(" AWS Profiles ")
		return
	}

	p := a.cfg.Colors
	nameColor := tcell.GetColor(p.Value)
	textColor := tcell.GetColor(p.Text)
	for i, prof := range profiles {
		row := i + 1
		region := prof.Region
		if region == "" {
			region = "-"
		}
		t.SetCell(row, 0, tview.NewTableCell(prof.Name).SetTextColor(nameColor).SetExpansion(2))
		t.SetCell(row, 1, tview.NewTableCell(region).SetTextColor(textColor).SetExpansion(1))
		t.SetCell(row, 2, tview.NewTableCell(string(prof.AuthType)).SetTextColor(textColor).SetExpansion(1))
	}
	t.SetTitle(fmt.Sprintf(" AWS Profiles (%d) ", len(profiles)))
}
