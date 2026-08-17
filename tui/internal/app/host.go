package app

import (
	"context"

	"github.com/rivo/tview"

	"github.com/ePex/cloudtui/tui/internal/awsprofile"
	"github.com/ePex/cloudtui/tui/internal/config"
	"github.com/ePex/cloudtui/tui/internal/queue"
	"github.com/ePex/cloudtui/tui/internal/ui"
)

var _ ui.Host = (*App)(nil)

func (a *App) ShowPage(name string)       { a.rootPages.ShowPage(name) }
func (a *App) HidePage(name string)       { a.rootPages.HidePage(name) }
func (a *App) SetFocus(p tview.Primitive) { a.tv.SetFocus(p) }
func (a *App) FocusMain()                 { a.tv.SetFocus(a.pages) }
func (a *App) QueueUpdateDraw(f func())   { a.tv.QueueUpdateDraw(f) }

func (a *App) SetStatus(text string)      { a.statusBar.SetText(text) }
func (a *App) SetContextHint(text string) { a.contextPanel.SetText(text) }

func (a *App) Config() config.Config { return a.cfg }

func (a *App) SwitchTheme(name string)      { a.switchTheme(name) }
func (a *App) SwitchConnection(name string) { a.switchConnection(name) }

func (a *App) ListAWSProfiles(ctx context.Context) ([]awsprofile.Profile, error) {
	return a.listAWSProfiles(ctx)
}

func (a *App) Backend() queue.Backend { return a.backend }

// ReloadAfterSend reloads the queues view and, if the messages view is
// currently showing queueName, reloads it too. Extracted from
// sendMessageOverlay.doSend, which did this inline before this method
// existed.
func (a *App) ReloadAfterSend(queueName string) {
	if a.queuesV != nil {
		a.queuesV.load()
	}
	if a.messagesV != nil && a.messagesV.queueName == queueName {
		a.messagesV.load()
	}
}

func (a *App) MessagesFilter() queue.MessageFilter {
	return a.messagesV.filter
}

// ApplyMessagesFilter sets f as the messages view's active filter,
// updates its title, and reloads. Extracted from messageFilter.apply
// and .clear, which each did this identical 3-line sequence inline
// before this method existed.
func (a *App) ApplyMessagesFilter(f queue.MessageFilter) {
	a.messagesV.filter = f
	a.messagesV.updateTitle()
	a.messagesV.load()
}

func (a *App) FocusMessages() { a.tv.SetFocus(a.messagesV.table) }
