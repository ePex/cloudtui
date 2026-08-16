package app

import (
	"log/slog"
	"strings"

	"github.com/gdamore/tcell/v2"
	"github.com/rivo/tview"

	"github.com/ePex/cloudtui/tui/internal/config"
	"github.com/ePex/cloudtui/tui/internal/ui"
)

// datadogEditor is the Datadog settings overlay (spec/39-fe-datadog-logs)
// — same shape as the connection editor, just two fields: Site (plain,
// not a secret) and Access Token (a Personal Access Token, masked — see
// config.DatadogConfig's doc comment for why this isn't the classic API
// Key + Application Key pair).
type datadogEditor struct {
	app     *App
	form    *tview.Form
	visible bool
}

// newDatadogEditor builds the Datadog settings overlay's form.
func newDatadogEditor(a *App) *datadogEditor {
	de := &datadogEditor{app: a}
	de.form = tview.NewForm()
	de.form.SetBorder(true).SetTitle(" Datadog ")
	de.form.
		AddInputField("Site", "", 30, nil, nil).
		AddPasswordField("Access Token", "", 40, '*', nil).
		AddButton("Save", func() { de.save() }).
		AddButton("Cancel", func() { de.close() })
	de.form.SetInputCapture(func(event *tcell.EventKey) *tcell.EventKey {
		if event.Key() == tcell.KeyEscape {
			de.close()
			return nil
		}
		return event
	})
	return de
}

// show opens the Datadog settings overlay, pre-filled with the current
// config.
func (de *datadogEditor) show() {
	de.form.GetFormItem(0).(*tview.InputField).SetText(de.app.cfg.Datadog.Site)
	de.form.GetFormItem(1).(*tview.InputField).SetText(de.app.cfg.Datadog.AccessToken)

	de.app.rootPages.ShowPage("datadog-editor")
	de.app.tv.SetFocus(de.form)
	de.visible = true
}

// close hides the editor without saving.
func (de *datadogEditor) close() {
	de.app.rootPages.HidePage("datadog-editor")
	de.visible = false
	de.app.tv.SetFocus(de.app.pages)
}

// ApplyPalette recolors the Datadog settings overlay for a live theme switch.
func (de *datadogEditor) ApplyPalette(p config.Palette) {
	de.form.SetBackgroundColor(tcell.GetColor(p.Background))
	de.form.SetBorderColor(tcell.GetColor(p.Border))
	de.form.SetTitleColor(tcell.GetColor(p.Border))
}

var _ ui.Themeable = (*datadogEditor)(nil)

// save persists the editor form into cfg.Datadog, then closes it. Unlike
// saveConnEditor, there's no required-field validation here — an empty
// Site defaults to datadoghq.com at search time
// (internal/datadoglogs.Search), and an empty AccessToken just means "not
// configured yet" (surfaced as a clear error only if the user actually
// tries to search).
func (de *datadogEditor) save() {
	a := de.app
	site := strings.TrimSpace(de.form.GetFormItem(0).(*tview.InputField).GetText())
	token := de.form.GetFormItem(1).(*tview.InputField).GetText()

	a.SaveDatadogConfig(config.DatadogConfig{Site: site, AccessToken: token})
	de.close()
}

// SaveDatadogConfig persists cfg.Datadog and refreshes the settings list.
func (a *App) SaveDatadogConfig(cfg config.DatadogConfig) {
	a.cfg.Datadog = cfg
	if err := config.SaveDefault(a.cfg); err != nil {
		slog.Error("SaveDatadogConfig: save failed", "error", err)
	}
	a.refreshSettingsList()
}
