package dialog

import (
	"testing"

	"github.com/gdamore/tcell/v2"
	"github.com/rivo/tview"
)

func TestAMQManagerSettingsEditorShowAndSave(t *testing.T) {
	host := newTestHost()
	host.cfg.AMQManager.ShowOnlyQueuesWithPendingMessages = true
	e := NewAMQManagerSettingsEditor(host)
	e.Show()

	if !e.Visible() {
		t.Fatal("Visible() = false after Show()")
	}
	if got := e.form.GetFormItem(0).(*tview.Checkbox).IsChecked(); !got {
		t.Fatal("checkbox = false after Show(), want true from config")
	}
	e.form.GetFormItem(0).(*tview.Checkbox).SetChecked(false)
	e.form.GetButton(e.form.GetButtonIndex("Save")).InputHandler()(
		tcell.NewEventKey(tcell.KeyEnter, 0, tcell.ModNone), func(tview.Primitive) {})

	if host.savedAMQManagerSettings == nil || host.savedAMQManagerSettings.ShowOnlyQueuesWithPendingMessages {
		t.Fatalf("saved AMQ Manager settings = %+v, want disabled", host.savedAMQManagerSettings)
	}
	if e.Visible() {
		t.Error("Visible() = true after save")
	}
}

func TestAMQManagerSettingsEditorCancelDoesNotSave(t *testing.T) {
	host := newTestHost()
	e := NewAMQManagerSettingsEditor(host)
	e.Show()
	e.form.GetFormItem(0).(*tview.Checkbox).SetChecked(true)
	e.form.GetButton(e.form.GetButtonIndex("Cancel")).InputHandler()(
		tcell.NewEventKey(tcell.KeyEnter, 0, tcell.ModNone), func(tview.Primitive) {})

	if host.savedAMQManagerSettings != nil {
		t.Errorf("Cancel saved settings: %+v", host.savedAMQManagerSettings)
	}
	if host.cfg.AMQManager.ShowOnlyQueuesWithPendingMessages {
		t.Error("Cancel changed the host setting")
	}
}
