package app

import (
	"context"
	"fmt"
	"log/slog"
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
//
// displayed tracks whether the value has been rendered on screen,
// separately from whether it's been fetched (dv.param.Value != "" for a
// SecureString): 'c' is available from the moment the view opens and
// works without ever revealing the value on screen — it fetches (if not
// already fetched) and copies straight to the clipboard, leaving the
// display masked. 'r' additionally renders the fetched value. Once
// fetched, pressing the other key doesn't re-fetch — 'r' after a prior
// silent 'c' just displays the cached value.
type paramDetailView struct {
	textView  *tview.TextView
	app       *App
	param     awsssm.Parameter
	displayed bool // the value has been rendered on screen; always true for String/StringList
}

func (dv *paramDetailView) Shortcuts() []ui.Shortcut {
	shortcuts := []ui.Shortcut{{Key: "Esc", Description: "back"}, {Key: "c", Description: "copy value"}}
	if dv.param.Type == awsssm.TypeSecureString && !dv.displayed {
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
		case event.Rune() == 'r' && dv.param.Type == awsssm.TypeSecureString && !dv.displayed:
			dv.reveal()
			return nil
		case event.Rune() == 'c':
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

// render displays param's detail, freshly masked (for a SecureString) or
// shown (for String/StringList, which carry their value from the list
// call already). Called on open — resetting displayed here is what makes
// this "open a fresh detail view" rather than "redraw the current one";
// reveal()'s callback calls renderBody directly instead, to update in
// place without losing the just-fetched value.
func (dv *paramDetailView) render(param awsssm.Parameter) {
	dv.param = param
	dv.displayed = param.Type != awsssm.TypeSecureString
	dv.renderBody()
}

func (dv *paramDetailView) renderBody() {
	p := dv.app.cfg.Colors
	accent, text := p.Label, p.Text

	var b strings.Builder
	line := func(label, value string) {
		fmt.Fprintf(&b, "[%s]%s:[-] [%s]%s[-]\n", accent, label, text, tview.Escape(value))
	}
	line("Name", dv.param.Name)
	line("Type", string(dv.param.Type))
	if !dv.param.LastModified.IsZero() {
		line("Last Modified", dv.param.LastModified.Local().Format("2006-01-02 15:04:05"))
	}

	fmt.Fprintf(&b, "\n[%s]Value:[-]\n", accent)
	switch {
	case dv.param.Type == awsssm.TypeSecureString && !dv.displayed:
		fmt.Fprintf(&b, "[%s](encrypted — press 'r' to reveal)[-]", text)
	default:
		fmt.Fprintf(&b, "[%s]%s[-]", text, tview.Escape(dv.param.Value))
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

// copyValue writes the parameter's value to the system clipboard —
// fetching it first, silently, if a prior 'c' or 'r' hasn't already (only
// possible for a SecureString; String/StringList values are already
// present from the list call). The display stays masked either way; only
// the status message (naming the parameter, never its value) confirms
// what happened — a decrypted SecureString value must never appear
// anywhere but the masked/revealed detail text itself.
func (dv *paramDetailView) copyValue() {
	if dv.param.Type == awsssm.TypeSecureString && dv.param.Value == "" {
		dv.fetchThen(dv.copyFetchedValue)
		return
	}
	dv.copyFetchedValue()
}

func (dv *paramDetailView) copyFetchedValue() {
	dv.app.copyToClipboard(dv.param.Value)
	dv.app.statusBar.SetText(fmt.Sprintf("Copied %s to clipboard", dv.param.Name))
}

// reveal displays the parameter's value on screen — fetching it first if
// a prior silent 'c' hasn't already cached it.
func (dv *paramDetailView) reveal() {
	if dv.param.Value != "" {
		dv.displayed = true
		dv.renderBody()
		return
	}
	dv.fetchThen(func() {
		dv.displayed = true
		dv.renderBody()
	})
}

// fetchThen fetches and decrypts a SecureString parameter's value and
// hands the outcome to handleFetchResult on the tview event loop.
func (dv *paramDetailView) fetchThen(onSuccess func()) {
	profile := dv.app.cfg.ActiveAWSProfile
	name := dv.param.Name
	go func() {
		value, err := dv.app.revealParameter(context.Background(), profile, name)
		dv.app.tv.QueueUpdateDraw(func() {
			dv.handleFetchResult(value, err, onSuccess)
		})
	}()
}

// handleFetchResult processes the outcome of a GetParameter call: on
// error, logs and shows it; on success, caches the value on dv.param and
// calls onSuccess — which decides whether that means displaying the value
// (reveal) or just copying it (copyValue). Split out from fetchThen so
// this — the part with actual logic — is directly testable without
// spawning a goroutine or needing a running tview event loop
// (QueueUpdateDraw blocks forever without one).
func (dv *paramDetailView) handleFetchResult(value string, err error, onSuccess func()) {
	name := dv.param.Name
	if err != nil {
		slog.Error("param detail: failed to reveal parameter", "name", name, "error", err)
		dv.app.statusBar.SetText(fmt.Sprintf("[red]Error revealing %q: %s[-]", name, err))
		return
	}
	dv.param.Value = value
	onSuccess()
}
