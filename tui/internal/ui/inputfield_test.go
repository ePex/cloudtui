package ui

import (
	"testing"

	"github.com/gdamore/tcell/v2"
	"github.com/rivo/tview"
)

// drawAndGetCursor focuses, draws, and reports the cursor state of field on
// a fresh simulation screen - the same screen implementation tview uses in
// its own tests, so this exercises the real Draw() code path rather than
// inspecting unexported state.
func drawAndGetCursor(t *testing.T, field *tview.InputField) (visible bool) {
	t.Helper()

	screen := tcell.NewSimulationScreen("")
	if err := screen.Init(); err != nil {
		t.Fatalf("screen.Init() failed: %v", err)
	}
	defer screen.Fini()
	screen.SetSize(40, 3)

	field.SetRect(0, 0, 40, 1)
	field.Focus(func(tview.Primitive) {})
	field.Draw(screen)

	_, _, visible = screen.GetCursor()
	return visible
}

func TestSetInputFieldTextShowsCursorForOverflowingValue(t *testing.T) {
	field := tview.NewInputField().SetFieldWidth(10)
	// tview's TextArea only learns its own rendered width from a real Draw()
	// call (there's no other assignment site for it) - draw once first to
	// match the real-world case this bug affects: a field that's already
	// part of a rendered form/dialog gets repopulated with a different
	// value (e.g. reopening the connection editor on another connection).
	// A field populated before its very first Draw() doesn't hit the bug at
	// all - Draw()'s own first-time handling resolves and clamps the cursor
	// itself in that case.
	drawAndGetCursor(t, field)

	SetInputFieldText(field, "a value much longer than the field's width")

	if !drawAndGetCursor(t, field) {
		t.Error("cursor not visible after SetInputFieldText populated an already-drawn field with a value longer than its width")
	}
}

func TestSetInputFieldTextShowsCursorForShortValue(t *testing.T) {
	field := tview.NewInputField().SetFieldWidth(40)
	drawAndGetCursor(t, field)

	SetInputFieldText(field, "short")

	if !drawAndGetCursor(t, field) {
		t.Error("cursor not visible for a value shorter than the field width (control case)")
	}
}

// TestInputFieldSetTextHidesCursorForOverflowingValue documents the upstream
// bug itself: plain SetText (without the synthetic End keypress) leaves the
// cursor hidden once the value overflows an already-drawn field, which is
// exactly what SetInputFieldText above works around. If this ever starts
// failing, tview has fixed the bug upstream and SetInputFieldText's
// workaround is likely safe to remove.
func TestInputFieldSetTextHidesCursorForOverflowingValue(t *testing.T) {
	field := tview.NewInputField().SetFieldWidth(10)
	drawAndGetCursor(t, field)

	field.SetText("a value much longer than the field's width")

	if drawAndGetCursor(t, field) {
		t.Error("expected the upstream tview bug to hide the cursor for plain SetText with an overflowing value; tview may have fixed it")
	}
}
