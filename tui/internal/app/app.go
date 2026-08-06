// Package app wires up the k9s-style shell: a top bar (connection info /
// command prompt on the left, shortcuts and logo on the right),
// a pages area that views are switched into, a minimal status bar, and a
// global-hotkey-driven help overlay.
package app

import (
	"context"
	"fmt"
	"log/slog"
	"os"
	"sort"
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
	messagesV      *messagesView
	messageDetailV *messageDetailView
	confirmFlex      *tview.Flex
	confirmText      *tview.TextView
	confirmList      *tview.List
	confirmVisible   bool
	movePickerFlex      *tview.Flex
	movePickerList      *tview.List
	movePickerSearch    *tview.InputField
	movePickerQueues    []string
	movePickerPreferred string
	movePickerVisible   bool
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
	a.messagesV = newMessagesView(a)
	a.messageDetailV = newMessageDetailView(a)

	// Wire Enter in the queues table to open the messages view for the
	// selected queue. Done here because messagesV must exist first.
	a.queuesV.table.SetSelectedFunc(func(row, _ int) {
		cell := a.queuesV.table.GetCell(row, 0)
		if cell == nil || cell.Text == "" {
			return
		}
		a.openMessages(cell.Text)
	})

	// Wire Enter in the messages table to open the detail view for the
	// selected message. Done here because messageDetailV must exist first.
	a.messagesV.table.SetSelectedFunc(func(row, _ int) {
		msgIdx := row - 1 // row 0 is the header
		if msgIdx < 0 || msgIdx >= len(a.messagesV.msgs) {
			return
		}
		a.openMessageDetail(a.messagesV.queueName, a.messagesV.msgs[msgIdx])
	})

	a.views = []ui.View{homeView, settingsView, a.logV, a.queuesV}
	for _, v := range a.views {
		prim := v.Primitive()
		a.colorBordered(v, prim)
		a.pages.AddPage(v.Name(), prim, true, false)
	}
	// messages and message-detail pages: not in a.views (no home entry / switchTo).
	a.pages.AddPage("messages", a.messagesV.table, true, false)
	a.pages.AddPage("message-detail", a.messageDetailV.textView, true, false)

	layout := tview.NewFlex().SetDirection(tview.FlexRow).
		AddItem(tb.root, tb.height, 0, false).
		AddItem(a.pages, 0, 1, true).
		AddItem(a.statusBar, 1, 0, false)

	a.confirmText = tview.NewTextView().SetWrap(true)
	a.confirmList = tview.NewList().ShowSecondaryText(false)
	a.confirmFlex = tview.NewFlex().SetDirection(tview.FlexRow).
		AddItem(a.confirmText, 2, 0, false).
		AddItem(a.confirmList, 0, 1, true)
	a.confirmFlex.SetBorder(true).SetTitle(" Confirm ")
	confirmOverlay := centered(a.confirmFlex, 52, 8)

	a.movePickerList = tview.NewList().ShowSecondaryText(false)
	a.movePickerSearch = tview.NewInputField().SetLabel(" / filter: ")
	a.movePickerFlex = tview.NewFlex().SetDirection(tview.FlexRow).
		AddItem(a.movePickerList, 0, 1, true).
		AddItem(a.movePickerSearch, 1, 0, false)
	a.movePickerFlex.SetBorder(true).SetTitle(" Move to Queue ")

	// SetChangedFunc is registered in showMovePicker (needs sourceQueue/msg closure).
	a.movePickerSearch.SetDoneFunc(func(_ tcell.Key) {
		a.tv.SetFocus(a.movePickerList)
	})
	a.movePickerSearch.SetInputCapture(func(event *tcell.EventKey) *tcell.EventKey {
		if event.Key() == tcell.KeyEscape {
			a.movePickerSearch.SetText("")
			a.tv.SetFocus(a.movePickerList)
			return nil
		}
		return event
	})

	movePickerOverlay := centered(a.movePickerFlex, 52, 22)

	helpOverlay := centered(newHelpModal(cfg), helpModalWidth, helpModalHeight)
	a.rootPages = tview.NewPages().
		AddPage("main", layout, true, true).
		AddPage("help", helpOverlay, true, false).
		AddPage("confirm", confirmOverlay, true, false).
		AddPage("move-picker", movePickerOverlay, true, false)

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
// prompt, all inert while the prompt or any input field has focus.
func (a *App) onGlobalKey(event *tcell.EventKey) *tcell.EventKey {
	if a.tv.GetFocus() == a.prompt {
		return event
	}
	if a.queuesV != nil && a.tv.GetFocus() == a.queuesV.filterInput {
		return event
	}

	if a.confirmVisible || a.movePickerVisible {
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



// showConfirm presents a confirmation dialog with the given question.
// onConfirm is called when the user selects "Yes". "No" is item 0 (default
// focus) to prevent accidental actions.
func (a *App) showConfirm(question string, onConfirm func()) {
	a.confirmText.SetText(question)
	a.confirmList.Clear()

	dismiss := func() { a.closeConfirm() }

	a.confirmList.AddItem("No", "", 0, dismiss)
	a.confirmList.AddItem("Yes", "", 0, func() {
		a.closeConfirm()
		onConfirm()
	})

	a.confirmList.SetInputCapture(func(event *tcell.EventKey) *tcell.EventKey {
		if event.Key() == tcell.KeyEscape {
			a.closeConfirm()
			return nil
		}
		return event
	})

	a.rootPages.ShowPage("confirm")
	a.tv.SetFocus(a.confirmList)
	a.confirmVisible = true
}

// closeConfirm hides the confirmation dialog and restores focus.
func (a *App) closeConfirm() {
	a.rootPages.HidePage("confirm")
	a.tv.SetFocus(a.pages)
	a.confirmVisible = false
}

// isSystemQueue reports whether name belongs to a built-in AMQ system
// destination (activemq.*, statistics.*).
func isSystemQueue(name string) bool {
	lower := strings.ToLower(name)
	return strings.HasPrefix(lower, "activemq.") || strings.HasPrefix(lower, "statistics.")
}

// isDLQQueue reports whether name is a dead-letter queue (dlq.*).
func isDLQQueue(name string) bool {
	return strings.HasPrefix(strings.ToLower(name), "dlq.")
}

// sortPickerQueues orders names into four tiers:
//  1. Preferred — the non-DLQ counterpart of sourceQueue when source is a DLQ
//     (e.g. "dlq.foo" → "foo"). Pinned first.
//  2. Regular — all other queues, alphabetical.
//  3. DLQ — queues starting with "dlq.", alphabetical.
//  4. System — queues starting with "activemq." or "statistics.", alphabetical.
func sortPickerQueues(sourceQueue string, names []string) []string {
	var preferred string
	var regular, dlqs, system []string

	isDLQSrc := isDLQQueue(sourceQueue)
	var candidateLower string
	if isDLQSrc {
		candidateLower = strings.ToLower(sourceQueue[4:])
	}

	for _, name := range names {
		lower := strings.ToLower(name)
		switch {
		case isDLQSrc && lower == candidateLower:
			preferred = name
		case isSystemQueue(name):
			system = append(system, name)
		case isDLQQueue(name):
			dlqs = append(dlqs, name)
		default:
			regular = append(regular, name)
		}
	}

	sort.Strings(regular)
	sort.Strings(dlqs)
	sort.Strings(system)

	result := make([]string, 0, len(names))
	if preferred != "" {
		result = append(result, preferred)
	}
	result = append(result, regular...)
	result = append(result, dlqs...)
	result = append(result, system...)
	return result
}

// showMovePicker opens the queue-picker overlay for moving msg from sourceQueue.
// It immediately shows a "Loading…" placeholder, then asynchronously loads
// the queue list and replaces the placeholder with the real entries.
func (a *App) showMovePicker(sourceQueue string, msg queue.Message) {
	a.movePickerList.Clear()
	a.movePickerList.AddItem("Loading…", "", 0, nil)
	a.movePickerSearch.SetText("")

	a.movePickerList.SetInputCapture(func(event *tcell.EventKey) *tcell.EventKey {
		switch {
		case event.Rune() == 'j':
			return tcell.NewEventKey(tcell.KeyDown, 0, tcell.ModNone)
		case event.Rune() == 'k':
			return tcell.NewEventKey(tcell.KeyUp, 0, tcell.ModNone)
		case event.Rune() == '/':
			a.tv.SetFocus(a.movePickerSearch)
			return nil
		case event.Key() == tcell.KeyEscape:
			a.closeMovePicker()
			return nil
		}
		return event
	})

	// Register SetChangedFunc here so it closes over sourceQueue and msg.
	a.movePickerSearch.SetChangedFunc(func(text string) {
		a.fillPickerList(sourceQueue, msg, text)
	})

	a.rootPages.ShowPage("move-picker")
	a.tv.SetFocus(a.movePickerList)
	a.movePickerVisible = true
	ac := a.cfg.Colors.Accent
	a.contextPanel.SetText(fmt.Sprintf("[%s]<Esc>[-] cancel  [%s]</>[-] search", ac, ac))

	go func() {
		summaries, err := a.backend.List(context.Background())
		a.tv.QueueUpdateDraw(func() {
			if err != nil {
				slog.Error("move-picker: failed to list queues", "error", err)
				a.movePickerList.Clear()
				a.movePickerList.AddItem("Error loading queues", "", 0, nil)
				return
			}
			names := make([]string, 0, len(summaries))
			for _, s := range summaries {
				if s.Name != sourceQueue {
					names = append(names, s.Name)
				}
			}
			a.movePickerQueues = sortPickerQueues(sourceQueue, names)
			// Determine the preferred (DLQ-corresponding) queue for star display.
			a.movePickerPreferred = ""
			if strings.HasPrefix(strings.ToLower(sourceQueue), "dlq.") {
				candidate := strings.ToLower(sourceQueue[4:])
				for _, name := range names {
					if strings.ToLower(name) == candidate {
						a.movePickerPreferred = name
						break
					}
				}
			}
			a.fillPickerList(sourceQueue, msg, "")
		})
	}()
}

// fillPickerList repopulates movePickerList from movePickerQueues, keeping only
// entries whose name contains filter (case-insensitive; empty = show all).
func (a *App) fillPickerList(sourceQueue string, msg queue.Message, filter string) {
	a.movePickerList.Clear()
	lower := strings.ToLower(filter)
	for _, name := range a.movePickerQueues {
		if lower != "" && !strings.Contains(strings.ToLower(name), lower) {
			continue
		}
		n := name
		var displayName string
		switch {
		case n == a.movePickerPreferred:
			displayName = "⭐ " + n
		case isDLQQueue(n):
			displayName = "➖ " + n
		case isSystemQueue(n):
			displayName = "❓ " + n
		default:
			displayName = n
		}
		a.movePickerList.AddItem(displayName, "", 0, func() {
			a.closeMovePicker()
			go func() {
				err := a.backend.MoveMessage(context.Background(), sourceQueue, msg.ID, n)
				a.tv.QueueUpdateDraw(func() {
					if err != nil {
						slog.Error("move: failed", "src", sourceQueue, "dst", n, "id", msg.ID, "error", err)
						a.statusBar.SetText(fmt.Sprintf("[red]Error: %s[-]", err))
						return
					}
					a.pages.SwitchToPage("messages")
					a.tv.SetFocus(a.messagesV.table)
					lines := make([]string, 0, len(a.messagesV.Shortcuts()))
					for _, sc := range a.messagesV.Shortcuts() {
						lines = append(lines, fmt.Sprintf("[%s]<%s>[-] %s", a.cfg.Colors.Accent, sc.Key, sc.Description))
					}
					a.contextPanel.SetText(strings.Join(lines, "\n"))
					a.messagesV.load()
				})
			}()
		})
	}
}

// closeMovePicker hides the queue-picker overlay and restores focus and the
// context panel to the message detail view.
func (a *App) closeMovePicker() {
	a.rootPages.HidePage("move-picker")
	a.tv.SetFocus(a.pages)
	a.movePickerVisible = false
	lines := make([]string, 0, len(a.messageDetailV.Shortcuts()))
	for _, sc := range a.messageDetailV.Shortcuts() {
		lines = append(lines, fmt.Sprintf("[%s]<%s>[-] %s", a.cfg.Colors.Accent, sc.Key, sc.Description))
	}
	a.contextPanel.SetText(strings.Join(lines, "\n"))
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

// openMessages switches to the messages page for the given queue, sets the
// title, and starts loading messages asynchronously.
func (a *App) openMessages(queueName string) {
	a.messagesV.queueName = queueName
	a.messagesV.table.SetTitle(fmt.Sprintf(" Messages — %s ", queueName))
	a.messagesV.setHeader()
	a.pages.SwitchToPage("messages")
	a.tv.SetFocus(a.pages)
	// Show messagesV shortcuts in the context panel.
	lines := make([]string, 0, len(a.messagesV.Shortcuts()))
	for _, sc := range a.messagesV.Shortcuts() {
		lines = append(lines, fmt.Sprintf("[%s]<%s>[-] %s", a.cfg.Colors.Accent, sc.Key, sc.Description))
	}
	a.contextPanel.SetText(strings.Join(lines, "\n"))
	a.messagesV.load()
}

// openMessageDetail renders the full detail for msg and switches to the
// message-detail page.
func (a *App) openMessageDetail(queueName string, msg queue.Message) {
	a.messageDetailV.render(queueName, msg)
	a.messageDetailV.textView.SetTitle(fmt.Sprintf(" Message Details — %s ", queueName))
	a.pages.SwitchToPage("message-detail")
	a.tv.SetFocus(a.pages)
	lines := make([]string, 0, len(a.messageDetailV.Shortcuts()))
	for _, sc := range a.messageDetailV.Shortcuts() {
		lines = append(lines, fmt.Sprintf("[%s]<%s>[-] %s", a.cfg.Colors.Accent, sc.Key, sc.Description))
	}
	a.contextPanel.SetText(strings.Join(lines, "\n"))
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
