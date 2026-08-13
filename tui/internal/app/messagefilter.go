package app

import (
	"fmt"
	"time"

	"github.com/rivo/tview"

	"github.com/ePex/cloudtui/tui/internal/queue"
)

// showMessageFilter opens the message filter overlay, prefilled from
// messagesV's currently-applied filter.
func (a *App) showMessageFilter() {
	mv := a.messagesV
	a.messageFilterForm.GetFormItem(0).(*tview.InputField).SetText(mv.filter.JMSType)
	a.messageFilterForm.GetFormItem(1).(*tview.InputField).SetText(formatFilterDate(mv.filter.FromDate))
	a.messageFilterForm.GetFormItem(2).(*tview.InputField).SetText(formatFilterDate(mv.filter.ToDate))
	maxCount := ""
	if mv.filter.MaxCount > 0 {
		maxCount = fmt.Sprintf("%d", mv.filter.MaxCount)
	}
	a.messageFilterForm.GetFormItem(3).(*tview.InputField).SetText(maxCount)

	a.rootPages.ShowPage("message-filter")
	a.tv.SetFocus(a.messageFilterForm)
	a.messageFilterVisible = true
}

// formatFilterDate renders t for the filter form's From/To fields, or ""
// for a zero value (unset).
func formatFilterDate(t time.Time) string {
	if t.IsZero() {
		return ""
	}
	return t.Format("2006-01-02")
}

// closeMessageFilter hides the overlay and returns focus to the messages
// table, without changing the active filter.
func (a *App) closeMessageFilter() {
	a.rootPages.HidePage("message-filter")
	a.messageFilterVisible = false
	a.tv.SetFocus(a.messagesV.table)
}

// applyMessageFilter parses the form's fields, and — on success — sets it as
// messagesV's active filter, closes the overlay, and reloads. On a parse
// error, the status bar reports it and the form stays open for correction.
func (a *App) applyMessageFilter() {
	jmsType := a.messageFilterForm.GetFormItem(0).(*tview.InputField).GetText()
	from := a.messageFilterForm.GetFormItem(1).(*tview.InputField).GetText()
	to := a.messageFilterForm.GetFormItem(2).(*tview.InputField).GetText()
	maxCount := a.messageFilterForm.GetFormItem(3).(*tview.InputField).GetText()

	filter, err := parseMessageFilterForm(jmsType, from, to, maxCount)
	if err != nil {
		a.statusBar.SetText(fmt.Sprintf("[red]%s[-]", err))
		return
	}

	a.messagesV.filter = filter
	a.messagesV.updateTitle()
	a.closeMessageFilter()
	a.messagesV.load()
}

// clearMessageFilter resets the form and messagesV's active filter, closes
// the overlay, and reloads.
func (a *App) clearMessageFilter() {
	for i := 0; i < 4; i++ {
		a.messageFilterForm.GetFormItem(i).(*tview.InputField).SetText("")
	}
	a.messagesV.filter = queue.MessageFilter{}
	a.messagesV.updateTitle()
	a.closeMessageFilter()
	a.messagesV.load()
}
