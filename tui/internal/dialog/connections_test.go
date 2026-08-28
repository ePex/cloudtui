package dialog

import (
	"strings"
	"testing"

	"github.com/gdamore/tcell/v2"
	"github.com/rivo/tview"

	"github.com/ePex/cloudtui/tui/internal/config"
)

func newTestConnEditor(t *testing.T) (*ConnEditor, *testHost) {
	t.Helper()
	host := newTestHost()
	manager := NewConnManager(host, NewConfirmDialog(host))
	return NewConnEditor(host, manager), host
}

// TestConnEditorEscapeCloses guards against a UX gap where the connection
// editor had no way to cancel via Esc — only by tabbing all the way to the
// Cancel button.
func TestConnEditorEscapeCloses(t *testing.T) {
	ce, _ := newTestConnEditor(t)
	ce.Show(config.Connection{}, true, "")
	if !ce.visible {
		t.Fatal("ConnEditor.Show() did not open the editor")
	}

	capture := ce.form.GetInputCapture()
	if capture == nil {
		t.Fatal("ConnEditor.form has no input capture set")
	}
	if got := capture(tcell.NewEventKey(tcell.KeyEscape, 0, tcell.ModNone)); got != nil {
		t.Errorf("Esc capture returned %v, want nil (event consumed)", got)
	}
	if ce.visible {
		t.Error("Esc did not close the connection editor")
	}
}

// TestConnEditorOtherKeysPassThrough ensures the Esc handler doesn't
// swallow other keys needed for normal form interaction (e.g. typing).
func TestConnEditorOtherKeysPassThrough(t *testing.T) {
	ce, _ := newTestConnEditor(t)
	ce.Show(config.Connection{}, true, "")

	capture := ce.form.GetInputCapture()
	event := tcell.NewEventKey(tcell.KeyRune, 'x', tcell.ModNone)
	if got := capture(event); got != event {
		t.Errorf("capture() altered/swallowed a non-Esc key: got %v, want it passed through unchanged", got)
	}
	if !ce.visible {
		t.Error("a non-Esc key should not close the editor")
	}
}

// TestConnEditorSecretAWSProfileFieldTracksPasswordSource confirms
// "Secret AWS Profile" only exists alongside "Password Secret (AWS)" —
// selecting "AWS Secret" adds both trailing fields, selecting "Plain"
// swaps back to a single "Password" field.
func TestConnEditorSecretAWSProfileFieldTracksPasswordSource(t *testing.T) {
	ce, _ := newTestConnEditor(t)
	ce.Show(config.Connection{}, true, "")

	if _, ok := ce.form.GetFormItemByLabel("Secret AWS Profile").(*tview.InputField); ok {
		t.Fatal(`"Secret AWS Profile" present before selecting "AWS Secret"`)
	}

	source := ce.form.GetFormItemByLabel("Password Source").(*tview.DropDown)
	source.SetCurrentOption(1) // AWS Secret

	if _, ok := ce.form.GetFormItemByLabel("Secret AWS Profile").(*tview.InputField); !ok {
		t.Fatal(`"Secret AWS Profile" missing after selecting "AWS Secret"`)
	}
	if _, ok := ce.form.GetFormItemByLabel("Password Secret (AWS)").(*tview.InputField); !ok {
		t.Fatal(`"Password Secret (AWS)" missing after selecting "AWS Secret"`)
	}

	source.SetCurrentOption(0) // Plain

	if _, ok := ce.form.GetFormItemByLabel("Secret AWS Profile").(*tview.InputField); ok {
		t.Fatal(`"Secret AWS Profile" still present after selecting "Plain"`)
	}
	if _, ok := ce.form.GetFormItemByLabel("Password").(*tview.InputField); !ok {
		t.Fatal(`"Password" missing after selecting "Plain"`)
	}
}

// TestConnEditorSecretAWSProfileSurvivesBackendToggle confirms
// rebuildTail (fired by the Backend dropdown) preserves whatever was
// typed into "Secret AWS Profile" across a jolokia -> proxy -> jolokia
// round trip, the same guarantee already relied on for the Password
// Secret (AWS) field itself.
func TestConnEditorSecretAWSProfileSurvivesBackendToggle(t *testing.T) {
	ce, _ := newTestConnEditor(t)
	ce.Show(config.Connection{}, true, "")

	ce.form.GetFormItemByLabel("Password Source").(*tview.DropDown).SetCurrentOption(1) // AWS Secret
	ce.form.GetFormItemByLabel("Secret AWS Profile").(*tview.InputField).SetText("work")
	ce.form.GetFormItemByLabel("Password Secret (AWS)").(*tview.InputField).SetText("my/secret")

	backend := ce.form.GetFormItem(1).(*tview.DropDown)
	backend.SetCurrentOption(1) // proxy
	backend.SetCurrentOption(0) // back to jolokia

	if got := ce.form.GetFormItemByLabel("Secret AWS Profile").(*tview.InputField).GetText(); got != "work" {
		t.Errorf(`Secret AWS Profile = %q after Backend round trip, want "work"`, got)
	}
	if got := ce.form.GetFormItemByLabel("Password Secret (AWS)").(*tview.InputField).GetText(); got != "my/secret" {
		t.Errorf(`Password Secret (AWS) = %q after Backend round trip, want "my/secret"`, got)
	}
}

// TestConnEditorPasswordSecretAWSProfileRoundTrips confirms
// Show() populates "Secret AWS Profile" from
// conn.Queue.PasswordSecretAWSProfile and save() writes the edited
// value back into the same field on the saved connection.
func TestConnEditorPasswordSecretAWSProfileRoundTrips(t *testing.T) {
	ce, host := newTestConnEditor(t)
	conn := config.Connection{
		Name:    "orders",
		Backend: "jolokia",
		Queue: config.QueueConfig{
			URL:                      "http://localhost:8161",
			PasswordSecret:           "my/secret",
			PasswordSecretAWSProfile: "work",
		},
	}
	ce.Show(conn, false, "orders")

	if got := ce.form.GetFormItemByLabel("Secret AWS Profile").(*tview.InputField).GetText(); got != "work" {
		t.Fatalf(`Secret AWS Profile after Show() = %q, want "work"`, got)
	}

	ce.form.GetFormItemByLabel("Secret AWS Profile").(*tview.InputField).SetText("prod")
	ce.save()

	if host.savedConnection == nil {
		t.Fatal("save() did not call SaveConnection")
	}
	if got := host.savedConnection.conn.Queue.PasswordSecretAWSProfile; got != "prod" {
		t.Errorf("saved Queue.PasswordSecretAWSProfile = %q, want %q", got, "prod")
	}
}

// TestConnEditorSaveRequiresSecretAWSProfileWhenAWSSecretSelected is a
// regression test for the validation added alongside the new field:
// saving with "AWS Secret" selected and "Secret AWS Profile" left blank
// must be rejected rather than silently persisting an unresolvable
// connection.
func TestConnEditorSaveRequiresSecretAWSProfileWhenAWSSecretSelected(t *testing.T) {
	ce, host := newTestConnEditor(t)
	ce.Show(config.Connection{}, true, "")

	ce.form.GetFormItem(0).(*tview.InputField).SetText("orders")
	ce.form.GetFormItemByLabel("Password Source").(*tview.DropDown).SetCurrentOption(1) // AWS Secret
	ce.form.GetFormItemByLabel("Password Secret (AWS)").(*tview.InputField).SetText("my/secret")
	// Secret AWS Profile left blank.

	ce.save()

	if host.savedConnection != nil {
		t.Fatal("save() persisted a connection with a blank Secret AWS Profile")
	}
	if !ce.visible {
		t.Error("save() closed the editor despite failing validation")
	}
	if !strings.Contains(host.status, "AWS Profile is required") {
		t.Errorf("status = %q, want it to mention the missing AWS Profile", host.status)
	}
}
