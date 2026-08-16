package app

import (
	"testing"

	"github.com/gdamore/tcell/v2"

	"github.com/ePex/cloudtui/tui/internal/config"
)

// TestConnEditorEscapeCloses guards against a UX gap where the connection
// editor had no way to cancel via Esc — only by tabbing all the way to the
// Cancel button.
func TestConnEditorEscapeCloses(t *testing.T) {
	a := New(config.Default())
	a.connEditor.show(config.Connection{}, true, "")
	if !a.connEditor.visible {
		t.Fatal("connEditor.show() did not open the editor")
	}

	capture := a.connEditor.form.GetInputCapture()
	if capture == nil {
		t.Fatal("connEditor.form has no input capture set")
	}
	if got := capture(tcell.NewEventKey(tcell.KeyEscape, 0, tcell.ModNone)); got != nil {
		t.Errorf("Esc capture returned %v, want nil (event consumed)", got)
	}
	if a.connEditor.visible {
		t.Error("Esc did not close the connection editor")
	}
}

// TestConnEditorOtherKeysPassThrough ensures the Esc handler doesn't
// swallow other keys needed for normal form interaction (e.g. typing).
func TestConnEditorOtherKeysPassThrough(t *testing.T) {
	a := New(config.Default())
	a.connEditor.show(config.Connection{}, true, "")

	capture := a.connEditor.form.GetInputCapture()
	event := tcell.NewEventKey(tcell.KeyRune, 'x', tcell.ModNone)
	if got := capture(event); got != event {
		t.Errorf("capture() altered/swallowed a non-Esc key: got %v, want it passed through unchanged", got)
	}
	if !a.connEditor.visible {
		t.Error("a non-Esc key should not close the editor")
	}
}
