package dialog

import (
	"context"
	"fmt"
	"strings"
	"time"

	"github.com/gdamore/tcell/v2"
	"github.com/rivo/tview"

	"github.com/ePex/cloudtui/tui/internal/config"
	"github.com/ePex/cloudtui/tui/internal/queue"
	"github.com/ePex/cloudtui/tui/internal/ui"
)

// jmsTypeScanCount is how many messages the opt-in "scan for more JMS
// types" suggestion browses. Deliberately fixed, not user-configurable —
// see the "Known limitation" note on spec/08: neither backend can be
// asked for a truly unbounded/complete scan (mq-proxy's list-messages
// requires a positive maxCount on every call; an unbounded Jolokia fetch
// risks real latency on a very large queue), so this can only ever widen
// the sample, never guarantee completeness.
const jmsTypeScanCount = 5000

// jmsTypeScanSentinel is the always-present, non-data suggestion entry
// that triggers the opt-in scan — only when the user chooses it
// deliberately (jmsTypeAutocomplete); its text never goes into the field.
var jmsTypeScanSentinel = fmt.Sprintf("↻ Scan up to %d messages for JMS types", jmsTypeScanCount)

// scanStatus is the status line after a successful JMS Type scan, shared
// by MessageFilter and JMSTypePrompt: empty when the scan saw the whole
// queue, otherwise a note that types further back aren't listed — the
// broker stops browsing at its browse limit (400 by default), so a scan
// can end well short of what it asked for.
func scanStatus(scan ui.JMSTypeScan) string {
	if !scan.HasMore {
		return ""
	}
	return fmt.Sprintf("Scanned the first %d messages — the queue holds more; types further back aren't listed", scan.Scanned)
}

// MessageFilter is the server-side message filter overlay (FE 46) — the
// counterpart to messagesView's quick search: JMS type + date range + max
// count, applied by the backend rather than client-side.
type MessageFilter struct {
	host        ui.Host
	form        *tview.Form
	jmsTypeItem *tview.InputField
	ac          *jmsTypeAutocomplete
	visible     bool
	scanned     []string // extra JMS types found by the last scan this dialog session; nil until a scan completes
	scanning    bool     // true while a scan is in flight, to ignore a duplicate trigger
}

// NewMessageFilter builds the message filter overlay's form.
func NewMessageFilter(host ui.Host) *MessageFilter {
	mf := &MessageFilter{host: host}
	mf.form = tview.NewForm()
	mf.form.SetBorder(true).SetTitle(" Message Filter ")
	mf.form.
		AddInputField("JMS Type", "", 30, nil, nil).
		AddInputField("From (RFC3339 or YYYY-MM-DD)", "", 30, nil, nil).
		AddInputField("To (RFC3339 or YYYY-MM-DD)", "", 30, nil, nil).
		AddInputField("Max Count", "", 10, nil, nil).
		AddButton("Apply", func() { mf.apply() }).
		AddButton("Clear", func() { mf.clear() }).
		AddButton("Cancel", func() { mf.close() })
	mf.form.SetInputCapture(func(event *tcell.EventKey) *tcell.EventKey {
		if event.Key() == tcell.KeyEscape {
			mf.close()
			return nil
		}
		return event
	})

	mf.jmsTypeItem = mf.form.GetFormItem(0).(*tview.InputField)
	// Enter/Tab only accept a suggestion the user chose with the arrow
	// keys, and only that one is highlighted; otherwise they move on in
	// the form with the typed text — see jmsTypeAutocomplete, which also
	// styles the drop-down.
	mf.ac = newJMSTypeAutocomplete(mf.jmsTypeItem, host.Config().Colors, mf.jmsTypeSuggestions, mf.startScan)

	return mf
}

// jmsTypeSuggestions returns the ':'-prompt-style autocomplete entries for
// the JMS Type field: distinct types from currently-loaded messages plus
// any found by a completed scan this dialog session, prefix-filtered by
// currentText, followed by the scan-trigger sentinel — which is always
// present regardless of currentText, since it's an action, not a data
// suggestion.
func (mf *MessageFilter) jmsTypeSuggestions(currentText string) []string {
	seen := make(map[string]bool)
	var matches []string
	add := func(s string) {
		if !seen[s] {
			seen[s] = true
			matches = append(matches, s)
		}
	}
	for _, t := range mf.host.LoadedJMSTypes() {
		if strings.HasPrefix(t, currentText) {
			add(t)
		}
	}
	for _, t := range mf.scanned {
		if strings.HasPrefix(t, currentText) {
			add(t)
		}
	}
	matches = append(matches, jmsTypeScanSentinel)
	return matches
}

// startScan runs the opt-in JMS Type scan (see jmsTypeScanCount) — when
// the user chooses the scan entry (jmsTypeAutocomplete's onScan) — and
// merges the result into future suggestions for the rest of this dialog
// session. No-ops if a scan is already in flight.
func (mf *MessageFilter) startScan() {
	if mf.scanning {
		return
	}
	mf.scanning = true
	mf.host.SetStatus(fmt.Sprintf("Scanning up to %d messages for JMS types...", jmsTypeScanCount))
	// Read on this (UI) goroutine, not the scan's — the open queue can
	// change while the scan goroutine waits to run.
	queueName := mf.host.MessagesQueueName()
	go func() {
		scan, err := mf.host.ScanJMSTypes(context.Background(), queueName, jmsTypeScanCount)
		mf.host.QueueUpdateDraw(func() {
			mf.handleScanResult(scan, err)
		})
	}()
}

// handleScanResult applies a completed scan's outcome — split out from
// startScan's goroutine so it can be called directly in a test, the same
// way this codebase's view.repaint()/showError() methods are tested
// without ever running their own load() goroutine (see
// ssmparams_test.go's TestSSMParamsViewLoadErrorsWithoutActiveProfile doc
// comment). The field's text is left alone — the scan entry's text never
// goes into it (jmsTypeAutocomplete), so whatever is there was typed.
func (mf *MessageFilter) handleScanResult(scan ui.JMSTypeScan, err error) {
	mf.scanning = false
	if err != nil {
		mf.host.SetStatus(fmt.Sprintf("[red]JMS type scan failed: %s[-]", err))
		return
	}
	mf.scanned = scan.Types
	mf.host.SetStatus(scanStatus(scan))
	mf.jmsTypeItem.Autocomplete()
}

// Show opens the overlay, prefilled from messagesV's currently-applied
// filter.
func (mf *MessageFilter) Show() {
	mf.scanned = nil
	mf.scanning = false
	mf.ac.reset()

	f := mf.host.MessagesFilter()
	ui.SetInputFieldText(mf.jmsTypeItem, f.JMSType)
	// SetText doesn't itself refresh an active SetAutocompleteFunc
	// drop-down (only a live keystroke does) — without this, the field
	// shows whatever suggestions were current at SetAutocompleteFunc's
	// own wiring time in NewMessageFilter (before messagesV existed, so
	// empty aside from the sentinel), same gotcha as the ':' prompt (see
	// spec/01) and its fix.
	mf.jmsTypeItem.Autocomplete()
	ui.SetInputFieldText(mf.form.GetFormItem(1).(*tview.InputField), formatFilterDate(f.FromDate))
	ui.SetInputFieldText(mf.form.GetFormItem(2).(*tview.InputField), formatFilterDate(f.ToDate))
	maxCount := ""
	if f.MaxCount > 0 {
		maxCount = fmt.Sprintf("%d", f.MaxCount)
	}
	ui.SetInputFieldText(mf.form.GetFormItem(3).(*tview.InputField), maxCount)

	mf.host.ShowPage("message-filter")
	// tview.Form remembers its focused item across Show calls; without
	// this it reopens wherever it was left, e.g. on Apply.
	mf.form.SetFocus(0)
	mf.host.SetFocus(mf.form)
	mf.visible = true
}

// formatFilterDate renders t for the filter form's From/To fields, or ""
// for a zero value (unset).
func formatFilterDate(t time.Time) string {
	if t.IsZero() {
		return ""
	}
	return t.Format("2006-01-02")
}

// close hides the overlay and returns focus to the messages table,
// without changing the active filter.
func (mf *MessageFilter) close() {
	mf.host.HidePage("message-filter")
	mf.visible = false
	mf.host.FocusMessages()
}

// ApplyPalette recolors the message filter overlay for a live theme switch.
func (mf *MessageFilter) ApplyPalette(p config.Palette) {
	mf.form.SetBackgroundColor(tcell.GetColor(p.Background))
	mf.form.SetBorderColor(tcell.GetColor(p.Border))
	mf.form.SetTitleColor(tcell.GetColor(p.Border))
	ui.StyleForm(mf.form, p)
	mf.ac.applyPalette(p)
}

var _ ui.Themeable = (*MessageFilter)(nil)

// Primitive returns MessageFilter's root widget, for sizing/embedding.
func (mf *MessageFilter) Primitive() tview.Primitive { return mf.form }

// Visible reports whether MessageFilter is currently shown.
func (mf *MessageFilter) Visible() bool { return mf.visible }

// apply parses the form's fields, and — on success — sets it as
// messagesV's active filter, closes the overlay, and reloads. On a parse
// error, the status bar reports it and the form stays open for correction.
func (mf *MessageFilter) apply() {
	jmsType := mf.jmsTypeItem.GetText()
	from := mf.form.GetFormItem(1).(*tview.InputField).GetText()
	to := mf.form.GetFormItem(2).(*tview.InputField).GetText()
	maxCount := mf.form.GetFormItem(3).(*tview.InputField).GetText()

	filter, err := ui.ParseMessageFilterForm(jmsType, from, to, maxCount)
	if err != nil {
		mf.host.SetStatus(fmt.Sprintf("[red]%s[-]", err))
		return
	}

	mf.host.ApplyMessagesFilter(filter)
	mf.close()
}

// clear resets the form and messagesV's active filter, closes the
// overlay, and reloads.
func (mf *MessageFilter) clear() {
	for i := 0; i < 4; i++ {
		mf.form.GetFormItem(i).(*tview.InputField).SetText("")
	}
	mf.host.ApplyMessagesFilter(queue.MessageFilter{})
	mf.close()
}
