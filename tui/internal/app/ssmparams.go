package app

import (
	"context"
	"fmt"
	"log/slog"
	"strings"

	"github.com/gdamore/tcell/v2"
	"github.com/rivo/tview"

	"github.com/ePex/cloudtui/tui/internal/awsauth"
	"github.com/ePex/cloudtui/tui/internal/awsssm"
	"github.com/ePex/cloudtui/tui/internal/ui"
)

// ssmParamsView is the SSM Parameters screen: a filterable, read-only
// tview.Table listing AWS Systems Manager Parameter Store entries under
// "/" for the currently active AWS profile (config.Config.ActiveAWSProfile
// — see spec/29-fe-aws-profile-selection). A registered top-level ui.View
// (Home's "AWS" section), not a Settings entry —
// this is a primary browsing feature, not app configuration.
type ssmParamsView struct {
	table       *tview.Table
	filterInput *tview.InputField
	flex        *tview.Flex
	app         *App
	filter      string
	all         []awsssm.Parameter // full unfiltered list from last load
	filtered    []awsssm.Parameter // currently displayed subset, row-indexed
}

var _ ui.View = (*ssmParamsView)(nil)
var _ ui.Shortcuttable = (*ssmParamsView)(nil)

func (pv *ssmParamsView) Name() string               { return "ssm-parameters" }
func (pv *ssmParamsView) Title() string              { return "SSM Parameters" }
func (pv *ssmParamsView) Primitive() tview.Primitive { return pv.flex }

func (pv *ssmParamsView) Shortcuts() []ui.Shortcut {
	return []ui.Shortcut{
		{Key: "r", Description: "refresh"},
		{Key: "/", Description: "filter"},
	}
}

// newSSMParamsView constructs the SSM Parameters view.
func newSSMParamsView(a *App) *ssmParamsView {
	table := tview.NewTable()
	table.SetBorder(true).SetTitle(" SSM Parameters ")
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

	pv := &ssmParamsView{table: table, filterInput: filterInput, flex: flex, app: a}
	pv.setHeader()

	filterInput.SetChangedFunc(func(text string) {
		pv.applyFilter(text)
	})
	filterInput.SetDoneFunc(func(_ tcell.Key) {
		pv.applyFilter(pv.filterInput.GetText())
		pv.app.tv.SetFocus(pv.table)
	})
	filterInput.SetInputCapture(func(event *tcell.EventKey) *tcell.EventKey {
		if event.Key() == tcell.KeyUp || event.Key() == tcell.KeyDown {
			pv.applyFilter(pv.filterInput.GetText())
			pv.app.tv.SetFocus(pv.table)
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
			pv.filterInput.SetText(pv.filter)
			pv.app.tv.SetFocus(pv.filterInput)
			return nil
		case 'j':
			return tcell.NewEventKey(tcell.KeyDown, 0, tcell.ModNone)
		case 'k':
			return tcell.NewEventKey(tcell.KeyUp, 0, tcell.ModNone)
		}
		return event
	})

	return pv
}

// Activate reloads the parameter list. Called by switchTo each time the
// view becomes active.
func (pv *ssmParamsView) Activate() {
	pv.load()
}

func (pv *ssmParamsView) setHeader() {
	p := pv.app.cfg.Colors
	bg := tcell.GetColor(p.Label)
	fg := tcell.GetColor(p.Background)
	for i, label := range []string{"NAME", "TYPE", "LAST MODIFIED"} {
		pv.table.SetCell(0, i,
			tview.NewTableCell(label).
				SetTextColor(fg).
				SetBackgroundColor(bg).
				SetSelectable(false).
				SetExpansion(1).
				SetAlign(tview.AlignCenter))
	}
}

// load fetches parameters from a.listParameters in a goroutine (a real AWS
// API call, unlike awsprofile's local file read) and repaints via
// QueueUpdateDraw. Requires an active AWS profile; errors clearly rather
// than calling into awsssm with an empty one. If the call fails because
// the profile's cached SSO token is missing/expired, awsauth.WithReauth
// opens the browser to log in and retries once before giving up — see
// spec/36-fe-aws-sso-reauth.
func (pv *ssmParamsView) load() {
	profile := pv.app.cfg.ActiveAWSProfile
	if profile == "" {
		pv.showError(fmt.Errorf("no AWS profile selected — use :ap to select one"))
		return
	}
	go func() {
		ctx := context.Background()
		authType, _ := pv.app.awsAuthTypeFor(ctx, profile)
		params, err := awsauth.WithReauth(ctx, profile, authType, pv.app.awsSSOLogin,
			func() {
				pv.app.tv.QueueUpdateDraw(func() {
					pv.showStatus("AWS SSO session expired — opening browser to log in...")
				})
			},
			func(ctx context.Context) ([]awsssm.Parameter, error) {
				return pv.app.listParameters(ctx, profile, "/")
			},
		)
		pv.app.tv.QueueUpdateDraw(func() {
			if err != nil {
				slog.Error("ssm parameters: failed to list parameters", "error", err)
				pv.showError(err)
				return
			}
			pv.repaint(params)
		})
	}()
}

func (pv *ssmParamsView) applyFilter(s string) {
	pv.filter = s
	pv.repaint(pv.all)
}

func (pv *ssmParamsView) repaint(params []awsssm.Parameter) {
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
	pv.filtered = filtered

	for pv.table.GetRowCount() > 1 {
		pv.table.RemoveRow(pv.table.GetRowCount() - 1)
	}

	p := pv.app.cfg.Colors
	nameColor := tcell.GetColor(p.Value)
	textColor := tcell.GetColor(p.Text)
	for i, prm := range filtered {
		row := i + 1
		pv.table.SetCell(row, 0, tview.NewTableCell(prm.Name).SetTextColor(nameColor).SetExpansion(3))
		pv.table.SetCell(row, 1, tview.NewTableCell(string(prm.Type)).SetTextColor(textColor).SetExpansion(1))
		lm := "-"
		if !prm.LastModified.IsZero() {
			lm = prm.LastModified.Local().Format("2006-01-02 15:04:05")
		}
		pv.table.SetCell(row, 2, tview.NewTableCell(lm).SetTextColor(textColor).SetExpansion(2))
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

func (pv *ssmParamsView) showError(err error) {
	pv.all = nil
	pv.filtered = nil
	for pv.table.GetRowCount() > 1 {
		pv.table.RemoveRow(pv.table.GetRowCount() - 1)
	}
	pv.table.SetCell(1, 0,
		tview.NewTableCell(fmt.Sprintf("Error: %v", err)).
			SetTextColor(tcell.ColorRed).
			SetExpansion(3),
	)
	pv.table.SetTitle(" SSM Parameters ")
}

// showStatus displays an in-progress, non-error message (e.g. while an
// SSO re-auth is running) — same shape as showError but accent-colored
// so it doesn't read as a failure.
func (pv *ssmParamsView) showStatus(msg string) {
	pv.all = nil
	pv.filtered = nil
	for pv.table.GetRowCount() > 1 {
		pv.table.RemoveRow(pv.table.GetRowCount() - 1)
	}
	pv.table.SetCell(1, 0,
		tview.NewTableCell(msg).
			SetTextColor(tcell.GetColor(pv.app.cfg.Colors.Accent)).
			SetExpansion(3),
	)
	pv.table.SetTitle(" SSM Parameters ")
}
