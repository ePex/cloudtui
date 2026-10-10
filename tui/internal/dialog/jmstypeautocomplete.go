package dialog

import (
	"strings"

	"github.com/gdamore/tcell/v2"
	"github.com/rivo/tview"

	"github.com/ePex/cloudtui/tui/internal/config"
	"github.com/ePex/cloudtui/tui/internal/ui"
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
//
// The drop-down's look follows the same rule: tview always has a current
// entry and draws it in the autocomplete's selected style, so while
// nothing is chosen that style is the plain row style, and no entry looks
// selected (see applyStyles). Only the first arrow key shows the
// highlight, and any other key but Enter/Tab removes it (see capture).
type jmsTypeAutocomplete struct {
	field       *tview.InputField
	palette     config.Palette
	suggest     func(currentText string) []string
	onScan      func()
	chosen      bool   // an entry chosen with the arrow keys since the last keystroke
	highlighted string // the entry the arrow keys last moved to (tests navigate by it)
	suppress    bool   // makes suggestions return nothing, to close the drop-down
}

// newJMSTypeAutocomplete wires field's autocomplete with suggest as the
// suggestion source and onScan as the scan entry's action, styled from p.
// It styles the drop-down before SetAutocompleteFunc builds it
// (tui/CLAUDE.md, "style first, then wire"). The caller must not set the
// field's own changed func, input capture or autocomplete styles
// afterwards, which this owns; a live theme switch goes through
// applyPalette.
func newJMSTypeAutocomplete(field *tview.InputField, p config.Palette, suggest func(string) []string, onScan func()) *jmsTypeAutocomplete {
	ac := &jmsTypeAutocomplete{field: field, palette: p, suggest: suggest, onScan: onScan}
	ac.applyStyles()
	field.SetAutocompleteFunc(ac.suggestions)
	field.SetAutocompletedFunc(ac.autocompleted)
	field.SetChangedFunc(func(string) { ac.textChanged() })
	field.SetInputCapture(ac.capture)
	return ac
}

// reset forgets any arrow-key choice, for when the dialog (re)opens, and
// drops the cached drop-down so the next one is built without a
// highlight.
func (ac *jmsTypeAutocomplete) reset() {
	ac.chosen = false
	ac.applyStyles()
	ac.closeDropDown()
}

// applyPalette restyles the drop-down for a live theme switch, keeping
// its current look (plain or chosen). Like ui.StyleInputFieldAutocomplete,
// it blurs an unfocused field to drop the cached drop-down, which tview
// built with the old styles; a focused one is left alone.
func (ac *jmsTypeAutocomplete) applyPalette(p config.Palette) {
	ac.palette = p
	ac.applyStyles()
	if !ac.field.HasFocus() {
		ac.field.Blur()
	}
}

// applyStyles sets the field's autocomplete styles for the current state:
// the theme's selection colors for the current entry only when chosen,
// otherwise the plain row style, so no entry looks selected. tview copies
// these into the drop-down only when it builds one, so this reaches the
// next drop-down, not an open one.
func (ac *jmsTypeAutocomplete) applyStyles() {
	background, main, selected := ui.AutocompleteStyles(ac.palette)
	if !ac.chosen {
		selected = main
	}
	ac.field.SetAutocompleteStyles(background, main, selected)
}

// textChanged is the field's changed func: any text change (typing, or
// SetText) drops the choice and its look. It runs inside tview's key
// handling, which holds the drop-down's lock, so it can't rebuild the
// drop-down itself; capture already did for keys, and the next drop-down
// is built plain.
func (ac *jmsTypeAutocomplete) textChanged() {
	ac.chosen = false
	ac.applyStyles()
}

// closeDropDown removes the drop-down, if one is open: tview has no
// public "close", but Autocomplete() with no entries removes it.
func (ac *jmsTypeAutocomplete) closeDropDown() {
	ac.suppress = true
	ac.field.Autocomplete()
	ac.suppress = false
}

// setChosen switches between the plain and the chosen look. tview copies
// the styles only into a new drop-down, so this rebuilds an open one (and
// opens one if there are suggestions); the rebuilt drop-down's current
// entry is chosen by tview's rule, see firstMatch. Never call it from the
// changed func or anything else running inside tview's key handling.
func (ac *jmsTypeAutocomplete) setChosen(chosen bool) {
	ac.chosen = chosen
	ac.applyStyles()
	ac.closeDropDown()
	ac.field.Autocomplete()
}

// firstMatch returns the entry a new drop-down starts on, by tview v0.42's
// rule: the first entry starting with text, else the first entry.
func firstMatch(entries []string, text string) string {
	for _, e := range entries {
		if strings.HasPrefix(e, text) {
			return e
		}
	}
	return entries[0]
}

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
// Accepting uses up the choice: tview drops the drop-down, and the next
// one is built plain (the scan entry leaves the text alone, so the
// changed func doesn't do this for it).
func (ac *jmsTypeAutocomplete) autocompleted(text string, _ int, source int) bool {
	if source == tview.AutocompletedNavigate {
		ac.chosen = true
		ac.highlighted = text
		return false
	}
	ac.chosen = false
	ac.applyStyles()
	if text == jmsTypeScanSentinel {
		ac.onScan()
	} else {
		ac.field.SetText(text) // tview leaves setting the text to the callback
	}
	return true
}

// capture runs before tview's own key handling, outside its lock.
//
// On Enter/Tab with nothing chosen it closes the drop-down, so the key
// reaches the field's normal handling with the typed text untouched. A
// no-op when no drop-down is open.
//
// The first arrow key (or PgUp/PgDn) with nothing chosen chooses the
// drop-down's current entry instead of moving it: it shows the highlight
// on that entry (opening the drop-down if it was closed) and swallows the
// key, so what lights up is what tview had current, and Enter takes it.
// This also covers a one-entry list (typically the scan entry alone, when
// there are no types to suggest yet), where the current entry can't move
// and tview reports no navigation. Later arrow keys go to tview, whose
// navigation callback (autocompleted) records the entry they move to.
//
// Any other key while chosen drops the choice and its highlight before
// tview handles it, so typing, Backspace, ←/→ or Esc never leave a
// highlight that Enter would ignore.
func (ac *jmsTypeAutocomplete) capture(event *tcell.EventKey) *tcell.EventKey {
	switch event.Key() {
	case tcell.KeyEnter, tcell.KeyTab:
		if !ac.chosen {
			ac.closeDropDown()
		}
		return event
	case tcell.KeyDown, tcell.KeyUp, tcell.KeyPgDn, tcell.KeyPgUp:
		if ac.chosen {
			return event
		}
		text := ac.field.GetText()
		entries := ac.suggestions(text)
		if len(entries) == 0 {
			return event
		}
		ac.setChosen(true)
		ac.highlighted = firstMatch(entries, text)
		return nil
	}
	if ac.chosen {
		ac.setChosen(false)
	}
	return event
}
