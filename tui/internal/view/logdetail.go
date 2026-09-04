package view

import (
	"fmt"
	"strings"

	"github.com/gdamore/tcell/v2"
	"github.com/rivo/tview"

	"github.com/ePex/cloudtui/tui/internal/awslogs"
	"github.com/ePex/cloudtui/tui/internal/config"
	"github.com/ePex/cloudtui/tui/internal/ui"
)

// LogDetailView shows the full detail of a single CloudWatch Logs event.
// It is not a registered ui.View; it is opened via App.OpenLogEventDetail
// and returns to the search view on Esc/Backspace. Unlike
// paramDetailView/secretDetailView, nothing here is masked — a log event
// is never a secret in the AWS-service sense — so 'c' is always
// available, no reveal-gating needed.
type LogDetailView struct {
	textView *tview.TextView
	host     ui.CloudWatchLogsHost
	event    awslogs.LogEvent
}

var _ ui.Themeable = (*LogDetailView)(nil)

// ApplyPalette recolors the log event detail view for a live theme switch.
func (dv *LogDetailView) ApplyPalette(p config.Palette) {
	dv.textView.SetBackgroundColor(tcell.GetColor(p.Background))
	dv.textView.SetBorderColor(tcell.GetColor(p.ViewColor("cloudwatch-logs")))
	dv.textView.SetTitleColor(tcell.GetColor(p.ViewColor("cloudwatch-logs")))
}

func (dv *LogDetailView) Primitive() tview.Primitive { return dv.textView }

func (dv *LogDetailView) Shortcuts() []ui.Shortcut {
	return []ui.Shortcut{
		{Key: "c", Description: "copy message"},
		{Key: "Esc", Description: "back"},
	}
}

func NewLogDetailView(a ui.CloudWatchLogsHost, onBack func()) *LogDetailView {
	tv := tview.NewTextView()
	tv.SetBorder(true).SetTitle(" Log Event ")
	tv.SetDynamicColors(true)
	tv.SetScrollable(true)
	tv.SetWrap(true)

	dv := &LogDetailView{textView: tv, host: a}

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
		case event.Key() == tcell.KeyEscape, event.Key() == tcell.KeyBackspace, event.Key() == tcell.KeyBackspace2:
			onBack()
			return nil
		}
		return event
	})

	return dv
}

// Render displays event's detail.
func (dv *LogDetailView) Render(event awslogs.LogEvent) {
	dv.event = event
	p := dv.host.Config().Colors
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

func (dv *LogDetailView) refreshContextPanel() {
	lines := make([]string, 0, len(dv.Shortcuts()))
	for _, sc := range dv.Shortcuts() {
		lines = append(lines, fmt.Sprintf("[%s]<%s>[-] %s", dv.host.Config().Colors.Accent, sc.Key, sc.Description))
	}
	dv.host.SetContextHint(strings.Join(lines, "\n"))
}
