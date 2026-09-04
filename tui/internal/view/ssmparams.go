package view

import (
	"context"
	"fmt"
	"log/slog"
	"strings"

	"github.com/gdamore/tcell/v2"
	"github.com/rivo/tview"

	"github.com/ePex/cloudtui/tui/internal/awsssm"
	"github.com/ePex/cloudtui/tui/internal/config"
	"github.com/ePex/cloudtui/tui/internal/ui"
)

// SSMParamsView is the SSM Parameters screen: a filterable, read-only
// tview.Table listing AWS Systems Manager Parameter Store entries under
// "/" for the currently active AWS profile (config.Config.ActiveAWSProfile
// — see spec/29-fe-aws-profile-selection). A registered top-level ui.View
// (Home's "AWS" section), not a Settings entry —
// this is a primary browsing feature, not app configuration.
type SSMParamsView struct {
	table       *tview.Table
	filterInput *tview.InputField
	flex        *tview.Flex
	host        ui.SSMParamsHost
	filter      string
	all         []awsssm.Parameter // full unfiltered list from last load
	filtered    []awsssm.Parameter // currently displayed subset, row-indexed
	loadSeq     int                // incremented per load() call; guards against a stale response landing after a newer one
}

var _ ui.View = (*SSMParamsView)(nil)
var _ ui.Shortcuttable = (*SSMParamsView)(nil)
var _ ui.Themeable = (*SSMParamsView)(nil)
var _ ui.ReauthStatusShower = (*SSMParamsView)(nil)

// loadingParametersStatus is load()'s placeholder text — also what
// ShowReauthDone reverts to, so both stay in sync.
const loadingParametersStatus = "Loading parameters…"

// ApplyPalette recolors the SSM parameters view for a live theme switch.
func (pv *SSMParamsView) ApplyPalette(p config.Palette) {
	bg := tcell.GetColor(p.Background)
	pv.table.SetBackgroundColor(bg)
	pv.table.SetBorderColor(tcell.GetColor(p.ViewColor("ssm-parameters")))
	pv.table.SetTitleColor(tcell.GetColor(p.ViewColor("ssm-parameters")))
	pv.filterInput.SetLabelColor(tcell.GetColor(p.Label))
	pv.filterInput.SetFieldBackgroundColor(tcell.GetColor(p.SelectionBg))
	pv.filterInput.SetFieldTextColor(tcell.GetColor(p.SelectionText))
}

func (pv *SSMParamsView) Name() string               { return "ssm-parameters" }
func (pv *SSMParamsView) Title() string              { return "SSM Parameters" }
func (pv *SSMParamsView) Primitive() tview.Primitive { return pv.flex }
func (pv *SSMParamsView) Table() *tview.Table        { return pv.table }
func (pv *SSMParamsView) FilterInputs() []tview.Primitive {
	return []tview.Primitive{pv.filterInput}
}

func (pv *SSMParamsView) Shortcuts() []ui.Shortcut {
	return []ui.Shortcut{
		{Key: "r", Description: "refresh"},
		{Key: "/", Description: "filter"},
		{Key: "f", Description: "favorite"},
	}
}

// NewSSMParamsView constructs the SSM Parameters view.
func NewSSMParamsView(a ui.SSMParamsHost, onSelect func(param awsssm.Parameter)) *SSMParamsView {
	table := tview.NewTable()
	table.SetBorder(true).SetTitle(" SSM Parameters ")
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

	pv := &SSMParamsView{table: table, filterInput: filterInput, flex: flex, host: a}
	pv.setHeader()

	filterInput.SetChangedFunc(func(text string) {
		pv.applyFilter(text)
	})
	filterInput.SetDoneFunc(func(_ tcell.Key) {
		pv.applyFilter(pv.filterInput.GetText())
		pv.host.SetFocus(pv.table)
	})
	filterInput.SetInputCapture(func(event *tcell.EventKey) *tcell.EventKey {
		if event.Key() == tcell.KeyUp || event.Key() == tcell.KeyDown {
			pv.applyFilter(pv.filterInput.GetText())
			pv.host.SetFocus(pv.table)
			pv.table.InputHandler()(event, func(tview.Primitive) {})
			return nil
		}
		return event
	})

	table.SetInputCapture(func(event *tcell.EventKey) *tcell.EventKey {
		switch event.Rune() {
		case 'r':
			pv.load()
			return nil
		case '/':
			ui.SetInputFieldText(pv.filterInput, pv.filter)
			pv.host.SetFocus(pv.filterInput)
			return nil
		case 'f':
			row, _ := pv.table.GetSelection()
			idx := row - 1
			if idx < 0 || idx >= len(pv.filtered) {
				return nil
			}
			pv.host.ToggleFavorite(config.FavoriteSSMParameter, pv.host.Config().ActiveAWSProfile, pv.filtered[idx].Name)
			pv.repaint(pv.all)
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
		if idx < 0 || idx >= len(pv.filtered) {
			return
		}
		onSelect(pv.filtered[idx])
	})

	return pv
}

// Activate reloads the parameter list. Called by SwitchTo each time the
// view becomes active.
func (pv *SSMParamsView) Activate() {
	pv.load()
}

func (pv *SSMParamsView) setHeader() {
	p := pv.host.Config().Colors
	bg := tcell.GetColor(p.Label)
	fg := tcell.GetColor(p.Background)
	pv.table.SetCell(0, 0,
		tview.NewTableCell("").
			SetBackgroundColor(bg).
			SetSelectable(false).
			SetExpansion(0))
	for i, label := range []string{"NAME", "TYPE", "LAST MODIFIED"} {
		pv.table.SetCell(0, i+1,
			tview.NewTableCell(label).
				SetTextColor(fg).
				SetBackgroundColor(bg).
				SetSelectable(false).
				SetExpansion(1).
				SetAlign(tview.AlignCenter))
	}
}

// load fetches parameters from host.ListParameters via the shared
// runAWSLoad helper (internal/view/awsload.go) and repaints via
// QueueUpdateDraw. Requires an active AWS profile; errors clearly rather
// than calling into awsssm with an empty one. Shows a loading placeholder
// immediately, since a real AWS API call has normal network latency — see
// queues.go's Load() for the same reasoning. loadSeq guards against a slow,
// superseded response clobbering a newer one if load() is called again
// before the first call resolves. If the call fails because the profile's
// cached SSO token is missing/expired, runAWSLoad's use of awsauth.Do opens
// the browser to log in and retries once before giving up — see
// spec/36-fe-aws-sso-reauth.
func (pv *SSMParamsView) load() {
	runAWSLoad(pv.host, &pv.loadSeq, pv.showStatus,
		func(err error) {
			slog.Error("ssm parameters: failed to list parameters", "error", err)
			pv.showError(err)
		},
		loadingParametersStatus,
		func(ctx context.Context, profile string) ([]awsssm.Parameter, error) {
			return pv.host.ListParameters(ctx, profile, "/")
		},
		pv.repaint,
	)
}

// ShowReauthWaiting and ShowReauthDone implement ui.ReauthStatusShower for
// structural consistency with QueuesView, though load() itself no longer
// calls them — runAWSLoad calls showStatus directly for the same effect
// (see awsload.go's doc comment for why). Kept for any future external
// caller that reaches this view's status display the way QueuesView's
// re-auth is dispatched via secretbackend/app.go.
func (pv *SSMParamsView) ShowReauthWaiting(msg string) {
	pv.showStatus(msg)
}

func (pv *SSMParamsView) ShowReauthDone() {
	pv.showStatus(loadingParametersStatus)
}

func (pv *SSMParamsView) applyFilter(s string) {
	pv.filter = s
	pv.repaint(pv.all)
}

func (pv *SSMParamsView) repaint(params []awsssm.Parameter) {
	pv.all = params

	filtered := params
	if pv.filter != "" {
		lower := strings.ToLower(pv.filter)
		filtered = make([]awsssm.Parameter, 0, len(params))
		for _, prm := range params {
			if strings.Contains(strings.ToLower(prm.Name), lower) {
				filtered = append(filtered, prm)
			}
		}
	}
	profile := pv.host.Config().ActiveAWSProfile
	favorites := pv.host.Config().AWSFavorites
	isFavorite := func(prm awsssm.Parameter) bool {
		return favorites.IsFavorite(config.FavoriteSSMParameter, profile, prm.Name)
	}
	filtered = sortFavoritesFirst(filtered, isFavorite)
	pv.filtered = filtered

	for pv.table.GetRowCount() > 1 {
		pv.table.RemoveRow(pv.table.GetRowCount() - 1)
	}

	p := pv.host.Config().Colors
	nameColor := tcell.GetColor(p.Value)
	textColor := tcell.GetColor(p.Text)
	for i, prm := range filtered {
		row := i + 1
		pv.table.SetCell(row, 0, favoriteCell(isFavorite(prm), p))
		pv.table.SetCell(row, 1, tview.NewTableCell(prm.Name).SetTextColor(nameColor).SetExpansion(3))
		pv.table.SetCell(row, 2, tview.NewTableCell(string(prm.Type)).SetTextColor(textColor).SetExpansion(1))
		lm := "-"
		if !prm.LastModified.IsZero() {
			lm = prm.LastModified.Local().Format("2006-01-02 15:04:05")
		}
		pv.table.SetCell(row, 3, tview.NewTableCell(lm).SetTextColor(textColor).SetExpansion(2))
	}

	if pv.table.GetRowCount() > 1 {
		pv.table.Select(1, 0)
		// Select alone isn't enough — see queues.go's repaint for why
		// SetOffset is also needed (tview.Table's "track end" auto-scroll
		// latches on during the table's first, still-empty draw).
		pv.table.SetOffset(0, 0)
	}

	// "(text)", not "[text]" — see queues.go's updateTitle for why.
	if pv.filter != "" {
		pv.table.SetTitle(fmt.Sprintf(" SSM Parameters (%s) ", pv.filter))
	} else {
		pv.table.SetTitle(fmt.Sprintf(" SSM Parameters (%d) ", len(params)))
	}
}

func (pv *SSMParamsView) showError(err error) {
	pv.all = nil
	pv.filtered = nil
	for pv.table.GetRowCount() > 1 {
		pv.table.RemoveRow(pv.table.GetRowCount() - 1)
	}
	pv.table.SetCell(1, 1,
		tview.NewTableCell(fmt.Sprintf("Error: %v", err)).
			SetTextColor(tcell.ColorRed).
			SetExpansion(3),
	)
	pv.table.SetTitle(" SSM Parameters ")
}

// showStatus displays an in-progress, non-error message (e.g. while an
// SSO re-auth is running) — same shape as showError but accent-colored
// so it doesn't read as a failure.
func (pv *SSMParamsView) showStatus(msg string) {
	pv.all = nil
	pv.filtered = nil
	for pv.table.GetRowCount() > 1 {
		pv.table.RemoveRow(pv.table.GetRowCount() - 1)
	}
	pv.table.SetCell(1, 1,
		tview.NewTableCell(msg).
			SetTextColor(tcell.GetColor(pv.host.Config().Colors.Accent)).
			SetExpansion(3),
	)
	pv.table.SetTitle(" SSM Parameters ")
}
