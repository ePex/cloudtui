package app

import (
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
	host    ui.Host
	form    *tview.Form
	visible bool
}

// newDatadogEditor builds the Datadog settings overlay's form.
func newDatadogEditor(host ui.Host) *datadogEditor {
	de := &datadogEditor{host: host}
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
	datadog := de.host.Config().Datadog
	de.form.GetFormItem(0).(*tview.InputField).SetText(datadog.Site)
	de.form.GetFormItem(1).(*tview.InputField).SetText(datadog.AccessToken)

	de.host.ShowPage("datadog-editor")
	de.host.SetFocus(de.form)
	de.visible = true
}

// close hides the editor without saving.
func (de *datadogEditor) close() {
	de.host.HidePage("datadog-editor")
	de.visible = false
	de.host.FocusMain()
}

// ApplyPalette recolors the Datadog settings overlay for a live theme switch.
func (de *datadogEditor) ApplyPalette(p config.Palette) {
	de.form.SetBackgroundColor(tcell.GetColor(p.Background))
	de.form.SetBorderColor(tcell.GetColor(p.Border))
	de.form.SetTitleColor(tcell.GetColor(p.Border))
}

var _ ui.Themeable = (*datadogEditor)(nil)

// Primitive returns datadogEditor's root widget, for sizing/embedding.
func (de *datadogEditor) Primitive() tview.Primitive { return de.form }

// Visible reports whether datadogEditor is currently shown.
func (de *datadogEditor) Visible() bool { return de.visible }

// save persists the editor form into cfg.Datadog, then closes it. Unlike
// saveConnEditor, there's no required-field validation here — an empty
// Site defaults to datadoghq.com at search time
// (internal/datadoglogs.Search), and an empty AccessToken just means "not
// configured yet" (surfaced as a clear error only if the user actually
// tries to search).
func (de *datadogEditor) save() {
	site := strings.TrimSpace(de.form.GetFormItem(0).(*tview.InputField).GetText())
	token := de.form.GetFormItem(1).(*tview.InputField).GetText()

	de.host.SaveDatadogConfig(config.DatadogConfig{Site: site, AccessToken: token})
	de.close()
}
