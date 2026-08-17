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

	// Backend / queue data
	Backend() queue.Backend
	ReloadAfterSend(queueName string)

	// Messages view interaction
	MessagesFilter() queue.MessageFilter
	ApplyMessagesFilter(f queue.MessageFilter)
	FocusMessages()
}
