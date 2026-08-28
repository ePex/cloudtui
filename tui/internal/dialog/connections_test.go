package dialog

import (
	"context"
	"errors"
	"strings"
	"testing"

	"github.com/gdamore/tcell/v2"
	"github.com/rivo/tview"

	"github.com/ePex/cloudtui/tui/internal/awsprofile"
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

// TestConnEditorAWSProfileFieldTracksPasswordSource confirms "AWS
// Profile" only exists alongside "Password Secret Name", appears above
// it, and both are added by selecting "AWS Secret" / removed by
// selecting "Plain".
func TestConnEditorAWSProfileFieldTracksPasswordSource(t *testing.T) {
	ce, _ := newTestConnEditor(t)
	ce.Show(config.Connection{}, true, "")

	if _, ok := ce.form.GetFormItemByLabel("AWS Profile").(*tview.InputField); ok {
		t.Fatal(`"AWS Profile" present before selecting "AWS Secret"`)
	}

	source := ce.form.GetFormItemByLabel("Password Source").(*tview.DropDown)
	source.SetCurrentOption(1) // AWS Secret

	if _, ok := ce.form.GetFormItemByLabel("AWS Profile").(*tview.InputField); !ok {
		t.Fatal(`"AWS Profile" missing after selecting "AWS Secret"`)
	}
	if _, ok := ce.form.GetFormItemByLabel("Password Secret Name").(*tview.InputField); !ok {
		t.Fatal(`"Password Secret Name" missing after selecting "AWS Secret"`)
	}
	if profileIdx, secretIdx := ce.form.GetFormItemIndex("AWS Profile"), ce.form.GetFormItemIndex("Password Secret Name"); profileIdx != secretIdx-1 {
		t.Errorf(`"AWS Profile" (index %d) is not directly above "Password Secret Name" (index %d)`, profileIdx, secretIdx)
	}

	source.SetCurrentOption(0) // Plain

	if _, ok := ce.form.GetFormItemByLabel("AWS Profile").(*tview.InputField); ok {
		t.Fatal(`"AWS Profile" still present after selecting "Plain"`)
	}
	if _, ok := ce.form.GetFormItemByLabel("Password").(*tview.InputField); !ok {
		t.Fatal(`"Password" missing after selecting "Plain"`)
	}
}

// TestConnEditorAWSProfileSurvivesBackendToggle confirms rebuildTail
// (fired by the Backend dropdown) preserves whatever was typed into
// "AWS Profile" across a jolokia -> proxy -> jolokia round trip, the
// same guarantee already relied on for the Password Secret Name field
// itself.
func TestConnEditorAWSProfileSurvivesBackendToggle(t *testing.T) {
	ce, _ := newTestConnEditor(t)
	ce.Show(config.Connection{}, true, "")

	ce.form.GetFormItemByLabel("Password Source").(*tview.DropDown).SetCurrentOption(1) // AWS Secret
	ce.form.GetFormItemByLabel("AWS Profile").(*tview.InputField).SetText("work")
	ce.form.GetFormItemByLabel("Password Secret Name").(*tview.InputField).SetText("my/secret")

	backend := ce.form.GetFormItem(1).(*tview.DropDown)
	backend.SetCurrentOption(1) // proxy
	backend.SetCurrentOption(0) // back to jolokia

	if got := ce.form.GetFormItemByLabel("AWS Profile").(*tview.InputField).GetText(); got != "work" {
		t.Errorf(`AWS Profile = %q after Backend round trip, want "work"`, got)
	}
	if got := ce.form.GetFormItemByLabel("Password Secret Name").(*tview.InputField).GetText(); got != "my/secret" {
		t.Errorf(`Password Secret Name = %q after Backend round trip, want "my/secret"`, got)
	}
}

// TestConnEditorPasswordSecretAWSProfileRoundTrips confirms Show()
// populates "AWS Profile" from conn.Queue.PasswordSecretAWSProfile and
// save() writes the edited value back into the same field on the
// saved connection.
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

	if got := ce.form.GetFormItemByLabel("AWS Profile").(*tview.InputField).GetText(); got != "work" {
		t.Fatalf(`AWS Profile after Show() = %q, want "work"`, got)
	}

	ce.form.GetFormItemByLabel("AWS Profile").(*tview.InputField).SetText("prod")
	ce.save()

	if host.savedConnection == nil {
		t.Fatal("save() did not call SaveConnection")
	}
	if got := host.savedConnection.conn.Queue.PasswordSecretAWSProfile; got != "prod" {
		t.Errorf("saved Queue.PasswordSecretAWSProfile = %q, want %q", got, "prod")
	}
}

// TestConnEditorSaveRequiresAWSProfileWhenAWSSecretSelected is a
// regression test for the validation added alongside the new field:
// saving with "AWS Secret" selected and "AWS Profile" left blank must
// be rejected rather than silently persisting an unresolvable
// connection.
func TestConnEditorSaveRequiresAWSProfileWhenAWSSecretSelected(t *testing.T) {
	ce, host := newTestConnEditor(t)
	ce.Show(config.Connection{}, true, "")

	ce.form.GetFormItem(0).(*tview.InputField).SetText("orders")
	ce.form.GetFormItemByLabel("Password Source").(*tview.DropDown).SetCurrentOption(1) // AWS Secret
	ce.form.GetFormItemByLabel("Password Secret Name").(*tview.InputField).SetText("my/secret")
	// AWS Profile left blank.

	ce.save()

	if host.savedConnection != nil {
		t.Fatal("save() persisted a connection with a blank AWS Profile")
	}
	if !ce.visible {
		t.Error("save() closed the editor despite failing validation")
	}
	if !strings.Contains(host.status, "AWS Profile is required") {
		t.Errorf("status = %q, want it to mention the missing AWS Profile", host.status)
	}
}

// TestConnEditorAWSProfileSuggestionsFiltersByPrefix confirms the "AWS
// Profile" field's autocomplete offers the discovered profile names
// (via host.ListAWSProfiles) filtered by prefix, the same convention
// MessageFilter's jmsTypeSuggestions already uses.
func TestConnEditorAWSProfileSuggestionsFiltersByPrefix(t *testing.T) {
	ce, host := newTestConnEditor(t)
	host.listAWSProfiles = func(context.Context) ([]awsprofile.Profile, error) {
		return []awsprofile.Profile{{Name: "work-dev"}, {Name: "work-prod"}, {Name: "personal"}}, nil
	}
	ce.Show(config.Connection{}, true, "")

	got := ce.awsProfileSuggestions("work")
	want := []string{"work-dev", "work-prod"}
	if len(got) != len(want) {
		t.Fatalf("awsProfileSuggestions(\"work\") = %v, want %v", got, want)
	}
	for i := range want {
		if got[i] != want[i] {
			t.Errorf("awsProfileSuggestions(\"work\")[%d] = %q, want %q (full: %v)", i, got[i], want[i], got)
		}
	}
}

// TestConnEditorAWSProfileSuggestionsEmptyOnDiscoveryError confirms a
// failed AWS profile discovery degrades to no suggestions rather than
// breaking Show() or the field itself (still usable as freeform text).
func TestConnEditorAWSProfileSuggestionsEmptyOnDiscoveryError(t *testing.T) {
	ce, host := newTestConnEditor(t)
	host.listAWSProfiles = func(context.Context) ([]awsprofile.Profile, error) {
		return nil, errors.New("no AWS config file found")
	}
	ce.Show(config.Connection{}, true, "")

	if got := ce.awsProfileSuggestions(""); got != nil {
		t.Errorf("awsProfileSuggestions(\"\") after discovery error = %v, want nil", got)
	}
	if !ce.visible {
		t.Error("Show() should still open the editor despite a discovery error")
	}
}
