package app

import (
	"path/filepath"
	"testing"

	"github.com/gdamore/tcell/v2"
	"github.com/rivo/tview"

	"github.com/ePex/cloudtui/tui/internal/config"
)

func newTestDatadogEditor(t *testing.T) (*DatadogEditor, *testHost) {
	t.Helper()
	host := newTestHost()
	return NewDatadogEditor(host), host
}

// TestDatadogEditorEscapeCloses guards against the same UX gap
// TestConnEditorEscapeCloses guards against for the connection editor —
// Esc must close without tabbing all the way to Cancel.
func TestDatadogEditorEscapeCloses(t *testing.T) {
	de, _ := newTestDatadogEditor(t)
	de.Show()
	if !de.visible {
		t.Fatal("DatadogEditor.Show() did not open the editor")
	}

	capture := de.form.GetInputCapture()
	if capture == nil {
		t.Fatal("DatadogEditor.form has no input capture set")
	}
	if got := capture(tcell.NewEventKey(tcell.KeyEscape, 0, tcell.ModNone)); got != nil {
		t.Errorf("Esc capture returned %v, want nil (event consumed)", got)
	}
	if de.visible {
		t.Error("Esc did not close the Datadog editor")
	}
}

func TestDatadogEditorOtherKeysPassThrough(t *testing.T) {
	de, _ := newTestDatadogEditor(t)
	de.Show()

	capture := de.form.GetInputCapture()
	event := tcell.NewEventKey(tcell.KeyRune, 'x', tcell.ModNone)
	if got := capture(event); got != event {
		t.Errorf("capture() altered/swallowed a non-Esc key: got %v, want it passed through unchanged", got)
	}
	if !de.visible {
		t.Error("a non-Esc key should not close the editor")
	}
}

func TestDatadogEditorPrefillsFromConfig(t *testing.T) {
	de, host := newTestDatadogEditor(t)
	host.cfg.Datadog = config.DatadogConfig{Site: "datadoghq.eu", AccessToken: "tok-123"}

	de.Show()

	if got := de.form.GetFormItem(0).(*tview.InputField).GetText(); got != "datadoghq.eu" {
		t.Errorf("Site field = %q, want %q", got, "datadoghq.eu")
	}
	if got := de.form.GetFormItem(1).(*tview.InputField).GetText(); got != "tok-123" {
		t.Errorf("Access Token field = %q, want %q", got, "tok-123")
	}
}

// TestSaveDatadogEditorRoundTrip confirms the save path this repo's
// connEditorForm doesn't have an existing equivalent test for: filling
// the form and saving should update cfg.Datadog, persist to config.yaml,
// and close the editor.
func TestSaveDatadogEditorRoundTrip(t *testing.T) {
	dir := t.TempDir()
	t.Chdir(dir)

	a := New(config.Default())
	a.datadogEditor.Show()
	a.datadogEditor.form.GetFormItem(0).(*tview.InputField).SetText("datadoghq.eu")
	a.datadogEditor.form.GetFormItem(1).(*tview.InputField).SetText("tok-456")

	a.datadogEditor.save()

	if a.datadogEditor.visible {
		t.Error("datadogEditor.save() did not close the editor")
	}
	want := config.DatadogConfig{Site: "datadoghq.eu", AccessToken: "tok-456"}
	if a.cfg.Datadog != want {
		t.Errorf("cfg.Datadog = %+v, want %+v", a.cfg.Datadog, want)
	}

	got, err := config.Load(filepath.Join(dir, "config.yaml"))
	if err != nil {
		t.Fatalf("Load() error = %v", err)
	}
	if got.Datadog != want {
		t.Errorf("persisted Datadog = %+v, want %+v", got.Datadog, want)
	}
}
