package app

import (
	"fmt"
	"regexp"
	"strings"

	"github.com/gdamore/tcell/v2"
	"github.com/rivo/tview"

	"github.com/ePex/cloudtui/tui/internal/datadoglogs"
	"github.com/ePex/cloudtui/tui/internal/ui"
)

// datadogLogDetailView shows the full detail of a single Datadog log
// event. Not a registered ui.View; opened via App.openDatadogLogDetail
// and returns to the search view on Esc/Backspace. Nothing here is
// masked — a log event is never a secret in the AWS-service sense — so
// 'c' is always available, no reveal-gating needed (same as
// logDetailView).
type datadogLogDetailView struct {
	textView *tview.TextView
	app      *App
	event    datadoglogs.LogEvent
}

func (dv *datadogLogDetailView) Shortcuts() []ui.Shortcut {
	return []ui.Shortcut{
		{Key: "c", Description: "copy message"},
		{Key: "g", Description: "go to CloudWatch"},
		{Key: "Esc", Description: "back"},
	}
}

// correlationIDPattern matches the confirmed real Datadog log shape
// "CorrelationID: <uuid>" (label case-insensitive, UUID shape strict so
// trailing punctuation/words in the message aren't swept in) — see
// spec/41-fe-datadog-cloudwatch-correlation-jump.
var correlationIDPattern = regexp.MustCompile(`(?i)correlationid:\s*([0-9a-f]{8}-[0-9a-f]{4}-[0-9a-f]{4}-[0-9a-f]{4}-[0-9a-f]{12})`)

// extractCorrelationID pulls a "CorrelationID: <uuid>" value out of a
// Datadog log message. Returns ("", false) if the message doesn't
// contain one.
func extractCorrelationID(message string) (string, bool) {
	m := correlationIDPattern.FindStringSubmatch(message)
	if m == nil {
		return "", false
	}
	return m[1], true
}

func newDatadogLogDetailView(a *App) *datadogLogDetailView {
	tv := tview.NewTextView()
	tv.SetBorder(true).SetTitle(" Datadog Log Event ")
	tv.SetDynamicColors(true)
	tv.SetScrollable(true)
	tv.SetWrap(true)

	dv := &datadogLogDetailView{textView: tv, app: a}

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
		case event.Rune() == 'g':
			id, ok := extractCorrelationID(dv.event.Message)
			if !ok {
				dv.app.statusBar.SetText("[yellow]No CorrelationID found in this log message[-]")
				return nil
			}
			dv.app.pendingCloudWatchPattern = id
			dv.app.switchTo("cloudwatch-logs")
			return nil
		case event.Key() == tcell.KeyEscape, event.Key() == tcell.KeyBackspace, event.Key() == tcell.KeyBackspace2:
			a.pages.SwitchToPage("datadog-logs")
			a.tv.SetFocus(a.datadogLogsV.table)
			a.updateContextPanel(a.datadogLogsV)
			return nil
		}
		return event
	})

	return dv
}

// render displays event's detail.
func (dv *datadogLogDetailView) render(event datadoglogs.LogEvent) {
	dv.event = event
	p := dv.app.cfg.Colors
	accent, text := p.Label, p.Text

	var b strings.Builder
	line := func(label, value string) {
		fmt.Fprintf(&b, "[%s]%s:[-] [%s]%s[-]\n", accent, label, text, tview.Escape(value))
	}
	line("Timestamp", event.Timestamp.Local().Format("2006-01-02 15:04:05"))
	line("Service", event.Service)
	line("Status", event.Status)
	line("Host", event.Host)
	line("Tags", strings.Join(event.Tags, ", "))

	fmt.Fprintf(&b, "\n[%s]Message:[-]\n[%s]%s[-]", accent, text, tview.Escape(event.Message))

	dv.textView.SetText(b.String())
	dv.textView.ScrollToBeginning()
	dv.refreshContextPanel()
}

func (dv *datadogLogDetailView) refreshContextPanel() {
	lines := make([]string, 0, len(dv.Shortcuts()))
	for _, sc := range dv.Shortcuts() {
		lines = append(lines, fmt.Sprintf("[%s]<%s>[-] %s", dv.app.cfg.Colors.Accent, sc.Key, sc.Description))
	}
	dv.app.contextPanel.SetText(strings.Join(lines, "\n"))
}
