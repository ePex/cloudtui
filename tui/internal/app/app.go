// Package app wires up the k9s-style shell: a top bar (connection info /
// command prompt on the left, shortcuts and logo on the right),
// a pages area that views are switched into, a minimal status bar, and a
// global-hotkey-driven help overlay.
package app

import (
	"fmt"
	"os"
	"strings"

	"github.com/gdamore/tcell/v2"
	"github.com/rivo/tview"

	"github.com/ePex/cloudtui/tui/internal/config"
	"github.com/ePex/cloudtui/tui/internal/queue"
	"github.com/ePex/cloudtui/tui/internal/queue/jolokia"
	"github.com/ePex/cloudtui/tui/internal/ui"
	"github.com/ePex/cloudtui/tui/internal/ui/views"
)

// App is the root of the TUI: it owns the tview.Application and routes
// command-prompt/hotkey input to the registered views.
type App struct {
	tv             *tview.Application
	rootPages      *tview.Pages
	pages          *tview.Pages
	topLeft        *tview.Pages
	prompt         *tview.InputField
	helpVisible    bool
	views          []ui.View
	cfg            config.Config
	infoPanel      *tview.TextView
	divider        *tview.TextView
	contextPanel   *tview.TextView
	logoPanel      *tview.TextView
	statusBar      *tview.TextView
	settingsForm   *tview.Form
	logV           *logView
	queuesV        *queuesView
	backend        queue.Backend
	homeTable     *tview.Table
	homeSections  []views.SectionInfo
	topBarHeight  int
	switchingTheme bool
}

// New builds the app shell with cfg as the starting configuration.
// applyTheme is called first, before any primitive is constructed, so
// tview.Styles are set before primitives read them at construction time.
func New(cfg config.Config) *App {
	applyTheme(cfg.Colors)

	homeSections := []views.SectionInfo{
		{
			Title: "Apps",
			Entries: []views.ViewInfo{
				{Name: "queues", Description: "List ActiveMQ queues via Jolokia"},
			},
		},
		{
			Title: "System",
			Entries: []views.ViewInfo{
				{Name: "settings", Description: "Edit and persist app configuration"},
				{Name: "log", Description: "View the application log"},
			},
		},
	}

	a := &App{
		tv:            tview.NewApplication(),
		pages:         tview.NewPages(),
		cfg:           cfg,
		homeSections:  homeSections,
	}

	homeView := views.NewHome(homeSections, a.switchTo, cfg.Colors.Label, cfg.Colors.Text, cfg.Colors.Border, cfg.Colors.SelectionBg, cfg.Colors.SelectionText)
	a.homeTable, _ = homeView.Primitive().(*tview.Table)

	a.prompt = tview.NewInputField().
		SetLabel(" :").
		SetFieldBackgroundColor(tcell.ColorDefault)
	a.prompt.SetDoneFunc(a.onPromptDone)


	tb := newTopBar(cfg, a.prompt)
	a.topLeft = tb.left
	a.infoPanel = tb.info
	a.divider = tb.divider
	a.contextPanel = tb.contextPanel
	a.logoPanel = tb.logo
	a.topBarHeight = tb.height

	a.statusBar = newStatusBar(cfg)

	// All shell primitives are constructed; now create the settings view.
	// Its dropdown callback calls switchTheme, which calls reapplyTheme —
	// that is safe at this point because all live primitives are set.
	// switchingTheme is set during construction to suppress any spurious
	// initial callback that tview may fire when AddDropDown sets the initial
	// selection.
	a.switchingTheme = true
	settingsView := newSettingsView(a)
	a.switchingTheme = false
	a.logV = newLogView(a)
	a.backend = jolokia.NewClient(cfg.Queue)
	a.queuesV = newQueuesView(a, a.backend)

	a.views = []ui.View{homeView, settingsView, a.logV, a.queuesV}
	for _, v := range a.views {
		prim := v.Primitive()
		a.colorBordered(v, prim)
		a.pages.AddPage(v.Name(), prim, true, false)
	}

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

// onGlobalKey handles the app's hotkeys (h/s/q/?) and the ':' command
// prompt, all inert while the prompt has focus.
func (a *App) onGlobalKey(event *tcell.EventKey) *tcell.EventKey {
	if a.tv.GetFocus() == a.prompt {
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
	case 'l':
		a.switchTo("log")
		return nil
	case 'q':
		a.tv.Stop()
		return nil
	case '?':
		a.openHelp()
		return nil
	}
	return event
}

// onPromptDone handles Enter (execute command) and Escape (cancel) on the
// command prompt, restoring the top-left panel to connection info either way.
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
	switch {
	case cmd == "q" || cmd == "quit":
		a.tv.Stop()
	case cmd == "h" || cmd == "home":
		a.switchTo("home")
	case cmd == "s" || cmd == "settings":
		a.switchTo("settings")
	case strings.HasPrefix(cmd, "theme "):
		a.switchTheme(strings.TrimPrefix(cmd, "theme "))
	default:
		a.switchTo(cmd)
	}
}

// switchTheme applies the named built-in palette, saves config, and repaints
// all live shell primitives. Unknown names are silently ignored.
func (a *App) switchTheme(name string) {
	p, ok := config.PaletteForTheme(name)
	if !ok {
		return
	}
	a.switchingTheme = true
	defer func() { a.switchingTheme = false }()
	a.cfg.Theme = name
	a.cfg.Colors = p
	reapplyTheme(a, p)
	if err := config.SaveDefault(a.cfg); err != nil {
		fmt.Fprintf(os.Stderr, "cloudtui: saving config: %v\n", err)
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

// switchTo activates the named view if it is registered, updates the top
// bar's context panel, and calls Activate() if the view implements activatable.
func (a *App) switchTo(name string) {
	for _, v := range a.views {
		if v.Name() == name {
			a.pages.SwitchToPage(name)
			a.tv.SetFocus(a.pages)
			a.updateContextPanel(v)
			if act, ok := v.(activatable); ok {
				act.Activate()
			}
			return
		}
	}
}

// updateContextPanel renders v's shortcuts into the context panel, or clears
// it when v doesn't implement ui.Shortcuttable.
func (a *App) updateContextPanel(v ui.View) {
	s, ok := v.(ui.Shortcuttable)
	if !ok || len(s.Shortcuts()) == 0 {
		a.contextPanel.SetText("")
		return
	}
	lines := make([]string, 0, len(s.Shortcuts()))
	for _, sc := range s.Shortcuts() {
		lines = append(lines, fmt.Sprintf("[%s]<%s>[-] %s", a.cfg.Colors.Accent, sc.Key, sc.Description))
	}
	a.contextPanel.SetText(strings.Join(lines, "\n"))
}

// activeView returns the currently front-most registered view, or nil if
// the pages' front page doesn't match any registered view.
func (a *App) activeView() ui.View {
	name, _ := a.pages.GetFrontPage()
	for _, v := range a.views {
		if v.Name() == name {
			return v
		}
	}
	return nil
}

// colorBordered applies v's configured (or Border-fallback) color to prim's
// border and title, if prim supports it.
func (a *App) colorBordered(v ui.View, prim tview.Primitive) {
	b, ok := prim.(bordered)
	if !ok {
		return
	}
	c := tcell.GetColor(a.cfg.Colors.ViewColor(v.Name()))
	b.SetBorderColor(c)
	b.SetTitleColor(c)
}

// bordered is implemented by tview primitives (via an embedded *tview.Box)
// that expose settable border/title colors.
type bordered interface {
	SetBorderColor(color tcell.Color) *tview.Box
	SetTitleColor(color tcell.Color) *tview.Box
}

// activatable is implemented by views that want to refresh their content each
// time they become active (e.g. logView reloads the log file on switchTo).
type activatable interface {
	Activate()
}
