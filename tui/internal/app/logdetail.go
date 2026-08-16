package app

import (
	"fmt"
	"strings"

	"github.com/gdamore/tcell/v2"
	"github.com/rivo/tview"

	"github.com/ePex/cloudtui/tui/internal/awslogs"
	"github.com/ePex/cloudtui/tui/internal/ui"
)

// logDetailView shows the full detail of a single CloudWatch Logs event.
// It is not a registered ui.View; it is opened via App.openLogEventDetail
// and returns to the search view on Esc/Backspace. Unlike
// paramDetailView/secretDetailView, nothing here is masked — a log event
// is never a secret in the AWS-service sense — so 'c' is always
// available, no reveal-gating needed.
type logDetailView struct {
	textView *tview.TextView
	app      *App
	event    awslogs.LogEvent
}

func (dv *logDetailView) Shortcuts() []ui.Shortcut {
	return []ui.Shortcut{
		{Key: "c", Description: "copy message"},
		{Key: "Esc", Description: "back"},
	}
}

func newLogDetailView(a *App) *logDetailView {
	tv := tview.NewTextView()
	tv.SetBorder(true).SetTitle(" Log Event ")
	tv.SetDynamicColors(true)
	tv.SetScrollable(true)
	tv.SetWrap(true)

	dv := &logDetailView{textView: tv, app: a}

	tv.SetInputCapture(func(event *tcell.EventKey) *tcell.EventKey {
		switch {
		case event.Rune() == 'j':
			return tcell.NewEventKey(tcell.KeyDown, 0, tcell.ModNone)
		case event.Rune() == 'k':
			return tcell.NewEventKey(tcell.KeyUp, 0, tcell.ModNone)
		case event.Rune() == 'c':
			dv.app.copyToClipboard(dv.event.Message)
			dv.app.statusBar.SetText("Copied log message to clipboard")
			return nil
		case event.Key() == tcell.KeyEscape, event.Key() == tcell.KeyBackspace, event.Key() == tcell.KeyBackspace2:
			a.pages.SwitchToPage("log-search")
			a.tv.SetFocus(a.logSearchV.table)
			// logSearchView isn't a registered ui.View (opened directly,
			// like messagesView), so it can't go through the generic
			// updateContextPanel(ui.View) path — same manual pattern
			// openLogSearch uses.
			lines := make([]string, 0, len(a.logSearchV.Shortcuts()))
			for _, sc := range a.logSearchV.Shortcuts() {
				lines = append(lines, fmt.Sprintf("[%s]<%s>[-] %s", a.cfg.Colors.Accent, sc.Key, sc.Description))
			}
			a.contextPanel.SetText(strings.Join(lines, "\n"))
			return nil
		}
		return event
	})

	return dv
}

// render displays event's detail.
func (dv *logDetailView) render(event awslogs.LogEvent) {
	dv.event = event
	p := dv.app.cfg.Colors
	accent, text := p.Label, p.Text

	var b strings.Builder
	line := func(label, value string) {
		fmt.Fprintf(&b, "[%s]%s:[-] [%s]%s[-]\n", accent, label, text, tview.Escape(value))
	}
	line("Timestamp", event.Timestamp.Local().Format("2006-01-02 15:04:05"))
	line("Log Stream", event.LogStream)

	fmt.Fprintf(&b, "\n[%s]Message:[-]\n[%s]%s[-]", accent, text, tview.Escape(event.Message))

	dv.textView.SetText(b.String())
	dv.textView.ScrollToBeginning()
	dv.refreshContextPanel()
}

func (dv *logDetailView) refreshContextPanel() {
	lines := make([]string, 0, len(dv.Shortcuts()))
	for _, sc := range dv.Shortcuts() {
		lines = append(lines, fmt.Sprintf("[%s]<%s>[-] %s", dv.app.cfg.Colors.Accent, sc.Key, sc.Description))
	}
	dv.app.contextPanel.SetText(strings.Join(lines, "\n"))
}

// openLogEventDetail renders the full detail for event and switches to
// the log-event-detail page.
func (a *App) openLogEventDetail(event awslogs.LogEvent) {
	a.logDetailV.render(event)
	a.pages.SwitchToPage("log-event-detail")
	a.tv.SetFocus(a.pages)
}

// wireLogSearchOpensEventDetail wires Enter in the log search results
// table to open the detail view for the selected event. Called from
// New() once logDetailV exists.
func (a *App) wireLogSearchOpensEventDetail() {
	a.logSearchV.table.SetSelectedFunc(func(row, _ int) {
		idx := row - 1 // row 0 is the header
		if idx < 0 || idx >= len(a.logSearchV.results) {
			return
		}
		a.openLogEventDetail(a.logSearchV.results[idx])
	})
}
