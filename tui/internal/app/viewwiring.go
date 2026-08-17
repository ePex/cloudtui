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

	"github.com/ePex/cloudtui/tui/internal/awslogs"
	"github.com/ePex/cloudtui/tui/internal/awssecrets"
	"github.com/ePex/cloudtui/tui/internal/awsssm"
	"github.com/ePex/cloudtui/tui/internal/datadoglogs"
	"github.com/ePex/cloudtui/tui/internal/queue"
)

// OpenMessages switches to the messages page for the given queue, sets the
// title, and starts loading messages asynchronously. Quick search and the
// server-side filter persist when returning to the same queue, but reset
// when switching to a different one — carrying a leftover filter across
// queues would silently narrow what the user sees without them asking.
func (a *App) OpenMessages(queueName string) {
	if a.messagesV.queueName != queueName {
		a.messagesV.filter = queue.MessageFilter{}
		a.messagesV.quickSearch = ""
		a.messagesV.searchInput.SetText("")
	}
	a.messagesV.queueName = queueName
	a.messagesV.updateTitle()
	a.messagesV.setHeader()
	a.pages.SwitchToPage("messages")
	a.tv.SetFocus(a.pages)
	// Show messagesV shortcuts in the context panel.
	lines := make([]string, 0, len(a.messagesV.Shortcuts()))
	for _, sc := range a.messagesV.Shortcuts() {
		lines = append(lines, fmt.Sprintf("[%s]<%s>[-] %s", a.cfg.Colors.Accent, sc.Key, sc.Description))
	}
	a.contextPanel.SetText(strings.Join(lines, "\n"))
	a.messagesV.load()
}

// wireQueuesOpensMessages wires Enter in the queues table to open the
// messages view for the selected queue. Called from New() once
// messagesV exists.
func (a *App) wireQueuesOpensMessages() {
	a.queuesV.table.SetSelectedFunc(func(row, _ int) {
		cell := a.queuesV.table.GetCell(row, 0)
		if cell == nil || cell.Text == "" {
			return
		}
		a.OpenMessages(cell.Text)
	})
}

// OpenMessageDetail renders the full detail for msg and switches to the
// message-detail page.
func (a *App) OpenMessageDetail(queueName string, msg queue.Message) {
	a.messageDetailV.render(queueName, msg)
	a.messageDetailV.textView.SetTitle(fmt.Sprintf(" Message Details — %s ", queueName))
	a.pages.SwitchToPage("message-detail")
	a.tv.SetFocus(a.pages)
	lines := make([]string, 0, len(a.messageDetailV.Shortcuts()))
	for _, sc := range a.messageDetailV.Shortcuts() {
		lines = append(lines, fmt.Sprintf("[%s]<%s>[-] %s", a.cfg.Colors.Accent, sc.Key, sc.Description))
	}
	a.contextPanel.SetText(strings.Join(lines, "\n"))
}

// wireMessagesOpensDetail wires Enter in the messages table to open the
// detail view for the selected message. Called from New() once
// messageDetailV exists.
func (a *App) wireMessagesOpensDetail() {
	a.messagesV.table.SetSelectedFunc(func(row, _ int) {
		msgIdx := row - 1 // row 0 is the header
		if msgIdx < 0 || msgIdx >= len(a.messagesV.msgs) {
			return
		}
		a.OpenMessageDetail(a.messagesV.queueName, a.messagesV.msgs[msgIdx])
	})
}

// OpenParamDetail renders the full detail for param and switches to the
// ssm-param-detail page. paramDetailView.render sets the context panel
// itself (its shortcuts change once a SecureString is revealed).
func (a *App) OpenParamDetail(param awsssm.Parameter) {
	a.paramDetailV.render(param)
	a.paramDetailV.textView.SetTitle(fmt.Sprintf(" Parameter — %s ", param.Name))
	a.pages.SwitchToPage("ssm-param-detail")
	a.tv.SetFocus(a.pages)
}

// wireSSMParamsOpensDetail wires Enter in the SSM parameters table to
// open the detail view for the selected parameter. Called from New()
// once paramDetailV exists.
func (a *App) wireSSMParamsOpensDetail() {
	a.ssmParamsV.table.SetSelectedFunc(func(row, _ int) {
		idx := row - 1 // row 0 is the header
		if idx < 0 || idx >= len(a.ssmParamsV.filtered) {
			return
		}
		a.OpenParamDetail(a.ssmParamsV.filtered[idx])
	})
}

func (a *App) OpenSecretDetail(secret awssecrets.Secret) {
	a.secretDetailV.render(secret)
	a.secretDetailV.textView.SetTitle(fmt.Sprintf(" Secret — %s ", secret.Name))
	a.pages.SwitchToPage("secret-detail")
	a.tv.SetFocus(a.pages)
}

// wireSecretsOpensDetail wires Enter in the secrets table to open the
// detail view for the selected secret. Called from New() once
// secretDetailV exists.
func (a *App) wireSecretsOpensDetail() {
	a.secretsV.table.SetSelectedFunc(func(row, _ int) {
		idx := row - 1 // row 0 is the header
		if idx < 0 || idx >= len(a.secretsV.filtered) {
			return
		}
		a.OpenSecretDetail(a.secretsV.filtered[idx])
	})
}

// OpenLogSearch opens the search view for logGroupName and runs the
// first search immediately (see logSearchView.open). logSearchView isn't
// a registered ui.View, so its context panel is populated manually here
// — same pattern as OpenMessages.
func (a *App) OpenLogSearch(logGroupName string) {
	pattern := a.pendingCloudWatchPattern
	a.pendingCloudWatchPattern = ""
	a.logSearchV.open(logGroupName, pattern)
	a.pages.SwitchToPage("log-search")
	a.tv.SetFocus(a.pages)
	lines := make([]string, 0, len(a.logSearchV.Shortcuts()))
	for _, sc := range a.logSearchV.Shortcuts() {
		lines = append(lines, fmt.Sprintf("[%s]<%s>[-] %s", a.cfg.Colors.Accent, sc.Key, sc.Description))
	}
	a.contextPanel.SetText(strings.Join(lines, "\n"))
}

// wireLogsOpensSearch wires Enter in the log groups table to open the
// search view for the selected log group. Called from New() once
// logSearchV exists.
func (a *App) wireLogsOpensSearch() {
	a.logsV.table.SetSelectedFunc(func(row, _ int) {
		idx := row - 1 // row 0 is the header
		if idx < 0 || idx >= len(a.logsV.filtered) {
			return
		}
		a.OpenLogSearch(a.logsV.filtered[idx].Name)
	})
}

// OpenLogEventDetail renders the full detail for event and switches to
// the log-event-detail page.
func (a *App) OpenLogEventDetail(event awslogs.LogEvent) {
	a.logDetailV.render(event)
	a.pages.SwitchToPage("log-event-detail")
	a.tv.SetFocus(a.pages)
}

// wireLogSearchOpensEventDetail wires Enter in the log search results
// table to open the detail view for the selected event. Called from
// New() once logDetailV exists.
func (a *App) wireLogSearchOpensEventDetail() {
	a.logSearchV.table.SetSelectedFunc(func(row, _ int) {
		idx := row - 1 // row 0 is the header
		if idx < 0 || idx >= len(a.logSearchV.results) {
			return
		}
		a.OpenLogEventDetail(a.logSearchV.results[idx])
	})
}

// OpenDatadogLogDetail renders the full detail for event and switches
// to the datadog-log-detail page.
func (a *App) OpenDatadogLogDetail(event datadoglogs.LogEvent) {
	a.datadogLogDetailV.render(event)
	a.pages.SwitchToPage("datadog-log-detail")
	a.tv.SetFocus(a.pages)
}

// wireDatadogLogsOpensDetail wires Enter in the Datadog Logs results
// table to open the detail view for the selected event. Called from
// New() once datadogLogDetailV exists.
func (a *App) wireDatadogLogsOpensDetail() {
	a.datadogLogsV.table.SetSelectedFunc(func(row, _ int) {
		idx := row - 1 // row 0 is the header
		if idx < 0 || idx >= len(a.datadogLogsV.results) {
			return
		}
		a.OpenDatadogLogDetail(a.datadogLogsV.results[idx])
	})
}

// OpenCodePipelineDetail opens pipelineName's stage-status detail view
// and starts loading its current state.
func (a *App) OpenCodePipelineDetail(pipelineName string) {
	a.codePipelineDetailV.open(pipelineName)
	a.pages.SwitchToPage("codepipeline-detail")
	a.tv.SetFocus(a.pages)
	lines := make([]string, 0, len(a.codePipelineDetailV.Shortcuts()))
	for _, sc := range a.codePipelineDetailV.Shortcuts() {
		lines = append(lines, fmt.Sprintf("[%s]<%s>[-] %s", a.cfg.Colors.Accent, sc.Key, sc.Description))
	}
	a.contextPanel.SetText(strings.Join(lines, "\n"))
}

// wireCodePipelineListOpensDetail wires Enter in the CodePipeline table
// to open the stage-status detail view for the selected pipeline. Called
// from New() once codePipelineDetailV exists.
func (a *App) wireCodePipelineListOpensDetail() {
	a.codePipelineListV.table.SetSelectedFunc(func(row, _ int) {
		idx := row - 1 // row 0 is the header
		if idx < 0 || idx >= len(a.codePipelineListV.filtered) {
			return
		}
		a.OpenCodePipelineDetail(a.codePipelineListV.filtered[idx].Name)
	})
}
