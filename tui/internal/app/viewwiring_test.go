// viewwiring_test.go holds the tests that verify app.go's own
// cross-view wiring (viewwiring.go's Open* trampolines and the onBack
// closures passed to each detail view's constructor) — these need the
// real, fully-wired *App, since they assert on an actual page switch,
// not just on a view's own isolated behavior. See spec/84.
package app

import (
	"strings"
	"testing"
	"time"

	"github.com/ePex/cloudtui/tui/internal/awslogs"
	"github.com/ePex/cloudtui/tui/internal/awssecrets"
	"github.com/ePex/cloudtui/tui/internal/awsssm"
	"github.com/ePex/cloudtui/tui/internal/config"
	"github.com/ePex/cloudtui/tui/internal/datadoglogs"
	"github.com/ePex/cloudtui/tui/internal/queue"
	"github.com/ePex/cloudtui/tui/internal/ui"
	"github.com/gdamore/tcell/v2"
	"github.com/rivo/tview"
)

func TestOpenLogEventDetailSwitchesPage(t *testing.T) {
	a := New(config.Default())

	a.OpenLogEventDetail(awslogs.LogEvent{Message: "hello"})

	if name, _ := a.pages.GetFrontPage(); name != "log-event-detail" {
		t.Errorf("front page = %q, want %q", name, "log-event-detail")
	}
}

func TestLogDetailViewEscReturnsToLogSearch(t *testing.T) {
	a := New(config.Default())
	a.OpenLogEventDetail(awslogs.LogEvent{Message: "hello"})

	a.logDetailV.Primitive().InputHandler()(tcell.NewEventKey(tcell.KeyEscape, 0, tcell.ModNone), func(tview.Primitive) {})

	if name, _ := a.pages.GetFrontPage(); name != "log-search" {
		t.Errorf("front page after Esc = %q, want %q", name, "log-search")
	}
}

func TestOpenDatadogLogDetailSwitchesPage(t *testing.T) {
	a := New(config.Default())

	a.OpenDatadogLogDetail(datadoglogs.LogEvent{Message: "hello"})

	if name, _ := a.pages.GetFrontPage(); name != "datadog-log-detail" {
		t.Errorf("front page = %q, want %q", name, "datadog-log-detail")
	}
}

func TestDatadogLogDetailViewEscReturnsToDatadogLogs(t *testing.T) {
	a := New(config.Default())
	a.OpenDatadogLogDetail(datadoglogs.LogEvent{Message: "hello"})

	a.datadogLogDetailV.Primitive().InputHandler()(tcell.NewEventKey(tcell.KeyEscape, 0, tcell.ModNone), func(tview.Primitive) {})

	if name, _ := a.pages.GetFrontPage(); name != "datadog-logs" {
		t.Errorf("front page after Esc = %q, want %q", name, "datadog-logs")
	}
}

func TestDatadogLogDetailViewGoToCloudWatchWithCorrelationID(t *testing.T) {
	a := New(config.Default())
	a.OpenDatadogLogDetail(datadoglogs.LogEvent{
		Message: "something happened CorrelationID: 1745d042-94e8-49f0-b223-8900ed9e951e",
	})

	capture := a.datadogLogDetailV.Primitive().(*tview.TextView).GetInputCapture()
	capture(tcell.NewEventKey(tcell.KeyRune, 'g', tcell.ModNone))

	// Quoted (see the 'g' handler's comment): CloudWatch's filter-pattern
	// syntax otherwise tokenizes on the UUID's internal hyphens.
	if want := `"1745d042-94e8-49f0-b223-8900ed9e951e"`; a.pendingCloudWatchPattern != want {
		t.Errorf("pendingCloudWatchPattern = %q, want %q", a.pendingCloudWatchPattern, want)
	}
	if name, _ := a.pages.GetFrontPage(); name != "cloudwatch-logs" {
		t.Errorf("front page after 'g' = %q, want %q", name, "cloudwatch-logs")
	}
}

func TestDatadogLogDetailViewGoToCloudWatchWithoutCorrelationID(t *testing.T) {
	a := New(config.Default())
	a.OpenDatadogLogDetail(datadoglogs.LogEvent{Message: "no correlation id here"})

	capture := a.datadogLogDetailV.Primitive().(*tview.TextView).GetInputCapture()
	capture(tcell.NewEventKey(tcell.KeyRune, 'g', tcell.ModNone))

	if a.pendingCloudWatchPattern != "" {
		t.Errorf("pendingCloudWatchPattern = %q, want empty", a.pendingCloudWatchPattern)
	}
	if name, _ := a.pages.GetFrontPage(); name != "datadog-log-detail" {
		t.Errorf("front page after 'g' with no CorrelationID = %q, want unchanged %q", name, "datadog-log-detail")
	}
}

// TestOpenParamDetailSwitchesPage asserts only the page switch — the
// title-set is now folded into ParamDetailView.Render itself and covered
// there (TestParamDetailViewRenderShowsStringValueImmediately), not
// reachable from here since dv.textView is unexported in a different
// package post-move.
func TestOpenParamDetailSwitchesPage(t *testing.T) {
	a := New(config.Default())

	a.OpenParamDetail(awsssm.Parameter{Name: "/app/name", Type: awsssm.TypeString, Value: "hello"})

	if name, _ := a.pages.GetFrontPage(); name != "ssm-param-detail" {
		t.Errorf("front page = %q, want %q", name, "ssm-param-detail")
	}
}

func TestParamDetailViewEscReturnsToSSMParameters(t *testing.T) {
	a := New(config.Default())
	a.OpenParamDetail(awsssm.Parameter{Name: "/app/name", Type: awsssm.TypeString, Value: "hello"})

	a.paramDetailV.Primitive().InputHandler()(tcell.NewEventKey(tcell.KeyEscape, 0, tcell.ModNone), func(tview.Primitive) {})

	if name, _ := a.pages.GetFrontPage(); name != "ssm-parameters" {
		t.Errorf("front page after Esc = %q, want %q", name, "ssm-parameters")
	}
}

// TestOpenSecretDetailSwitchesPage asserts only the page switch — the
// title-set is now folded into SecretDetailView.Render itself and
// covered there (TestSecretDetailViewRenderShowsMaskedBeforeReveal), not
// reachable from here since dv.textView is unexported in a different
// package post-move.
func TestOpenSecretDetailSwitchesPage(t *testing.T) {
	a := New(config.Default())

	a.OpenSecretDetail(awssecrets.Secret{Name: "/app/db"})

	if name, _ := a.pages.GetFrontPage(); name != "secret-detail" {
		t.Errorf("front page = %q, want %q", name, "secret-detail")
	}
}

func TestSecretDetailViewEscReturnsToSecretsManager(t *testing.T) {
	a := New(config.Default())
	a.OpenSecretDetail(awssecrets.Secret{Name: "/app/db"})

	a.secretDetailV.Primitive().InputHandler()(tcell.NewEventKey(tcell.KeyEscape, 0, tcell.ModNone), func(tview.Primitive) {})

	if name, _ := a.pages.GetFrontPage(); name != "secrets-manager" {
		t.Errorf("front page after Esc = %q, want %q", name, "secrets-manager")
	}
}

func TestMessageDetailViewEscReturnsToMessages(t *testing.T) {
	a := New(config.Default())
	a.OpenMessageDetail("orders", queue.Message{ID: "ID:test:1:1", Timestamp: time.Now()})

	a.messageDetailV.Primitive().InputHandler()(tcell.NewEventKey(tcell.KeyEscape, 0, tcell.ModNone), func(tview.Primitive) {})

	if name, _ := a.pages.GetFrontPage(); name != "messages" {
		t.Errorf("front page after Esc = %q, want %q", name, "messages")
	}
}

// Whether selecting a row correctly maps through the current filter to
// the right pipeline is covered at the view level
// (TestCodePipelineListViewSelectedFuncMapsThroughFilter in
// internal/view/codepipelinelist_test.go, same pattern as
// TestSSMParamsViewSelectedFuncMapsThroughFilter); whether app.go wires
// a.OpenCodePipelineDetail as that view's onSelect callback is covered by
// TestOpenCodePipelineDetailSwitchesPage above — nothing left here that
// isn't redundant with those two.

func TestOpenCodePipelineDetailSwitchesPage(t *testing.T) {
	a := New(config.Default())
	a.cfg.ActiveAWSProfile = "" // load() bails out synchronously, no goroutine spawned

	a.OpenCodePipelineDetail("my-pipeline")

	if name, _ := a.pages.GetFrontPage(); name != "codepipeline-detail" {
		t.Errorf("front page = %q, want %q", name, "codepipeline-detail")
	}
	if got := a.codePipelineDetailV.PipelineName(); got != "my-pipeline" {
		t.Errorf("pipelineName = %q, want %q", got, "my-pipeline")
	}
}

func TestCodePipelineDetailViewEscReturnsToList(t *testing.T) {
	a := New(config.Default())
	a.cfg.ActiveAWSProfile = ""
	a.OpenCodePipelineDetail("my-pipeline")

	a.codePipelineDetailV.Primitive().InputHandler()(tcell.NewEventKey(tcell.KeyEscape, 0, tcell.ModNone), func(tview.Primitive) {})

	if name, _ := a.pages.GetFrontPage(); name != "codepipeline" {
		t.Errorf("front page after Esc = %q, want %q", name, "codepipeline")
	}
}

func TestLogSearchViewEscReturnsToCloudWatchLogs(t *testing.T) {
	a := New(config.Default())
	a.OpenLogSearch("/aws/lambda/foo")

	a.logSearchV.Primitive().InputHandler()(tcell.NewEventKey(tcell.KeyEscape, 0, tcell.ModNone), func(tview.Primitive) {})

	if name, _ := a.pages.GetFrontPage(); name != "cloudwatch-logs" {
		t.Errorf("front page after Esc = %q, want %q", name, "cloudwatch-logs")
	}
}

// TestSwitchThemeRefreshesSettingsList, TestSwitchConnectionRefreshesSettingsList,
// TestSaveConnectionRefreshesSettingsList, and
// TestSaveDatadogConfigRefreshesSettingsList confirm each config
// mutation that the Settings screen displays actually reaches
// SettingsView.Refresh() — SetActiveAWSProfile's equivalent is
// TestSetActiveAWSProfilePersistsAndUpdatesUI in host_test.go, which
// already covered this before the move.

func TestSwitchThemeRefreshesSettingsList(t *testing.T) {
	t.Chdir(t.TempDir())
	a := New(config.Default())
	t.Cleanup(func() { applyTheme(config.Default().Colors) })

	a.switchTheme("cyberpunk")

	main0, _ := a.settingsV.List().GetItemText(0)
	if !strings.Contains(main0, "cyberpunk") {
		t.Errorf("item 0 after switchTheme = %q, want it to contain 'cyberpunk'", main0)
	}
}

func TestSwitchConnectionRefreshesSettingsList(t *testing.T) {
	t.Chdir(t.TempDir())
	a := New(config.Default())
	a.cfg.Connections = append(a.cfg.Connections, config.Connection{Name: "other", Backend: "jolokia"})

	a.switchConnection("other")

	main1, _ := a.settingsV.List().GetItemText(1)
	if !strings.Contains(main1, "other") {
		t.Errorf("item 1 after switchConnection(\"other\") = %q, want it to contain %q", main1, "other")
	}
}

func TestSaveConnectionRefreshesSettingsList(t *testing.T) {
	t.Chdir(t.TempDir())
	a := New(config.Default())

	a.SaveConnection(config.Connection{Name: "renamed", Backend: "jolokia"}, "default", false)

	main1, _ := a.settingsV.List().GetItemText(1)
	if !strings.Contains(main1, "renamed") {
		t.Errorf("item 1 after SaveConnection = %q, want it to contain %q", main1, "renamed")
	}
}

func TestSaveDatadogConfigRefreshesSettingsList(t *testing.T) {
	t.Chdir(t.TempDir())
	a := New(config.Default())

	a.SaveDatadogConfig(config.DatadogConfig{Site: "datadoghq.eu", AccessToken: "tok"})

	main3, _ := a.settingsV.List().GetItemText(3)
	if !strings.Contains(main3, "datadoghq.eu") {
		t.Errorf("item 3 after SaveDatadogConfig = %q, want it to contain %q", main3, "datadoghq.eu")
	}
}

// TestLogViewIsWiredAsLogPage, TestLogViewImplementsShortcuttable, and
// TestLogViewShortcutsIncludeR moved here from log_test.go (now
// internal/view/log_test.go) — they exercise app.go's own wiring of
// the log view (does New() register it, does it come back through
// a.logV wired to satisfy the right interfaces), not anything
// LogView's own logic does differently. See spec/86.

func TestLogViewIsWiredAsLogPage(t *testing.T) {
	a := New(config.Default())
	if got := a.logV.Name(); got != "log" {
		t.Errorf("Name() = %q, want %q", got, "log")
	}
}

func TestLogViewImplementsShortcuttable(t *testing.T) {
	a := New(config.Default())
	_, ok := ui.View(a.logV).(ui.Shortcuttable)
	if !ok {
		t.Error("logV does not implement ui.Shortcuttable")
	}
}

func TestLogViewShortcutsIncludeR(t *testing.T) {
	a := New(config.Default())
	for _, s := range a.logV.Shortcuts() {
		if s.Key == "r" {
			return
		}
	}
	t.Error("Shortcuts() missing key \"r\"")
}
