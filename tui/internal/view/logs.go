package view

import (
	"context"
	"fmt"
	"log/slog"
	"strings"

	"github.com/gdamore/tcell/v2"
	"github.com/rivo/tview"

	"github.com/ePex/cloudtui/tui/internal/awslogs"
	"github.com/ePex/cloudtui/tui/internal/config"
	"github.com/ePex/cloudtui/tui/internal/ui"
)

// LogsView is the CloudWatch Logs screen: a filterable, read-only
// tview.Table listing log groups for the currently active AWS profile
// (config.Config.ActiveAWSProfile — see spec/29-fe-aws-profile-selection).
// A registered top-level ui.View (Home's "AWS" section, alongside
// "ssm-parameters"/"secrets-manager"). Selecting a log group
// opens the search view (logSearchView), not a static detail view — see
// spec/34-fe-cloudwatch-logs for why this feature's shape differs from
// the others.
type LogsView struct {
	table       *tview.Table
	filterInput *tview.InputField
	flex        *tview.Flex
	host        ui.CloudWatchLogsHost
	filter      string
	all         []awslogs.LogGroup // full unfiltered list from last load
	filtered    []awslogs.LogGroup // currently displayed subset, row-indexed
	loadSeq     int                // incremented per load() call; guards against a stale response landing after a newer one
}

var _ ui.View = (*LogsView)(nil)
var _ ui.Shortcuttable = (*LogsView)(nil)
var _ ui.Themeable = (*LogsView)(nil)
var _ ui.ReauthStatusShower = (*LogsView)(nil)

// loadingLogGroupsStatus is load()'s placeholder text — also what
// ShowReauthDone reverts to, so both stay in sync.
const loadingLogGroupsStatus = "Loading log groups…"

// ApplyPalette recolors the CloudWatch Logs view for a live theme switch.
func (lv *LogsView) ApplyPalette(p config.Palette) {
	bg := tcell.GetColor(p.Background)
	lv.table.SetBackgroundColor(bg)
	lv.table.SetBorderColor(tcell.GetColor(p.ViewColor("cloudwatch-logs")))
	lv.table.SetTitleColor(tcell.GetColor(p.ViewColor("cloudwatch-logs")))
	lv.filterInput.SetLabelColor(tcell.GetColor(p.Label))
	lv.filterInput.SetFieldBackgroundColor(tcell.GetColor(p.SelectionBg))
	lv.filterInput.SetFieldTextColor(tcell.GetColor(p.SelectionText))
}

func (lv *LogsView) Name() string               { return "cloudwatch-logs" }
func (lv *LogsView) Title() string              { return "CloudWatch Logs" }
func (lv *LogsView) Primitive() tview.Primitive { return lv.flex }
func (lv *LogsView) Table() *tview.Table        { return lv.table }
func (lv *LogsView) FilterInputs() []tview.Primitive {
	return []tview.Primitive{lv.filterInput}
}

func (lv *LogsView) Shortcuts() []ui.Shortcut {
	return []ui.Shortcut{
		{Key: "r", Description: "refresh"},
		{Key: "/", Description: "filter"},
		{Key: "f", Description: "favorite"},
	}
}

// NewLogsView constructs the CloudWatch Logs (log group list) view.
func NewLogsView(a ui.CloudWatchLogsHost, onSelect func(logGroupName string)) *LogsView {
	table := tview.NewTable()
	table.SetBorder(true).SetTitle(" CloudWatch Logs ")
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

	lv := &LogsView{table: table, filterInput: filterInput, flex: flex, host: a}
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
		switch event.Rune() {
		case 'r':
			lv.load()
			return nil
		case '/':
			ui.SetInputFieldText(lv.filterInput, lv.filter)
			lv.host.SetFocus(lv.filterInput)
			return nil
		case 'f':
			row, _ := lv.table.GetSelection()
			idx := row - 1
			if idx < 0 || idx >= len(lv.filtered) {
				return nil
			}
			lv.host.ToggleFavorite(config.FavoriteLogGroup, lv.host.Config().ActiveAWSProfile, lv.filtered[idx].Name)
			lv.repaint(lv.all)
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

// Activate reloads the log group list. Called by SwitchTo each time the
// view becomes active.
func (lv *LogsView) Activate() {
	lv.load()
}

func (lv *LogsView) setHeader() {
	p := lv.host.Config().Colors
	bg := tcell.GetColor(p.Label)
	fg := tcell.GetColor(p.Background)
	lv.table.SetCell(0, 0,
		tview.NewTableCell("").
			SetBackgroundColor(bg).
			SetSelectable(false).
			SetExpansion(0))
	for i, label := range []string{"NAME", "RETENTION", "CREATED"} {
		lv.table.SetCell(0, i+1,
			tview.NewTableCell(label).
				SetTextColor(fg).
				SetBackgroundColor(bg).
				SetSelectable(false).
				SetExpansion(1).
				SetAlign(tview.AlignCenter))
	}
}

// load fetches log groups from host.ListLogGroups via the shared
// runAWSLoad helper (internal/view/awsload.go) and repaints via
// QueueUpdateDraw. Requires an active AWS profile; errors clearly rather
// than calling into awslogs with an empty one. Shows a loading placeholder
// immediately, since a real AWS API call has normal network latency — see
// queues.go's Load() for the same reasoning. loadSeq guards against a
// slow, superseded response clobbering a newer one if load() is called
// again before the first call resolves. If the call fails because the
// profile's cached SSO token is missing/expired, runAWSLoad's use of
// awsauth.Do opens the browser to log in and retries once before giving
// up — see spec/36-fe-aws-sso-reauth.
func (lv *LogsView) load() {
	runAWSLoad(lv.host, &lv.loadSeq, lv.showStatus,
		func(err error) {
			slog.Error("cloudwatch logs: failed to list log groups", "error", err)
			lv.showError(err)
		},
		loadingLogGroupsStatus,
		func(ctx context.Context, profile string) ([]awslogs.LogGroup, error) {
			return lv.host.ListLogGroups(ctx, profile)
		},
		lv.repaint,
	)
}

// ShowReauthWaiting and ShowReauthDone implement ui.ReauthStatusShower for
// structural consistency with QueuesView, though load() itself no longer
// calls them — runAWSLoad calls showStatus directly for the same effect
// (see awsload.go's doc comment for why). Kept for any future external
// caller that reaches this view's status display the way QueuesView's
// re-auth is dispatched via secretbackend/app.go.
func (lv *LogsView) ShowReauthWaiting(msg string) {
	lv.showStatus(msg)
}

func (lv *LogsView) ShowReauthDone() {
	lv.showStatus(loadingLogGroupsStatus)
}

func (lv *LogsView) applyFilter(s string) {
	lv.filter = s
	lv.repaint(lv.all)
}

func (lv *LogsView) repaint(groups []awslogs.LogGroup) {
	lv.all = groups

	filtered := groups
	if lv.filter != "" {
		lower := strings.ToLower(lv.filter)
		filtered = make([]awslogs.LogGroup, 0, len(groups))
		for _, g := range groups {
			if strings.Contains(strings.ToLower(g.Name), lower) {
				filtered = append(filtered, g)
			}
		}
	}
	profile := lv.host.Config().ActiveAWSProfile
	favorites := lv.host.Config().AWSFavorites
	isFavorite := func(g awslogs.LogGroup) bool {
		return favorites.IsFavorite(config.FavoriteLogGroup, profile, g.Name)
	}
	filtered = sortFavoritesFirst(filtered, isFavorite)
	lv.filtered = filtered

	for lv.table.GetRowCount() > 1 {
		lv.table.RemoveRow(lv.table.GetRowCount() - 1)
	}

	p := lv.host.Config().Colors
	nameColor := tcell.GetColor(p.Value)
	textColor := tcell.GetColor(p.Text)
	for i, g := range filtered {
		row := i + 1
		retention := "never"
		if g.RetentionDays > 0 {
			retention = fmt.Sprintf("%d days", g.RetentionDays)
		}
		lv.table.SetCell(row, 0, favoriteCell(isFavorite(g), p))
		lv.table.SetCell(row, 1, tview.NewTableCell(g.Name).SetTextColor(nameColor).SetExpansion(3))
		lv.table.SetCell(row, 2, tview.NewTableCell(retention).SetTextColor(textColor).SetExpansion(1))
		created := "-"
		if !g.CreatedAt.IsZero() {
			created = g.CreatedAt.Local().Format("2006-01-02 15:04:05")
		}
		lv.table.SetCell(row, 3, tview.NewTableCell(created).SetTextColor(textColor).SetExpansion(2))
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
		lv.table.SetTitle(fmt.Sprintf(" CloudWatch Logs (%s) ", lv.filter))
	} else {
		lv.table.SetTitle(fmt.Sprintf(" CloudWatch Logs (%d) ", len(groups)))
	}
}

func (lv *LogsView) showError(err error) {
	lv.all = nil
	lv.filtered = nil
	for lv.table.GetRowCount() > 1 {
		lv.table.RemoveRow(lv.table.GetRowCount() - 1)
	}
	lv.table.SetCell(1, 1,
		tview.NewTableCell(fmt.Sprintf("Error: %v", err)).
			SetTextColor(tcell.ColorRed).
			SetExpansion(3),
	)
	lv.table.SetTitle(" CloudWatch Logs ")
}

// showStatus displays an in-progress, non-error message (e.g. while an
// SSO re-auth is running) — same shape as showError but accent-colored
// so it doesn't read as a failure.
func (lv *LogsView) showStatus(msg string) {
	lv.all = nil
	lv.filtered = nil
	for lv.table.GetRowCount() > 1 {
		lv.table.RemoveRow(lv.table.GetRowCount() - 1)
	}
	lv.table.SetCell(1, 1,
		tview.NewTableCell(msg).
			SetTextColor(tcell.GetColor(lv.host.Config().Colors.Accent)).
			SetExpansion(3),
	)
	lv.table.SetTitle(" CloudWatch Logs ")
}
