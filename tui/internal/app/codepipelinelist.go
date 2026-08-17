package app

import (
	"context"
	"fmt"
	"log/slog"
	"strings"

	"github.com/gdamore/tcell/v2"
	"github.com/rivo/tview"

	"github.com/ePex/cloudtui/tui/internal/awsauth"
	"github.com/ePex/cloudtui/tui/internal/awscodepipeline"
	"github.com/ePex/cloudtui/tui/internal/config"
	"github.com/ePex/cloudtui/tui/internal/ui"
)

// codePipelineListView is the AWS CodePipeline screen: a filterable,
// read-only tview.Table listing pipelines for the currently active AWS
// profile (config.Config.ActiveAWSProfile — see
// spec/29-fe-aws-profile-selection). A registered top-level ui.View
// (Home's "AWS" section). Selecting a pipeline opens the stage-status
// detail view (codePipelineDetailView), not a static detail view — same
// shape as logsView/logSearchView (spec/34-fe-cloudwatch-logs). 'w'
// toggles watching a pipeline directly from this list, without opening
// detail first — see spec/43-fe-codepipeline-monitor.
type codePipelineListView struct {
	table       *tview.Table
	filterInput *tview.InputField
	flex        *tview.Flex
	app         *App
	filter      string
	all         []awscodepipeline.Pipeline
	filtered    []awscodepipeline.Pipeline
}

var _ ui.View = (*codePipelineListView)(nil)
var _ ui.Shortcuttable = (*codePipelineListView)(nil)
var _ ui.Themeable = (*codePipelineListView)(nil)

// ApplyPalette recolors the CodePipeline list view for a live theme switch.
func (lv *codePipelineListView) ApplyPalette(p config.Palette) {
	bg := tcell.GetColor(p.Background)
	lv.table.SetBackgroundColor(bg)
	lv.table.SetBorderColor(tcell.GetColor(p.ViewColor("codepipeline")))
	lv.table.SetTitleColor(tcell.GetColor(p.ViewColor("codepipeline")))
	lv.filterInput.SetLabelColor(tcell.GetColor(p.Label))
	lv.filterInput.SetFieldBackgroundColor(tcell.GetColor(p.SelectionBg))
	lv.filterInput.SetFieldTextColor(tcell.GetColor(p.SelectionText))
}

func (lv *codePipelineListView) Name() string               { return "codepipeline" }
func (lv *codePipelineListView) Title() string              { return "AWS CodePipeline" }
func (lv *codePipelineListView) Primitive() tview.Primitive { return lv.flex }

func (lv *codePipelineListView) Shortcuts() []ui.Shortcut {
	return []ui.Shortcut{
		{Key: "r", Description: "refresh"},
		{Key: "/", Description: "filter"},
		{Key: "w", Description: "watch/unwatch"},
	}
}

// newCodePipelineListView constructs the CodePipeline list view.
func newCodePipelineListView(a *App, onSelect func(pipelineName string)) *codePipelineListView {
	table := tview.NewTable()
	table.SetBorder(true).SetTitle(" AWS CodePipeline ")
	table.SetSelectable(true, false)
	table.SetFixed(1, 0)

	p := a.cfg.Colors
	filterInput := tview.NewInputField()
	filterInput.SetLabel(" / filter: ")
	filterInput.SetLabelColor(tcell.GetColor(p.Label))
	filterInput.SetFieldBackgroundColor(tcell.GetColor(p.SelectionBg))
	filterInput.SetFieldTextColor(tcell.GetColor(p.SelectionText))

	flex := tview.NewFlex().SetDirection(tview.FlexRow).
		AddItem(table, 0, 1, true).
		AddItem(filterInput, 1, 0, false)

	lv := &codePipelineListView{table: table, filterInput: filterInput, flex: flex, app: a}
	lv.setHeader()

	filterInput.SetChangedFunc(func(text string) {
		lv.applyFilter(text)
	})
	filterInput.SetDoneFunc(func(_ tcell.Key) {
		lv.applyFilter(lv.filterInput.GetText())
		lv.app.tv.SetFocus(lv.table)
	})
	filterInput.SetInputCapture(func(event *tcell.EventKey) *tcell.EventKey {
		if event.Key() == tcell.KeyUp || event.Key() == tcell.KeyDown {
			lv.applyFilter(lv.filterInput.GetText())
			lv.app.tv.SetFocus(lv.table)
			lv.table.InputHandler()(event, func(tview.Primitive) {})
			return nil
		}
		return event
	})

	table.SetInputCapture(func(event *tcell.EventKey) *tcell.EventKey {
		switch event.Rune() {
		case 'r':
			lv.load()
			return nil
		case '/':
			lv.filterInput.SetText(lv.filter)
			lv.app.tv.SetFocus(lv.filterInput)
			return nil
		case 'w':
			lv.toggleWatchSelected()
			return nil
		case 'j':
			return tcell.NewEventKey(tcell.KeyDown, 0, tcell.ModNone)
		case 'k':
			return tcell.NewEventKey(tcell.KeyUp, 0, tcell.ModNone)
		}
		return event
	})

	table.SetSelectedFunc(func(row, _ int) {
		idx := row - 1 // row 0 is the header
		if idx < 0 || idx >= len(lv.filtered) {
			return
		}
		onSelect(lv.filtered[idx].Name)
	})

	return lv
}

// Activate reloads the pipeline list. Called by SwitchTo each time the
// view becomes active.
func (lv *codePipelineListView) Activate() {
	lv.load()
}

func (lv *codePipelineListView) setHeader() {
	p := lv.app.cfg.Colors
	bg := tcell.GetColor(p.Label)
	fg := tcell.GetColor(p.Background)
	for i, label := range []string{"NAME", "WATCHING", "UPDATED"} {
		lv.table.SetCell(0, i,
			tview.NewTableCell(label).
				SetTextColor(fg).
				SetBackgroundColor(bg).
				SetSelectable(false).
				SetExpansion(1).
				SetAlign(tview.AlignCenter))
	}
}

// load fetches pipelines from a.listPipelines in a goroutine (a real AWS
// API call) and repaints via QueueUpdateDraw. Requires an active AWS
// profile; errors clearly rather than calling into awscodepipeline with
// an empty one. If the call fails because the profile's cached SSO
// token is missing/expired, awsauth.WithReauth opens the browser to log
// in and retries once before giving up — see spec/36-fe-aws-sso-reauth.
func (lv *codePipelineListView) load() {
	profile := lv.app.cfg.ActiveAWSProfile
	if profile == "" {
		lv.showError(fmt.Errorf("no AWS profile selected — use :ap to select one"))
		return
	}
	go func() {
		ctx := context.Background()
		authType, _ := lv.app.awsAuthTypeFor(ctx, profile)
		pipelines, err := awsauth.WithReauth(ctx, profile, authType, lv.app.awsSSOLogin,
			func() {
				lv.app.tv.QueueUpdateDraw(func() {
					lv.showStatus("AWS SSO session expired — opening browser to log in...")
				})
			},
			func(ctx context.Context) ([]awscodepipeline.Pipeline, error) {
				return lv.app.listPipelines(ctx, profile)
			},
		)
		lv.app.tv.QueueUpdateDraw(func() {
			if err != nil {
				slog.Error("codepipeline: failed to list pipelines", "error", err)
				lv.showError(err)
				return
			}
			lv.repaint(pipelines)
		})
	}()
}

func (lv *codePipelineListView) applyFilter(s string) {
	lv.filter = s
	lv.repaint(lv.all)
}

// toggleWatchSelected starts or stops watching the pipeline under the
// table's current selection, then repaints so the WATCHING column
// reflects the change immediately.
func (lv *codePipelineListView) toggleWatchSelected() {
	row, _ := lv.table.GetSelection()
	idx := row - 1 // row 0 is the header
	if idx < 0 || idx >= len(lv.filtered) {
		return
	}
	name := lv.filtered[idx].Name
	if lv.app.IsWatchingPipeline(name) {
		lv.app.StopWatchingPipeline(name)
	} else {
		lv.app.StartWatchingPipeline(name)
	}
	lv.repaint(lv.all)
}

func (lv *codePipelineListView) repaint(pipelines []awscodepipeline.Pipeline) {
	lv.all = pipelines

	filtered := pipelines
	if lv.filter != "" {
		lower := strings.ToLower(lv.filter)
		filtered = make([]awscodepipeline.Pipeline, 0, len(pipelines))
		for _, p := range pipelines {
			if strings.Contains(strings.ToLower(p.Name), lower) {
				filtered = append(filtered, p)
			}
		}
	}
	lv.filtered = filtered

	for lv.table.GetRowCount() > 1 {
		lv.table.RemoveRow(lv.table.GetRowCount() - 1)
	}

	p := lv.app.cfg.Colors
	nameColor := tcell.GetColor(p.Value)
	textColor := tcell.GetColor(p.Text)
	accentColor := tcell.GetColor(p.Accent)
	for i, pl := range filtered {
		row := i + 1
		lv.table.SetCell(row, 0, tview.NewTableCell(pl.Name).SetTextColor(nameColor).SetExpansion(3))
		watching := ""
		watchColor := textColor
		if lv.app.IsWatchingPipeline(pl.Name) {
			watching = "▶ watching"
			watchColor = accentColor
		}
		lv.table.SetCell(row, 1, tview.NewTableCell(watching).SetTextColor(watchColor).SetExpansion(2))
		updated := "-"
		if !pl.Updated.IsZero() {
			updated = pl.Updated.Local().Format("2006-01-02 15:04:05")
		}
		lv.table.SetCell(row, 2, tview.NewTableCell(updated).SetTextColor(textColor).SetExpansion(2))
	}

	if lv.table.GetRowCount() > 1 {
		lv.table.Select(1, 0)
		// Select alone isn't enough — see queues.go's repaint for why
		// SetOffset is also needed (tview.Table's "track end" auto-scroll
		// latches on during the table's first, still-empty draw).
		lv.table.SetOffset(0, 0)
	}

	// "(text)", not "[text]" — see queues.go's updateTitle for why.
	if lv.filter != "" {
		lv.table.SetTitle(fmt.Sprintf(" AWS CodePipeline (%s) ", lv.filter))
	} else {
		lv.table.SetTitle(fmt.Sprintf(" AWS CodePipeline (%d) ", len(pipelines)))
	}
}

func (lv *codePipelineListView) showError(err error) {
	lv.all = nil
	lv.filtered = nil
	for lv.table.GetRowCount() > 1 {
		lv.table.RemoveRow(lv.table.GetRowCount() - 1)
	}
	lv.table.SetCell(1, 0,
		tview.NewTableCell(fmt.Sprintf("Error: %v", err)).
			SetTextColor(tcell.ColorRed).
			SetExpansion(3),
	)
	lv.table.SetTitle(" AWS CodePipeline ")
}

// showStatus displays an in-progress, non-error message (e.g. while an
// SSO re-auth is running) — same shape as showError but accent-colored
// so it doesn't read as a failure.
func (lv *codePipelineListView) showStatus(msg string) {
	lv.all = nil
	lv.filtered = nil
	for lv.table.GetRowCount() > 1 {
		lv.table.RemoveRow(lv.table.GetRowCount() - 1)
	}
	lv.table.SetCell(1, 0,
		tview.NewTableCell(msg).
			SetTextColor(tcell.GetColor(lv.app.cfg.Colors.Accent)).
			SetExpansion(3),
	)
	lv.table.SetTitle(" AWS CodePipeline ")
}
