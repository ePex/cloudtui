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

// TestSetInputFieldTextShowsCursorForNeverDrawnField is the primary
// regression case: a field populated for the very first time in a session
// (e.g. editing any connection for the first time after starting the TUI) -
// TextArea.lastWidth is still 0 at that point, which an earlier version of
// SetInputFieldText didn't account for (its synthetic End keypress was a
// no-op against a never-drawn field, and left the cursor "resolved" in a way
// that blocked tview's own limited self-heal on the next real draw - worse
// than doing nothing). Deliberately does NOT call drawAndGetCursor before
// SetInputFieldText, unlike the "already drawn" case below.
func TestSetInputFieldTextShowsCursorForNeverDrawnField(t *testing.T) {
	field := tview.NewInputField().SetFieldWidth(10)

	SetInputFieldText(field, "a value much longer than the field's width")

	if !drawAndGetCursor(t, field) {
		t.Error("cursor not visible after SetInputFieldText populated a never-before-drawn field with an overflowing value")
	}
}

// TestSetInputFieldTextShowsCursorForNeverDrawnDynamicWidthField is the same
// as above but for a field with no explicit SetFieldWidth (fieldWidth == 0,
// computed from the box's own width at draw time) - matches the
// view-package filter/search inputs, none of which set a fixed width.
func TestSetInputFieldTextShowsCursorForNeverDrawnDynamicWidthField(t *testing.T) {
	field := tview.NewInputField()

	SetInputFieldText(field, "a value much longer than the default field width tview gives a never-yet-laid-out box")

	if !drawAndGetCursor(t, field) {
		t.Error("cursor not visible after SetInputFieldText populated a never-before-drawn, dynamic-width field with an overflowing value")
	}
}

// TestSetInputFieldTextShowsCursorForAlreadyDrawnField covers the other real
// scenario: a field that's already part of a rendered form/dialog gets
// repopulated with a different value (e.g. reopening the connection editor
// on another connection, or reopening a filter/search box that restores its
// last-applied value).
func TestSetInputFieldTextShowsCursorForAlreadyDrawnField(t *testing.T) {
	field := tview.NewInputField().SetFieldWidth(10)
	drawAndGetCursor(t, field)

	SetInputFieldText(field, "a value much longer than the field's width")

	if !drawAndGetCursor(t, field) {
		t.Error("cursor not visible after SetInputFieldText populated an already-drawn field with a value longer than its width")
	}
}

func TestSetInputFieldTextShowsCursorForShortValue(t *testing.T) {
	field := tview.NewInputField().SetFieldWidth(40)

	SetInputFieldText(field, "short")

	if !drawAndGetCursor(t, field) {
		t.Error("cursor not visible for a value shorter than the field width (control case)")
	}
}

// TestInputFieldSetTextHidesCursorForOverflowingValue documents the upstream
// bug itself: plain SetText (without SetInputFieldText's workaround) leaves
// the cursor hidden once the value overflows the field - both for a field
// that's never been drawn before and for one that has. If either of these
// ever starts failing, tview has fixed the underlying bug upstream and
// SetInputFieldText's workaround is likely safe to remove.
func TestInputFieldSetTextHidesCursorForOverflowingValue(t *testing.T) {
	t.Run("never drawn before", func(t *testing.T) {
		field := tview.NewInputField().SetFieldWidth(10)
		field.SetText("a value much longer than the field's width")

		if drawAndGetCursor(t, field) {
			t.Error("expected the upstream tview bug to hide the cursor; tview may have fixed it")
		}
	})

	t.Run("already drawn", func(t *testing.T) {
		field := tview.NewInputField().SetFieldWidth(10)
		drawAndGetCursor(t, field)

		field.SetText("a value much longer than the field's width")

		if drawAndGetCursor(t, field) {
			t.Error("expected the upstream tview bug to hide the cursor; tview may have fixed it")
		}
	})
}
