package app

import (
	"context"
	"log/slog"
	"sort"

	"github.com/rivo/tview"

	"github.com/ePex/cloudtui/tui/internal/awsprofile"
	"github.com/ePex/cloudtui/tui/internal/config"
	"github.com/ePex/cloudtui/tui/internal/queue"
	"github.com/ePex/cloudtui/tui/internal/queue/secretbackend"
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

// LoadedJMSTypes returns the distinct, non-empty JMSType values among the
// messages currently loaded in the messages view (free — no network
// call). a.messagesV is nil here at construction time: NewMessageFilter
// (built before messagesV — see App.New()) wires this via
// SetAutocompleteFunc, which eagerly calls Autocomplete() once at wiring
// time, before messagesV exists.
func (a *App) LoadedJMSTypes() []string {
	if a.messagesV == nil {
		return nil
	}
	return distinctJMSTypes(a.messagesV.AllMessages())
}

// MessagesQueueName returns the queue the Messages view currently shows —
// MessageFilter's own way to learn which queue to scan (it has no queue
// name of its own).
func (a *App) MessagesQueueName() string {
	return a.messagesV.QueueName()
}

// ScanJMSTypes runs a fresh, unfiltered browse of queueName capped at
// maxCount purely to widen the JMS Type suggestion set — it does not
// touch the messages view's displayed list, regardless of which queue
// queueName names.
func (a *App) ScanJMSTypes(ctx context.Context, queueName string, maxCount int) ([]string, error) {
	msgs, err := a.backend.BrowseMessages(ctx, queueName, queue.MessageFilter{MaxCount: maxCount})
	if err != nil {
		return nil, err
	}
	return distinctJMSTypes(msgs), nil
}

// distinctJMSTypes returns the non-empty, deduplicated JMSType values in
// msgs, sorted for stable, predictable suggestion ordering.
func distinctJMSTypes(msgs []queue.Message) []string {
	seen := make(map[string]bool)
	var types []string
	for _, m := range msgs {
		if m.JMSType == "" || seen[m.JMSType] {
			continue
		}
		seen[m.JMSType] = true
		types = append(types, m.JMSType)
	}
	sort.Strings(types)
	return types
}

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
			a.backend = secretbackend.New(a.secretResolver, conn)
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

// SetActiveAWSProfile sets name as the active AWS profile, updates the
// info panel, refreshes the settings list, and persists. Does not touch
// a.backend: a secret-backed connection resolves its password via its
// own passwordSecretAWSProfile (spec/12-named-connections), independent
// of this value, so there's nothing to rebuild here (this used to
// rebuild the backend — see spec/88 — back when secret resolution
// depended on this same global profile).
func (a *App) SetActiveAWSProfile(name string) {
	a.cfg.ActiveAWSProfile = name
	a.infoPanel.SetText(ui.InfoPanelText(a.cfg))
	a.settingsV.Refresh()
	if err := config.SaveDefault(a.cfg); err != nil {
		slog.Error("SetActiveAWSProfile: save failed", "error", err)
	}
}

// ToggleFavorite flips name's favorite status under kind/profile and
// persists. The calling view is responsible for repainting itself; unlike
// SetActiveAWSProfile this doesn't change what's active anywhere else in
// the shell.
func (a *App) ToggleFavorite(kind config.FavoriteKind, profile, name string) {
	a.cfg.AWSFavorites = a.cfg.AWSFavorites.Toggle(kind, profile, name)
	if err := config.SaveDefault(a.cfg); err != nil {
		slog.Error("ToggleFavorite: save failed", "error", err)
	}
}
