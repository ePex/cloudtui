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

// jmsTypeAutoScanCount is how many messages Show() scans automatically,
// every time the prompt opens, to populate real JMS Type suggestions
// without requiring any action from the user. Smaller than
// jmsTypeScanCount (the opt-in "widen the search" scan triggered by
// selecting the sentinel) since this one runs unconditionally on every
// 'p'/'M' press — even when the user only wants to skip filtering
// entirely — so it's sized to be cheap and fast rather than exhaustive.
const jmsTypeAutoScanCount = 500

// JMSTypePrompt is the optional JMS Type filter step shown before purge
// and move-all in the Queues view (spec/09) — a single bordered
// tview.InputField, reused by both actions since they differ only in
// what happens once a type (or none) is chosen.
//
// Its autocomplete mirrors MessageFilter's JMS Type field (spec/08)
// closely (same styling, same jmsTypeAutocomplete wiring), but with no
// free tier: unlike the Messages view, no messages for the queue
// being purged/moved have necessarily been browsed yet, so there is
// nothing already-loaded to suggest from without a network call. Instead,
// Show() kicks off a small, automatic scan every time the prompt opens
// (see jmsTypeAutoScanCount) — found live: without this, the only
// visible suggestion on a fresh field was the scan-trigger sentinel
// itself, and it wasn't obvious that selecting it was the way to see any
// real type names at all, reading as "nothing is here" rather than "type
// something, or press Enter to skip." The sentinel (jmsTypeScanSentinel,
// shared with MessageFilter) still appears in the suggestion list as an
// opt-in way to widen the search further (jmsTypeScanCount, larger than
// the automatic scan's cap) if the automatic pass didn't surface the
// type wanted.
type JMSTypePrompt struct {
	host       ui.Host
	field      *tview.InputField
	ac         *jmsTypeAutocomplete
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
	// Enter/Tab only accept a suggestion the user chose with the arrow
	// keys; otherwise Enter reaches the done func below with the typed
	// text (blank = no filter) — see jmsTypeAutocomplete.
	jp.ac = newJMSTypeAutocomplete(jp.field, jp.jmsTypeSuggestions, func() { jp.startScan(jmsTypeScanCount) })
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
	jp.ac.reset()
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

	jp.startScan(jmsTypeAutoScanCount)
}

// jmsTypeSuggestions returns any types a completed scan found
// (auto-triggered by Show, or the opt-in wider one — see
// jmstypeprompt's own doc comment), prefix-filtered by currentText,
// followed by the scan-trigger sentinel — which is always present
// regardless of currentText, since it's an action (widen the search),
// not a data suggestion.
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

// startScan runs a JMS Type scan against queueName capped at maxCount —
// either Show's automatic pass (jmsTypeAutoScanCount) or the scan entry's
// opt-in wider one (jmsTypeScanCount, via jmsTypeAutocomplete's onScan). No-ops if a scan is already in
// flight, so selecting the sentinel while the automatic scan is still
// running (a narrow timing window in practice) just waits for whichever
// scan is already in progress rather than piling up a second request.
func (jp *JMSTypePrompt) startScan(maxCount int) {
	if jp.scanning {
		return
	}
	jp.scanning = true
	jp.host.SetStatus(fmt.Sprintf("Scanning up to %d messages for JMS types...", maxCount))
	// Read on this (UI) goroutine: Show writes queueName, and the prompt
	// can be reopened before the scan goroutine gets to run.
	queueName := jp.queueName
	go func() {
		scan, err := jp.host.ScanJMSTypes(context.Background(), queueName, maxCount)
		jp.host.QueueUpdateDraw(func() {
			jp.handleScanResult(scan, err)
		})
	}()
}

// handleScanResult applies a completed scan's outcome — split out from
// startScan's goroutine so it can be called directly in a test, the same
// way MessageFilter's is (see its doc comment). Applies equally whether
// the completed scan was Show's automatic one or the sentinel's opt-in
// wider one — the caller doesn't need to distinguish which.
//
// It leaves the field's text alone: whatever the user typed while the
// scan ran stays, and the new types become suggestions for it. (The scan
// entry's own text never goes into the field — jmsTypeAutocomplete.)
func (jp *JMSTypePrompt) handleScanResult(scan ui.JMSTypeScan, err error) {
	jp.scanning = false
	if err != nil {
		jp.host.SetStatus(fmt.Sprintf("[red]JMS type scan failed: %s[-]", err))
		return
	}
	jp.scanned = scan.Types
	jp.host.SetStatus(scanStatus(scan))
	jp.field.Autocomplete()
}

// continueNow reads the field, closes the prompt, and calls onContinue —
// even while a scan is still running: the field only ever holds what the
// user typed (blank = no filter), never the scan entry.
func (jp *JMSTypePrompt) continueNow() {
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
	ui.StyleFilterInput(jp.field, p)
	ui.StyleInputFieldAutocomplete(jp.field, p)
}

var _ ui.Themeable = (*JMSTypePrompt)(nil)

// Primitive returns JMSTypePrompt's root widget, for sizing/embedding.
func (jp *JMSTypePrompt) Primitive() tview.Primitive { return jp.field }

// Visible reports whether JMSTypePrompt is currently shown.
func (jp *JMSTypePrompt) Visible() bool { return jp.visible }
