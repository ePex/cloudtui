package dialog

import (
	"context"

	"github.com/rivo/tview"

	"github.com/ePex/cloudtui/tui/internal/awsprofile"
	"github.com/ePex/cloudtui/tui/internal/config"
	"github.com/ePex/cloudtui/tui/internal/queue"
	"github.com/ePex/cloudtui/tui/internal/ui"
)

// testHost is a minimal ui.Host test double: side-effecting methods
// record their call, data methods return injectable/canned values. Not
// exported — internal/dialog gets its own copy when the overlays move
// there (see spec/76's Out of scope).
type testHost struct {
	cfg             config.Config
	backend         queue.Backend
	listAWSProfiles func(context.Context) ([]awsprofile.Profile, error)
	messagesFilter  queue.MessageFilter
	deleteResult    bool // DeleteConnection's return value

	shownPages         []string
	hiddenPages        []string
	focused            tview.Primitive
	focusMainCalls     int
	status             string
	contextHint        string
	switchedTheme      string
	switchedConnection string
	savedConnection    *savedConnectionCall
	deletedConnection  string
	savedDatadogConfig *config.DatadogConfig
	activeAWSProfile   string
	toggledFavorite    *toggledFavoriteCall
	loadedJMSTypes     []string
	messagesQueueName  string
	scanJMSTypesFn     func(ctx context.Context, queueName string, maxCount int) ([]string, error)
	reloadedQueue      string
	appliedFilter      *queue.MessageFilter
	focusMessagesCalls int
}

type savedConnectionCall struct {
	conn     config.Connection
	origName string
	isNew    bool
}

type toggledFavoriteCall struct {
	kind    config.FavoriteKind
	profile string
	name    string
}

// newTestHost builds a testHost with config.Default() and a
// zero-value fakeQueueBackend — matches what New(config.Default())
// gave every overlay constructor before this CR.
func newTestHost() *testHost {
	return &testHost{cfg: config.Default(), backend: &fakeQueueBackend{}}
}

func (h *testHost) ShowPage(name string)       { h.shownPages = append(h.shownPages, name) }
func (h *testHost) HidePage(name string)       { h.hiddenPages = append(h.hiddenPages, name) }
func (h *testHost) SetFocus(p tview.Primitive) { h.focused = p }
func (h *testHost) FocusMain()                 { h.focusMainCalls++ }
func (h *testHost) QueueUpdateDraw(f func())   { f() }

func (h *testHost) SetStatus(text string)      { h.status = text }
func (h *testHost) SetContextHint(text string) { h.contextHint = text }

func (h *testHost) Config() config.Config        { return h.cfg }
func (h *testHost) SwitchTheme(name string)      { h.switchedTheme = name }
func (h *testHost) SwitchConnection(name string) { h.switchedConnection = name }
func (h *testHost) SaveConnection(conn config.Connection, origName string, isNew bool) {
	h.savedConnection = &savedConnectionCall{conn: conn, origName: origName, isNew: isNew}
}
func (h *testHost) DeleteConnection(name string) bool {
	h.deletedConnection = name
	return h.deleteResult
}
func (h *testHost) SaveDatadogConfig(cfg config.DatadogConfig) { h.savedDatadogConfig = &cfg }
func (h *testHost) SetActiveAWSProfile(name string)            { h.activeAWSProfile = name }
func (h *testHost) ListAWSProfiles(ctx context.Context) ([]awsprofile.Profile, error) {
	if h.listAWSProfiles == nil {
		return nil, nil
	}
	return h.listAWSProfiles(ctx)
}
func (h *testHost) ToggleFavorite(kind config.FavoriteKind, profile, name string) {
	h.toggledFavorite = &toggledFavoriteCall{kind: kind, profile: profile, name: name}
}

func (h *testHost) Backend() queue.Backend           { return h.backend }
func (h *testHost) ReloadAfterSend(queueName string) { h.reloadedQueue = queueName }

func (h *testHost) MessagesFilter() queue.MessageFilter       { return h.messagesFilter }
func (h *testHost) ApplyMessagesFilter(f queue.MessageFilter) { h.appliedFilter = &f }
func (h *testHost) FocusMessages()                            { h.focusMessagesCalls++ }

func (h *testHost) LoadedJMSTypes() []string  { return h.loadedJMSTypes }
func (h *testHost) MessagesQueueName() string { return h.messagesQueueName }
func (h *testHost) ScanJMSTypes(ctx context.Context, queueName string, maxCount int) ([]string, error) {
	if h.scanJMSTypesFn == nil {
		return nil, nil
	}
	return h.scanJMSTypesFn(ctx, queueName, maxCount)
}

var _ ui.Host = (*testHost)(nil)
