// Package app wires up the k9s-style shell: a top bar (connection info /
// command prompt on the left, shortcuts and logo on the right), a pages
// area that resource views are switched into, and a minimal status bar.
package app

import (
	"fmt"
	"os"

	"github.com/gdamore/tcell/v2"
	"github.com/rivo/tview"

	"github.com/ePex/cloudtui/tui/internal/config"
	"github.com/ePex/cloudtui/tui/internal/ui"
	"github.com/ePex/cloudtui/tui/internal/ui/views"
)

// App is the root of the TUI: it owns the tview.Application and routes
// command-prompt input to the registered resource views.
type App struct {
	tv      *tview.Application
	pages   *tview.Pages
	topLeft *tview.Pages
	prompt  *tview.InputField
	views   []ui.View
}

// New builds the app shell and registers the placeholder resource views.
func New() *App {
	cfg, err := config.LoadDefault()
	if err != nil {
		fmt.Fprintf(os.Stderr, "cloudtui: loading config: %v (using defaults)\n", err)
		cfg = config.Default()
	}

	a := &App{
		tv:    tview.NewApplication(),
		pages: tview.NewPages(),
		views: []ui.View{
			views.NewSecrets(),
			views.NewParams(),
			views.NewQueues(),
		},
	}

	for _, v := range a.views {
		a.pages.AddPage(v.Name(), v.Primitive(), true, false)
	}

	a.prompt = tview.NewInputField().
		SetLabel(" :").
		SetFieldBackgroundColor(tcell.ColorDefault)
	a.prompt.SetDoneFunc(a.onPromptDone)

	tb := newTopBar(cfg, a.prompt)
	a.topLeft = tb.left

	layout := tview.NewFlex().SetDirection(tview.FlexRow).
		AddItem(tb.root, tb.height, 0, false).
		AddItem(a.pages, 0, 1, true).
		AddItem(newStatusBar(), 1, 0, false)

	a.switchTo(a.views[0].Name())

	a.tv.SetRoot(layout, true).SetFocus(a.pages)
	a.tv.SetInputCapture(a.onGlobalKey)

	return a
}

// Run starts the terminal event loop; it blocks until the app exits.
func (a *App) Run() error {
	return a.tv.Run()
}

// onGlobalKey focuses the command prompt when ':' is pressed anywhere
// outside the prompt itself, k9s-style, swapping the top-left panel from
// connection info to the prompt.
func (a *App) onGlobalKey(event *tcell.EventKey) *tcell.EventKey {
	if a.tv.GetFocus() == a.prompt {
		return event
	}
	if event.Rune() == ':' {
		a.prompt.SetText("")
		a.topLeft.SwitchToPage("prompt")
		a.tv.SetFocus(a.prompt)
		return nil
	}
	return event
}

// onPromptDone handles Enter (switch view) and Escape (cancel) on the
// command prompt, restoring the top-left panel to connection info either
// way.
func (a *App) onPromptDone(key tcell.Key) {
	defer func() {
		a.prompt.SetText("")
		a.topLeft.SwitchToPage("info")
		a.tv.SetFocus(a.pages)
	}()

	if key != tcell.KeyEnter {
		return
	}

	cmd := a.prompt.GetText()
	if cmd == "q" || cmd == "quit" {
		a.tv.Stop()
		return
	}
	a.switchTo(cmd)
}

// switchTo activates the named view if it is registered.
func (a *App) switchTo(name string) {
	for _, v := range a.views {
		if v.Name() == name {
			a.pages.SwitchToPage(name)
			return
		}
	}
}
