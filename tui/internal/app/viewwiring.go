// viewwiring.go holds the (a *App) trampolines that wire one resource
// view to open another (queues -> messages -> message detail, and the
// equivalent list/detail pairs for SSM params, secrets, CloudWatch
// Logs, Datadog Logs, and CodePipeline). Each pair reaches directly
// into the target view's unexported state — kept together, and kept
// out of the view types' own files, since neither view "owns" the
// wiring between them; App does. See spec/79.
package app

import (
	"fmt"
	"strings"
	"time"

	"github.com/ePex/cloudtui/tui/internal/awslogs"
	"github.com/ePex/cloudtui/tui/internal/awssecrets"
	"github.com/ePex/cloudtui/tui/internal/awsssm"
	"github.com/ePex/cloudtui/tui/internal/datadoglogs"
	"github.com/ePex/cloudtui/tui/internal/queue"
	"github.com/ePex/cloudtui/tui/internal/ui"
)

// OpenMessages switches to the messages page for the given queue, sets the
// title, and starts loading messages asynchronously. Quick search and the
// server-side filter persist when returning to the same queue, but reset
// when switching to a different one — carrying a leftover filter across
// queues would silently narrow what the user sees without them asking.
func (a *App) OpenMessages(queueName string) {
	a.messagesV.Open(queueName)
	a.pages.SwitchToPage("messages")
	a.tv.SetFocus(a.pages)
	// Show messagesV shortcuts in the context panel.
	lines := make([]string, 0, len(a.messagesV.Shortcuts()))
	for _, sc := range a.messagesV.Shortcuts() {
		lines = append(lines, fmt.Sprintf("[%s]<%s>[-] %s", a.cfg.Colors.Accent, sc.Key, sc.Description))
	}
	a.contextPanel.SetText(strings.Join(lines, "\n"))
}

// OpenMessageDetail renders the full detail for msg and switches to the
// message-detail page.
func (a *App) OpenMessageDetail(queueName string, msg queue.Message) {
	a.messageDetailV.Render(queueName, msg)
	a.pages.SwitchToPage("message-detail")
	a.tv.SetFocus(a.pages)
	lines := make([]string, 0, len(a.messageDetailV.Shortcuts()))
	for _, sc := range a.messageDetailV.Shortcuts() {
		lines = append(lines, fmt.Sprintf("[%s]<%s>[-] %s", a.cfg.Colors.Accent, sc.Key, sc.Description))
	}
	a.contextPanel.SetText(strings.Join(lines, "\n"))
}

// OpenParamDetail renders the full detail for param and switches to the
// ssm-param-detail page. paramDetailView.render sets the context panel
// itself (its shortcuts change once a SecureString is revealed).
func (a *App) OpenParamDetail(param awsssm.Parameter) {
	a.paramDetailV.Render(param)
	a.pages.SwitchToPage("ssm-param-detail")
	a.tv.SetFocus(a.pages)
}

func (a *App) OpenSecretDetail(secret awssecrets.Secret) {
	a.secretDetailV.Render(secret)
	a.pages.SwitchToPage("secret-detail")
	a.tv.SetFocus(a.pages)
}

// correlationJumpWindowBuffer is how far before/after the originating
// Datadog event's timestamp the CorrelationID jump's absolute CloudWatch
// search window extends — see spec-origin/91-bugfix-correlation-jump-timerange.
// Generous enough to absorb normal cross-system delay/clock skew,
// tight enough to avoid reintroducing the high-volume problem CR 90
// addressed.
const correlationJumpWindowBuffer = 15 * time.Minute

// OpenLogSearch opens the search view for logGroupName and runs the
// first search immediately (see logSearchView.open). logSearchView isn't
// a registered ui.View, so its context panel is populated manually here
// — same pattern as OpenMessages.
//
// If a CorrelationID jump queued a timestamp alongside its pattern, the
// search opens on an absolute window centered on it
// (correlationJumpWindowBuffer either side) instead of the usual
// relative default — otherwise the jump could land on a CloudWatch
// search that structurally cannot contain the event it's looking for.
func (a *App) OpenLogSearch(logGroupName string) {
	pattern := a.pendingCloudWatchPattern
	ts := a.pendingCloudWatchTimestamp
	a.pendingCloudWatchPattern = ""
	a.pendingCloudWatchTimestamp = time.Time{}

	var tr *ui.TimeRange
	if !ts.IsZero() {
		tr = &ui.TimeRange{
			Mode: ui.TimeRangeAbsolute,
			From: ts.Add(-correlationJumpWindowBuffer),
			To:   ts.Add(correlationJumpWindowBuffer),
		}
	}
	a.logSearchV.Open(logGroupName, pattern, tr)
	a.pages.SwitchToPage("log-search")
	a.tv.SetFocus(a.pages)
	lines := make([]string, 0, len(a.logSearchV.Shortcuts()))
	for _, sc := range a.logSearchV.Shortcuts() {
		lines = append(lines, fmt.Sprintf("[%s]<%s>[-] %s", a.cfg.Colors.Accent, sc.Key, sc.Description))
	}
	a.contextPanel.SetText(strings.Join(lines, "\n"))
}

// OpenLogEventDetail renders the full detail for event and switches to
// the log-event-detail page.
func (a *App) OpenLogEventDetail(event awslogs.LogEvent) {
	a.logDetailV.Render(event)
	a.pages.SwitchToPage("log-event-detail")
	a.tv.SetFocus(a.pages)
}

// OpenDatadogLogDetail renders the full detail for event and switches
// to the datadog-log-detail page.
func (a *App) OpenDatadogLogDetail(event datadoglogs.LogEvent) {
	a.datadogLogDetailV.Render(event)
	a.pages.SwitchToPage("datadog-log-detail")
	a.tv.SetFocus(a.pages)
}

// OpenCodePipelineDetail opens pipelineName's stage-status detail view
// and starts loading its current state.
func (a *App) OpenCodePipelineDetail(pipelineName string) {
	a.codePipelineDetailV.Open(pipelineName)
	a.pages.SwitchToPage("codepipeline-detail")
	a.tv.SetFocus(a.pages)
	lines := make([]string, 0, len(a.codePipelineDetailV.Shortcuts()))
	for _, sc := range a.codePipelineDetailV.Shortcuts() {
		lines = append(lines, fmt.Sprintf("[%s]<%s>[-] %s", a.cfg.Colors.Accent, sc.Key, sc.Description))
	}
	a.contextPanel.SetText(strings.Join(lines, "\n"))
}
