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
	"strings"
	"time"

	"github.com/gdamore/tcell/v2"
	"github.com/rivo/tview"

	"github.com/ePex/cloudtui/tui/internal/awsauth"
	"github.com/ePex/cloudtui/tui/internal/awscodepipeline"
	"github.com/ePex/cloudtui/tui/internal/awslogs"
	"github.com/ePex/cloudtui/tui/internal/awsprofile"
	"github.com/ePex/cloudtui/tui/internal/awssecrets"
	"github.com/ePex/cloudtui/tui/internal/awsssm"
	"github.com/ePex/cloudtui/tui/internal/config"
	"github.com/ePex/cloudtui/tui/internal/datadoglogs"
	"github.com/ePex/cloudtui/tui/internal/dialog"
	"github.com/ePex/cloudtui/tui/internal/queue"
	"github.com/ePex/cloudtui/tui/internal/queue/secretbackend"
	"github.com/ePex/cloudtui/tui/internal/ui"
	"github.com/ePex/cloudtui/tui/internal/ui/views"
	"github.com/ePex/cloudtui/tui/internal/view"
)

// App is the root of the TUI: it owns the tview.Application and routes
// command-prompt/hotkey input to the registered views.
type App struct {
	tv          *tview.Application
	rootPages   *tview.Pages
	pages       *tview.Pages
	topLeft     *tview.Pages
	prompt      *tview.InputField
	helpVisible bool
	// focusExemptInputs are input fields that swallow global hotkeys while
	// focused (e.g. so typing "s" in a search box doesn't trigger the "s"
	// hotkey). Populated once in New(), after every view exists.
	focusExemptInputs []tview.Primitive
	// overlayVisible holds every modal overlay via its Visible() accessor;
	// anyOverlayVisible loops over it instead of a hand-maintained OR-chain.
	overlayVisible []visibler
	// themables is every view/overlay that recolors itself on a live theme
	// switch — reapplyTheme loops over this instead of naming each type.
	themables      []ui.Themeable
	views          []ui.View
	cfg            config.Config
	infoPanel      *tview.TextView
	divider        *tview.TextView
	contextPanel   *tview.TextView
	logoPanel      *tview.TextView
	statusBar      *tview.TextView
	settingsV      *view.SettingsView
	logV           *view.LogView
	queuesV        *view.QueuesView
	messagesV      *view.MessagesView
	messageDetailV *view.MessageDetailView
	confirm        *dialog.ConfirmDialog
	movePicker     *dialog.MovePicker
	sendMessage    *dialog.SendMessageOverlay
	connManager    *dialog.ConnManager
	connEditor     *dialog.ConnEditor
	messageFilter  *dialog.MessageFilter
	jmsTypePrompt  *dialog.JMSTypePrompt
	timeRangeModal *dialog.TimeRangeModal
	datadogEditor  *dialog.DatadogEditor
	// pendingCloudWatchPattern is a one-shot CorrelationID queued by
	// FE 41's Datadog->CloudWatch jump, consumed by OpenLogSearch and
	// dropped by SwitchTo if abandoned — see spec/41.
	// pendingCloudWatchTimestamp is the originating Datadog event's
	// timestamp, queued/consumed/dropped alongside it — see
	// spec-origin/91-bugfix-correlation-jump-timerange.
	pendingCloudWatchPattern   string
	pendingCloudWatchTimestamp time.Time
	themePicker                *dialog.ThemePicker
	awsProfiles                *dialog.AWSProfilesPicker
	backend                    queue.Backend
	homeTable                  *tview.Table
	homeSections               []views.SectionInfo
	topBarHeight               int
	listAWSProfiles            func(context.Context) ([]awsprofile.Profile, error)
	awsAuthTypeFor             func(ctx context.Context, profile string) (awsprofile.AuthType, error)
	awsSSOLogin                func(ctx context.Context, profile string, onCode func(code, url string)) error
	ssmParamsV                 *view.SSMParamsView
	secretsV                   *view.SecretsView
	paramDetailV               *view.ParamDetailView
	secretDetailV              *view.SecretDetailView
	logsV                      *view.LogsView
	logSearchV                 *view.LogSearchView
	logDetailV                 *view.LogDetailView
	datadogLogsV               *view.DatadogLogsView
	datadogLogDetailV          *view.DatadogLogDetailView
	codePipelineListV          *view.CodePipelineListView
	codePipelineDetailV        *view.CodePipelineDetailView
	pipelineWatcher            *view.PipelineWatcher
	listParameters             func(ctx context.Context, profile, path string) ([]awsssm.Parameter, error)
	revealParameter            func(ctx context.Context, profile, name string) (string, error)
	listSecrets                func(ctx context.Context, profile string) ([]awssecrets.Secret, error)
	revealSecret               func(ctx context.Context, profile, name string) (value string, isBinary bool, err error)
	listLogGroups              func(ctx context.Context, profile string) ([]awslogs.LogGroup, error)
	filterLogEvents            func(ctx context.Context, profile, logGroupName string, start, end time.Time, pattern, nextToken string) (events []awslogs.LogEvent, next string, err error)
	searchDatadogLogs          func(ctx context.Context, cfg config.DatadogConfig, query string, from, to time.Time) (events []datadoglogs.LogEvent, hasMore bool, err error)
	listDatadogFacetValues     func(ctx context.Context, cfg config.DatadogConfig, facet string, from, to time.Time) ([]string, error)
	listPipelines              func(ctx context.Context, profile string) ([]awscodepipeline.Pipeline, error)
	getPipelineState           func(ctx context.Context, profile, pipelineName string) ([]awscodepipeline.StageStatus, error)
	notify                     func(title, message string)
	screen                     tcell.Screen
	// secretResolver resolves AWS-Secrets-Manager-backed connection
	// passwords (see internal/queue/secretbackend /
	// spec/56-fe-amq-connection-aws-secret-password).
	secretResolver *secretbackend.SecretResolver
}

// New builds the app shell with cfg as the starting configuration.
// applyTheme is called first, before any primitive is constructed, so
// tview.Styles are set before primitives read them at construction time.
func New(cfg config.Config) *App {
	applyTheme(cfg.Colors)

	homeSections := []views.SectionInfo{
		{
			Title: "ActiveMQ",
			Entries: []views.ViewInfo{
				{Name: "queues", Description: "List ActiveMQ queues"},
			},
		},
		{
			Title: "AWS",
			Entries: []views.ViewInfo{
				{Name: "ssm-parameters", Description: "Browse AWS SSM parameters"},
				{Name: "secrets-manager", Description: "Browse AWS Secrets Manager secrets"},
				{Name: "cloudwatch-logs", Description: "Search CloudWatch Logs"},
				{Name: "codepipeline", Description: "Monitor AWS CodePipeline pipelines"},
			},
		},
		{
			Title: "Datadog",
			Entries: []views.ViewInfo{
				{Name: "datadog-logs", Description: "Search Datadog Logs"},
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

	homeView := views.NewHome(homeSections, a.SwitchTo, cfg.Colors.Label, cfg.Colors.Text, cfg.Colors.Border, cfg.Colors.SelectionBg, cfg.Colors.SelectionText)
	a.homeTable, _ = homeView.Primitive().(*tview.Table)

	a.prompt = tview.NewInputField().
		SetLabel(" :").
		SetFieldBackgroundColor(tcell.ColorDefault)
	a.prompt.SetDoneFunc(a.onPromptDone)
	// StyleInputFieldAutocomplete must run before SetAutocompleteFunc:
	// SetAutocompleteFunc eagerly calls Autocomplete(), which lazily builds
	// and permanently styles the drop-down's internal tview.List using
	// whatever autocompleteStyles are set at that exact moment — later
	// Autocomplete() calls only refresh the list's entries, never its
	// style. Styling first ensures the very first drop-down uses the
	// theme's colors instead of tview's unreadable built-in defaults.
	ui.StyleInputFieldAutocomplete(a.prompt, cfg.Colors)
	a.prompt.SetAutocompleteFunc(a.promptSuggestions)

	tb := ui.NewTopBar(cfg, a.prompt)
	a.topLeft = tb.Left
	a.infoPanel = tb.Info
	a.divider = tb.Divider
	a.contextPanel = tb.ContextPanel
	a.logoPanel = tb.Logo
	a.topBarHeight = tb.Height

	a.statusBar = ui.NewStatusBar(cfg)
	a.listAWSProfiles = awsprofile.List
	a.awsAuthTypeFor = awsprofile.AuthTypeFor
	a.awsSSOLogin = awsauth.Login
	a.listParameters = awsssm.List
	a.revealParameter = awsssm.Reveal
	a.listSecrets = awssecrets.List
	a.revealSecret = awssecrets.Reveal
	a.listLogGroups = awslogs.ListLogGroups
	a.filterLogEvents = awslogs.FilterEvents
	a.searchDatadogLogs = datadoglogs.Search
	a.listDatadogFacetValues = datadoglogs.ListFacetValues
	a.listPipelines = awscodepipeline.ListPipelines
	a.getPipelineState = awscodepipeline.GetPipelineState
	a.notify = ui.DesktopNotify
	a.secretResolver = secretbackend.NewSecretResolver(a.revealSecret, a.AWSAuthTypeFor, a.AWSSSOLogin,
		func() { a.QueueUpdateDraw(func() { a.showReauthWaiting(awsSSOReauthWaitingMsg) }) },
		func() { a.QueueUpdateDraw(a.showReauthDone) },
		func(code, url string) {
			a.QueueUpdateDraw(func() {
				a.showReauthWaiting(fmt.Sprintf("%s Verify code %s at %s", awsSSOReauthWaitingMsg, code, url))
			})
		},
	)

	// tview.Application never exposes its tcell.Screen directly (no
	// GetScreen()); SetAfterDrawFunc is the only hook that hands it back,
	// so capture it once here for CopyToClipboard's use. Reserved for this
	// purpose — if another feature ever needs an after-draw hook too, chain
	// it from here rather than overwriting.
	a.tv.SetAfterDrawFunc(func(screen tcell.Screen) {
		a.screen = screen
	})

	// All shell primitives are constructed.
	a.logV = view.NewLogView()
	a.backend = secretbackend.New(a.secretResolver, cfg.ActiveConn())

	// Dialogs used directly by the views below (rather than through one
	// of internal/ui's per-resource host interfaces — see spec/80/83
	// and spec/03's "ViewHost deliberately doesn't expose dialog
	// access") must exist before those views are constructed, since
	// each takes the dialogs it needs as constructor parameters. Their
	// overlay Primitives are still wired up in their original position
	// further down.
	a.confirm = dialog.NewConfirmDialog(a)
	a.movePicker = dialog.NewMovePicker(a)
	a.sendMessage = dialog.NewSendMessageOverlay(a)
	a.messageFilter = dialog.NewMessageFilter(a)
	a.jmsTypePrompt = dialog.NewJMSTypePrompt(a)
	a.timeRangeModal = dialog.NewTimeRangeModal(a)
	a.connManager = dialog.NewConnManager(a, a.confirm)
	a.datadogEditor = dialog.NewDatadogEditor(a)
	a.themePicker = dialog.NewThemePicker(a)
	a.awsProfiles = dialog.NewAWSProfilesPicker(a)

	a.settingsV = view.NewSettingsView(a, a.themePicker, a.connManager, a.awsProfiles, a.datadogEditor)

	a.queuesV = view.NewQueuesView(a, a.backend, a.confirm, a.movePicker, a.sendMessage, a.jmsTypePrompt, a.OpenMessages)
	a.messagesV = view.NewMessagesView(a, a.messageFilter, a.sendMessage, a.confirm, a.movePicker, a.OpenMessageDetail)
	a.messageDetailV = view.NewMessageDetailView(a, a.movePicker, a.confirm,
		func() {
			a.pages.SwitchToPage("messages")
			a.tv.SetFocus(a.messagesV.Table())
			lines := make([]string, 0, len(a.messagesV.Shortcuts()))
			for _, sc := range a.messagesV.Shortcuts() {
				lines = append(lines, fmt.Sprintf("[%s]<%s>[-] %s", a.Config().Colors.Accent, sc.Key, sc.Description))
			}
			a.SetContextHint(strings.Join(lines, "\n"))
		},
		func() { a.messagesV.Load() },
	)
	a.ssmParamsV = view.NewSSMParamsView(a, a.OpenParamDetail)
	a.paramDetailV = view.NewParamDetailView(a, func() {
		a.pages.SwitchToPage("ssm-parameters")
		a.tv.SetFocus(a.ssmParamsV.Table())
		a.UpdateContextPanel(a.ssmParamsV)
	})
	a.secretsV = view.NewSecretsView(a, a.OpenSecretDetail)
	a.logsV = view.NewLogsView(a, a.OpenLogSearch)
	a.logSearchV = view.NewLogSearchView(a, a.timeRangeModal, a.OpenLogEventDetail, func() {
		a.pages.SwitchToPage("cloudwatch-logs")
		a.tv.SetFocus(a.logsV.Table())
		a.UpdateContextPanel(a.logsV)
	})
	a.logDetailV = view.NewLogDetailView(a, func() {
		a.pages.SwitchToPage("log-search")
		a.tv.SetFocus(a.logSearchV.Table())
		// logSearchView isn't a registered ui.View (opened directly,
		// like messagesView), so it can't go through the generic
		// UpdateContextPanel(ui.View) path — same manual pattern
		// OpenLogSearch uses.
		lines := make([]string, 0, len(a.logSearchV.Shortcuts()))
		for _, sc := range a.logSearchV.Shortcuts() {
			lines = append(lines, fmt.Sprintf("[%s]<%s>[-] %s", a.Config().Colors.Accent, sc.Key, sc.Description))
		}
		a.SetContextHint(strings.Join(lines, "\n"))
	})
	a.datadogLogsV = view.NewDatadogLogsView(a, a.timeRangeModal, a.OpenDatadogLogDetail)
	a.datadogLogDetailV = view.NewDatadogLogDetailView(a, func() {
		a.pages.SwitchToPage("datadog-logs")
		a.tv.SetFocus(a.datadogLogsV.Table())
		a.UpdateContextPanel(a.datadogLogsV)
	})
	a.codePipelineListV = view.NewCodePipelineListView(a, a.OpenCodePipelineDetail)
	a.codePipelineDetailV = view.NewCodePipelineDetailView(a, func() {
		a.pages.SwitchToPage("codepipeline")
		a.tv.SetFocus(a.codePipelineListV.Table())
		a.UpdateContextPanel(a.codePipelineListV)
	})
	a.pipelineWatcher = view.NewPipelineWatcher(a, a.notify, a.codePipelineListV, a.codePipelineDetailV)
	a.secretDetailV = view.NewSecretDetailView(a, func() {
		a.pages.SwitchToPage("secrets-manager")
		a.tv.SetFocus(a.secretsV.Table())
		a.UpdateContextPanel(a.secretsV)
	})

	a.views = []ui.View{homeView, a.settingsV, a.logV, a.queuesV, a.ssmParamsV, a.secretsV, a.logsV, a.datadogLogsV, a.codePipelineListV}
	for _, v := range a.views {
		prim := v.Primitive()
		a.colorBordered(v, prim)
		a.pages.AddPage(v.Name(), prim, true, false)
	}
	// messages, message-detail, and param-detail pages: not in a.views (no
	// home entry / SwitchTo — opened directly via their own open* helper).
	a.pages.AddPage("messages", a.messagesV.Primitive(), true, false)
	a.pages.AddPage("message-detail", a.messageDetailV.Primitive(), true, false)
	a.pages.AddPage("secret-detail", a.secretDetailV.Primitive(), true, false)
	a.pages.AddPage("log-search", a.logSearchV.Primitive(), true, false)
	a.pages.AddPage("log-event-detail", a.logDetailV.Primitive(), true, false)
	a.pages.AddPage("datadog-log-detail", a.datadogLogDetailV.Primitive(), true, false)
	a.pages.AddPage("codepipeline-detail", a.codePipelineDetailV.Primitive(), true, false)
	a.pages.AddPage("ssm-param-detail", a.paramDetailV.Primitive(), true, false)

	layout := tview.NewFlex().SetDirection(tview.FlexRow).
		AddItem(tb.Root, tb.Height, 0, false).
		AddItem(a.pages, 0, 1, true).
		AddItem(a.statusBar, 1, 0, false)

	confirmOverlay := ui.Centered(a.confirm.Primitive(), 52, 8)
	movePickerOverlay := ui.Centered(a.movePicker.Primitive(), 52, 22)
	sendMessageOverlay := ui.Centered(a.sendMessage.Primitive(), 70, 14)

	connManagerOverlay := ui.Centered(a.connManager.Primitive(), 64, 20)

	a.connEditor = dialog.NewConnEditor(a, a.connManager)
	a.connManager.SetEditor(a.connEditor)
	// Height must cover border+padding (4 rows) + 11 items * (field + item
	// padding) (22 rows) + button row (1 row) = 27; give it one spare row.
	// 11 is the tallest case: the 3 section headers (General/Destination/
	// Auth), Name, Backend, Broker Name, URL, Username, Authentication
	// Mode, AWS Profile, Secret Name — jolokia backend with AWS Secret
	// selected.
	connEditorOverlay := ui.Centered(a.connEditor.Primitive(), 64, 28)

	// Height must cover border+padding (4 rows) + 4 items * 2 (10 rows) +
	// button row (1 row) = 15; give it one spare row.
	messageFilterOverlay := ui.Centered(a.messageFilter.Primitive(), 64, 16)

	// Border top+bottom (2 rows) + content (1 row) + spare rows below the
	// field for the autocomplete drop-down. tview.InputField draws that
	// drop-down starting exactly one row below the field's own content
	// row (InputField.Draw's ly := y+1), regardless of the box's declared
	// height — with too little height, that row lands on the box's own
	// bottom border, drawing the drop-down's first entry directly on top
	// of the border line (found live: the sentinel's "Scan up to 5000..."
	// text rendered smashed into "╚═══...═══╝", unreadable). 12 leaves 9
	// spare rows, comfortably covering a typical scanned-types list (the
	// sentinel plus a handful of real types) without touching the
	// border — unlike messageFilterOverlay above (which gets its spare
	// vertical space for free from its own sibling form fields/buttons
	// below the JMS Type field), this is a single field with nothing
	// else in the box to borrow room from.
	jmsTypePromptOverlay := ui.Centered(a.jmsTypePrompt.Primitive(), 64, 12)

	// Width: the Absolute tab's "Until (YYYY-MM-DD HH:MM or RFC3339)"
	// label (35 chars) + its 30-wide field overflows a narrower box
	// (caught live via verify-live, spec/53 — the same failure mode as
	// messageFilterOverlay's width, just a longer label this time).
	// Height: 9 relative presets need 9 content rows; 14 leaves a couple
	// spare, matching the "give it one spare row" convention elsewhere.
	timeRangeOverlay := ui.Centered(a.timeRangeModal.Primitive(), 72, 14)

	// Height: border+padding (4 rows) + 2 items * 2 rows (10) + button
	// row (1) + one spare row = 10.
	datadogEditorOverlay := ui.Centered(a.datadogEditor.Primitive(), 56, 10)

	themePickerOverlay := ui.Centered(a.themePicker.Primitive(), 40, 14)

	awsProfilesOverlay := ui.Centered(a.awsProfiles.Primitive(), 64, 20)

	helpOverlay := ui.Centered(ui.NewHelpModal(cfg), ui.HelpModalWidth, ui.HelpModalHeight)
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
		AddPage("message-filter", messageFilterOverlay, true, false).
		AddPage("jmstype-prompt", jmsTypePromptOverlay, true, false).
		AddPage("time-range", timeRangeOverlay, true, false).
		AddPage("datadog-editor", datadogEditorOverlay, true, false).
		AddPage("theme-picker", themePickerOverlay, true, false).
		AddPage("aws-profiles", awsProfilesOverlay, true, false).
		AddPage("confirm", confirmOverlay, true, false)

	a.SwitchTo(a.views[0].Name())

	// Built here, once every view/overlay above exists — onGlobalKey and
	// onPromptDone loop over these instead of a hand-maintained chain of
	// per-view/per-overlay checks.
	a.focusExemptInputs = []tview.Primitive{
		a.prompt,
	}
	a.focusExemptInputs = append(a.focusExemptInputs, a.ssmParamsV.FilterInputs()...)
	a.focusExemptInputs = append(a.focusExemptInputs, a.secretsV.FilterInputs()...)
	a.focusExemptInputs = append(a.focusExemptInputs, a.logsV.FilterInputs()...)
	a.focusExemptInputs = append(a.focusExemptInputs, a.datadogLogsV.FilterInputs()...)
	a.focusExemptInputs = append(a.focusExemptInputs, a.codePipelineListV.FilterInputs()...)
	a.focusExemptInputs = append(a.focusExemptInputs, a.queuesV.FilterInputs()...)
	a.focusExemptInputs = append(a.focusExemptInputs, a.logSearchV.FilterInputs()...)
	a.focusExemptInputs = append(a.focusExemptInputs, a.messagesV.FilterInputs()...)
	a.overlayVisible = []visibler{
		a.confirm,
		a.movePicker,
		a.sendMessage,
		a.connManager,
		a.connEditor,
		a.messageFilter,
		a.jmsTypePrompt,
		a.timeRangeModal,
		a.datadogEditor,
		a.themePicker,
		a.awsProfiles,
	}
	a.themables = []ui.Themeable{
		a.settingsV,
		a.logV,
		a.queuesV,
		a.messagesV,
		a.messageDetailV,
		a.ssmParamsV,
		a.paramDetailV,
		a.secretsV,
		a.secretDetailV,
		a.logsV,
		a.logSearchV,
		a.logDetailV,
		a.datadogLogsV,
		a.datadogLogDetailV,
		a.codePipelineListV,
		a.codePipelineDetailV,
		a.confirm,
		a.movePicker,
		a.sendMessage,
		a.connManager,
		a.connEditor,
		a.messageFilter,
		a.jmsTypePrompt,
		a.timeRangeModal,
		a.datadogEditor,
		a.themePicker,
		a.awsProfiles,
	}

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
	// HasFocus(), not an a.tv.GetFocus() identity check: tview.DropDown
	// delegates focus to a private, internal *tview.List once its popup
	// is open (Focus(delegate) calls delegate(d.list), not d.Box.Focus),
	// so Application.GetFocus() no longer equals the *DropDown itself at
	// that point — the identity check silently stopped matching for
	// every keystroke typed into an open dropdown (found live:
	// DatadogLogsView's Service/Env filter dropdowns, 'q' quit the whole
	// app while a dropdown's popup was open). DropDown.HasFocus()
	// correctly delegates through (returns d.list.HasFocus() while
	// open), and every other focusExemptInputs entry is a plain
	// *tview.InputField, for which HasFocus() is identity-equivalent —
	// so this closes the gap with no behavior change for the
	// non-dropdown cases.
	for _, in := range a.focusExemptInputs {
		if in.HasFocus() {
			return event
		}
	}

	if a.anyOverlayVisible() {
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
		// SetText doesn't itself refresh the autocomplete drop-down (only a
		// live keystroke does, via InputField's own InputHandler) — without
		// this, the prompt would show whatever suggestions were current at
		// SetAutocompleteFunc's wiring time in New() (before a.views was
		// populated), not the full up-to-date list.
		a.prompt.Autocomplete()
		a.topLeft.SwitchToPage("prompt")
		a.tv.SetFocus(a.prompt)
		return nil
	case 'h':
		a.SwitchTo("home")
		return nil
	case 's':
		a.SwitchTo("settings")
		return nil
	case 'l':
		a.SwitchTo("log")
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
		if !a.anyOverlayVisible() {
			a.tv.SetFocus(a.pages)
		}
	}()

	if key != tcell.KeyEnter {
		return
	}

	cmd := a.prompt.GetText()
	for _, pc := range promptCommandTable() {
		for _, n := range pc.names {
			if cmd == n {
				pc.run(a)
				return
			}
		}
	}
	switch {
	case strings.HasPrefix(cmd, "theme "):
		a.switchTheme(strings.TrimPrefix(cmd, "theme "))
	default:
		a.SwitchTo(cmd)
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
	a.settingsV.Refresh()

	// Save only theme name + sparse overrides (not the full derived palette).
	saveConfig := a.cfg
	saveConfig.Colors = userOverrides
	if err := config.SaveDefault(saveConfig); err != nil {
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
	a.backend = secretbackend.New(a.secretResolver, conn)
	a.queuesV.SetBackend(a.backend)
	a.infoPanel.SetText(ui.InfoPanelText(a.cfg))
	a.settingsV.Refresh()
	a.SwitchTo("queues")
	if err := config.SaveDefault(a.cfg); err != nil {
		slog.Error("switchConnection: save failed", "error", err)
	}
}

// SwitchTo activates the named view if it is registered, updates the top
// bar's context panel, and calls Activate() if the view implements activatable.
func (a *App) SwitchTo(name string) {
	// A CorrelationID queued by FE 41's Datadog->CloudWatch jump is
	// one-shot: if the user lands on the group list but then navigates
	// anywhere else without picking a group, drop it — otherwise it'd
	// silently pre-fill some later, unrelated log group's search.
	// SwitchTo("cloudwatch-logs") itself (the jump) never hits this,
	// since name == "cloudwatch-logs" there.
	if name != "cloudwatch-logs" {
		a.pendingCloudWatchPattern = ""
		a.pendingCloudWatchTimestamp = time.Time{}
	}
	for _, v := range a.views {
		if v.Name() == name {
			a.pages.SwitchToPage(name)
			a.tv.SetFocus(v.Primitive())
			a.UpdateContextPanel(v)
			if act, ok := v.(activatable); ok {
				act.Activate()
			}
			return
		}
	}
}

// CopyToClipboard writes data to the system clipboard via the terminal's
// OSC 52 escape sequence (tcell.Screen.SetClipboard) rather than shelling
// out to pbcopy/xclip/clip.exe: no new dependency, and it works the same
// over SSH as it does locally. a.screen is nil until the first draw (see
// New()'s SetAfterDrawFunc), which in practice means before Run() starts —
// a no-op then is correct since there's nothing on screen to have copied.
func (a *App) CopyToClipboard(data string) {
	if a.screen == nil {
		return
	}
	a.screen.SetClipboard([]byte(data))
}

// UpdateContextPanel renders v's shortcuts into the context panel, or clears
// it when v doesn't implement ui.Shortcuttable.
func (a *App) UpdateContextPanel(v ui.View) {
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

// anyOverlayVisible reports whether any modal overlay is currently shown.
func (a *App) anyOverlayVisible() bool {
	for _, v := range a.overlayVisible {
		if v.Visible() {
			return true
		}
	}
	return false
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

// awsSSOReauthWaitingMsg is shown (via showReauthWaiting) the moment an
// SSO re-auth starts, then again with the device verification code/URL
// appended once awsauth.Login's subprocess prints them.
const awsSSOReauthWaitingMsg = "AWS SSO session expired — opening browser to log in..."

// showReauthWaiting and showReauthDone are secretbackend.SecretResolver's
// onReauth/onReauthDone callbacks (wrapped in a.QueueUpdateDraw at the
// call site, since Resolve runs on a background goroutine): if the
// active view shows its own reauth status (ui.ReauthStatusShower — e.g.
// QueuesView's loading placeholder), it handles the message and the
// bottom status bar is left untouched, avoiding the same message
// appearing in two places at once (found live). Only a view with no such
// display falls back to the status bar, so a fetch on some hypothetical
// future view without this support still gets some indication.
func (a *App) showReauthWaiting(msg string) {
	if av, ok := a.activeView().(ui.ReauthStatusShower); ok {
		av.ShowReauthWaiting(msg)
	} else {
		a.SetStatus(msg)
	}
}

func (a *App) showReauthDone() {
	if av, ok := a.activeView().(ui.ReauthStatusShower); ok {
		av.ShowReauthDone()
	} else {
		a.SetStatus("")
	}
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
// time they become active (e.g. view.LogView reloads the log file on SwitchTo).
type activatable interface {
	Activate()
}

// visibler is satisfied by every modal overlay via its Visible() accessor —
// overlayVisible holds these instead of raw *bool pointers so App doesn't
// need to reach into an overlay's unexported field (blocking once the
// overlays move to a different package).
type visibler interface{ Visible() bool }
