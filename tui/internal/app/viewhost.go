package app

import (
	"context"
	"time"

	"github.com/ePex/cloudtui/tui/internal/awscodepipeline"
	"github.com/ePex/cloudtui/tui/internal/awslogs"
	"github.com/ePex/cloudtui/tui/internal/awsprofile"
	"github.com/ePex/cloudtui/tui/internal/awssecrets"
	"github.com/ePex/cloudtui/tui/internal/awsssm"
	"github.com/ePex/cloudtui/tui/internal/config"
	"github.com/ePex/cloudtui/tui/internal/datadoglogs"
	"github.com/ePex/cloudtui/tui/internal/ui"
)

// SwitchToPage switches the main content pages to name (unlike
// ShowPage/HidePage, which operate on rootPages for modal overlays).
func (a *App) SwitchToPage(name string) { a.pages.SwitchToPage(name) }

// SetPendingCloudWatchPattern queues a one-shot CorrelationID for FE
// 41's Datadog->CloudWatch jump.
func (a *App) SetPendingCloudWatchPattern(pattern string) {
	a.pendingCloudWatchPattern = pattern
}

func (a *App) ListParameters(ctx context.Context, profile, path string) ([]awsssm.Parameter, error) {
	return a.listParameters(ctx, profile, path)
}

func (a *App) RevealParameter(ctx context.Context, profile, name string) (string, error) {
	return a.revealParameter(ctx, profile, name)
}

func (a *App) ListSecrets(ctx context.Context, profile string) ([]awssecrets.Secret, error) {
	return a.listSecrets(ctx, profile)
}

func (a *App) RevealSecret(ctx context.Context, profile, name string) (value string, isBinary bool, err error) {
	return a.revealSecret(ctx, profile, name)
}

func (a *App) ListLogGroups(ctx context.Context, profile string) ([]awslogs.LogGroup, error) {
	return a.listLogGroups(ctx, profile)
}

func (a *App) FilterLogEvents(ctx context.Context, profile, logGroupName string, start, end time.Time, pattern string) (events []awslogs.LogEvent, hasMore bool, err error) {
	return a.filterLogEvents(ctx, profile, logGroupName, start, end, pattern)
}

func (a *App) SearchDatadogLogs(ctx context.Context, cfg config.DatadogConfig, query string, from, to time.Time) (events []datadoglogs.LogEvent, hasMore bool, err error) {
	return a.searchDatadogLogs(ctx, cfg, query, from, to)
}

func (a *App) ListDatadogFacetValues(ctx context.Context, cfg config.DatadogConfig, facet string, from, to time.Time) ([]string, error) {
	return a.listDatadogFacetValues(ctx, cfg, facet, from, to)
}

func (a *App) ListPipelines(ctx context.Context, profile string) ([]awscodepipeline.Pipeline, error) {
	return a.listPipelines(ctx, profile)
}

func (a *App) GetPipelineState(ctx context.Context, profile, pipelineName string) ([]awscodepipeline.StageStatus, error) {
	return a.getPipelineState(ctx, profile, pipelineName)
}

func (a *App) AWSAuthTypeFor(ctx context.Context, profile string) (awsprofile.AuthType, error) {
	return a.awsAuthTypeFor(ctx, profile)
}

func (a *App) AWSSSOLogin(ctx context.Context, profile string) error {
	return a.awsSSOLogin(ctx, profile)
}

var _ ui.ViewHost = (*App)(nil)
