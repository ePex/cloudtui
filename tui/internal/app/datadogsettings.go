package app

import (
	"log/slog"
	"strings"

	"github.com/rivo/tview"

	"github.com/ePex/cloudtui/tui/internal/config"
)

// showDatadogEditor opens the Datadog settings overlay, pre-filled with
// the current config.
func (a *App) showDatadogEditor() {
	a.datadogEditorForm.GetFormItem(0).(*tview.InputField).SetText(a.cfg.Datadog.Site)
	a.datadogEditorForm.GetFormItem(1).(*tview.InputField).SetText(a.cfg.Datadog.AccessToken)

	a.rootPages.ShowPage("datadog-editor")
	a.tv.SetFocus(a.datadogEditorForm)
	a.datadogEditorVisible = true
}

// closeDatadogEditor hides the editor without saving.
func (a *App) closeDatadogEditor() {
	a.rootPages.HidePage("datadog-editor")
	a.datadogEditorVisible = false
	a.tv.SetFocus(a.pages)
}

// saveDatadogEditor persists the editor form into cfg.Datadog, then
// closes it. Unlike saveConnEditor, there's no required-field
// validation here — an empty Site defaults to datadoghq.com at search
// time (internal/datadoglogs.Search), and an empty AccessToken just
// means "not configured yet" (surfaced as a clear error only if the
// user actually tries to search).
func (a *App) saveDatadogEditor() {
	site := strings.TrimSpace(a.datadogEditorForm.GetFormItem(0).(*tview.InputField).GetText())
	token := a.datadogEditorForm.GetFormItem(1).(*tview.InputField).GetText()

	a.cfg.Datadog = config.DatadogConfig{Site: site, AccessToken: token}

	if err := config.SaveDefault(a.cfg); err != nil {
		slog.Error("saveDatadogEditor: save failed", "error", err)
	}
	a.refreshSettingsList()
	a.closeDatadogEditor()
}
