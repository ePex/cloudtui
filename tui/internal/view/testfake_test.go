package view

import (
	"context"
	"time"

	"github.com/rivo/tview"

	"github.com/ePex/cloudtui/tui/internal/awscodepipeline"
	"github.com/ePex/cloudtui/tui/internal/awslogs"
	"github.com/ePex/cloudtui/tui/internal/awsprofile"
	"github.com/ePex/cloudtui/tui/internal/awssecrets"
	"github.com/ePex/cloudtui/tui/internal/awsssm"
	"github.com/ePex/cloudtui/tui/internal/config"
	"github.com/ePex/cloudtui/tui/internal/datadoglogs"
	"github.com/ePex/cloudtui/tui/internal/queue"
	"github.com/ePex/cloudtui/tui/internal/ui"
)

var _ ui.ViewHost = (*fakeViewHost)(nil)

// fakeViewHost is a minimal ui.ViewHost double for view-level tests: it
// records what a view asked for instead of driving a real *App. Every
// data-fetcher method returns zero values unless a test sets the matching
// func field — same "inject a func, override per test" shape this codebase
// already used for App.listParameters etc. pre-CR-80.
type fakeViewHost struct {
	cfg     config.Config
	backend queue.Backend

	focused     tview.Primitive
	shownPage   string
	status      string
	contextHint string
	copiedData  string
	watching    map[string]bool

	listParametersFn         func(ctx context.Context, profile, path string) ([]awsssm.Parameter, error)
	revealParameterFn        func(ctx context.Context, profile, name string) (string, error)
	listSecretsFn            func(ctx context.Context, profile string) ([]awssecrets.Secret, error)
	revealSecretFn           func(ctx context.Context, profile, name string) (string, bool, error)
	listLogGroupsFn          func(ctx context.Context, profile string) ([]awslogs.LogGroup, error)
	filterLogEventsFn        func(ctx context.Context, profile, logGroupName string, start, end time.Time, pattern, nextToken string) ([]awslogs.LogEvent, string, error)
	searchDatadogLogsFn      func(ctx context.Context, cfg config.DatadogConfig, query string, from, to time.Time) ([]datadoglogs.LogEvent, bool, error)
	listDatadogFacetValuesFn func(ctx context.Context, cfg config.DatadogConfig, facet string, from, to time.Time) ([]string, error)
	listPipelinesFn          func(ctx context.Context, profile string) ([]awscodepipeline.Pipeline, error)
	getPipelineStateFn       func(ctx context.Context, profile, pipelineName string) ([]awscodepipeline.StageStatus, error)
	awsAuthTypeForFn         func(ctx context.Context, profile string) (awsprofile.AuthType, error)
	awsSSOLoginFn            func(ctx context.Context, profile string) error
}

// newFakeViewHost defaults backend to a no-op fakeQueueBackend (never nil)
// — several dialogs (e.g. MovePicker) spawn a goroutine on Show() that
// calls host.Backend().List(...) unconditionally; a nil backend panics
// that goroutine, which can crash a later, unrelated test since it isn't
// awaited by the test that triggered it.
func newFakeViewHost() *fakeViewHost {
	return &fakeViewHost{cfg: config.Default(), backend: &fakeQueueBackend{}, watching: map[string]bool{}}
}

// -- ui.Host: recorded state where a test needs to assert on it --
func (f *fakeViewHost) SetFocus(p tview.Primitive) { f.focused = p }
func (f *fakeViewHost) SetStatus(text string)      { f.status = text }
func (f *fakeViewHost) SetContextHint(text string) { f.contextHint = text }
func (f *fakeViewHost) Config() config.Config      { return f.cfg }
func (f *fakeViewHost) Backend() queue.Backend     { return f.backend }
func (f *fakeViewHost) QueueUpdateDraw(fn func())  { fn() } // no real event loop; run inline

// -- ui.Host: no-ops (nothing under test needs these) --
func (f *fakeViewHost) ShowPage(name string)         {}
func (f *fakeViewHost) HidePage(name string)         {}
func (f *fakeViewHost) FocusMain()                   {}
func (f *fakeViewHost) SwitchTheme(name string)      {}
func (f *fakeViewHost) SwitchConnection(name string) {}
func (f *fakeViewHost) SaveConnection(conn config.Connection, origName string, isNew bool) {
}
func (f *fakeViewHost) DeleteConnection(name string) (wasActive bool) { return false }
func (f *fakeViewHost) SaveDatadogConfig(cfg config.DatadogConfig)    {}
func (f *fakeViewHost) SetActiveAWSProfile(name string)               { f.cfg.ActiveAWSProfile = name }
func (f *fakeViewHost) ListAWSProfiles(ctx context.Context) ([]awsprofile.Profile, error) {
	return nil, nil
}
func (f *fakeViewHost) ToggleFavorite(kind config.FavoriteKind, profile, name string) {
	f.cfg.AWSFavorites = f.cfg.AWSFavorites.Toggle(kind, profile, name)
}
func (f *fakeViewHost) ReloadAfterSend(queueName string)           {}
func (f *fakeViewHost) MessagesFilter() queue.MessageFilter        { return queue.MessageFilter{} }
func (f *fakeViewHost) ApplyMessagesFilter(fl queue.MessageFilter) {}
func (f *fakeViewHost) FocusMessages()                             {}

// -- ui.ViewHost chrome --
func (f *fakeViewHost) SwitchToPage(name string)     { f.shownPage = name }
func (f *fakeViewHost) UpdateContextPanel(v ui.View) {}
func (f *fakeViewHost) SwitchTo(name string)         { f.shownPage = name }
func (f *fakeViewHost) CopyToClipboard(data string)  { f.copiedData = data }

// -- ui.ViewHost cross-view navigation: never called by a view under test
// in isolation (a view invokes its injected onSelect/onBack closure, not
// host.OpenX) — pure stubs, present only to satisfy the interface. --
func (f *fakeViewHost) OpenMessages(queueName string)                                   {}
func (f *fakeViewHost) OpenMessageDetail(queueName string, msg queue.Message)           {}
func (f *fakeViewHost) OpenParamDetail(param awsssm.Parameter)                          {}
func (f *fakeViewHost) OpenSecretDetail(secret awssecrets.Secret)                       {}
func (f *fakeViewHost) OpenLogSearch(logGroupName string)                               {}
func (f *fakeViewHost) OpenLogEventDetail(event awslogs.LogEvent)                       {}
func (f *fakeViewHost) OpenDatadogLogDetail(event datadoglogs.LogEvent)                 {}
func (f *fakeViewHost) OpenCodePipelineDetail(pipelineName string)                      {}
func (f *fakeViewHost) SetPendingCloudWatchPattern(pattern string, timestamp time.Time) {}

func (f *fakeViewHost) IsWatchingPipeline(name string) bool { return f.watching[name] }
func (f *fakeViewHost) StartWatchingPipeline(name string)   { f.watching[name] = true }
func (f *fakeViewHost) StopWatchingPipeline(name string)    { delete(f.watching, name) }

// -- ui.ViewHost data-fetchers: injectable func field, zero value if unset --
func (f *fakeViewHost) ListParameters(ctx context.Context, profile, path string) ([]awsssm.Parameter, error) {
	if f.listParametersFn != nil {
		return f.listParametersFn(ctx, profile, path)
	}
	return nil, nil
}

func (f *fakeViewHost) RevealParameter(ctx context.Context, profile, name string) (string, error) {
	if f.revealParameterFn != nil {
		return f.revealParameterFn(ctx, profile, name)
	}
	return "", nil
}

func (f *fakeViewHost) ListSecrets(ctx context.Context, profile string) ([]awssecrets.Secret, error) {
	if f.listSecretsFn != nil {
		return f.listSecretsFn(ctx, profile)
	}
	return nil, nil
}

func (f *fakeViewHost) RevealSecret(ctx context.Context, profile, name string) (string, bool, error) {
	if f.revealSecretFn != nil {
		return f.revealSecretFn(ctx, profile, name)
	}
	return "", false, nil
}

func (f *fakeViewHost) ListLogGroups(ctx context.Context, profile string) ([]awslogs.LogGroup, error) {
	if f.listLogGroupsFn != nil {
		return f.listLogGroupsFn(ctx, profile)
	}
	return nil, nil
}

func (f *fakeViewHost) FilterLogEvents(ctx context.Context, profile, logGroupName string, start, end time.Time, pattern, nextToken string) ([]awslogs.LogEvent, string, error) {
	if f.filterLogEventsFn != nil {
		return f.filterLogEventsFn(ctx, profile, logGroupName, start, end, pattern, nextToken)
	}
	return nil, "", nil
}

func (f *fakeViewHost) SearchDatadogLogs(ctx context.Context, cfg config.DatadogConfig, query string, from, to time.Time) ([]datadoglogs.LogEvent, bool, error) {
	if f.searchDatadogLogsFn != nil {
		return f.searchDatadogLogsFn(ctx, cfg, query, from, to)
	}
	return nil, false, nil
}

func (f *fakeViewHost) ListDatadogFacetValues(ctx context.Context, cfg config.DatadogConfig, facet string, from, to time.Time) ([]string, error) {
	if f.listDatadogFacetValuesFn != nil {
		return f.listDatadogFacetValuesFn(ctx, cfg, facet, from, to)
	}
	return nil, nil
}

func (f *fakeViewHost) ListPipelines(ctx context.Context, profile string) ([]awscodepipeline.Pipeline, error) {
	if f.listPipelinesFn != nil {
		return f.listPipelinesFn(ctx, profile)
	}
	return nil, nil
}

func (f *fakeViewHost) GetPipelineState(ctx context.Context, profile, pipelineName string) ([]awscodepipeline.StageStatus, error) {
	if f.getPipelineStateFn != nil {
		return f.getPipelineStateFn(ctx, profile, pipelineName)
	}
	return nil, nil
}

func (f *fakeViewHost) AWSAuthTypeFor(ctx context.Context, profile string) (awsprofile.AuthType, error) {
	if f.awsAuthTypeForFn != nil {
		return f.awsAuthTypeForFn(ctx, profile)
	}
	return "", nil
}

func (f *fakeViewHost) AWSSSOLogin(ctx context.Context, profile string) error {
	if f.awsSSOLoginFn != nil {
		return f.awsSSOLoginFn(ctx, profile)
	}
	return nil
}
