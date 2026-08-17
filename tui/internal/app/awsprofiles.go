package app

import (
	"context"
	"fmt"
	"strings"

	"github.com/gdamore/tcell/v2"
	"github.com/rivo/tview"

	"github.com/ePex/cloudtui/tui/internal/awsprofile"
	"github.com/ePex/cloudtui/tui/internal/config"
	"github.com/ePex/cloudtui/tui/internal/ui"
)

// awsProfilesPicker is the AWS Profiles overlay: a filterable, read-only
// list of every discoverable AWS CLI profile, with Enter activating the
// selected one. Named awsProfilesPicker, not awsProfiles — too easy to
// confuse with the imported awsprofile package (singular) at a glance.
type awsProfilesPicker struct {
	host        ui.Host
	flex        *tview.Flex
	table       *tview.Table
	filterInput *tview.InputField
	hints       *tview.TextView
	visible     bool
	filter      string
	all         []awsprofile.Profile
	filtered    []awsprofile.Profile
}

// newAWSProfilesPicker builds the AWS Profiles overlay's widgets.
func newAWSProfilesPicker(host ui.Host) *awsProfilesPicker {
	ap := &awsProfilesPicker{host: host}
	colors := host.Config().Colors
	ap.table = tview.NewTable()
	ap.table.SetBorder(true).SetTitle(" AWS Profiles ")
	ap.table.SetSelectable(true, false)
	ap.table.SetFixed(1, 0)
	ap.filterInput = tview.NewInputField()
	ap.filterInput.SetLabel(" / filter: ")
	ap.filterInput.SetLabelColor(tcell.GetColor(colors.Label))
	ap.filterInput.SetFieldBackgroundColor(tcell.GetColor(colors.SelectionBg))
	ap.filterInput.SetFieldTextColor(tcell.GetColor(colors.SelectionText))
	ap.hints = tview.NewTextView().SetDynamicColors(true)
	ac := colors.Accent
	ap.hints.SetText(fmt.Sprintf("[%s]<Enter>[-] activate  [%s]<r>[-] refresh  [%s]</>[-] filter  [%s]<Esc>[-] close",
		ac, ac, ac, ac))
	ap.flex = tview.NewFlex().SetDirection(tview.FlexRow).
		AddItem(ap.table, 0, 1, true).
		AddItem(ap.filterInput, 1, 0, false).
		AddItem(ap.hints, 1, 0, false)
	ap.setHeader()

	ap.table.SetSelectedFunc(func(row, _ int) {
		idx := row - 1
		if idx < 0 || idx >= len(ap.filtered) {
			return
		}
		ap.activate(ap.filtered[idx].Name)
	})
	ap.table.SetInputCapture(func(event *tcell.EventKey) *tcell.EventKey {
		switch {
		case event.Key() == tcell.KeyEscape:
			ap.close()
			return nil
		case event.Rune() == 'r':
			ap.populate()
			return nil
		case event.Rune() == '/':
			ap.filterInput.SetText(ap.filter)
			host.SetFocus(ap.filterInput)
			return nil
		case event.Rune() == 'j':
			return tcell.NewEventKey(tcell.KeyDown, 0, tcell.ModNone)
		case event.Rune() == 'k':
			return tcell.NewEventKey(tcell.KeyUp, 0, tcell.ModNone)
		}
		return event
	})
	ap.filterInput.SetChangedFunc(func(text string) {
		ap.applyFilter(text)
	})
	ap.filterInput.SetDoneFunc(func(_ tcell.Key) {
		ap.applyFilter(ap.filterInput.GetText())
		host.SetFocus(ap.table)
	})
	ap.filterInput.SetInputCapture(func(event *tcell.EventKey) *tcell.EventKey {
		if event.Key() == tcell.KeyUp || event.Key() == tcell.KeyDown {
			ap.applyFilter(ap.filterInput.GetText())
			host.SetFocus(ap.table)
			ap.table.InputHandler()(event, func(tview.Primitive) {})
			return nil
		}
		return event
	})
	return ap
}

// setHeader draws the overlay's column header row.
func (ap *awsProfilesPicker) setHeader() {
	p := ap.host.Config().Colors
	bg := tcell.GetColor(p.Label)
	fg := tcell.GetColor(p.Background)
	for i, label := range []string{"NAME", "REGION", "AUTH"} {
		ap.table.SetCell(0, i,
			tview.NewTableCell(label).
				SetTextColor(fg).
				SetBackgroundColor(bg).
				SetSelectable(false).
				SetExpansion(1).
				SetAlign(tview.AlignCenter))
	}
}

// show opens the AWS Profiles overlay, resetting any filter left over
// from a previous visit. Discovery is local file I/O (~/.aws/config,
// ~/.aws/credentials), fast enough to run synchronously — unlike the
// broker-touching views, which use a goroutine + QueueUpdateDraw
// specifically because they're network calls.
func (ap *awsProfilesPicker) show() {
	ap.filter = ""
	ap.filterInput.SetText("")
	ap.populate()
	ap.host.ShowPage("aws-profiles")
	ap.host.SetFocus(ap.table)
	ap.visible = true
}

// close hides the AWS Profiles overlay and restores focus.
func (ap *awsProfilesPicker) close() {
	ap.host.HidePage("aws-profiles")
	ap.visible = false
	ap.host.FocusMain()
}

// ApplyPalette recolors the AWS Profiles overlay for a live theme switch.
func (ap *awsProfilesPicker) ApplyPalette(p config.Palette) {
	bg := tcell.GetColor(p.Background)
	ap.flex.SetBackgroundColor(bg)
	ap.flex.SetBorderColor(tcell.GetColor(p.Border))
	ap.flex.SetTitleColor(tcell.GetColor(p.Border))
	ap.table.SetBackgroundColor(bg)
	ap.table.SetBorderColor(tcell.GetColor(p.Border))
	ap.table.SetTitleColor(tcell.GetColor(p.Border))
	ap.hints.SetBackgroundColor(bg)
	ap.hints.SetTextColor(tcell.GetColor(p.Text))
}

var _ ui.Themeable = (*awsProfilesPicker)(nil)

// populate re-runs discovery via app.listAWSProfiles and redraws the
// table. Called on open and on 'r' (the files may have changed since
// cloudtui started, or since the overlay was last opened).
func (ap *awsProfilesPicker) populate() {
	profiles, err := ap.host.ListAWSProfiles(context.Background())
	if err != nil {
		ap.all = nil
		ap.filtered = nil
		t := ap.table
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
	ap.all = profiles
	ap.repaint()
}

// applyFilter updates the active filter and repaints from the
// already-discovered list, without touching disk again.
func (ap *awsProfilesPicker) applyFilter(s string) {
	ap.filter = s
	ap.repaint()
}

// repaint redraws the table from ap.all, applying the current filter
// (case-insensitive substring match on name, matching queuesView's
// filter convention). Marks the active profile with ⭐, same convention
// as the connection manager.
func (ap *awsProfilesPicker) repaint() {
	filtered := ap.all
	if ap.filter != "" {
		lower := strings.ToLower(ap.filter)
		filtered = make([]awsprofile.Profile, 0, len(ap.all))
		for _, prof := range ap.all {
			if strings.Contains(strings.ToLower(prof.Name), lower) {
				filtered = append(filtered, prof)
			}
		}
	}
	ap.filtered = filtered

	t := ap.table
	for t.GetRowCount() > 1 {
		t.RemoveRow(t.GetRowCount() - 1)
	}

	cfg := ap.host.Config()
	p := cfg.Colors
	nameColor := tcell.GetColor(p.Value)
	textColor := tcell.GetColor(p.Text)
	accentColor := tcell.GetColor(p.Accent)
	for i, prof := range filtered {
		row := i + 1
		name := prof.Name
		nc := nameColor
		if prof.Name == cfg.ActiveAWSProfile {
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
	if ap.filter != "" {
		t.SetTitle(fmt.Sprintf(" AWS Profiles (%s) ", ap.filter))
	} else {
		t.SetTitle(fmt.Sprintf(" AWS Profiles (%d) ", len(ap.all)))
	}
}

// activate records name as the selected AWS profile, persists it,
// updates the info panel, and closes the overlay. This slice of AWS
// support doesn't do anything else with the selection yet (no backend/
// broker is wired to it) — see spec/28-fe-aws-profile-discovery's open
// question.
func (ap *awsProfilesPicker) activate(name string) {
	ap.host.SetActiveAWSProfile(name)
	ap.close()
	ap.host.SetStatus(fmt.Sprintf("AWS profile: %s", name))
}
