package app

import (
	"path/filepath"
	"testing"

	"github.com/gdamore/tcell/v2"
	"github.com/rivo/tview"

	"github.com/ePex/cloudtui/tui/internal/config"
)

// TestDatadogEditorEscapeCloses guards against the same UX gap
// TestConnEditorEscapeCloses guards against for the connection editor —
// Esc must close without tabbing all the way to Cancel.
func TestDatadogEditorEscapeCloses(t *testing.T) {
	a := New(config.Default())
	a.datadogEditor.show()
	if !a.datadogEditor.visible {
		t.Fatal("datadogEditor.show() did not open the editor")
	}

	capture := a.datadogEditor.form.GetInputCapture()
	if capture == nil {
		t.Fatal("datadogEditor.form has no input capture set")
	}
	if got := capture(tcell.NewEventKey(tcell.KeyEscape, 0, tcell.ModNone)); got != nil {
		t.Errorf("Esc capture returned %v, want nil (event consumed)", got)
	}
	if a.datadogEditor.visible {
		t.Error("Esc did not close the Datadog editor")
	}
}

func TestDatadogEditorOtherKeysPassThrough(t *testing.T) {
	a := New(config.Default())
	a.datadogEditor.show()

	capture := a.datadogEditor.form.GetInputCapture()
	event := tcell.NewEventKey(tcell.KeyRune, 'x', tcell.ModNone)
	if got := capture(event); got != event {
		t.Errorf("capture() altered/swallowed a non-Esc key: got %v, want it passed through unchanged", got)
	}
	if !a.datadogEditor.visible {
		t.Error("a non-Esc key should not close the editor")
	}
}

func TestDatadogEditorPrefillsFromConfig(t *testing.T) {
	a := New(config.Default())
	a.cfg.Datadog = config.DatadogConfig{Site: "datadoghq.eu", AccessToken: "tok-123"}

	a.datadogEditor.show()

	if got := a.datadogEditor.form.GetFormItem(0).(*tview.InputField).GetText(); got != "datadoghq.eu" {
		t.Errorf("Site field = %q, want %q", got, "datadoghq.eu")
	}
	if got := a.datadogEditor.form.GetFormItem(1).(*tview.InputField).GetText(); got != "tok-123" {
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
	a.datadogEditor.show()
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

func TestDatadogSettingsLabel(t *testing.T) {
	cases := []struct {
		name string
		cfg  config.DatadogConfig
		want string
	}{
		{"unconfigured", config.DatadogConfig{}, "(none)"},
		{"token without site defaults label", config.DatadogConfig{AccessToken: "tok"}, "datadoghq.com"},
		{"token with site shows site", config.DatadogConfig{Site: "datadoghq.eu", AccessToken: "tok"}, "datadoghq.eu"},
		{"site without token still (none)", config.DatadogConfig{Site: "datadoghq.eu"}, "(none)"},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			if got := datadogSettingsLabel(c.cfg); got != c.want {
				t.Errorf("datadogSettingsLabel(%+v) = %q, want %q", c.cfg, got, c.want)
			}
		})
	}
}
