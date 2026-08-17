package app

import (
	"fmt"
	"time"

	"github.com/gdamore/tcell/v2"
	"github.com/rivo/tview"

	"github.com/ePex/cloudtui/tui/internal/config"
	"github.com/ePex/cloudtui/tui/internal/queue"
	"github.com/ePex/cloudtui/tui/internal/ui"
)

// messageFilter is the server-side message filter overlay (FE 46) — the
// counterpart to messagesView's quick search: JMS type + date range + max
// count, applied by the backend rather than client-side.
type messageFilter struct {
	app     *App
	form    *tview.Form
	visible bool
}

// newMessageFilter builds the message filter overlay's form.
func newMessageFilter(a *App) *messageFilter {
	mf := &messageFilter{app: a}
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
	return mf
}

// show opens the overlay, prefilled from messagesV's currently-applied
// filter.
func (mf *messageFilter) show() {
	mv := mf.app.messagesV
	mf.form.GetFormItem(0).(*tview.InputField).SetText(mv.filter.JMSType)
	mf.form.GetFormItem(1).(*tview.InputField).SetText(formatFilterDate(mv.filter.FromDate))
	mf.form.GetFormItem(2).(*tview.InputField).SetText(formatFilterDate(mv.filter.ToDate))
	maxCount := ""
	if mv.filter.MaxCount > 0 {
		maxCount = fmt.Sprintf("%d", mv.filter.MaxCount)
	}
	mf.form.GetFormItem(3).(*tview.InputField).SetText(maxCount)

	mf.app.rootPages.ShowPage("message-filter")
	mf.app.tv.SetFocus(mf.form)
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
func (mf *messageFilter) close() {
	mf.app.rootPages.HidePage("message-filter")
	mf.visible = false
	mf.app.tv.SetFocus(mf.app.messagesV.table)
}

// ApplyPalette recolors the message filter overlay for a live theme switch.
func (mf *messageFilter) ApplyPalette(p config.Palette) {
	mf.form.SetBackgroundColor(tcell.GetColor(p.Background))
	mf.form.SetBorderColor(tcell.GetColor(p.Border))
	mf.form.SetTitleColor(tcell.GetColor(p.Border))
}

var _ ui.Themeable = (*messageFilter)(nil)

// apply parses the form's fields, and — on success — sets it as
// messagesV's active filter, closes the overlay, and reloads. On a parse
// error, the status bar reports it and the form stays open for correction.
func (mf *messageFilter) apply() {
	a := mf.app
	jmsType := mf.form.GetFormItem(0).(*tview.InputField).GetText()
	from := mf.form.GetFormItem(1).(*tview.InputField).GetText()
	to := mf.form.GetFormItem(2).(*tview.InputField).GetText()
	maxCount := mf.form.GetFormItem(3).(*tview.InputField).GetText()

	filter, err := parseMessageFilterForm(jmsType, from, to, maxCount)
	if err != nil {
		a.statusBar.SetText(fmt.Sprintf("[red]%s[-]", err))
		return
	}

	a.ApplyMessagesFilter(filter)
	mf.close()
}

// clear resets the form and messagesV's active filter, closes the
// overlay, and reloads.
func (mf *messageFilter) clear() {
	for i := 0; i < 4; i++ {
		mf.form.GetFormItem(i).(*tview.InputField).SetText("")
	}
	mf.app.ApplyMessagesFilter(queue.MessageFilter{})
	mf.close()
}
