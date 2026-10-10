package dialog

import (
	"github.com/gdamore/tcell/v2"
	"github.com/rivo/tview"
)

// jmsTypeAutocomplete wires the autocomplete of a JMS Type field — the
// purge/move-all prompt's and the message filter's (spec/08, spec/09) — so
// that Enter and Tab never pick a suggestion the user didn't choose.
//
// Two tview v0.42 behaviors make the plain wiring unsafe here. While the
// drop-down is open, InputField treats Tab exactly like Enter: "accept the
// highlighted entry". And when it rebuilds the drop-down it keeps whichever
// entry was highlighted before, so the scan entry (jmsTypeScanSentinel),
// the only entry on the very first build, stays highlighted until the user
// moves the highlight with the arrow keys. Without this, Enter or Tab on an
// empty field, an unlisted type or even a matching prefix started a scan
// and lost the typed text.
//
// The rule: Enter/Tab accept a suggestion only if the user moved the
// highlight onto it with the arrow keys since they last typed (chosen), or
// clicked it. Otherwise the drop-down is closed first and the key does its
// normal job (the prompt's done func, the form's "next field") on the
// typed text. Choosing the scan entry calls onScan; its text never goes
// into the field.
type jmsTypeAutocomplete struct {
	field       *tview.InputField
	suggest     func(currentText string) []string
	onScan      func()
	chosen      bool   // highlight moved with the arrow keys since the last text change
	highlighted string // the entry the arrow keys last moved to (tests navigate by it)
	suppress    bool   // makes suggestions return nothing, to close the drop-down
}

// newJMSTypeAutocomplete wires field's autocomplete with suggest as the
// suggestion source and onScan as the scan entry's action. The caller
// styles the field first (ui.StyleInputFieldAutocomplete must run before
// SetAutocompleteFunc — tui/CLAUDE.md) and must not set the field's own
// changed func or input capture afterwards, which this owns.
func newJMSTypeAutocomplete(field *tview.InputField, suggest func(string) []string, onScan func()) *jmsTypeAutocomplete {
	ac := &jmsTypeAutocomplete{field: field, suggest: suggest, onScan: onScan}
	field.SetAutocompleteFunc(ac.suggestions)
	field.SetAutocompletedFunc(ac.autocompleted)
	field.SetChangedFunc(func(string) { ac.chosen = false })
	field.SetInputCapture(ac.capture)
	return ac
}

// reset forgets any arrow-key choice, for when the dialog (re)opens.
func (ac *jmsTypeAutocomplete) reset() { ac.chosen = false }

func (ac *jmsTypeAutocomplete) suggestions(currentText string) []string {
	if ac.suppress {
		return nil
	}
	return ac.suggest(currentText)
}

// autocompleted is tview's callback for a drop-down selection. Navigation
// only records the choice and leaves the text alone: tview rebuilds the
// drop-down only when a key changed the text, so it stays open. Enter/Tab
// get here only when chosen (see capture); a click is deliberate anyway.
func (ac *jmsTypeAutocomplete) autocompleted(text string, _ int, source int) bool {
	if source == tview.AutocompletedNavigate {
		ac.chosen = true
		ac.highlighted = text
		return false
	}
	if text == jmsTypeScanSentinel {
		ac.onScan()
	} else {
		ac.field.SetText(text) // tview leaves setting the text to the callback
	}
	return true
}

// capture runs before tview's own key handling. On Enter/Tab with nothing
// chosen it closes the drop-down — tview has no public "close", but
// Autocomplete() with no entries removes it — so the key reaches the
// field's normal handling with the typed text untouched. A no-op when no
// drop-down is open.
func (ac *jmsTypeAutocomplete) capture(event *tcell.EventKey) *tcell.EventKey {
	if (event.Key() == tcell.KeyEnter || event.Key() == tcell.KeyTab) && !ac.chosen {
		ac.suppress = true
		ac.field.Autocomplete()
		ac.suppress = false
	}
	return event
}
