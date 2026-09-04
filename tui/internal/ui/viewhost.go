package ui

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
	"github.com/ePex/cloudtui/tui/internal/queue"
)

// ViewHost is the contract resource views depend on instead of the
// concrete *App, so views can move to their own package without an
// import cycle (internal/view importing internal/app back for *App).
// Mirrors Host's role for the modal overlays in internal/dialog.
//
// Unlike dialogs, views don't need Host-style access to open modal
// overlays — internal/view can import internal/dialog directly (it's
// a fully independent package since the dialog move), so each view
// takes the specific dialog instances it needs as constructor
// parameters instead, the same pattern ConnEditor already uses for
// its ConnManager sibling reference.
type ViewHost interface {
	Host

	// Chrome
	SwitchToPage(name string)
	UpdateContextPanel(v View)
	SwitchTo(name string)
	CopyToClipboard(data string)

	// Cross-view navigation — implemented by App's viewwiring.go,
	// which reaches into the target view's own state on the caller's
	// behalf (App owns the tview.Pages routing and focus, not any one
	// view).
	OpenMessages(queueName string)
	OpenMessageDetail(queueName string, msg queue.Message)
	OpenParamDetail(param awsssm.Parameter)
	OpenSecretDetail(secret awssecrets.Secret)
	OpenLogSearch(logGroupName string)
	OpenLogEventDetail(event awslogs.LogEvent)
	OpenDatadogLogDetail(event datadoglogs.LogEvent)
	OpenCodePipelineDetail(pipelineName string)

	// SetPendingCloudWatchPattern queues a one-shot CorrelationID and
	// the originating Datadog event's timestamp for FE 41's
	// Datadog->CloudWatch jump (write-only — App itself reads and
	// clears it when the jump completes or is abandoned).
	SetPendingCloudWatchPattern(pattern string, timestamp time.Time)

	// CodePipeline background watcher — App forwards to
	// view.PipelineWatcher (internal/app/codepipelinewatch.go's 3
	// trampoline methods), which owns the actual poll loop and state.
	IsWatchingPipeline(name string) bool
	StartWatchingPipeline(name string)
	StopWatchingPipeline(name string)

	// Injectable data-fetchers — mirror the equivalent App func
	// fields (ListAWSProfiles's counterpart already lives on Host).
	ListParameters(ctx context.Context, profile, path string) ([]awsssm.Parameter, error)
	RevealParameter(ctx context.Context, profile, name string) (string, error)
	ListSecrets(ctx context.Context, profile string) ([]awssecrets.Secret, error)
	RevealSecret(ctx context.Context, profile, name string) (value string, isBinary bool, err error)
	ListLogGroups(ctx context.Context, profile string) ([]awslogs.LogGroup, error)
	FilterLogEvents(ctx context.Context, profile, logGroupName string, start, end time.Time, pattern, nextToken string) (events []awslogs.LogEvent, next string, err error)
	SearchDatadogLogs(ctx context.Context, cfg config.DatadogConfig, query string, from, to time.Time) (events []datadoglogs.LogEvent, hasMore bool, err error)
	ListDatadogFacetValues(ctx context.Context, cfg config.DatadogConfig, facet string, from, to time.Time) ([]string, error)
	ListPipelines(ctx context.Context, profile string) ([]awscodepipeline.Pipeline, error)
	GetPipelineState(ctx context.Context, profile, pipelineName string) ([]awscodepipeline.StageStatus, error)
	AWSAuthTypeFor(ctx context.Context, profile string) (awsprofile.AuthType, error)
	AWSSSOLogin(ctx context.Context, profile string, onCode func(code, url string)) error
}

// AWSAuthHost is the small subset every AWS-SSO-gated resource view's
// load() needs. Embedded by every per-resource host interface below
// whose load() goes through internal/view/awsload.go's shared
// runAWSLoad helper (SSMParamsHost, SecretsHost, CloudWatchLogsHost,
// CodePipelineHost) — not DatadogLogsHost, since Datadog's own
// API-key auth is unrelated to AWS SSO re-auth.
type AWSAuthHost interface {
	AWSAuthTypeFor(ctx context.Context, profile string) (awsprofile.AuthType, error)
	AWSSSOLogin(ctx context.Context, profile string, onCode func(code, url string)) error
}

// SSMParamsHost is what SSMParamsView and ParamDetailView need.
type SSMParamsHost interface {
	Host
	AWSAuthHost
	ListParameters(ctx context.Context, profile, path string) ([]awsssm.Parameter, error)
	RevealParameter(ctx context.Context, profile, name string) (string, error)
	CopyToClipboard(data string)
}

// SecretsHost is what SecretsView and SecretDetailView need.
type SecretsHost interface {
	Host
	AWSAuthHost
	ListSecrets(ctx context.Context, profile string) ([]awssecrets.Secret, error)
	RevealSecret(ctx context.Context, profile, name string) (value string, isBinary bool, err error)
	CopyToClipboard(data string)
}

// CloudWatchLogsHost is what LogsView, LogSearchView, and
// LogDetailView need.
type CloudWatchLogsHost interface {
	Host
	AWSAuthHost
	ListLogGroups(ctx context.Context, profile string) ([]awslogs.LogGroup, error)
	FilterLogEvents(ctx context.Context, profile, logGroupName string, start, end time.Time, pattern, nextToken string) (events []awslogs.LogEvent, next string, err error)
	CopyToClipboard(data string)
}

// DatadogLogsHost is what DatadogLogsView and DatadogLogDetailView need.
type DatadogLogsHost interface {
	Host
	SearchDatadogLogs(ctx context.Context, cfg config.DatadogConfig, query string, from, to time.Time) (events []datadoglogs.LogEvent, hasMore bool, err error)
	ListDatadogFacetValues(ctx context.Context, cfg config.DatadogConfig, facet string, from, to time.Time) ([]string, error)
	SetPendingCloudWatchPattern(pattern string, timestamp time.Time)
	SwitchTo(name string)
	CopyToClipboard(data string)
}

// CodePipelineHost is what CodePipelineListView, CodePipelineDetailView,
// and PipelineWatcher need. PipelineWatcher itself only calls
// Config/QueueUpdateDraw (via Host), GetPipelineState, AWSAuthTypeFor,
// and AWSSSOLogin on its host — not the 3 watch-toggle methods below,
// which it owns and implements itself — but reuses this same interface
// as its sibling views rather than needing a near-duplicate interface
// for one caller.
type CodePipelineHost interface {
	Host
	AWSAuthHost
	ListPipelines(ctx context.Context, profile string) ([]awscodepipeline.Pipeline, error)
	GetPipelineState(ctx context.Context, profile, pipelineName string) ([]awscodepipeline.StageStatus, error)
	IsWatchingPipeline(name string) bool
	StartWatchingPipeline(name string)
	StopWatchingPipeline(name string)
}

// MessagesHost is what MessagesView needs beyond plain Host: SwitchTo,
// to return to "queues" when Esc/Backspace is pressed.
type MessagesHost interface {
	Host
	SwitchTo(name string)
}
