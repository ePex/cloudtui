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
// closely (same styling, same `SetChangedFunc`-based scan handling to
// avoid the reentrant-`SetText` buffer corruption found there), but with
// no free tier: unlike the Messages view, no messages for the queue
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
	// Unlike MessageFilter's JMS Type field (which submits via a separate
	// Apply button, never through the field's own Enter key) or the ':'
	// prompt (whose suggestion list is never just a single always-present
	// action item), an untouched field here can have the scan-trigger
	// sentinel as its sole, already-highlighted drop-down entry (e.g.
	// before Show()'s auto-scan — see below — has completed, or if it
	// found nothing). tview.InputField's own Enter handling accepts
	// whatever's highlighted in an open drop-down before ever reaching
	// SetDoneFunc at all, so pressing Enter on a blank field would
	// otherwise risk accepting that sentinel and kicking off the wider
	// scan instead of continuing with no filter, contrary to Show's
	// documented "blank + Enter proceeds with no filter" contract.
	// Found live (verify-live): with no free, always-available
	// suggestion tier here, this collision is unavoidable without
	// intercepting Enter explicitly. SetInputCapture runs before
	// InputField's own InputHandler (Box's own doc comment), so this
	// only fires when the field is genuinely empty — typing (even just
	// enough to filter suggestions) or navigating into the drop-down
	// with arrows still uses tview's normal accept-then-second-Enter
	// flow, same as every other autocomplete field in this app.
	jp.field.SetInputCapture(func(event *tcell.EventKey) *tcell.EventKey {
		if event.Key() == tcell.KeyEnter && jp.field.GetText() == "" {
			jp.continueNow()
			return nil
		}
		return event
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

// onJMSTypeChanged detects the scan-trigger sentinel and starts the
// wider, opt-in scan. It deliberately does not clear the field's text
// here — see MessageFilter.onJMSTypeChanged's doc comment for why that
// would reentrantly corrupt tview's text buffer; handleScanResult clears
// it instead, from a safe, non-reentrant context.
func (jp *JMSTypePrompt) onJMSTypeChanged(text string) {
	if text == jmsTypeScanSentinel {
		jp.startScan(jmsTypeScanCount)
	}
}

// startScan runs a JMS Type scan against queueName capped at maxCount —
// either Show's automatic pass (jmsTypeAutoScanCount) or the sentinel's
// opt-in wider one (jmsTypeScanCount). No-ops if a scan is already in
// flight, so selecting the sentinel while the automatic scan is still
// running (a narrow timing window in practice) just waits for whichever
// scan is already in progress rather than piling up a second request.
func (jp *JMSTypePrompt) startScan(maxCount int) {
	if jp.scanning {
		return
	}
	jp.scanning = true
	jp.host.SetStatus(fmt.Sprintf("Scanning up to %d messages for JMS types...", maxCount))
	go func() {
		types, err := jp.host.ScanJMSTypes(context.Background(), jp.queueName, maxCount)
		jp.host.QueueUpdateDraw(func() {
			jp.handleScanResult(types, err)
		})
	}()
}

// handleScanResult applies a completed scan's outcome — split out from
// startScan's goroutine so it can be called directly in a test, the same
// way MessageFilter's is (see its doc comment). Applies equally whether
// the completed scan was Show's automatic one or the sentinel's opt-in
// wider one — the caller doesn't need to distinguish which.
func (jp *JMSTypePrompt) handleScanResult(types []string, err error) {
	jp.scanning = false
	// Clears whatever the field was showing when the scan that just
	// completed was started — "" if it was the automatic scan (the
	// field was never touched), or the sentinel's own literal text if it
	// was the opt-in one (see onJMSTypeChanged's doc comment for why
	// that text can only be cleared here, not synchronously).
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
// Refuses only while the field is literally showing the scan-trigger
// sentinel's own text (i.e. the opt-in wider scan was just triggered and
// hasn't completed/cleared the field yet) — that text must never be read
// as a real JMS type. Show's automatic scan never puts the sentinel into
// the field, so it never blocks a blank "continue with no filter" this
// way, however long it's still running.
func (jp *JMSTypePrompt) continueNow() {
	if jp.field.GetText() == jmsTypeScanSentinel {
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
