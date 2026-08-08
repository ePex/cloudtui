package app

import (
	"context"
	"encoding/json"
	"fmt"
	"log/slog"
	"strings"

	"github.com/gdamore/tcell/v2"
	"github.com/rivo/tview"

	"github.com/ePex/cloudtui/tui/internal/awssecrets"
	"github.com/ePex/cloudtui/tui/internal/ui"
)

// secretDetailView shows the full detail of a single Secrets Manager
// secret. It is not a registered ui.View; it is opened via
// App.openSecretDetail and returns to "secrets-manager" on Esc/Backspace.
// Unlike paramDetailView, a Secret never carries a value at all — Secrets
// Manager's ListSecrets structurally has no value field — so a real,
// separate GetSecretValue call (see awssecrets.Reveal) is always needed to
// get one, whether the user asks to see it ('r') or just copy it ('c').
//
// fetched and revealed are deliberately separate: 'c' is available from
// the moment the view opens and works without ever revealing the value on
// screen — it fetches (if not already fetched) and copies straight to the
// clipboard, leaving the display masked. 'r' additionally renders the
// fetched value on screen. Once fetched, pressing the other key doesn't
// re-fetch — 'r' after a prior silent 'c' just displays the cached value,
// and 'c' after a prior 'r' just copies it again.
type secretDetailView struct {
	textView *tview.TextView
	app      *App
	secret   awssecrets.Secret

	fetched      bool   // GetSecretValue has completed successfully at least once
	revealed     bool   // the value has been rendered on screen (implies fetched)
	isBinary     bool   // valid once fetched: true if the secret has no SecretString
	displayValue string // pretty-printed (if JSON) fetched value; empty if isBinary
}

func (dv *secretDetailView) Shortcuts() []ui.Shortcut {
	shortcuts := []ui.Shortcut{{Key: "Esc", Description: "back"}}
	if !(dv.fetched && dv.isBinary) {
		shortcuts = append([]ui.Shortcut{{Key: "c", Description: "copy value"}}, shortcuts...)
	}
	if !dv.revealed {
		shortcuts = append([]ui.Shortcut{{Key: "r", Description: "reveal"}}, shortcuts...)
	}
	return shortcuts
}

func newSecretDetailView(a *App) *secretDetailView {
	tv := tview.NewTextView()
	tv.SetBorder(true).SetTitle(" Secret ")
	tv.SetDynamicColors(true)
	tv.SetScrollable(true)
	tv.SetWrap(true)

	dv := &secretDetailView{textView: tv, app: a}

	tv.SetInputCapture(func(event *tcell.EventKey) *tcell.EventKey {
		switch {
		case event.Rune() == 'j':
			return tcell.NewEventKey(tcell.KeyDown, 0, tcell.ModNone)
		case event.Rune() == 'k':
			return tcell.NewEventKey(tcell.KeyUp, 0, tcell.ModNone)
		case event.Rune() == 'r' && !dv.revealed:
			dv.reveal()
			return nil
		case event.Rune() == 'c' && !(dv.fetched && dv.isBinary):
			dv.copyValue()
			return nil
		case event.Key() == tcell.KeyEscape, event.Key() == tcell.KeyBackspace, event.Key() == tcell.KeyBackspace2:
			a.pages.SwitchToPage("secrets-manager")
			a.tv.SetFocus(a.secretsV.table)
			a.updateContextPanel(a.secretsV)
			return nil
		}
		return event
	})

	return dv
}

// render displays secret's detail, freshly masked and unfetched. Called on
// open — resetting all reveal/fetch state here is what makes this "open a
// fresh detail view" rather than "redraw the current one"; reveal() and
// copyValue()'s callbacks call renderBody directly instead, to update in
// place without losing the just-fetched value.
func (dv *secretDetailView) render(secret awssecrets.Secret) {
	dv.secret = secret
	dv.fetched = false
	dv.revealed = false
	dv.isBinary = false
	dv.displayValue = ""
	dv.renderBody()
}

func (dv *secretDetailView) renderBody() {
	p := dv.app.cfg.Colors
	accent, text := p.Label, p.Text

	var b strings.Builder
	line := func(label, value string) {
		fmt.Fprintf(&b, "[%s]%s:[-] [%s]%s[-]\n", accent, label, text, tview.Escape(value))
	}
	line("Name", dv.secret.Name)
	line("ARN", dv.secret.ARN)
	rotation := "no"
	if dv.secret.RotationEnabled {
		rotation = "yes"
	}
	line("Rotation Enabled", rotation)
	if !dv.secret.LastChanged.IsZero() {
		line("Last Changed", dv.secret.LastChanged.Local().Format("2006-01-02 15:04:05"))
	}

	fmt.Fprintf(&b, "\n[%s]Value:[-]\n", accent)
	switch {
	case !dv.revealed:
		fmt.Fprintf(&b, "[%s](encrypted — press 'r' to reveal)[-]", text)
	case dv.isBinary:
		fmt.Fprintf(&b, "[%s](binary secret — cannot display)[-]", text)
	default:
		fmt.Fprintf(&b, "[%s]%s[-]", text, tview.Escape(dv.displayValue))
	}

	dv.textView.SetText(b.String())
	dv.textView.ScrollToBeginning()
	dv.refreshContextPanel()
}

// refreshContextPanel rebuilds the context panel from dv.Shortcuts(),
// which changes as reveal progresses (the "r: reveal" entry drops out,
// "c: copy value" appears for a non-binary secret). secretDetailView isn't
// a registered ui.View, so this can't go through the generic
// updateContextPanel(ui.View) path — same manual pattern paramDetailView
// uses.
func (dv *secretDetailView) refreshContextPanel() {
	lines := make([]string, 0, len(dv.Shortcuts()))
	for _, sc := range dv.Shortcuts() {
		lines = append(lines, fmt.Sprintf("[%s]<%s>[-] %s", dv.app.cfg.Colors.Accent, sc.Key, sc.Description))
	}
	dv.app.contextPanel.SetText(strings.Join(lines, "\n"))
}

// copyValue writes the fetched value to the system clipboard — fetching it
// first, silently, if a prior 'c' or 'r' hasn't already. The display stays
// masked either way; only the status message (naming the secret, never
// its value) confirms what happened.
func (dv *secretDetailView) copyValue() {
	if !dv.fetched {
		dv.fetchThen(dv.copyFetchedValue)
		return
	}
	dv.copyFetchedValue()
}

func (dv *secretDetailView) copyFetchedValue() {
	if dv.isBinary {
		dv.app.statusBar.SetText(fmt.Sprintf("[red]Cannot copy %s: binary secret[-]", dv.secret.Name))
		return
	}
	dv.app.copyToClipboard(dv.displayValue)
	dv.app.statusBar.SetText(fmt.Sprintf("Copied %s to clipboard", dv.secret.Name))
}

// reveal displays the fetched value on screen — fetching it first if a
// prior silent 'c' hasn't already cached it.
func (dv *secretDetailView) reveal() {
	if dv.fetched {
		dv.revealed = true
		dv.renderBody()
		return
	}
	dv.fetchThen(func() {
		dv.revealed = true
		dv.renderBody()
	})
}

// fetchThen fetches and decrypts the secret's current (AWSCURRENT) value
// and hands the outcome to handleFetchResult on the tview event loop.
func (dv *secretDetailView) fetchThen(onSuccess func()) {
	profile := dv.app.cfg.ActiveAWSProfile
	name := dv.secret.Name
	go func() {
		value, isBinary, err := dv.app.revealSecret(context.Background(), profile, name)
		dv.app.tv.QueueUpdateDraw(func() {
			dv.handleFetchResult(value, isBinary, err, onSuccess)
		})
	}()
}

// handleFetchResult processes the outcome of a GetSecretValue call: on
// error, logs and shows it; on success, caches the value/binary flag and
// calls onSuccess — which decides whether that means displaying the value
// (reveal) or just copying it (copyValue). Split out from fetchThen so
// this — the part with actual logic — is directly testable without
// spawning a goroutine or needing a running tview event loop
// (QueueUpdateDraw blocks forever without one).
func (dv *secretDetailView) handleFetchResult(value string, isBinary bool, err error, onSuccess func()) {
	name := dv.secret.Name
	if err != nil {
		slog.Error("secret detail: failed to reveal secret", "name", name, "error", err)
		dv.app.statusBar.SetText(fmt.Sprintf("[red]Error revealing %q: %s[-]", name, err))
		return
	}
	dv.fetched = true
	dv.isBinary = isBinary
	if !isBinary {
		dv.displayValue = prettyPrintJSON(value)
	}
	dv.refreshContextPanel() // "c" drops out now if isBinary, even though the screen is still masked
	onSuccess()
}

// prettyPrintJSON indents raw if it parses as JSON, returning it unchanged
// otherwise (plain-string secrets, or invalid/empty input). Secrets
// Manager secrets are very commonly JSON key/value blobs (e.g. database
// credentials), so this makes the common case readable without attempting
// to interpret anything that isn't actually JSON.
func prettyPrintJSON(raw string) string {
	var v any
	if err := json.Unmarshal([]byte(raw), &v); err != nil {
		return raw
	}
	pretty, err := json.MarshalIndent(v, "", "  ")
	if err != nil {
		return raw
	}
	return string(pretty)
}
