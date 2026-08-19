package view

import (
	"fmt"
	"regexp"
	"strings"

	"github.com/gdamore/tcell/v2"
	"github.com/rivo/tview"

	"github.com/ePex/cloudtui/tui/internal/config"
	"github.com/ePex/cloudtui/tui/internal/datadoglogs"
	"github.com/ePex/cloudtui/tui/internal/ui"
)

// DatadogLogDetailView shows the full detail of a single Datadog log
// event. Not a registered ui.View; opened via App.OpenDatadogLogDetail
// and returns to the search view on Esc/Backspace. Nothing here is
// masked — a log event is never a secret in the AWS-service sense — so
// 'c' is always available, no reveal-gating needed (same as
// logDetailView).
type DatadogLogDetailView struct {
	textView *tview.TextView
	host     ui.ViewHost
	event    datadoglogs.LogEvent
}

var _ ui.Themeable = (*DatadogLogDetailView)(nil)

// ApplyPalette recolors the Datadog log detail view for a live theme switch.
func (dv *DatadogLogDetailView) ApplyPalette(p config.Palette) {
	dv.textView.SetBackgroundColor(tcell.GetColor(p.Background))
	dv.textView.SetBorderColor(tcell.GetColor(p.ViewColor("datadog-logs")))
	dv.textView.SetTitleColor(tcell.GetColor(p.ViewColor("datadog-logs")))
}

func (dv *DatadogLogDetailView) Primitive() tview.Primitive { return dv.textView }

func (dv *DatadogLogDetailView) Shortcuts() []ui.Shortcut {
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

func NewDatadogLogDetailView(a ui.ViewHost, onBack func()) *DatadogLogDetailView {
	tv := tview.NewTextView()
	tv.SetBorder(true).SetTitle(" Datadog Log Event ")
	tv.SetDynamicColors(true)
	tv.SetScrollable(true)
	tv.SetWrap(true)

	dv := &DatadogLogDetailView{textView: tv, host: a}

	tv.SetInputCapture(func(event *tcell.EventKey) *tcell.EventKey {
		switch {
		case event.Rune() == 'j':
			return tcell.NewEventKey(tcell.KeyDown, 0, tcell.ModNone)
		case event.Rune() == 'k':
			return tcell.NewEventKey(tcell.KeyUp, 0, tcell.ModNone)
		case event.Rune() == 'c':
			dv.host.CopyToClipboard(dv.event.Message)
			dv.host.SetStatus("Copied log message to clipboard")
			return nil
		case event.Rune() == 'g':
			id, ok := extractCorrelationID(dv.event.Message)
			if !ok {
				dv.host.SetStatus("[yellow]No CorrelationID found in this log message[-]")
				return nil
			}
			// Quoted: CloudWatch's filter-pattern syntax tokenizes an
			// unquoted term on internal punctuation (a UUID's hyphens),
			// so an unquoted ID doesn't match as the literal phrase it
			// is. Scoped to this programmatically-injected value only —
			// a user's own typed search pattern is still passed straight
			// through unmodified (spec/34-fe-cloudwatch-logs decision 3).
			dv.host.SetPendingCloudWatchPattern(fmt.Sprintf("%q", id), dv.event.Timestamp)
			dv.host.SwitchTo("cloudwatch-logs")
			return nil
		case event.Key() == tcell.KeyEscape, event.Key() == tcell.KeyBackspace, event.Key() == tcell.KeyBackspace2:
			onBack()
			return nil
		}
		return event
	})

	return dv
}

// Render displays event's detail.
func (dv *DatadogLogDetailView) Render(event datadoglogs.LogEvent) {
	dv.event = event
	p := dv.host.Config().Colors
	accent, text := p.Label, p.Text

	var b strings.Builder
	line := func(label, value string) {
		fmt.Fprintf(&b, "[%s]%s:[-] [%s]%s[-]\n", accent, label, text, tview.Escape(value))
	}
	line("Timestamp", event.Timestamp.Local().Format("2006-01-02 15:04:05"))
	line("Service", event.Service)
	line("Env", event.Env)
	line("Status", event.Status)
	line("Host", event.Host)
	line("Tags", strings.Join(event.Tags, ", "))

	fmt.Fprintf(&b, "\n[%s]Message:[-]\n[%s]%s[-]", accent, text, tview.Escape(event.Message))

	dv.textView.SetText(b.String())
	dv.textView.ScrollToBeginning()
	dv.refreshContextPanel()
}

func (dv *DatadogLogDetailView) refreshContextPanel() {
	lines := make([]string, 0, len(dv.Shortcuts()))
	for _, sc := range dv.Shortcuts() {
		lines = append(lines, fmt.Sprintf("[%s]<%s>[-] %s", dv.host.Config().Colors.Accent, sc.Key, sc.Description))
	}
	dv.host.SetContextHint(strings.Join(lines, "\n"))
}
