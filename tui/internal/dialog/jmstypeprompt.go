package dialog

import (
	"context"
	"fmt"
	"strings"

	"github.com/gdamore/tcell/v2"
	"github.com/rivo/tview"

	"github.com/ePex/cloudtui/tui/internal/config"
	"github.com/ePex/cloudtui/tui/internal/ui"
)

// JMSTypePrompt is the optional JMS Type filter step shown before purge
// and move-all in the Queues view (spec/09) — a single bordered
// tview.InputField, reused by both actions since they differ only in
// what happens once a type (or none) is chosen.
//
// Its autocomplete mirrors MessageFilter's JMS Type field (spec/08)
// almost exactly (same styling, same scan-trigger sentinel, same
// SetChangedFunc-based handling to avoid the reentrant-SetText buffer
// corruption found there), but with only the scan tier: unlike the
// Messages view, no messages for the queue being purged/moved have
// necessarily been browsed yet, so there is no free, already-loaded set
// to suggest from — jmsTypeScanSentinel is always the only entry until a
// scan completes.
type JMSTypePrompt struct {
	host       ui.Host
	field      *tview.InputField
	visible    bool
	queueName  string
	scanned    []string
	scanning   bool
	onContinue func(jmsType string)
	onClose    func()
}

// NewJMSTypePrompt builds the prompt's field.
func NewJMSTypePrompt(host ui.Host) *JMSTypePrompt {
	jp := &JMSTypePrompt{host: host}
	jp.field = tview.NewInputField().SetLabel(" JMS Type: ")
	jp.field.SetBorder(true)
	p := host.Config().Colors
	jp.field.SetFieldBackgroundColor(tcell.GetColor(p.SelectionBg))
	jp.field.SetFieldTextColor(tcell.GetColor(p.SelectionText))
	// StyleInputFieldAutocomplete must run before SetAutocompleteFunc —
	// see tui/internal/app/app.go's identical ordering requirement for
	// the ':' prompt (SetAutocompleteFunc eagerly builds and permanently
	// styles the drop-down's internal list from whatever styles are set
	// at that exact moment).
	ui.StyleInputFieldAutocomplete(jp.field, p)
	jp.field.SetAutocompleteFunc(jp.jmsTypeSuggestions)
	jp.field.SetChangedFunc(jp.onJMSTypeChanged)
	jp.field.SetDoneFunc(func(key tcell.Key) {
		switch key {
		case tcell.KeyEnter:
			jp.continueNow()
		case tcell.KeyEscape:
			jp.close()
		}
	})
	return jp
}

// Show opens the prompt for queueName. action names the operation for
// the title (e.g. "Purge", "Move All"). onContinue is called (after
// close()) with the entered JMS Type — an empty string means "no
// filter, proceed as before". onClose is called on every dismissal
// (Enter or Esc), before onContinue if applicable — same contract as
// MovePicker.Show.
func (jp *JMSTypePrompt) Show(action, queueName string, onContinue func(jmsType string), onClose func()) {
	jp.queueName = queueName
	jp.onContinue = onContinue
	jp.onClose = onClose
	jp.scanned = nil
	jp.scanning = false
	jp.field.SetText("")
	jp.field.SetTitle(fmt.Sprintf(" %s %q — JMS Type (optional) ", action, queueName))
	// SetText doesn't itself refresh an active SetAutocompleteFunc
	// drop-down (only a live keystroke does) — see MessageFilter.Show's
	// identical fix for why this matters even though the field starts
	// empty (the drop-down's cached contents were built once, eagerly,
	// at NewJMSTypePrompt's SetAutocompleteFunc call).
	jp.field.Autocomplete()

	jp.host.ShowPage("jmstype-prompt")
	jp.host.SetFocus(jp.field)
	jp.visible = true
}

// jmsTypeSuggestions returns the scan-trigger sentinel, plus any types a
// completed scan found (see jmstypeprompt's own doc comment for why
// there's no free tier here). Merges/prefix-filters the same way
// MessageFilter's do.
func (jp *JMSTypePrompt) jmsTypeSuggestions(currentText string) []string {
	seen := make(map[string]bool)
	var matches []string
	add := func(s string) {
		if !seen[s] {
			seen[s] = true
			matches = append(matches, s)
		}
	}
	for _, t := range jp.scanned {
		if strings.HasPrefix(t, currentText) {
			add(t)
		}
	}
	matches = append(matches, jmsTypeScanSentinel)
	return matches
}

// onJMSTypeChanged detects the scan-trigger sentinel and starts a scan.
// It deliberately does not clear the field's text here — see
// MessageFilter.onJMSTypeChanged's doc comment for why that would
// reentrantly corrupt tview's text buffer; handleScanResult clears it
// instead, from a safe, non-reentrant context.
func (jp *JMSTypePrompt) onJMSTypeChanged(text string) {
	if text == jmsTypeScanSentinel {
		jp.startScan()
	}
}

// startScan runs the opt-in JMS Type scan against queueName. No-ops if a
// scan is already in flight.
func (jp *JMSTypePrompt) startScan() {
	if jp.scanning {
		return
	}
	jp.scanning = true
	jp.host.SetStatus(fmt.Sprintf("Scanning up to %d messages for JMS types...", jmsTypeScanCount))
	go func() {
		types, err := jp.host.ScanJMSTypes(context.Background(), jp.queueName, jmsTypeScanCount)
		jp.host.QueueUpdateDraw(func() {
			jp.handleScanResult(types, err)
		})
	}()
}

// handleScanResult applies a completed scan's outcome — split out from
// startScan's goroutine so it can be called directly in a test, the same
// way MessageFilter's is (see its doc comment).
func (jp *JMSTypePrompt) handleScanResult(types []string, err error) {
	jp.scanning = false
	jp.field.SetText("")
	if err != nil {
		jp.host.SetStatus(fmt.Sprintf("[red]JMS type scan failed: %s[-]", err))
		return
	}
	jp.scanned = types
	jp.host.SetStatus("")
	jp.field.Autocomplete()
}

// continueNow reads the field, closes the prompt, and calls onContinue.
// Refuses while a scan is in flight — the field still holds the
// scan-trigger sentinel's own text for that whole window (see
// onJMSTypeChanged's doc comment), which must never be read as a real
// JMS type.
func (jp *JMSTypePrompt) continueNow() {
	if jp.scanning {
		jp.host.SetStatus("[yellow]Still scanning for JMS types — wait for it to finish[-]")
		return
	}
	jmsType := jp.field.GetText()
	jp.close()
	if jp.onContinue != nil {
		jp.onContinue(jmsType)
	}
}

// close hides the prompt and calls onClose to let the caller restore
// focus and the context panel.
func (jp *JMSTypePrompt) close() {
	jp.host.HidePage("jmstype-prompt")
	jp.visible = false
	if jp.onClose != nil {
		jp.onClose()
	}
}

// ApplyPalette recolors the prompt for a live theme switch.
func (jp *JMSTypePrompt) ApplyPalette(p config.Palette) {
	bg := tcell.GetColor(p.Background)
	jp.field.SetBackgroundColor(bg)
	jp.field.SetBorderColor(tcell.GetColor(p.Border))
	jp.field.SetTitleColor(tcell.GetColor(p.Border))
	jp.field.SetLabelColor(tcell.GetColor(p.Label))
	jp.field.SetFieldBackgroundColor(tcell.GetColor(p.SelectionBg))
	jp.field.SetFieldTextColor(tcell.GetColor(p.SelectionText))
	ui.StyleInputFieldAutocomplete(jp.field, p)
}

var _ ui.Themeable = (*JMSTypePrompt)(nil)

// Primitive returns JMSTypePrompt's root widget, for sizing/embedding.
func (jp *JMSTypePrompt) Primitive() tview.Primitive { return jp.field }

// Visible reports whether JMSTypePrompt is currently shown.
func (jp *JMSTypePrompt) Visible() bool { return jp.visible }
