package app

import (
	"context"
	"fmt"
	"log/slog"

	"github.com/gdamore/tcell/v2"
	"github.com/rivo/tview"

	"github.com/ePex/cloudtui/tui/internal/awsauth"
	"github.com/ePex/cloudtui/tui/internal/awscodepipeline"
	"github.com/ePex/cloudtui/tui/internal/ui"
)

// codePipelineDetailView shows a single pipeline's current per-stage
// status: opened via App.openCodePipelineDetail, not a registered
// ui.View (same non-registered shape as logSearchView — see
// spec/34-fe-cloudwatch-logs for why). 'w' toggles watching this
// pipeline; while watching, this view live-refreshes whenever a poll
// completes and it's the currently open screen (see
// App.handlePipelinePoll) — see spec/43-fe-codepipeline-monitor.
type codePipelineDetailView struct {
	table        *tview.Table
	app          *App
	pipelineName string
	stages       []awscodepipeline.StageStatus
}

var _ ui.Shortcuttable = (*codePipelineDetailView)(nil)

func (dv *codePipelineDetailView) Shortcuts() []ui.Shortcut {
	return []ui.Shortcut{
		{Key: "r", Description: "refresh"},
		{Key: "w", Description: "watch/unwatch"},
		{Key: "Esc", Description: "back"},
	}
}

// newCodePipelineDetailView constructs the CodePipeline stage-status
// detail view.
func newCodePipelineDetailView(a *App) *codePipelineDetailView {
	table := tview.NewTable()
	table.SetBorder(true)
	table.SetSelectable(true, false)
	table.SetFixed(1, 0)

	dv := &codePipelineDetailView{table: table, app: a}
	dv.setHeader()

	table.SetInputCapture(func(event *tcell.EventKey) *tcell.EventKey {
		switch event.Rune() {
		case 'r':
			dv.load()
			return nil
		case 'w':
			dv.toggleWatch()
			return nil
		case 'j':
			return tcell.NewEventKey(tcell.KeyDown, 0, tcell.ModNone)
		case 'k':
			return tcell.NewEventKey(tcell.KeyUp, 0, tcell.ModNone)
		}
		switch event.Key() {
		case tcell.KeyEscape, tcell.KeyBackspace, tcell.KeyBackspace2:
			a.pages.SwitchToPage("codepipeline")
			a.tv.SetFocus(a.codePipelineListV.table)
			a.updateContextPanel(a.codePipelineListV)
			return nil
		}
		return event
	})

	return dv
}

func (dv *codePipelineDetailView) setHeader() {
	p := dv.app.cfg.Colors
	bg := tcell.GetColor(p.Label)
	fg := tcell.GetColor(p.Background)
	for i, label := range []string{"STAGE", "STATUS"} {
		dv.table.SetCell(0, i,
			tview.NewTableCell(label).
				SetTextColor(fg).
				SetBackgroundColor(bg).
				SetSelectable(false).
				SetExpansion(1).
				SetAlign(tview.AlignCenter))
	}
}

// open resets the view for a freshly-selected pipeline and runs the
// first load.
func (dv *codePipelineDetailView) open(pipelineName string) {
	dv.pipelineName = pipelineName
	dv.stages = nil
	for dv.table.GetRowCount() > 1 {
		dv.table.RemoveRow(dv.table.GetRowCount() - 1)
	}
	dv.setHeader()
	dv.table.SetTitle(fmt.Sprintf(" %s ", pipelineName))
	dv.load()
}

func (dv *codePipelineDetailView) toggleWatch() {
	if dv.app.isWatchingPipeline(dv.pipelineName) {
		dv.app.stopWatchingPipeline(dv.pipelineName)
	} else {
		dv.app.startWatchingPipeline(dv.pipelineName)
	}
	dv.updateTitle()
}

// load fetches the pipeline's current stage status in a goroutine (a
// real AWS API call) and renders via QueueUpdateDraw. If the call fails
// because the profile's cached SSO token is missing/expired,
// awsauth.WithReauth opens the browser to log in and retries once
// before giving up — see spec/36-fe-aws-sso-reauth.
func (dv *codePipelineDetailView) load() {
	profile := dv.app.cfg.ActiveAWSProfile
	if profile == "" {
		dv.showError(fmt.Errorf("no AWS profile selected — use :ap to select one"))
		return
	}
	pipelineName := dv.pipelineName
	go func() {
		ctx := context.Background()
		authType, _ := dv.app.awsAuthTypeFor(ctx, profile)
		stages, err := awsauth.WithReauth(ctx, profile, authType, dv.app.awsSSOLogin,
			func() {
				dv.app.tv.QueueUpdateDraw(func() {
					dv.showStatus("AWS SSO session expired — opening browser to log in...")
				})
			},
			func(ctx context.Context) ([]awscodepipeline.StageStatus, error) {
				return dv.app.getPipelineState(ctx, profile, pipelineName)
			},
		)
		dv.app.tv.QueueUpdateDraw(func() {
			if err != nil {
				slog.Error("codepipeline: failed to get pipeline state", "pipeline", pipelineName, "error", err)
				dv.showError(err)
				return
			}
			dv.render(stages)
		})
	}()
}

// render displays stages' current status. Also called by
// App.handlePipelinePoll when a background watch's poll completes and
// this view is the one currently open for that pipeline — see
// spec/43-fe-codepipeline-monitor decision 10.
func (dv *codePipelineDetailView) render(stages []awscodepipeline.StageStatus) {
	dv.stages = stages
	for dv.table.GetRowCount() > 1 {
		dv.table.RemoveRow(dv.table.GetRowCount() - 1)
	}

	p := dv.app.cfg.Colors
	nameColor := tcell.GetColor(p.Value)
	for i, s := range stages {
		row := i + 1
		dv.table.SetCell(row, 0, tview.NewTableCell(s.Name).SetTextColor(nameColor).SetExpansion(1))
		dv.table.SetCell(row, 1, tview.NewTableCell(statusLabel(s.Status)).SetTextColor(statusColor(s.Status)).SetExpansion(1))
	}

	if dv.table.GetRowCount() > 1 {
		dv.table.Select(1, 0)
		dv.table.SetOffset(0, 0)
	}

	dv.updateTitle()
}

// updateTitle never uses "[text]" — see queues.go's updateTitle for why
// (tview.Box titles run through the same tag-parsing Print() that Table
// cells do, silently swallowing square brackets).
func (dv *codePipelineDetailView) updateTitle() {
	title := " " + dv.pipelineName
	if dv.app.isWatchingPipeline(dv.pipelineName) {
		title += " — ▶ watching"
	}
	dv.table.SetTitle(title + " ")
}

func (dv *codePipelineDetailView) showError(err error) {
	dv.stages = nil
	for dv.table.GetRowCount() > 1 {
		dv.table.RemoveRow(dv.table.GetRowCount() - 1)
	}
	dv.table.SetCell(1, 0,
		tview.NewTableCell(fmt.Sprintf("Error: %v", err)).
			SetTextColor(tcell.ColorRed).
			SetExpansion(3),
	)
	dv.table.SetTitle(fmt.Sprintf(" %s ", dv.pipelineName))
}

// showStatus displays an in-progress, non-error message (e.g. while an
// SSO re-auth is running) — same shape as showError but accent-colored
// so it doesn't read as a failure.
func (dv *codePipelineDetailView) showStatus(msg string) {
	dv.stages = nil
	for dv.table.GetRowCount() > 1 {
		dv.table.RemoveRow(dv.table.GetRowCount() - 1)
	}
	dv.table.SetCell(1, 0,
		tview.NewTableCell(msg).
			SetTextColor(tcell.GetColor(dv.app.cfg.Colors.Accent)).
			SetExpansion(3),
	)
	dv.table.SetTitle(fmt.Sprintf(" %s ", dv.pipelineName))
}

// statusLabel shows "(never run)" for a stage with no execution yet,
// rather than a blank cell that could be misread as a loading glitch.
func statusLabel(status string) string {
	if status == "" {
		return "(never run)"
	}
	return status
}

// statusColor is a fixed, theme-independent semantic color per stage
// status — same precedent as showError's hardcoded tcell.ColorRed
// elsewhere in this app, deliberately not theme-driven since red/yellow/
// green need to mean the same thing regardless of the active palette.
func statusColor(status string) tcell.Color {
	switch status {
	case "Succeeded":
		return tcell.ColorGreen
	case "Failed", "Stopped":
		return tcell.ColorRed
	case "InProgress", "Stopping":
		return tcell.ColorYellow
	default: // Cancelled, Skipped, "" (never run)
		return tcell.ColorGray
	}
}
