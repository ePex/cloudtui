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
// that triggers the opt-in scan. Detected in onJMSTypeChanged rather than
// via tview.InputField.SetAutocompletedFunc — that API loses tview's
// built-in dodge around re-triggering Autocomplete() on every arrow-key
// navigation (a private variable this package can't reach), which risks
// the suggestion list collapsing after the first arrow press. Reusing
// SetChangedFunc (already how this codebase's other filter inputs get
// live behavior) avoids that risk, at the cost of the field very briefly
// showing this literal text before onJMSTypeChanged clears it back.
var jmsTypeScanSentinel = fmt.Sprintf("↻ Scan up to %d messages for JMS types", jmsTypeScanCount)

// MessageFilter is the server-side message filter overlay (FE 46) — the
// counterpart to messagesView's quick search: JMS type + date range + max
// count, applied by the backend rather than client-side.
type MessageFilter struct {
	host        ui.Host
	form        *tview.Form
	jmsTypeItem *tview.InputField
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
	// StyleInputFieldAutocomplete must run before SetAutocompleteFunc —
	// see tui/internal/app/app.go's identical ordering requirement for
	// the ':' prompt (SetAutocompleteFunc eagerly builds and permanently
	// styles the drop-down's internal list from whatever styles are set
	// at that exact moment).
	ui.StyleInputFieldAutocomplete(mf.jmsTypeItem, host.Config().Colors)
	mf.jmsTypeItem.SetAutocompleteFunc(mf.jmsTypeSuggestions)
	mf.jmsTypeItem.SetChangedFunc(mf.onJMSTypeChanged)

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

// onJMSTypeChanged detects the scan-trigger sentinel and starts a scan;
// any other change is the field's own text changing normally, which
// needs no action here (tview.InputField refreshes its own suggestions).
func (mf *MessageFilter) onJMSTypeChanged(text string) {
	if text == jmsTypeScanSentinel {
		mf.jmsTypeItem.SetText("")
		mf.startScan()
	}
}

// startScan runs the opt-in JMS Type scan (see jmsTypeScanCount) and
// merges the result into future suggestions for the rest of this dialog
// session. No-ops if a scan is already in flight.
func (mf *MessageFilter) startScan() {
	if mf.scanning {
		return
	}
	mf.scanning = true
	mf.host.SetStatus(fmt.Sprintf("Scanning up to %d messages for JMS types...", jmsTypeScanCount))
	go func() {
		types, err := mf.host.ScanJMSTypes(context.Background(), jmsTypeScanCount)
		mf.host.QueueUpdateDraw(func() {
			mf.handleScanResult(types, err)
		})
	}()
}

// handleScanResult applies a completed scan's outcome — split out from
// startScan's goroutine so it can be called directly in a test, the same
// way this codebase's view.repaint()/showError() methods are tested
// without ever running their own load() goroutine (see
// ssmparams_test.go's TestSSMParamsViewLoadErrorsWithoutActiveProfile doc
// comment).
func (mf *MessageFilter) handleScanResult(types []string, err error) {
	mf.scanning = false
	if err != nil {
		mf.host.SetStatus(fmt.Sprintf("[red]JMS type scan failed: %s[-]", err))
		return
	}
	mf.scanned = types
	mf.host.SetStatus("")
	mf.jmsTypeItem.Autocomplete()
}

// Show opens the overlay, prefilled from messagesV's currently-applied
// filter.
func (mf *MessageFilter) Show() {
	mf.scanned = nil
	mf.scanning = false

	f := mf.host.MessagesFilter()
	mf.form.GetFormItem(0).(*tview.InputField).SetText(f.JMSType)
	mf.form.GetFormItem(1).(*tview.InputField).SetText(formatFilterDate(f.FromDate))
	mf.form.GetFormItem(2).(*tview.InputField).SetText(formatFilterDate(f.ToDate))
	maxCount := ""
	if f.MaxCount > 0 {
		maxCount = fmt.Sprintf("%d", f.MaxCount)
	}
	mf.form.GetFormItem(3).(*tview.InputField).SetText(maxCount)

	mf.host.ShowPage("message-filter")
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
	ui.StyleInputFieldAutocomplete(mf.jmsTypeItem, p)
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
	jmsType := mf.form.GetFormItem(0).(*tview.InputField).GetText()
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
