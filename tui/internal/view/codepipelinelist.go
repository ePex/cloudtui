package view

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

// CodePipelineListView is the AWS CodePipeline screen: a filterable,
// read-only tview.Table listing pipelines for the currently active AWS
// profile (config.Config.ActiveAWSProfile — see
// spec/29-fe-aws-profile-selection). A registered top-level ui.View
// (Home's "AWS" section). Selecting a pipeline opens the stage-status
// detail view (codePipelineDetailView), not a static detail view — same
// shape as logsView/logSearchView (spec/34-fe-cloudwatch-logs). 'w'
// toggles watching a pipeline directly from this list, without opening
// detail first — see spec/43-fe-codepipeline-monitor.
type CodePipelineListView struct {
	table       *tview.Table
	filterInput *tview.InputField
	flex        *tview.Flex
	host        ui.ViewHost
	filter      string
	all         []awscodepipeline.Pipeline
	filtered    []awscodepipeline.Pipeline
	wrapNav     ui.TableWrap
}

var _ ui.View = (*CodePipelineListView)(nil)
var _ ui.Shortcuttable = (*CodePipelineListView)(nil)
var _ ui.Themeable = (*CodePipelineListView)(nil)

// ApplyPalette recolors the CodePipeline list view for a live theme switch.
func (lv *CodePipelineListView) ApplyPalette(p config.Palette) {
	bg := tcell.GetColor(p.Background)
	lv.table.SetBackgroundColor(bg)
	lv.table.SetBorderColor(tcell.GetColor(p.ViewColor("codepipeline")))
	lv.table.SetTitleColor(tcell.GetColor(p.ViewColor("codepipeline")))
	lv.filterInput.SetLabelColor(tcell.GetColor(p.Label))
	lv.filterInput.SetFieldBackgroundColor(tcell.GetColor(p.SelectionBg))
	lv.filterInput.SetFieldTextColor(tcell.GetColor(p.SelectionText))
}

func (lv *CodePipelineListView) Name() string               { return "codepipeline" }
func (lv *CodePipelineListView) Title() string              { return "AWS CodePipeline" }
func (lv *CodePipelineListView) Primitive() tview.Primitive { return lv.flex }
func (lv *CodePipelineListView) Table() *tview.Table        { return lv.table }
func (lv *CodePipelineListView) FilterInputs() []tview.Primitive {
	return []tview.Primitive{lv.filterInput}
}

// Repaint redraws the table from the already-cached pipeline list — used
// by the background watcher (codepipelinewatch.go) to reflect a live stage
// update without exposing the underlying slice.
func (lv *CodePipelineListView) Repaint() { lv.repaint(lv.all) }

func (lv *CodePipelineListView) Shortcuts() []ui.Shortcut {
	wrap := "off"
	if lv.wrapNav.Enabled() {
		wrap = "on"
	}
	return []ui.Shortcut{
		{Key: "r", Description: "refresh"},
		{Key: "/", Description: "filter"},
		{Key: "w", Description: "watch/unwatch"},
		{Key: "W", Description: "wrap: " + wrap},
	}
}

// NewCodePipelineListView constructs the CodePipeline list view.
func NewCodePipelineListView(a ui.ViewHost, onSelect func(pipelineName string)) *CodePipelineListView {
	table := tview.NewTable()
	table.SetBorder(true).SetTitle(" AWS CodePipeline ")
	table.SetSelectable(true, false)
	table.SetFixed(1, 0)

	p := a.Config().Colors
	filterInput := tview.NewInputField()
	filterInput.SetLabel(" / filter: ")
	filterInput.SetLabelColor(tcell.GetColor(p.Label))
	filterInput.SetFieldBackgroundColor(tcell.GetColor(p.SelectionBg))
	filterInput.SetFieldTextColor(tcell.GetColor(p.SelectionText))

	flex := tview.NewFlex().SetDirection(tview.FlexRow).
		AddItem(table, 0, 1, true).
		AddItem(filterInput, 1, 0, false)

	lv := &CodePipelineListView{table: table, filterInput: filterInput, flex: flex, host: a}
	lv.setHeader()

	filterInput.SetChangedFunc(func(text string) {
		lv.applyFilter(text)
	})
	filterInput.SetDoneFunc(func(_ tcell.Key) {
		lv.applyFilter(lv.filterInput.GetText())
		lv.host.SetFocus(lv.table)
	})
	filterInput.SetInputCapture(func(event *tcell.EventKey) *tcell.EventKey {
		if event.Key() == tcell.KeyUp || event.Key() == tcell.KeyDown {
			lv.applyFilter(lv.filterInput.GetText())
			lv.host.SetFocus(lv.table)
			lv.table.InputHandler()(event, func(tview.Primitive) {})
			return nil
		}
		return event
	})

	table.SetInputCapture(func(event *tcell.EventKey) *tcell.EventKey {
		if event.Rune() == 'j' || event.Rune() == 'k' ||
			event.Key() == tcell.KeyDown || event.Key() == tcell.KeyUp {
			return lv.wrapNav.HandleNav(lv.table, 1, event)
		}
		switch event.Rune() {
		case 'r':
			lv.load()
			return nil
		case '/':
			lv.filterInput.SetText(lv.filter)
			lv.host.SetFocus(lv.filterInput)
			return nil
		case 'w':
			lv.toggleWatchSelected()
			return nil
		case 'W':
			lv.wrapNav.Toggle()
			lines := make([]string, 0, len(lv.Shortcuts()))
			for _, sc := range lv.Shortcuts() {
				lines = append(lines, fmt.Sprintf("[%s]<%s>[-] %s", lv.host.Config().Colors.Accent, sc.Key, sc.Description))
			}
			lv.host.SetContextHint(strings.Join(lines, "\n"))
			return nil
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
func (lv *CodePipelineListView) Activate() {
	lv.load()
}

func (lv *CodePipelineListView) setHeader() {
	p := lv.host.Config().Colors
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

// load fetches pipelines from host.ListPipelines in a goroutine (a real AWS
// API call) and repaints via QueueUpdateDraw. Requires an active AWS
// profile; errors clearly rather than calling into awscodepipeline with
// an empty one. If the call fails because the profile's cached SSO
// token is missing/expired, awsauth.WithReauth opens the browser to log
// in and retries once before giving up — see spec/36-fe-aws-sso-reauth.
func (lv *CodePipelineListView) load() {
	profile := lv.host.Config().ActiveAWSProfile
	if profile == "" {
		lv.showError(fmt.Errorf("no AWS profile selected — use :ap to select one"))
		return
	}
	go func() {
		ctx := context.Background()
		authType, _ := lv.host.AWSAuthTypeFor(ctx, profile)
		pipelines, err := awsauth.WithReauth(ctx, profile, authType, lv.host.AWSSSOLogin,
			func() {
				lv.host.QueueUpdateDraw(func() {
					lv.showStatus("AWS SSO session expired — opening browser to log in...")
				})
			},
			func(ctx context.Context) ([]awscodepipeline.Pipeline, error) {
				return lv.host.ListPipelines(ctx, profile)
			},
		)
		lv.host.QueueUpdateDraw(func() {
			if err != nil {
				slog.Error("codepipeline: failed to list pipelines", "error", err)
				lv.showError(err)
				return
			}
			lv.repaint(pipelines)
		})
	}()
}

func (lv *CodePipelineListView) applyFilter(s string) {
	lv.filter = s
	lv.repaint(lv.all)
}

// toggleWatchSelected starts or stops watching the pipeline under the
// table's current selection, then repaints so the WATCHING column
// reflects the change immediately.
func (lv *CodePipelineListView) toggleWatchSelected() {
	row, _ := lv.table.GetSelection()
	idx := row - 1 // row 0 is the header
	if idx < 0 || idx >= len(lv.filtered) {
		return
	}
	name := lv.filtered[idx].Name
	if lv.host.IsWatchingPipeline(name) {
		lv.host.StopWatchingPipeline(name)
	} else {
		lv.host.StartWatchingPipeline(name)
	}
	lv.repaint(lv.all)
}

func (lv *CodePipelineListView) repaint(pipelines []awscodepipeline.Pipeline) {
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

	p := lv.host.Config().Colors
	nameColor := tcell.GetColor(p.Value)
	textColor := tcell.GetColor(p.Text)
	accentColor := tcell.GetColor(p.Accent)
	for i, pl := range filtered {
		row := i + 1
		lv.table.SetCell(row, 0, tview.NewTableCell(pl.Name).SetTextColor(nameColor).SetExpansion(3))
		watching := ""
		watchColor := textColor
		if lv.host.IsWatchingPipeline(pl.Name) {
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

func (lv *CodePipelineListView) showError(err error) {
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
func (lv *CodePipelineListView) showStatus(msg string) {
	lv.all = nil
	lv.filtered = nil
	for lv.table.GetRowCount() > 1 {
		lv.table.RemoveRow(lv.table.GetRowCount() - 1)
	}
	lv.table.SetCell(1, 0,
		tview.NewTableCell(msg).
			SetTextColor(tcell.GetColor(lv.host.Config().Colors.Accent)).
			SetExpansion(3),
	)
	lv.table.SetTitle(" AWS CodePipeline ")
}
