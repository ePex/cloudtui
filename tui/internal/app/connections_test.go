package app

import (
	"testing"

	"github.com/gdamore/tcell/v2"

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
