package view

import (
	"context"
	"fmt"

	"github.com/ePex/cloudtui/tui/internal/awsauth"
	"github.com/ePex/cloudtui/tui/internal/ui"
)

// awsLoadHost is the minimal host surface runAWSLoad needs — every
// caller's own host type (ui.SSMParamsHost, ui.SecretsHost,
// ui.CloudWatchLogsHost, ui.CodePipelineHost) already embeds both
// ui.Host and ui.AWSAuthHost, so none needs a wider type just to call
// runAWSLoad.
type awsLoadHost interface {
	ui.Host
	ui.AWSAuthHost
}

// runAWSLoad is the shared shape behind SSMParamsView, SecretsView,
// LogsView, CodePipelineListView, and CodePipelineDetailView's load():
// guard on an empty AWS profile, bump *loadSeq, show a loading
// placeholder, fetch in a goroutine with SSO-reauth retry (awsauth.Do),
// then dispatch the result back on the UI goroutine — discarding it if
// a newer load() has since started. QueuesView isn't among the
// callers: its re-auth goes through secretbackend.SecretResolver/
// app.go's dispatch, never awsauth.WithReauth/Do directly, so it has
// no call site shaped like this one.
//
// showStatus/showError are taken as plain funcs (each view's own
// private methods) rather than the ui.ReauthStatusShower interface —
// that interface is for external callers reaching a view's status
// display (currently unused for these 5 views, kept for structural
// consistency with QueuesView); internally, ShowReauthWaiting/
// ShowReauthDone are already just one-line wrappers around showStatus,
// so calling ShowReauthDone() here to display the *initial* loading
// text (not "reauth done") would read wrong.
func runAWSLoad[T any](
	host awsLoadHost,
	loadSeq *int,
	showStatus func(string),
	showError func(error),
	loadingMsg string,
	fetch func(ctx context.Context, profile string) (T, error),
	onSuccess func(T),
) {
	profile := host.Config().ActiveAWSProfile
	if profile == "" {
		showError(fmt.Errorf("no AWS profile selected — use :ap to select one"))
		return
	}
	*loadSeq++
	seq := *loadSeq
	showStatus(loadingMsg)
	const reauthWaitingMsg = "AWS SSO session expired — opening browser to log in..."
	go func() {
		ctx := context.Background()
		result, err := awsauth.Do(ctx, profile, host.AWSAuthTypeFor, host.AWSSSOLogin,
			func() {
				host.QueueUpdateDraw(func() { showStatus(reauthWaitingMsg) })
			},
			func(code, url string) {
				host.QueueUpdateDraw(func() {
					showStatus(fmt.Sprintf("%s Verify code %s at %s", reauthWaitingMsg, code, url))
				})
			},
			func(ctx context.Context) (T, error) { return fetch(ctx, profile) },
		)
		host.QueueUpdateDraw(func() {
			if seq != *loadSeq {
				return // superseded by a newer load()
			}
			if err != nil {
				showError(err)
				return
			}
			onSuccess(result)
		})
	}()
}
