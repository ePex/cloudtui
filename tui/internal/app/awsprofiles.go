package app

import (
	"context"
	"fmt"
	"log/slog"
	"strings"

	"github.com/gdamore/tcell/v2"
	"github.com/rivo/tview"

	"github.com/ePex/cloudtui/tui/internal/awsprofile"
	"github.com/ePex/cloudtui/tui/internal/config"
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

// showAWSProfiles opens the AWS Profiles overlay, resetting any filter left
// over from a previous visit. Discovery is local file I/O (~/.aws/config,
// ~/.aws/credentials), fast enough to run synchronously — unlike the
// broker-touching views, which use a goroutine + QueueUpdateDraw
// specifically because they're network calls.
func (a *App) showAWSProfiles() {
	a.awsProfilesFilter = ""
	a.awsProfilesFilterInput.SetText("")
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
// since cloudtui started, or since the overlay was last opened).
func (a *App) populateAWSProfilesTable() {
	profiles, err := a.listAWSProfiles(context.Background())
	if err != nil {
		a.awsProfilesAll = nil
		a.awsProfilesFiltered = nil
		t := a.awsProfilesTable
		for t.GetRowCount() > 1 {
			t.RemoveRow(t.GetRowCount() - 1)
		}
		t.SetCell(1, 0,
			tview.NewTableCell(fmt.Sprintf("Error: %v", err)).
				SetTextColor(tcell.ColorRed).
				SetExpansion(3),
		)
		t.SetTitle(" AWS Profiles ")
		return
	}
	a.awsProfilesAll = profiles
	a.repaintAWSProfiles()
}

// applyAWSProfilesFilter updates the active filter and repaints from the
// already-discovered list, without touching disk again.
func (a *App) applyAWSProfilesFilter(s string) {
	a.awsProfilesFilter = s
	a.repaintAWSProfiles()
}

// repaintAWSProfiles redraws the table from a.awsProfilesAll, applying the
// current filter (case-insensitive substring match on name, matching
// queuesView's filter convention). Marks the active profile with ⭐, same
// convention as the connection manager.
func (a *App) repaintAWSProfiles() {
	filtered := a.awsProfilesAll
	if a.awsProfilesFilter != "" {
		lower := strings.ToLower(a.awsProfilesFilter)
		filtered = make([]awsprofile.Profile, 0, len(a.awsProfilesAll))
		for _, prof := range a.awsProfilesAll {
			if strings.Contains(strings.ToLower(prof.Name), lower) {
				filtered = append(filtered, prof)
			}
		}
	}
	a.awsProfilesFiltered = filtered

	t := a.awsProfilesTable
	for t.GetRowCount() > 1 {
		t.RemoveRow(t.GetRowCount() - 1)
	}

	p := a.cfg.Colors
	nameColor := tcell.GetColor(p.Value)
	textColor := tcell.GetColor(p.Text)
	accentColor := tcell.GetColor(p.Accent)
	for i, prof := range filtered {
		row := i + 1
		name := prof.Name
		nc := nameColor
		if prof.Name == a.cfg.ActiveAWSProfile {
			name = "⭐ " + name
			nc = accentColor
		}
		region := prof.Region
		if region == "" {
			region = "-"
		}
		t.SetCell(row, 0, tview.NewTableCell(name).SetTextColor(nc).SetExpansion(2))
		t.SetCell(row, 1, tview.NewTableCell(region).SetTextColor(textColor).SetExpansion(1))
		t.SetCell(row, 2, tview.NewTableCell(string(prof.AuthType)).SetTextColor(textColor).SetExpansion(1))
	}

	if t.GetRowCount() > 1 {
		t.Select(1, 0)
		// Select alone isn't enough — see queues.go's repaint for why
		// SetOffset is also needed (tview.Table's "track end" auto-scroll
		// latches on during the table's first, still-empty draw).
		t.SetOffset(0, 0)
	}

	// "(text)", not "[text]" — see queues.go's updateTitle for why.
	if a.awsProfilesFilter != "" {
		t.SetTitle(fmt.Sprintf(" AWS Profiles (%s) ", a.awsProfilesFilter))
	} else {
		t.SetTitle(fmt.Sprintf(" AWS Profiles (%d) ", len(a.awsProfilesAll)))
	}
}

// activateAWSProfile records name as the selected AWS profile, persists it,
// updates the info panel, and closes the overlay. This slice of AWS support
// doesn't do anything else with the selection yet (no backend/broker is
// wired to it) — see spec/28-fe-aws-profile-discovery's open question.
func (a *App) activateAWSProfile(name string) {
	a.cfg.ActiveAWSProfile = name
	a.infoPanel.SetText(infoPanelText(a.cfg))
	a.refreshSettingsList()
	a.closeAWSProfiles()
	a.statusBar.SetText(fmt.Sprintf("AWS profile: %s", name))
	if err := config.SaveDefault(a.cfg); err != nil {
		slog.Error("activateAWSProfile: save failed", "error", err)
	}
}
