package ui

import (
	"context"

	"github.com/ePex/cloudtui/tui/internal/awsprofile"
	"github.com/ePex/cloudtui/tui/internal/config"
	"github.com/ePex/cloudtui/tui/internal/queue"
	"github.com/rivo/tview"
)

// Host is the contract overlays depend on instead of the concrete *App,
// so overlays can move to their own package without an import cycle
// (internal/dialog importing internal/app back for *App).
type Host interface {
	// Chrome / focus
	ShowPage(name string)
	HidePage(name string)
	SetFocus(p tview.Primitive)
	FocusMain()
	QueueUpdateDraw(f func())

	// Status / context feedback
	SetStatus(text string)
	SetContextHint(text string)

	// Config / theme
	Config() config.Config
	SwitchTheme(name string)
	SwitchConnection(name string)
	SaveConnection(conn config.Connection, origName string, isNew bool)
	DeleteConnection(name string) (wasActive bool)
	SaveDatadogConfig(cfg config.DatadogConfig)
	SetActiveAWSProfile(name string)
	ListAWSProfiles(ctx context.Context) ([]awsprofile.Profile, error)
	ToggleFavorite(kind config.FavoriteKind, profile, name string)

	// Backend / queue data
	Backend() queue.Backend
	ReloadAfterSend(queueName string)

	// Messages view interaction
	MessagesFilter() queue.MessageFilter
	ApplyMessagesFilter(f queue.MessageFilter)
	FocusMessages()

	// JMS Type suggestions for the message filter overlay's "JMS Type"
	// field (spec/08) and the purge/move-all JMS Type filter prompt
	// (spec/09). LoadedJMSTypes is synchronous/free — the distinct types
	// among messages already loaded in the Messages view (there is none
	// to ask about from the Queues view, which doesn't pre-load
	// messages — callers there rely on ScanJMSTypes only).
	// MessagesQueueName is MessageFilter's own way to learn which queue
	// it's scanning (it has no queue name of its own, unlike
	// JMSTypePrompt, which is told one via its Show parameter).
	// ScanJMSTypes is the opt-in, network-costly widening: a fresh,
	// unfiltered browse of queueName capped at maxCount, purely to find
	// more types without changing what's displayed in the Messages view.
	LoadedJMSTypes() []string
	MessagesQueueName() string
	ScanJMSTypes(ctx context.Context, queueName string, maxCount int) ([]string, error)
}
