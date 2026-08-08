package app

import (
	"context"
	"fmt"
	"strings"

	"github.com/gdamore/tcell/v2"
	"github.com/rivo/tview"

	"github.com/ePex/cloudtui/tui/internal/awsssm"
	"github.com/ePex/cloudtui/tui/internal/ui"
)

// paramDetailView shows the full detail of a single SSM parameter.
// It is not a registered ui.View; it is opened via App.openParamDetail
// and returns to "ssm-parameters" on Esc/Backspace. A SecureString
// parameter's value starts masked — 'r' explicitly reveals it via a real,
// separate GetParameter call (see awsssm.Reveal), matching the connection
// editor's masked-password convention rather than decrypting by default.
type paramDetailView struct {
	textView *tview.TextView
	app      *App
	param    awsssm.Parameter
}

func (dv *paramDetailView) Shortcuts() []ui.Shortcut {
	shortcuts := []ui.Shortcut{{Key: "Esc", Description: "back"}}
	if dv.param.Value != "" {
		shortcuts = append([]ui.Shortcut{{Key: "c", Description: "copy value"}}, shortcuts...)
	}
	if dv.param.Type == awsssm.TypeSecureString && dv.param.Value == "" {
		shortcuts = append([]ui.Shortcut{{Key: "r", Description: "reveal"}}, shortcuts...)
	}
	return shortcuts
}

func newParamDetailView(a *App) *paramDetailView {
	tv := tview.NewTextView()
	tv.SetBorder(true).SetTitle(" Parameter ")
	tv.SetDynamicColors(true)
	tv.SetScrollable(true)
	tv.SetWrap(true)

	dv := &paramDetailView{textView: tv, app: a}

	tv.SetInputCapture(func(event *tcell.EventKey) *tcell.EventKey {
		switch {
		case event.Rune() == 'j':
			return tcell.NewEventKey(tcell.KeyDown, 0, tcell.ModNone)
		case event.Rune() == 'k':
			return tcell.NewEventKey(tcell.KeyUp, 0, tcell.ModNone)
		case event.Rune() == 'r' && dv.param.Type == awsssm.TypeSecureString && dv.param.Value == "":
			dv.reveal()
			return nil
		case event.Rune() == 'c' && dv.param.Value != "":
			dv.copyValue()
			return nil
		case event.Key() == tcell.KeyEscape, event.Key() == tcell.KeyBackspace, event.Key() == tcell.KeyBackspace2:
			a.pages.SwitchToPage("ssm-parameters")
			a.tv.SetFocus(a.ssmParamsV.table)
			a.updateContextPanel(a.ssmParamsV)
			return nil
		}
		return event
	})

	return dv
}

// render displays param's detail. Called on open and again after a
// successful reveal.
func (dv *paramDetailView) render(param awsssm.Parameter) {
	dv.param = param
	p := dv.app.cfg.Colors
	accent, text := p.Label, p.Text

	var b strings.Builder
	line := func(label, value string) {
		fmt.Fprintf(&b, "[%s]%s:[-] [%s]%s[-]\n", accent, label, text, tview.Escape(value))
	}
	line("Name", param.Name)
	line("Type", string(param.Type))
	if !param.LastModified.IsZero() {
		line("Last Modified", param.LastModified.Local().Format("2006-01-02 15:04:05"))
	}

	fmt.Fprintf(&b, "\n[%s]Value:[-]\n", accent)
	switch {
	case param.Type == awsssm.TypeSecureString && param.Value == "":
		fmt.Fprintf(&b, "[%s](encrypted — press 'r' to reveal)[-]", text)
	default:
		fmt.Fprintf(&b, "[%s]%s[-]", text, tview.Escape(param.Value))
	}

	dv.textView.SetText(b.String())
	dv.textView.ScrollToBeginning()
	dv.refreshContextPanel()
}

// refreshContextPanel rebuilds the context panel from dv.Shortcuts(),
// which changes once a SecureString has been revealed (the "r: reveal"
// entry drops out). paramDetailView isn't a registered ui.View (like
// messageDetailView, it's opened directly rather than switched to by
// name), so this can't go through the generic updateContextPanel(ui.View)
// path — same manual pattern openMessageDetail uses.
func (dv *paramDetailView) refreshContextPanel() {
	lines := make([]string, 0, len(dv.Shortcuts()))
	for _, sc := range dv.Shortcuts() {
		lines = append(lines, fmt.Sprintf("[%s]<%s>[-] %s", dv.app.cfg.Colors.Accent, sc.Key, sc.Description))
	}
	dv.app.contextPanel.SetText(strings.Join(lines, "\n"))
}

// copyValue writes the currently-displayed value to the system clipboard.
// The status message names the parameter, never its value — a decrypted
// SecureString value must never appear anywhere but the masked/revealed
// detail text itself.
func (dv *paramDetailView) copyValue() {
	dv.app.copyToClipboard(dv.param.Value)
	dv.app.statusBar.SetText(fmt.Sprintf("Copied %s to clipboard", dv.param.Name))
}

// reveal fetches and decrypts a SecureString parameter's value, then
// re-renders in place.
func (dv *paramDetailView) reveal() {
	profile := dv.app.cfg.ActiveAWSProfile
	name := dv.param.Name
	go func() {
		value, err := dv.app.revealParameter(context.Background(), profile, name)
		dv.app.tv.QueueUpdateDraw(func() {
			if err != nil {
				dv.app.statusBar.SetText(fmt.Sprintf("[red]Error revealing %q: %s[-]", name, err))
				return
			}
			dv.param.Value = value
			dv.render(dv.param)
		})
	}()
}
