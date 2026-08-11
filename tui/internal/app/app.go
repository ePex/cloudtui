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
	"time"

	"github.com/gdamore/tcell/v2"
	"github.com/rivo/tview"

	"github.com/ePex/cloudtui/tui/internal/awsauth"
	"github.com/ePex/cloudtui/tui/internal/awslogs"
	"github.com/ePex/cloudtui/tui/internal/awsprofile"
	"github.com/ePex/cloudtui/tui/internal/awssecrets"
	"github.com/ePex/cloudtui/tui/internal/awsssm"
	"github.com/ePex/cloudtui/tui/internal/config"
	"github.com/ePex/cloudtui/tui/internal/queue"
	"github.com/ePex/cloudtui/tui/internal/queue/jolokia"
	"github.com/ePex/cloudtui/tui/internal/queue/proxy"
	"github.com/ePex/cloudtui/tui/internal/ui"
	"github.com/ePex/cloudtui/tui/internal/ui/views"
)

// App is the root of the TUI: it owns the tview.Application and routes
// command-prompt/hotkey input to the registered views.
type App struct {
	tv                     *tview.Application
	rootPages              *tview.Pages
	pages                  *tview.Pages
	topLeft                *tview.Pages
	prompt                 *tview.InputField
	helpVisible            bool
	views                  []ui.View
	cfg                    config.Config
	infoPanel              *tview.TextView
	divider                *tview.TextView
	contextPanel           *tview.TextView
	logoPanel              *tview.TextView
	statusBar              *tview.TextView
	settingsList           *tview.List
	logV                   *logView
	queuesV                *queuesView
	messagesV              *messagesView
	messageDetailV         *messageDetailView
	confirmFlex            *tview.Flex
	confirmText            *tview.TextView
	confirmList            *tview.List
	confirmVisible         bool
	movePickerFlex         *tview.Flex
	movePickerList         *tview.List
	movePickerSearch       *tview.InputField
	movePickerQueues       []string
	movePickerPreferred    string
	movePickerOnSelect     func(string)
	movePickerOnClose      func()
	movePickerVisible      bool
	sendMessageFlex        *tview.Flex
	sendMessageArea        *tview.TextArea
	sendMessageList        *tview.List
	sendMessageOnClose     func()
	sendMessageVisible     bool
	connManagerFlex        *tview.Flex
	connManagerList        *tview.List
	connManagerHints       *tview.TextView
	connManagerVisible     bool
	connEditorForm         *tview.Form
	connEditorVisible      bool
	connEditorIsNew        bool
	connEditorOrigName     string
	themePickerFlex        *tview.Flex
	themePickerList        *tview.List
	themePickerVisible     bool
	awsProfilesFlex        *tview.Flex
	awsProfilesTable       *tview.Table
	awsProfilesFilterInput *tview.InputField
	awsProfilesHints       *tview.TextView
	awsProfilesVisible     bool
	awsProfilesFilter      string
	awsProfilesAll         []awsprofile.Profile
	awsProfilesFiltered    []awsprofile.Profile
	backend                queue.Backend
	homeTable              *tview.Table
	homeSections           []views.SectionInfo
	topBarHeight           int
	listAWSProfiles        func(context.Context) ([]awsprofile.Profile, error)
	awsAuthTypeFor         func(ctx context.Context, profile string) (awsprofile.AuthType, error)
	awsSSOLogin            func(ctx context.Context, profile string) error
	ssmParamsV             *ssmParamsView
	secretsV               *secretsView
	paramDetailV           *paramDetailView
	secretDetailV          *secretDetailView
	logsV                  *logsView
	logSearchV             *logSearchView
	logDetailV             *logDetailView
	listParameters         func(ctx context.Context, profile, path string) ([]awsssm.Parameter, error)
	revealParameter        func(ctx context.Context, profile, name string) (string, error)
	listSecrets            func(ctx context.Context, profile string) ([]awssecrets.Secret, error)
	revealSecret           func(ctx context.Context, profile, name string) (value string, isBinary bool, err error)
	listLogGroups          func(ctx context.Context, profile string) ([]awslogs.LogGroup, error)
	filterLogEvents        func(ctx context.Context, profile, logGroupName string, start, end time.Time, pattern string) (events []awslogs.LogEvent, hasMore bool, err error)
	screen                 tcell.Screen
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
				{Name: "queues", Description: "List ActiveMQ queues"},
				{Name: "ssm-parameters", Description: "Browse AWS SSM parameters"},
				{Name: "secrets-manager", Description: "Browse AWS Secrets Manager secrets"},
				{Name: "cloudwatch-logs", Description: "Search CloudWatch Logs"},
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
		tv:           tview.NewApplication(),
		pages:        tview.NewPages(),
		cfg:          cfg,
		homeSections: homeSections,
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
	a.listAWSProfiles = awsprofile.List
	a.awsAuthTypeFor = awsprofile.AuthTypeFor
	a.awsSSOLogin = awsauth.Login
	a.listParameters = awsssm.List
	a.revealParameter = awsssm.Reveal
	a.listSecrets = awssecrets.List
	a.revealSecret = awssecrets.Reveal
	a.listLogGroups = awslogs.ListLogGroups
	a.filterLogEvents = awslogs.FilterEvents

	// tview.Application never exposes its tcell.Screen directly (no
	// GetScreen()); SetAfterDrawFunc is the only hook that hands it back,
	// so capture it once here for copyToClipboard's use. Reserved for this
	// purpose — if another feature ever needs an after-draw hook too, chain
	// it from here rather than overwriting.
	a.tv.SetAfterDrawFunc(func(screen tcell.Screen) {
		a.screen = screen
	})

	// All shell primitives are constructed; now create the settings view.
	// Its list callbacks call showThemePicker / showConnectionManager, which
	// are safe at this point because all live primitives are set.
	settingsView := newSettingsView(a)
	a.logV = newLogView(a)
	a.backend = newBackendForConn(cfg.ActiveConn())
	a.queuesV = newQueuesView(a, a.backend)
	a.messagesV = newMessagesView(a)
	a.messageDetailV = newMessageDetailView(a)
	a.ssmParamsV = newSSMParamsView(a)
	a.paramDetailV = newParamDetailView(a)
	a.secretsV = newSecretsView(a)
	a.logsV = newLogsView(a)
	a.logSearchV = newLogSearchView(a)
	a.logDetailV = newLogDetailView(a)

	// Wire Enter in the log groups table to open the search view for
	// the selected log group. Done here because logSearchV must exist first.
	a.logsV.table.SetSelectedFunc(func(row, _ int) {
		idx := row - 1 // row 0 is the header
		if idx < 0 || idx >= len(a.logsV.filtered) {
			return
		}
		a.openLogSearch(a.logsV.filtered[idx].Name)
	})

	// Wire Enter in the log search results table to open the detail view
	// for the selected event. Done here because logDetailV must exist first.
	a.logSearchV.table.SetSelectedFunc(func(row, _ int) {
		idx := row - 1 // row 0 is the header
		if idx < 0 || idx >= len(a.logSearchV.results) {
			return
		}
		a.openLogEventDetail(a.logSearchV.results[idx])
	})
	a.secretDetailV = newSecretDetailView(a)

	// Wire Enter in the SSM parameters table to open the detail view for
	// the selected parameter. Done here because paramDetailV must exist first.
	a.ssmParamsV.table.SetSelectedFunc(func(row, _ int) {
		idx := row - 1 // row 0 is the header
		if idx < 0 || idx >= len(a.ssmParamsV.filtered) {
			return
		}
		a.openParamDetail(a.ssmParamsV.filtered[idx])
	})

	// Wire Enter in the secrets table to open the detail view for the
	// selected secret. Done here because secretDetailV must exist first.
	a.secretsV.table.SetSelectedFunc(func(row, _ int) {
		idx := row - 1 // row 0 is the header
		if idx < 0 || idx >= len(a.secretsV.filtered) {
			return
		}
		a.openSecretDetail(a.secretsV.filtered[idx])
	})

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

	a.views = []ui.View{homeView, settingsView, a.logV, a.queuesV, a.ssmParamsV, a.secretsV, a.logsV}
	for _, v := range a.views {
		prim := v.Primitive()
		a.colorBordered(v, prim)
		a.pages.AddPage(v.Name(), prim, true, false)
	}
	// messages, message-detail, and param-detail pages: not in a.views (no
	// home entry / switchTo — opened directly via their own open* helper).
	a.pages.AddPage("messages", a.messagesV.table, true, false)
	a.pages.AddPage("message-detail", a.messageDetailV.textView, true, false)
	a.pages.AddPage("secret-detail", a.secretDetailV.textView, true, false)
	a.pages.AddPage("log-search", a.logSearchV.flex, true, false)
	a.pages.AddPage("log-event-detail", a.logDetailV.textView, true, false)
	a.pages.AddPage("ssm-param-detail", a.paramDetailV.textView, true, false)

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

	a.sendMessageArea = tview.NewTextArea()
	a.sendMessageList = tview.NewList().ShowSecondaryText(false)
	a.sendMessageList.AddItem("Submit", "", 0, nil)
	a.sendMessageList.AddItem("Cancel", "", 0, nil)
	a.sendMessageFlex = tview.NewFlex().SetDirection(tview.FlexRow).
		AddItem(a.sendMessageArea, 0, 1, true).
		AddItem(a.sendMessageList, 2, 0, false)
	a.sendMessageFlex.SetBorder(true).SetTitle(" Send Message ")

	a.sendMessageArea.SetInputCapture(func(event *tcell.EventKey) *tcell.EventKey {
		if event.Key() == tcell.KeyTab {
			a.tv.SetFocus(a.sendMessageList)
			return nil
		}
		if event.Key() == tcell.KeyEscape {
			a.closeSendMessage()
			return nil
		}
		return event
	})
	a.sendMessageList.SetInputCapture(func(event *tcell.EventKey) *tcell.EventKey {
		switch {
		case event.Rune() == 'j':
			return tcell.NewEventKey(tcell.KeyDown, 0, tcell.ModNone)
		case event.Rune() == 'k':
			return tcell.NewEventKey(tcell.KeyUp, 0, tcell.ModNone)
		case event.Key() == tcell.KeyEscape:
			a.tv.SetFocus(a.sendMessageArea)
			return nil
		}
		return event
	})

	sendMessageOverlay := centered(a.sendMessageFlex, 70, 14)

	// Connection manager overlay
	a.connManagerList = tview.NewList().ShowSecondaryText(false)
	a.connManagerHints = tview.NewTextView().SetDynamicColors(true)
	a.connManagerFlex = tview.NewFlex().SetDirection(tview.FlexRow).
		AddItem(a.connManagerList, 0, 1, true).
		AddItem(a.connManagerHints, 2, 0, false)
	a.connManagerFlex.SetBorder(true).SetTitle(" Connections ")
	connManagerOverlay := centered(a.connManagerFlex, 64, 20)

	a.connManagerList.SetInputCapture(func(event *tcell.EventKey) *tcell.EventKey {
		switch {
		case event.Key() == tcell.KeyEscape:
			a.closeConnManager()
			return nil
		case event.Rune() == 'n':
			a.showConnEditor(config.Connection{}, true, "")
			return nil
		case event.Rune() == 'e':
			idx := a.connManagerList.GetCurrentItem()
			if idx >= 0 && idx < len(a.cfg.Connections) {
				c := a.cfg.Connections[idx]
				a.showConnEditor(c, false, c.Name)
			}
			return nil
		case event.Rune() == 'd':
			idx := a.connManagerList.GetCurrentItem()
			if idx >= 0 && idx < len(a.cfg.Connections) {
				dup := a.cfg.Connections[idx]
				dup.Name = dup.Name + "-copy"
				al := dup.Alias + "2"
				if len([]rune(al)) > 8 {
					al = string([]rune(al)[:8])
				}
				dup.Alias = al
				a.showConnEditor(dup, true, "")
			}
			return nil
		case event.Key() == tcell.KeyDelete || event.Rune() == 'x':
			a.deleteConnFromManager()
			return nil
		case event.Rune() == 'j':
			return tcell.NewEventKey(tcell.KeyDown, 0, tcell.ModNone)
		case event.Rune() == 'k':
			return tcell.NewEventKey(tcell.KeyUp, 0, tcell.ModNone)
		}
		return event
	})

	a.connManagerList.SetSelectedFunc(func(idx int, _ string, _ string, _ rune) {
		if idx >= 0 && idx < len(a.cfg.Connections) {
			name := a.cfg.Connections[idx].Name
			a.closeConnManager()
			a.switchConnection(name)
		}
	})

	// Connection editor overlay
	a.connEditorForm = tview.NewForm()
	a.connEditorForm.SetBorder(true).SetTitle(" Connection ")
	a.connEditorForm.
		AddInputField("Name", "", 30, nil, nil).
		AddInputField("Alias", "", 10, nil, nil).
		AddDropDown("Backend", []string{"jolokia", "proxy"}, 0, nil).
		AddInputField("Broker Name", "", 30, nil, nil).
		AddInputField("URL", "", 40, nil, nil).
		AddInputField("Username", "", 20, nil, nil).
		AddPasswordField("Password", "", 20, '*', nil).
		AddButton("Save", func() { a.saveConnEditor() }).
		AddButton("Cancel", func() { a.closeConnEditor() })
	if dd, ok := a.connEditorForm.GetFormItem(2).(*tview.DropDown); ok {
		styleDropDown(dd, cfg.Colors)
	}
	a.connEditorForm.SetInputCapture(func(event *tcell.EventKey) *tcell.EventKey {
		if event.Key() == tcell.KeyEscape {
			a.closeConnEditor()
			return nil
		}
		return event
	})
	// Height must cover border+padding (4 rows) + 7 items * (field + item
	// padding) (14 rows) + button row (1 row) = 19; give it one spare row.
	connEditorOverlay := centered(a.connEditorForm, 64, 20)

	// Theme picker overlay
	a.themePickerList = tview.NewList().ShowSecondaryText(false)
	a.themePickerFlex = tview.NewFlex().SetDirection(tview.FlexRow).
		AddItem(a.themePickerList, 0, 1, true)
	a.themePickerFlex.SetBorder(true).SetTitle(" Theme ")
	a.themePickerList.SetInputCapture(func(event *tcell.EventKey) *tcell.EventKey {
		switch {
		case event.Key() == tcell.KeyEscape:
			a.closeThemePicker()
			return nil
		case event.Rune() == 'j':
			return tcell.NewEventKey(tcell.KeyDown, 0, tcell.ModNone)
		case event.Rune() == 'k':
			return tcell.NewEventKey(tcell.KeyUp, 0, tcell.ModNone)
		}
		return event
	})
	themePickerOverlay := centered(a.themePickerFlex, 40, 14)

	// AWS Profiles overlay — filterable; Enter activates the selected profile.
	a.awsProfilesTable = tview.NewTable()
	a.awsProfilesTable.SetBorder(true).SetTitle(" AWS Profiles ")
	a.awsProfilesTable.SetSelectable(true, false)
	a.awsProfilesTable.SetFixed(1, 0)
	a.awsProfilesFilterInput = tview.NewInputField()
	a.awsProfilesFilterInput.SetLabel(" / filter: ")
	a.awsProfilesFilterInput.SetLabelColor(tcell.GetColor(cfg.Colors.Label))
	a.awsProfilesFilterInput.SetFieldBackgroundColor(tcell.GetColor(cfg.Colors.SelectionBg))
	a.awsProfilesFilterInput.SetFieldTextColor(tcell.GetColor(cfg.Colors.SelectionText))
	a.awsProfilesHints = tview.NewTextView().SetDynamicColors(true)
	a.awsProfilesHints.SetText(fmt.Sprintf("[%s]<Enter>[-] activate  [%s]<r>[-] refresh  [%s]</>[-] filter  [%s]<Esc>[-] close",
		cfg.Colors.Accent, cfg.Colors.Accent, cfg.Colors.Accent, cfg.Colors.Accent))
	a.awsProfilesFlex = tview.NewFlex().SetDirection(tview.FlexRow).
		AddItem(a.awsProfilesTable, 0, 1, true).
		AddItem(a.awsProfilesFilterInput, 1, 0, false).
		AddItem(a.awsProfilesHints, 1, 0, false)
	a.setAWSProfilesHeader()

	a.awsProfilesTable.SetSelectedFunc(func(row, _ int) {
		idx := row - 1
		if idx < 0 || idx >= len(a.awsProfilesFiltered) {
			return
		}
		a.activateAWSProfile(a.awsProfilesFiltered[idx].Name)
	})
	a.awsProfilesTable.SetInputCapture(func(event *tcell.EventKey) *tcell.EventKey {
		switch {
		case event.Key() == tcell.KeyEscape:
			a.closeAWSProfiles()
			return nil
		case event.Rune() == 'r':
			a.populateAWSProfilesTable()
			return nil
		case event.Rune() == '/':
			a.awsProfilesFilterInput.SetText(a.awsProfilesFilter)
			a.tv.SetFocus(a.awsProfilesFilterInput)
			return nil
		case event.Rune() == 'j':
			return tcell.NewEventKey(tcell.KeyDown, 0, tcell.ModNone)
		case event.Rune() == 'k':
			return tcell.NewEventKey(tcell.KeyUp, 0, tcell.ModNone)
		}
		return event
	})
	a.awsProfilesFilterInput.SetChangedFunc(func(text string) {
		a.applyAWSProfilesFilter(text)
	})
	a.awsProfilesFilterInput.SetDoneFunc(func(_ tcell.Key) {
		a.applyAWSProfilesFilter(a.awsProfilesFilterInput.GetText())
		a.tv.SetFocus(a.awsProfilesTable)
	})
	a.awsProfilesFilterInput.SetInputCapture(func(event *tcell.EventKey) *tcell.EventKey {
		if event.Key() == tcell.KeyUp || event.Key() == tcell.KeyDown {
			a.applyAWSProfilesFilter(a.awsProfilesFilterInput.GetText())
			a.tv.SetFocus(a.awsProfilesTable)
			a.awsProfilesTable.InputHandler()(event, func(tview.Primitive) {})
			return nil
		}
		return event
	})
	awsProfilesOverlay := centered(a.awsProfilesFlex, 64, 20)

	helpOverlay := centered(newHelpModal(cfg), helpModalWidth, helpModalHeight)
	// "confirm" is added last so it always draws on top of any other overlay
	// (e.g. conn-manager) that may still be visible underneath it — tview.Pages
	// draws pages in AddPage order, later pages on top.
	a.rootPages = tview.NewPages().
		AddPage("main", layout, true, true).
		AddPage("help", helpOverlay, true, false).
		AddPage("move-picker", movePickerOverlay, true, false).
		AddPage("send-message", sendMessageOverlay, true, false).
		AddPage("conn-manager", connManagerOverlay, true, false).
		AddPage("conn-editor", connEditorOverlay, true, false).
		AddPage("theme-picker", themePickerOverlay, true, false).
		AddPage("aws-profiles", awsProfilesOverlay, true, false).
		AddPage("confirm", confirmOverlay, true, false)

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
	if a.ssmParamsV != nil && a.tv.GetFocus() == a.ssmParamsV.filterInput {
		return event
	}
	if a.secretsV != nil && a.tv.GetFocus() == a.secretsV.filterInput {
		return event
	}
	if a.logsV != nil && a.tv.GetFocus() == a.logsV.filterInput {
		return event
	}
	if a.logSearchV != nil && a.tv.GetFocus() == a.logSearchV.patternInput {
		return event
	}

	if a.confirmVisible || a.movePickerVisible || a.sendMessageVisible || a.connManagerVisible || a.connEditorVisible || a.themePickerVisible || a.awsProfilesVisible {
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
		// Commands that open a rootPages overlay (e.g. "aq") already set
		// focus on that overlay's primitive; a.pages is the underlying main
		// view, so focusing it here would steal focus out from under the
		// overlay even though it's still the frontmost visible page.
		if !a.connManagerVisible && !a.connEditorVisible && !a.themePickerVisible &&
			!a.confirmVisible && !a.movePickerVisible && !a.sendMessageVisible && !a.awsProfilesVisible {
			a.tv.SetFocus(a.pages)
		}
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
	case cmd == "aq" || cmd == "connections":
		a.showConnectionManager()
	case cmd == "ap" || cmd == "awsprofiles":
		a.showAWSProfiles()
	case strings.HasPrefix(cmd, "theme "):
		a.switchTheme(strings.TrimPrefix(cmd, "theme "))
	default:
		a.switchTo(cmd)
	}
}

// switchTheme applies the named built-in palette, saves config, and repaints
// all live shell primitives. Unknown names are silently ignored.
//
// Only the theme name (and any pre-existing user color overrides) are written
// to config.yaml — the full derived palette is never persisted.
func (a *App) switchTheme(name string) {
	newBase, ok := config.PaletteForTheme(name)
	if !ok {
		return
	}

	// Preserve any color overrides the user had relative to the old theme.
	oldBase, ok := config.PaletteForTheme(a.cfg.Theme)
	if !ok {
		oldBase, _ = config.PaletteForTheme("dark")
	}
	userOverrides := config.PaletteUserOverrides(a.cfg.Colors, oldBase)

	a.cfg.Theme = name
	a.cfg.Colors = config.ApplyPaletteOverrides(newBase, userOverrides)
	reapplyTheme(a, a.cfg.Colors)
	a.refreshSettingsList()

	// Save only theme name + sparse overrides (not the full derived palette).
	saveConfig := a.cfg
	saveConfig.Colors = userOverrides
	if err := config.SaveDefault(saveConfig); err != nil {
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

// isIMQQueue reports whether name is an in-memory / internal queue (imq.*).
func isIMQQueue(name string) bool {
	return strings.HasPrefix(strings.ToLower(name), "imq.")
}

// requeueQueueCandidate returns the non-prefixed counterpart of sourceQueue if
// it has a requeue prefix (dlq. or imq.), otherwise "".
// E.g. "dlq.foo.bar" → "foo.bar", "imq.foo.bar" → "foo.bar".
func requeueQueueCandidate(sourceQueue string) string {
	lower := strings.ToLower(sourceQueue)
	if strings.HasPrefix(lower, "dlq.") || strings.HasPrefix(lower, "imq.") {
		return sourceQueue[4:]
	}
	return ""
}

// sortPickerQueues orders names into four tiers:
//  1. Preferred — the non-prefixed counterpart when source is a dlq.* or imq.*
//     queue (e.g. "dlq.foo" → "foo", "imq.foo" → "foo"). Pinned first with ⭐.
//  2. Regular — all other queues, alphabetical.
//  3. Requeue — dlq.* and imq.* queues, alphabetical (➖).
//  4. System — activemq.* and statistics.* queues, alphabetical (❓).
func sortPickerQueues(sourceQueue string, names []string) []string {
	var preferred string
	var regular, requeue, system []string

	candidateLower := strings.ToLower(requeueQueueCandidate(sourceQueue))

	for _, name := range names {
		lower := strings.ToLower(name)
		switch {
		case candidateLower != "" && lower == candidateLower:
			preferred = name
		case isSystemQueue(name):
			system = append(system, name)
		case isDLQQueue(name) || isIMQQueue(name):
			requeue = append(requeue, name)
		default:
			regular = append(regular, name)
		}
	}

	sort.Strings(regular)
	sort.Strings(requeue)
	sort.Strings(system)

	result := make([]string, 0, len(names))
	if preferred != "" {
		result = append(result, preferred)
	}
	result = append(result, regular...)
	result = append(result, requeue...)
	result = append(result, system...)
	return result
}

// showMovePicker opens the queue-picker overlay. onSelect is called (in a new
// goroutine) with the chosen target queue name when the user makes a selection.
// onClose is called (on the UI goroutine) when the picker is dismissed — the
// caller uses it to restore focus and the context panel.
func (a *App) showMovePicker(sourceQueue string, onSelect func(string), onClose func()) {
	a.movePickerOnSelect = onSelect
	a.movePickerOnClose = onClose
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

	a.movePickerSearch.SetChangedFunc(func(text string) {
		a.fillPickerList(text)
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
			// Determine the preferred (requeue-corresponding) queue for ⭐ display.
			a.movePickerPreferred = ""
			if candidate := strings.ToLower(requeueQueueCandidate(sourceQueue)); candidate != "" {
				for _, name := range names {
					if strings.ToLower(name) == candidate {
						a.movePickerPreferred = name
						break
					}
				}
			}
			a.fillPickerList("")
		})
	}()
}

// fillPickerList repopulates movePickerList from movePickerQueues, keeping only
// entries whose name contains filter (case-insensitive; empty = show all).
// Each item's selected func invokes a.movePickerOnSelect in a new goroutine.
func (a *App) fillPickerList(filter string) {
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
		case isDLQQueue(n) || isIMQQueue(n):
			displayName = "➖ " + n
		case isSystemQueue(n):
			displayName = "❓ " + n
		default:
			displayName = n
		}
		a.movePickerList.AddItem(displayName, "", 0, func() {
			a.closeMovePicker()
			go a.movePickerOnSelect(n)
		})
	}
}

// closeMovePicker hides the queue-picker overlay and calls movePickerOnClose
// to let the caller restore focus and the context panel.
func (a *App) closeMovePicker() {
	a.rootPages.HidePage("move-picker")
	a.movePickerVisible = false
	if a.movePickerOnClose != nil {
		a.movePickerOnClose()
	}
}

// showSendMessage opens the send-message overlay for the given queue.
// onClose is called on the UI goroutine when the overlay is dismissed.
func (a *App) showSendMessage(queueName string, onClose func()) {
	a.sendMessageOnClose = onClose
	a.sendMessageArea.SetText("", true)
	a.sendMessageFlex.SetTitle(fmt.Sprintf(" Send Message — %s ", queueName))

	// Wire Submit and Cancel with the correct closure each time.
	a.sendMessageList.Clear()
	a.sendMessageList.AddItem("Submit", "", 0, func() { a.doSend(queueName) })
	a.sendMessageList.AddItem("Cancel", "", 0, a.closeSendMessage)

	a.rootPages.ShowPage("send-message")
	a.tv.SetFocus(a.sendMessageArea)
	a.sendMessageVisible = true
	ac := a.cfg.Colors.Accent
	a.contextPanel.SetText(fmt.Sprintf("[%s]<Tab>[-] actions  [%s]<Esc>[-] cancel", ac, ac))
}

// doSend reads the body from sendMessageArea, closes the overlay, and sends
// the message asynchronously, reporting the result in the status bar.
func (a *App) doSend(queueName string) {
	body := a.sendMessageArea.GetText()
	a.closeSendMessage()
	go func() {
		err := a.backend.SendMessage(context.Background(), queueName, body)
		a.tv.QueueUpdateDraw(func() {
			if err != nil {
				slog.Error("send message: failed", "queue", queueName, "error", err)
				a.statusBar.SetText(fmt.Sprintf("[red]Error: %s[-]", err))
				return
			}
			a.statusBar.SetText(fmt.Sprintf("Message sent to %q", queueName))
			if a.queuesV != nil {
				a.queuesV.load()
			}
			if a.messagesV != nil && a.messagesV.queueName == queueName {
				a.messagesV.load()
			}
		})
	}()
}

// closeSendMessage hides the send-message overlay and calls sendMessageOnClose
// to let the caller restore focus and the context panel.
func (a *App) closeSendMessage() {
	a.rootPages.HidePage("send-message")
	a.sendMessageVisible = false
	if a.sendMessageOnClose != nil {
		a.sendMessageOnClose()
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

// newBackendForConn constructs the appropriate queue.Backend for conn.
func newBackendForConn(conn config.Connection) queue.Backend {
	if conn.Backend == "proxy" {
		return proxy.NewClient(conn.Proxy)
	}
	return jolokia.NewClient(conn.Queue)
}

// switchConnection activates the named connection: reinitialises the backend,
// updates the info panel, navigates to the queues view, and persists the config.
func (a *App) switchConnection(name string) {
	var conn config.Connection
	found := false
	for _, c := range a.cfg.Connections {
		if c.Name == name {
			conn = c
			found = true
			break
		}
	}
	if !found {
		return
	}
	a.cfg.ActiveConnection = name
	a.backend = newBackendForConn(conn)
	a.queuesV.backend = a.backend
	a.infoPanel.SetText(infoPanelText(a.cfg))
	a.refreshSettingsList()
	a.switchTo("queues")
	if err := config.SaveDefault(a.cfg); err != nil {
		slog.Error("switchConnection: save failed", "error", err)
	}
}

// switchTo activates the named view if it is registered, updates the top
// bar's context panel, and calls Activate() if the view implements activatable.
func (a *App) switchTo(name string) {
	for _, v := range a.views {
		if v.Name() == name {
			a.pages.SwitchToPage(name)
			a.tv.SetFocus(v.Primitive())
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

// openParamDetail renders the full detail for param and switches to the
// ssm-param-detail page. paramDetailView.render sets the context panel
// itself (its shortcuts change once a SecureString is revealed).
func (a *App) openParamDetail(param awsssm.Parameter) {
	a.paramDetailV.render(param)
	a.paramDetailV.textView.SetTitle(fmt.Sprintf(" Parameter — %s ", param.Name))
	a.pages.SwitchToPage("ssm-param-detail")
	a.tv.SetFocus(a.pages)
}

func (a *App) openSecretDetail(secret awssecrets.Secret) {
	a.secretDetailV.render(secret)
	a.secretDetailV.textView.SetTitle(fmt.Sprintf(" Secret — %s ", secret.Name))
	a.pages.SwitchToPage("secret-detail")
	a.tv.SetFocus(a.pages)
}

// openLogSearch opens the search view for logGroupName and runs the
// first search immediately (see logSearchView.open). logSearchView isn't
// a registered ui.View, so its context panel is populated manually here
// — same pattern as openMessages.
func (a *App) openLogSearch(logGroupName string) {
	a.logSearchV.open(logGroupName)
	a.pages.SwitchToPage("log-search")
	a.tv.SetFocus(a.pages)
	lines := make([]string, 0, len(a.logSearchV.Shortcuts()))
	for _, sc := range a.logSearchV.Shortcuts() {
		lines = append(lines, fmt.Sprintf("[%s]<%s>[-] %s", a.cfg.Colors.Accent, sc.Key, sc.Description))
	}
	a.contextPanel.SetText(strings.Join(lines, "\n"))
}

// openLogEventDetail renders the full detail for event and switches to
// the log-event-detail page.
func (a *App) openLogEventDetail(event awslogs.LogEvent) {
	a.logDetailV.render(event)
	a.pages.SwitchToPage("log-event-detail")
	a.tv.SetFocus(a.pages)
}

// copyToClipboard writes data to the system clipboard via the terminal's
// OSC 52 escape sequence (tcell.Screen.SetClipboard) rather than shelling
// out to pbcopy/xclip/clip.exe: no new dependency, and it works the same
// over SSH as it does locally. a.screen is nil until the first draw (see
// New()'s SetAfterDrawFunc), which in practice means before Run() starts —
// a no-op then is correct since there's nothing on screen to have copied.
func (a *App) copyToClipboard(data string) {
	if a.screen == nil {
		return
	}
	a.screen.SetClipboard([]byte(data))
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
