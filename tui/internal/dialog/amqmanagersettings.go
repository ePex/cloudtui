package dialog

import (
	"github.com/gdamore/tcell/v2"
	"github.com/rivo/tview"

	"github.com/ePex/cloudtui/tui/internal/config"
	"github.com/ePex/cloudtui/tui/internal/ui"
)

// AMQManagerSettingsEditor edits global settings for the AMQ queue manager.
type AMQManagerSettingsEditor struct {
	host    ui.Host
	form    *tview.Form
	visible bool
}

// NewAMQManagerSettingsEditor builds the AMQ Manager settings overlay.
func NewAMQManagerSettingsEditor(host ui.Host) *AMQManagerSettingsEditor {
	e := &AMQManagerSettingsEditor{host: host}
	e.form = tview.NewForm()
	e.form.SetBorder(true).SetTitle(" AMQ Manager ")
	e.form.
		AddCheckbox("Show only queues with pending messages", false, nil).
		AddButton("Save", func() { e.save() }).
		AddButton("Cancel", func() { e.close() })
	e.form.SetInputCapture(func(event *tcell.EventKey) *tcell.EventKey {
		if event.Key() == tcell.KeyEscape {
			e.close()
			return nil
		}
		return event
	})
	return e
}

// Show opens the overlay with the current configuration.
func (e *AMQManagerSettingsEditor) Show() {
	settings := e.host.Config().AMQManager
	e.form.GetFormItem(0).(*tview.Checkbox).SetChecked(settings.ShowOnlyQueuesWithPendingMessages)
	e.host.ShowPage("amq-manager-settings")
	e.host.SetFocus(e.form)
	e.visible = true
}

func (e *AMQManagerSettingsEditor) close() {
	e.host.HidePage("amq-manager-settings")
	e.visible = false
	e.host.FocusMain()
}

func (e *AMQManagerSettingsEditor) save() {
	checked := e.form.GetFormItem(0).(*tview.Checkbox).IsChecked()
	e.host.SaveAMQManagerSettings(config.AMQManagerSettings{ShowOnlyQueuesWithPendingMessages: checked})
	e.close()
}

// ApplyPalette recolors the AMQ Manager settings overlay.
func (e *AMQManagerSettingsEditor) ApplyPalette(p config.Palette) {
	e.form.SetBackgroundColor(tcell.GetColor(p.Background))
	e.form.SetBorderColor(tcell.GetColor(p.Border))
	e.form.SetTitleColor(tcell.GetColor(p.Border))
	ui.StyleForm(e.form, p)
}

var _ ui.Themeable = (*AMQManagerSettingsEditor)(nil)

// Primitive returns the editor's root widget.
func (e *AMQManagerSettingsEditor) Primitive() tview.Primitive { return e.form }

// Visible reports whether the overlay is open.
func (e *AMQManagerSettingsEditor) Visible() bool { return e.visible }
