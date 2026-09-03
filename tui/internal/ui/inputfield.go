package ui

import (
	"github.com/gdamore/tcell/v2"
	"github.com/rivo/tview"
)

// SetInputFieldText populates field's text and works around a tview
// v0.42.0 bug: InputField.SetText -> TextArea.Replace calls
// findCursor(false, row), which recomputes the cursor's true position
// but skips scrolling columnOffset to keep it visible. A value longer
// than the field's width then renders with the cursor hidden and the
// field still showing its start, not its end.
//
// Firing a synthetic End keypress through the field's own input
// handler takes the same code path a real keypress would: InputField
// forwards KeyEnd to its TextArea, whose moveCursor() always ends with
// findCursor(true, row) - the clamping variant. Box's WrapInputHandler
// has no focus guard, so this works even when the field doesn't
// currently have focus (the common case right after populating a
// form).
func SetInputFieldText(field *tview.InputField, text string) {
	field.SetText(text)
	field.InputHandler()(tcell.NewEventKey(tcell.KeyEnd, 0, tcell.ModNone), func(tview.Primitive) {})
}
