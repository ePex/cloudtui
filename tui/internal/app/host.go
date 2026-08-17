package app

import (
	"context"
	"log/slog"

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
		a.queuesV.Load()
	}
	if a.messagesV != nil && a.messagesV.QueueName() == queueName {
		a.messagesV.Load()
	}
}

func (a *App) MessagesFilter() queue.MessageFilter {
	return a.messagesV.Filter()
}

// ApplyMessagesFilter sets f as the messages view's active filter,
// updates its title, and reloads. Extracted from MessageFilter.apply
// and .clear, which each did this identical 3-line sequence inline
// before this method existed.
func (a *App) ApplyMessagesFilter(f queue.MessageFilter) {
	a.messagesV.ApplyFilter(f)
}

func (a *App) FocusMessages() { a.tv.SetFocus(a.messagesV.Table()) }

// DeleteConnection removes name from Connections. If it was the active
// connection, activates the first remaining one (reusing switchConnection
// for the backend-rebuild+persist+refresh path); otherwise persists
// directly. Returns whether the removed connection was active, so the
// caller knows which post-delete UI path to take (switchConnection
// already navigated to "queues"; the non-active path needs the caller
// to repaint its own list instead).
func (a *App) DeleteConnection(name string) (wasActive bool) {
	wasActive = a.cfg.ActiveConnection == name
	conns := make([]config.Connection, 0, len(a.cfg.Connections)-1)
	for _, c := range a.cfg.Connections {
		if c.Name != name {
			conns = append(conns, c)
		}
	}
	a.cfg.Connections = conns
	if wasActive {
		a.cfg.ActiveConnection = a.cfg.Connections[0].Name
		a.switchConnection(a.cfg.ActiveConnection)
		return true
	}
	if err := config.SaveDefault(a.cfg); err != nil {
		slog.Error("DeleteConnection: save failed", "error", err)
	}
	return false
}

// SaveConnection appends conn (isNew) or replaces the connection named
// origName (edit) in Connections, rebuilding the active backend in
// place if the edited connection was the active one, then persists and
// refreshes the settings list.
func (a *App) SaveConnection(conn config.Connection, origName string, isNew bool) {
	wasActive := a.cfg.ActiveConnection == origName
	if isNew {
		a.cfg.Connections = append(a.cfg.Connections, conn)
	} else {
		for i, c := range a.cfg.Connections {
			if c.Name == origName {
				a.cfg.Connections[i] = conn
				break
			}
		}
		if wasActive {
			a.cfg.ActiveConnection = conn.Name
			a.backend = newBackendForConn(a, conn)
			a.queuesV.SetBackend(a.backend)
			a.infoPanel.SetText(ui.InfoPanelText(a.cfg))
		}
	}
	if err := config.SaveDefault(a.cfg); err != nil {
		slog.Error("SaveConnection: save failed", "error", err)
	}
	a.settingsV.Refresh()
}

// SaveDatadogConfig persists cfg.Datadog and refreshes the settings list.
func (a *App) SaveDatadogConfig(cfg config.DatadogConfig) {
	a.cfg.Datadog = cfg
	if err := config.SaveDefault(a.cfg); err != nil {
		slog.Error("SaveDatadogConfig: save failed", "error", err)
	}
	a.settingsV.Refresh()
}

// SetActiveAWSProfile sets name as the active AWS profile, rebuilds the
// backend (so a Secrets-Manager-backed connection re-resolves its
// password against the new profile rather than keeping the old one
// indefinitely — see spec/88), updates the info panel, refreshes the
// settings list, and persists.
func (a *App) SetActiveAWSProfile(name string) {
	a.cfg.ActiveAWSProfile = name
	a.backend = newBackendForConn(a, a.cfg.ActiveConn())
	a.queuesV.SetBackend(a.backend)
	a.infoPanel.SetText(ui.InfoPanelText(a.cfg))
	a.settingsV.Refresh()
	if err := config.SaveDefault(a.cfg); err != nil {
		slog.Error("SetActiveAWSProfile: save failed", "error", err)
	}
}
