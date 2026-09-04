package view

import (
	"context"
	"fmt"
	"log/slog"

	"github.com/gdamore/tcell/v2"
	"github.com/rivo/tview"

	"github.com/ePex/cloudtui/tui/internal/awscodepipeline"
	"github.com/ePex/cloudtui/tui/internal/config"
	"github.com/ePex/cloudtui/tui/internal/ui"
)

// CodePipelineDetailView shows a single pipeline's current per-stage
// status: opened via App.OpenCodePipelineDetail, not a registered
// ui.View (same non-registered shape as logSearchView — see
// spec/34-fe-cloudwatch-logs for why). 'w' toggles watching this
// pipeline; while watching, this view live-refreshes whenever a poll
// completes and it's the currently open screen (see
// App.handlePipelinePoll) — see spec/43-fe-codepipeline-monitor.
type CodePipelineDetailView struct {
	table        *tview.Table
	host         ui.CodePipelineHost
	pipelineName string
	stages       []awscodepipeline.StageStatus
	loadSeq      int // incremented per load() call; guards against a stale response landing after a newer one
}

var _ ui.Shortcuttable = (*CodePipelineDetailView)(nil)
var _ ui.Themeable = (*CodePipelineDetailView)(nil)
var _ ui.ReauthStatusShower = (*CodePipelineDetailView)(nil)

// ApplyPalette recolors the CodePipeline detail view for a live theme switch.
func (dv *CodePipelineDetailView) ApplyPalette(p config.Palette) {
	dv.table.SetBackgroundColor(tcell.GetColor(p.Background))
	dv.table.SetBorderColor(tcell.GetColor(p.ViewColor("codepipeline")))
	dv.table.SetTitleColor(tcell.GetColor(p.ViewColor("codepipeline")))
}

func (dv *CodePipelineDetailView) Primitive() tview.Primitive { return dv.table }
func (dv *CodePipelineDetailView) PipelineName() string       { return dv.pipelineName }

func (dv *CodePipelineDetailView) Shortcuts() []ui.Shortcut {
	return []ui.Shortcut{
		{Key: "r", Description: "refresh"},
		{Key: "w", Description: "watch/unwatch"},
		{Key: "Esc", Description: "back"},
	}
}

// NewCodePipelineDetailView constructs the CodePipeline stage-status
// detail view.
func NewCodePipelineDetailView(a ui.CodePipelineHost, onBack func()) *CodePipelineDetailView {
	table := tview.NewTable()
	table.SetBorder(true)
	table.SetSelectable(true, false)
	table.SetFixed(1, 0)

	dv := &CodePipelineDetailView{table: table, host: a}
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
			onBack()
			return nil
		}
		return event
	})

	return dv
}

func (dv *CodePipelineDetailView) setHeader() {
	p := dv.host.Config().Colors
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

// Open resets the view for a freshly-selected pipeline and runs the
// first load.
func (dv *CodePipelineDetailView) Open(pipelineName string) {
	dv.pipelineName = pipelineName
	dv.stages = nil
	clearTableBody(dv.table)
	dv.setHeader()
	dv.table.SetTitle(fmt.Sprintf(" %s ", pipelineName))
	dv.load()
}

func (dv *CodePipelineDetailView) toggleWatch() {
	if dv.host.IsWatchingPipeline(dv.pipelineName) {
		dv.host.StopWatchingPipeline(dv.pipelineName)
	} else {
		dv.host.StartWatchingPipeline(dv.pipelineName)
	}
	dv.updateTitle()
}

// load fetches the pipeline's current stage status via the shared
// runAWSLoad helper (internal/view/awsload.go) and renders via
// QueueUpdateDraw. pipelineName is captured once here, on the caller's
// goroutine, and closed over rather than re-read from dv.pipelineName
// inside the fetch/error closures below — those run after Open() may
// already have switched dv.pipelineName to a different pipeline, so
// re-reading it there would report the wrong pipeline's error/fetch
// against a possibly-different one. If the call fails because the
// profile's cached SSO token is missing/expired, runAWSLoad's use of
// awsauth.Do opens the browser to log in and retries once before giving
// up — see spec/36-fe-aws-sso-reauth.
func (dv *CodePipelineDetailView) load() {
	pipelineName := dv.pipelineName
	runAWSLoad(dv.host, &dv.loadSeq, dv.showStatus,
		func(err error) {
			slog.Error("codepipeline: failed to get pipeline state", "pipeline", pipelineName, "error", err)
			dv.showError(err)
		},
		fmt.Sprintf("Loading %s…", pipelineName),
		func(ctx context.Context, profile string) ([]awscodepipeline.StageStatus, error) {
			return dv.host.GetPipelineState(ctx, profile, pipelineName)
		},
		dv.Render,
	)
}

// ShowReauthWaiting implements ui.ReauthStatusShower for structural
// consistency with QueuesView, though load() itself no longer calls it —
// runAWSLoad calls showStatus directly for the same effect (see
// awsload.go's doc comment for why).
func (dv *CodePipelineDetailView) ShowReauthWaiting(msg string) {
	dv.showStatus(msg)
}

// ShowReauthDone implements ui.ReauthStatusShower.
func (dv *CodePipelineDetailView) ShowReauthDone() {
	dv.showStatus(fmt.Sprintf("Loading %s…", dv.pipelineName))
}

// Render displays stages' current status. Also called by
// App.handlePipelinePoll when a background watch's poll completes and
// this view is the one currently open for that pipeline — see
// spec/43-fe-codepipeline-monitor decision 10.
func (dv *CodePipelineDetailView) Render(stages []awscodepipeline.StageStatus) {
	dv.stages = stages
	clearTableBody(dv.table)

	p := dv.host.Config().Colors
	nameColor := tcell.GetColor(p.Value)
	for i, s := range stages {
		row := i + 1
		dv.table.SetCell(row, 0, tview.NewTableCell(s.Name).SetTextColor(nameColor).SetExpansion(1))
		dv.table.SetCell(row, 1, tview.NewTableCell(StatusLabel(s.Status)).SetTextColor(statusColor(s.Status)).SetExpansion(1))
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
func (dv *CodePipelineDetailView) updateTitle() {
	title := " " + dv.pipelineName
	if dv.host.IsWatchingPipeline(dv.pipelineName) {
		title += " — ▶ watching"
	}
	dv.table.SetTitle(title + " ")
}

func (dv *CodePipelineDetailView) showError(err error) {
	showStatusCell(dv.table, 0, fmt.Sprintf("Error: %v", err), tcell.ColorRed, fmt.Sprintf(" %s ", dv.pipelineName), func() {
		dv.stages = nil
	})
}

// showStatus displays an in-progress, non-error message (e.g. while an
// SSO re-auth is running) — same shape as showError but accent-colored
// so it doesn't read as a failure.
func (dv *CodePipelineDetailView) showStatus(msg string) {
	showStatusCell(dv.table, 0, msg, tcell.GetColor(dv.host.Config().Colors.Accent), fmt.Sprintf(" %s ", dv.pipelineName), func() {
		dv.stages = nil
	})
}

// StatusLabel shows "(never run)" for a stage with no execution yet,
// rather than a blank cell that could be misread as a loading glitch.
func StatusLabel(status string) string {
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
