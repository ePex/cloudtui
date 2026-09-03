package ui

import (
	"github.com/gdamore/tcell/v2"
	"github.com/rivo/tview"
)

// SetInputFieldText populates field's text and works around a tview
// v0.42.0 bug where a value longer than the field's visible width
// renders with the cursor hidden and the field still showing its
// start rather than its end.
//
// Two things have to both be true for the fix (a synthetic End
// keypress, see below) to actually do anything:
//
//  1. InputField.SetText -> TextArea.Replace calls
//     findCursor(false, row), which recomputes the cursor's true
//     position but never scrolls columnOffset to keep it visible.
//  2. A real keypress fixes this because its handling goes through
//     TextArea.moveCursor(), which always ends with
//     findCursor(true, row) - the scrolling variant. But moveCursor
//     bails out as a complete no-op while TextArea.lastWidth is still
//     0, which is the case until the field has been drawn for real at
//     least once. A brand new field - e.g. a connection being edited
//     for the first time in this session - has never been drawn, so a
//     synthetic keypress fired right after SetText would do nothing,
//     and worse, leaves the cursor "resolved" in a way that blocks
//     TextArea's own (very limited) self-heal on the next real draw.
//
// So this establishes lastWidth itself first, via a throwaway,
// off-screen Draw() using the field's own current rect. That works
// even before the field has ever been part of a visible layout:
// tview.Box defaults to a non-zero rect (NewBox: 15x10) from
// construction, and InputField.Draw only requires width >= 1 to
// proceed - it doesn't require the field to be on-screen or focused.
// If this throwaway width ends up narrower than the field's eventual
// real width (its likely direction, since 15 is tview's generic
// default and real layouts tend to give a field more room), the
// resulting clamp remains valid there too: a cursor within a narrower
// visible window is automatically within a wider one starting at the
// same offset.
func SetInputFieldText(field *tview.InputField, text string) {
	field.SetText(text)

	screen := tcell.NewSimulationScreen("")
	screen.Init()
	field.Draw(screen)
	screen.Fini()

	field.InputHandler()(tcell.NewEventKey(tcell.KeyEnd, 0, tcell.ModNone), func(tview.Primitive) {})
}
