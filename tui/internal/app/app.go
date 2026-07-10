// Package app wires up the k9s-style shell: a top bar (connection info /
// command prompt / filter input on the left, shortcuts and logo on the
// right), a pages area that resource views are switched into, a minimal
// status bar, and a global-hotkey-driven help overlay.
package app

import (
	"fmt"
	"os"

	"github.com/gdamore/tcell/v2"
	"github.com/rivo/tview"

	"github.com/ePex/cloudtui/tui/internal/config"
	"github.com/ePex/cloudtui/tui/internal/queue"
	"github.com/ePex/cloudtui/tui/internal/queue/proxy"
	"github.com/ePex/cloudtui/tui/internal/ui"
	"github.com/ePex/cloudtui/tui/internal/ui/views"
)

// App is the root of the TUI: it owns the tview.Application and routes
// command-prompt/hotkey input to the registered resource views.
type App struct {
	tv           *tview.Application
	rootPages    *tview.Pages
	pages        *tview.Pages
	topLeft      *tview.Pages
	prompt       *tview.InputField
	filterInput  *tview.InputField
	helpVisible  bool
	views        []ui.View
	cfg          config.Config
	infoPanel    *tview.TextView
	statusBar    *tview.TextView
	settingsList *tview.List

	backend          queue.Backend
	queuesRoot       *tview.Pages
	queuesList       *tview.List
	messagesList     *tview.List
	currentQueueName string
}

// New builds the app shell and registers the placeholder resource views.
func New() *App {
	cfg, err := config.LoadDefault()
	if err != nil {
		fmt.Fprintf(os.Stderr, "cloudtui: loading config: %v (using defaults)\n", err)
		cfg = config.Default()
	}

	backend, err := proxy.New(cfg.Queue.ProxyURL, cfg.Queue.Username, cfg.Queue.Password)
	if err != nil {
		fmt.Fprintf(os.Stderr, "cloudtui: creating mq-proxy client: %v\n", err)
	}

	a := &App{
		tv:    tview.NewApplication(),
		pages: tview.NewPages(),
		cfg:   cfg,
		views: []ui.View{
			views.NewHome(),
			views.NewSecrets(),
			views.NewParams(),
		},
	}
	a.views = append(a.views, newQueuesView(a, backend), newSettingsView(a))

	for _, v := range a.views {
		prim := v.Primitive()
		a.colorBordered(v, prim)
		a.pages.AddPage(v.Name(), prim, true, false)
	}

	a.prompt = tview.NewInputField().
		SetLabel(" :").
		SetFieldBackgroundColor(tcell.ColorDefault)
	a.prompt.SetDoneFunc(a.onPromptDone)

	a.filterInput = tview.NewInputField().
		SetLabel(" /").
		SetFieldBackgroundColor(tcell.ColorDefault)
	a.filterInput.SetDoneFunc(a.onFilterDone)

	tb := newTopBar(cfg, a.prompt, a.filterInput)
	a.topLeft = tb.left
	a.infoPanel = tb.info

	a.statusBar = newStatusBar()

	layout := tview.NewFlex().SetDirection(tview.FlexRow).
		AddItem(tb.root, tb.height, 0, false).
		AddItem(a.pages, 0, 1, true).
		AddItem(a.statusBar, 1, 0, false)

	helpOverlay := centered(newHelpModal(cfg), helpModalWidth, helpModalHeight)
	a.rootPages = tview.NewPages().
		AddPage("main", layout, true, true).
		AddPage("help", helpOverlay, true, false)

	a.switchTo(a.views[0].Name())

	a.tv.SetRoot(a.rootPages, true).SetFocus(a.pages)
	a.tv.SetInputCapture(a.onGlobalKey)

	return a
}

// Run starts the terminal event loop; it blocks until the app exits.
func (a *App) Run() error {
	return a.tv.Run()
}

// onGlobalKey handles the app's hotkeys (h/s/q/?//) and the ':' command
// prompt, all inert while the prompt or filter input has focus.
func (a *App) onGlobalKey(event *tcell.EventKey) *tcell.EventKey {
	if a.tv.GetFocus() == a.prompt || a.tv.GetFocus() == a.filterInput {
		return event
	}

	if a.helpVisible {
		if event.Key() == tcell.KeyEscape || event.Rune() == '?' {
			a.closeHelp()
		}
		return nil
	}

	switch event.Rune() {
	case ':':
		a.prompt.SetText("")
		a.topLeft.SwitchToPage("prompt")
		a.tv.SetFocus(a.prompt)
		return nil
	case 'h':
		a.switchTo("home")
		return nil
	case 's':
		a.switchTo("settings")
		return nil
	case 'q':
		a.tv.Stop()
		return nil
	case '?':
		a.openHelp()
		return nil
	case '/':
		a.beginFilter()
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

// beginFilter switches to the filter input if the active view supports
// filtering; otherwise it's a no-op.
func (a *App) beginFilter() {
	if _, ok := a.activeView().(ui.Filterable); !ok {
		return
	}
	a.filterInput.SetText("")
	a.topLeft.SwitchToPage("filter")
	a.tv.SetFocus(a.filterInput)
}

// onFilterDone applies the typed query to the active view on Enter (if it
// supports filtering), restoring the top-left panel to connection info
// either way.
func (a *App) onFilterDone(key tcell.Key) {
	defer func() {
		a.filterInput.SetText("")
		a.topLeft.SwitchToPage("info")
		a.tv.SetFocus(a.pages)
	}()

	if key != tcell.KeyEnter {
		return
	}

	if f, ok := a.activeView().(ui.Filterable); ok {
		f.Filter(a.filterInput.GetText())
	}
}

// openHelp shows the help overlay on top of the main layout.
func (a *App) openHelp() {
	a.rootPages.ShowPage("help")
	a.helpVisible = true
}

// closeHelp hides the help overlay.
func (a *App) closeHelp() {
	a.rootPages.HidePage("help")
	a.helpVisible = false
}

// activatable is implemented by views that need to (re)load data each
// time they become the active view, rather than only once at
// construction — e.g. the queues view's list, which would otherwise go
// stale. Implementations typically kick off a goroutine that finishes
// via tv.QueueUpdateDraw, which blocks forever unless tv's event loop is
// actually running (see runApp in queues_test.go) — tests that call
// switchTo on a view implementing this must account for that.
type activatable interface {
	activate()
}

// switchTo activates the named view if it is registered, re-focusing
// pages so the newly active view's own input handling (e.g. the
// settings list's navigation) actually receives key events — tview.Pages
// only re-delegates focus to its front item when Focus() is (re-)called
// on it, not automatically on SwitchToPage.
func (a *App) switchTo(name string) {
	for _, v := range a.views {
		if v.Name() == name {
			a.pages.SwitchToPage(name)
			a.tv.SetFocus(a.pages)
			if act, ok := v.(activatable); ok {
				act.activate()
			}
			return
		}
	}
}

// setStatus updates the bottom status bar.
func (a *App) setStatus(text string) {
	a.statusBar.SetText(text)
}

// refreshInfoPanel re-renders the connection-info panel from the current
// config — called after the AWS profile selection changes.
func (a *App) refreshInfoPanel() {
	a.infoPanel.SetText(infoPanelText(a.cfg))
}

// activeView returns the currently front-most registered view, or nil if
// pages' front page doesn't match any registered view.
func (a *App) activeView() ui.View {
	name, _ := a.pages.GetFrontPage()
	for _, v := range a.views {
		if v.Name() == name {
			return v
		}
	}
	return nil
}

// colorBordered applies v's configured (or Border-fallback) color to
// prim's border and title, if prim supports it.
func (a *App) colorBordered(v ui.View, prim tview.Primitive) {
	b, ok := prim.(bordered)
	if !ok {
		return
	}
	c := tcell.GetColor(a.cfg.Colors.ViewColor(v.Name()))
	b.SetBorderColor(c)
	b.SetTitleColor(c)
}

// bordered is implemented by tview primitives (via an embedded
// *tview.Box) that expose settable border/title colors — every current
// placeholder view, and any future real view built the same way.
type bordered interface {
	SetBorderColor(color tcell.Color) *tview.Box
	SetTitleColor(color tcell.Color) *tview.Box
}
