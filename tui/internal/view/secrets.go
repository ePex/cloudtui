package view

import (
	"context"
	"fmt"
	"log/slog"
	"strings"

	"github.com/gdamore/tcell/v2"
	"github.com/rivo/tview"

	"github.com/ePex/cloudtui/tui/internal/awsauth"
	"github.com/ePex/cloudtui/tui/internal/awssecrets"
	"github.com/ePex/cloudtui/tui/internal/config"
	"github.com/ePex/cloudtui/tui/internal/ui"
)

// SecretsView is the Secrets Manager screen: a filterable, read-only
// tview.Table listing AWS Secrets Manager entries for the currently
// active AWS profile (config.Config.ActiveAWSProfile — see
// spec/29-fe-aws-profile-selection). A registered top-level ui.View
// (Home's "AWS" section, alongside "ssm-parameters").
type SecretsView struct {
	table       *tview.Table
	filterInput *tview.InputField
	flex        *tview.Flex
	host        ui.ViewHost
	filter      string
	all         []awssecrets.Secret // full unfiltered list from last load
	filtered    []awssecrets.Secret // currently displayed subset, row-indexed
}

var _ ui.View = (*SecretsView)(nil)
var _ ui.Shortcuttable = (*SecretsView)(nil)
var _ ui.Themeable = (*SecretsView)(nil)

// ApplyPalette recolors the secrets manager view for a live theme switch.
func (sv *SecretsView) ApplyPalette(p config.Palette) {
	bg := tcell.GetColor(p.Background)
	sv.table.SetBackgroundColor(bg)
	sv.table.SetBorderColor(tcell.GetColor(p.ViewColor("secrets-manager")))
	sv.table.SetTitleColor(tcell.GetColor(p.ViewColor("secrets-manager")))
	sv.filterInput.SetLabelColor(tcell.GetColor(p.Label))
	sv.filterInput.SetFieldBackgroundColor(tcell.GetColor(p.SelectionBg))
	sv.filterInput.SetFieldTextColor(tcell.GetColor(p.SelectionText))
}

func (sv *SecretsView) Name() string               { return "secrets-manager" }
func (sv *SecretsView) Title() string              { return "Secrets Manager" }
func (sv *SecretsView) Primitive() tview.Primitive { return sv.flex }
func (sv *SecretsView) Table() *tview.Table        { return sv.table }
func (sv *SecretsView) FilterInputs() []tview.Primitive {
	return []tview.Primitive{sv.filterInput}
}

func (sv *SecretsView) Shortcuts() []ui.Shortcut {
	return []ui.Shortcut{
		{Key: "r", Description: "refresh"},
		{Key: "/", Description: "filter"},
		{Key: "f", Description: "favorite"},
	}
}

// NewSecretsView constructs the Secrets Manager view.
func NewSecretsView(a ui.ViewHost, onSelect func(secret awssecrets.Secret)) *SecretsView {
	table := tview.NewTable()
	table.SetBorder(true).SetTitle(" Secrets Manager ")
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

	sv := &SecretsView{table: table, filterInput: filterInput, flex: flex, host: a}
	sv.setHeader()

	filterInput.SetChangedFunc(func(text string) {
		sv.applyFilter(text)
	})
	filterInput.SetDoneFunc(func(_ tcell.Key) {
		sv.applyFilter(sv.filterInput.GetText())
		sv.host.SetFocus(sv.table)
	})
	filterInput.SetInputCapture(func(event *tcell.EventKey) *tcell.EventKey {
		if event.Key() == tcell.KeyUp || event.Key() == tcell.KeyDown {
			sv.applyFilter(sv.filterInput.GetText())
			sv.host.SetFocus(sv.table)
			sv.table.InputHandler()(event, func(tview.Primitive) {})
			return nil
		}
		return event
	})

	table.SetInputCapture(func(event *tcell.EventKey) *tcell.EventKey {
		switch event.Rune() {
		case 'r':
			sv.load()
			return nil
		case '/':
			sv.filterInput.SetText(sv.filter)
			sv.host.SetFocus(sv.filterInput)
			return nil
		case 'f':
			row, _ := sv.table.GetSelection()
			idx := row - 1
			if idx < 0 || idx >= len(sv.filtered) {
				return nil
			}
			sv.host.ToggleFavorite(config.FavoriteSecret, sv.host.Config().ActiveAWSProfile, sv.filtered[idx].Name)
			sv.repaint(sv.all)
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
		if idx < 0 || idx >= len(sv.filtered) {
			return
		}
		onSelect(sv.filtered[idx])
	})

	return sv
}

// Activate reloads the secret list. Called by SwitchTo each time the
// view becomes active.
func (sv *SecretsView) Activate() {
	sv.load()
}

func (sv *SecretsView) setHeader() {
	p := sv.host.Config().Colors
	bg := tcell.GetColor(p.Label)
	fg := tcell.GetColor(p.Background)
	sv.table.SetCell(0, 0,
		tview.NewTableCell("").
			SetBackgroundColor(bg).
			SetSelectable(false).
			SetExpansion(0))
	for i, label := range []string{"NAME", "ROTATION", "LAST CHANGED"} {
		sv.table.SetCell(0, i+1,
			tview.NewTableCell(label).
				SetTextColor(fg).
				SetBackgroundColor(bg).
				SetSelectable(false).
				SetExpansion(1).
				SetAlign(tview.AlignCenter))
	}
}

// load fetches secrets from host.ListSecrets in a goroutine (a real AWS API
// call) and repaints via QueueUpdateDraw. Requires an active AWS profile;
// errors clearly rather than calling into awssecrets with an empty one.
// If the call fails because the profile's cached SSO token is
// missing/expired, awsauth.WithReauth opens the browser to log in and
// retries once before giving up — see spec/36-fe-aws-sso-reauth.
func (sv *SecretsView) load() {
	profile := sv.host.Config().ActiveAWSProfile
	if profile == "" {
		sv.showError(fmt.Errorf("no AWS profile selected — use :ap to select one"))
		return
	}
	go func() {
		ctx := context.Background()
		authType, _ := sv.host.AWSAuthTypeFor(ctx, profile)
		secrets, err := awsauth.WithReauth(ctx, profile, authType, sv.host.AWSSSOLogin,
			func() {
				sv.host.QueueUpdateDraw(func() {
					sv.showStatus("AWS SSO session expired — opening browser to log in...")
				})
			},
			nil, // TODO(fe-aws-sso-device-code task 3): show the device code/URL
			func(ctx context.Context) ([]awssecrets.Secret, error) {
				return sv.host.ListSecrets(ctx, profile)
			},
		)
		sv.host.QueueUpdateDraw(func() {
			if err != nil {
				slog.Error("secrets manager: failed to list secrets", "error", err)
				sv.showError(err)
				return
			}
			sv.repaint(secrets)
		})
	}()
}

func (sv *SecretsView) applyFilter(s string) {
	sv.filter = s
	sv.repaint(sv.all)
}

func (sv *SecretsView) repaint(secrets []awssecrets.Secret) {
	sv.all = secrets

	filtered := secrets
	if sv.filter != "" {
		lower := strings.ToLower(sv.filter)
		filtered = make([]awssecrets.Secret, 0, len(secrets))
		for _, s := range secrets {
			if strings.Contains(strings.ToLower(s.Name), lower) {
				filtered = append(filtered, s)
			}
		}
	}
	profile := sv.host.Config().ActiveAWSProfile
	favorites := sv.host.Config().AWSFavorites
	isFavorite := func(s awssecrets.Secret) bool {
		return favorites.IsFavorite(config.FavoriteSecret, profile, s.Name)
	}
	filtered = sortFavoritesFirst(filtered, isFavorite)
	sv.filtered = filtered

	for sv.table.GetRowCount() > 1 {
		sv.table.RemoveRow(sv.table.GetRowCount() - 1)
	}

	p := sv.host.Config().Colors
	nameColor := tcell.GetColor(p.Value)
	textColor := tcell.GetColor(p.Text)
	for i, s := range filtered {
		row := i + 1
		rotation := "no"
		if s.RotationEnabled {
			rotation = "yes"
		}
		sv.table.SetCell(row, 0, favoriteCell(isFavorite(s), p))
		sv.table.SetCell(row, 1, tview.NewTableCell(s.Name).SetTextColor(nameColor).SetExpansion(3))
		sv.table.SetCell(row, 2, tview.NewTableCell(rotation).SetTextColor(textColor).SetExpansion(1))
		lc := "-"
		if !s.LastChanged.IsZero() {
			lc = s.LastChanged.Local().Format("2006-01-02 15:04:05")
		}
		sv.table.SetCell(row, 3, tview.NewTableCell(lc).SetTextColor(textColor).SetExpansion(2))
	}

	if sv.table.GetRowCount() > 1 {
		sv.table.Select(1, 0)
		// Select alone isn't enough — see queues.go's repaint for why
		// SetOffset is also needed (tview.Table's "track end" auto-scroll
		// latches on during the table's first, still-empty draw).
		sv.table.SetOffset(0, 0)
	}

	// "(text)", not "[text]" — see queues.go's updateTitle for why.
	if sv.filter != "" {
		sv.table.SetTitle(fmt.Sprintf(" Secrets Manager (%s) ", sv.filter))
	} else {
		sv.table.SetTitle(fmt.Sprintf(" Secrets Manager (%d) ", len(secrets)))
	}
}

func (sv *SecretsView) showError(err error) {
	sv.all = nil
	sv.filtered = nil
	for sv.table.GetRowCount() > 1 {
		sv.table.RemoveRow(sv.table.GetRowCount() - 1)
	}
	sv.table.SetCell(1, 1,
		tview.NewTableCell(fmt.Sprintf("Error: %v", err)).
			SetTextColor(tcell.ColorRed).
			SetExpansion(3),
	)
	sv.table.SetTitle(" Secrets Manager ")
}

// showStatus displays an in-progress, non-error message (e.g. while an
// SSO re-auth is running) — same shape as showError but accent-colored
// so it doesn't read as a failure.
func (sv *SecretsView) showStatus(msg string) {
	sv.all = nil
	sv.filtered = nil
	for sv.table.GetRowCount() > 1 {
		sv.table.RemoveRow(sv.table.GetRowCount() - 1)
	}
	sv.table.SetCell(1, 1,
		tview.NewTableCell(msg).
			SetTextColor(tcell.GetColor(sv.host.Config().Colors.Accent)).
			SetExpansion(3),
	)
	sv.table.SetTitle(" Secrets Manager ")
}
