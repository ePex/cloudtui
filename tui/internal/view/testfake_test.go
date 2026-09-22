package view

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"
	"time"

	"github.com/gdamore/tcell/v2"
	"github.com/rivo/tview"

	"github.com/ePex/cloudtui/tui/internal/awscodepipeline"
	"github.com/ePex/cloudtui/tui/internal/awslogs"
	"github.com/ePex/cloudtui/tui/internal/awsprofile"
	"github.com/ePex/cloudtui/tui/internal/awssecrets"
	"github.com/ePex/cloudtui/tui/internal/awsssm"
	"github.com/ePex/cloudtui/tui/internal/config"
	"github.com/ePex/cloudtui/tui/internal/datadoglogs"
	"github.com/ePex/cloudtui/tui/internal/dialog"
	"github.com/ePex/cloudtui/tui/internal/queue"
	"github.com/ePex/cloudtui/tui/internal/snippet"
	"github.com/ePex/cloudtui/tui/internal/ui"
)

var _ ui.Host = (*fakeViewHost)(nil)
var _ ui.SSMParamsHost = (*fakeViewHost)(nil)
var _ ui.SecretsHost = (*fakeViewHost)(nil)
var _ ui.CloudWatchLogsHost = (*fakeViewHost)(nil)
var _ ui.DatadogLogsHost = (*fakeViewHost)(nil)
var _ ui.CodePipelineHost = (*fakeViewHost)(nil)
var _ ui.MessagesHost = (*fakeViewHost)(nil)

// fakeViewHost is a single wide double satisfying every one of the ui
// package's per-resource host interfaces at once (deliberately not
// split into one fake per interface — see
// spec-wip/cr-viewhost-interface-segregation/spec.md's "Out of scope"
// for why): it records what a view asked for instead of driving a
// real *App. Every data-fetcher method returns zero values unless a
// test sets the matching func field — same "inject a func, override
// per test" shape this codebase already used for App.listParameters
// etc. pre-CR-80.
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
	awsSSOLoginFn            func(ctx context.Context, profile string, onCode func(code, url string)) error
	loadedJMSTypes           []string
	messagesQueueName        string
	scanJMSTypesFn           func(ctx context.Context, queueName string, maxCount int) ([]string, error)
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
func (f *fakeViewHost) LoadedJMSTypes() []string                   { return f.loadedJMSTypes }
func (f *fakeViewHost) MessagesQueueName() string                  { return f.messagesQueueName }
func (f *fakeViewHost) ScanJMSTypes(ctx context.Context, queueName string, maxCount int) ([]string, error) {
	if f.scanJMSTypesFn == nil {
		return nil, nil
	}
	return f.scanJMSTypesFn(ctx, queueName, maxCount)
}

// -- chrome methods needed by one or more of the narrow per-resource
// interfaces (SwitchTo: DatadogLogsHost/MessagesHost; CopyToClipboard:
// SSMParamsHost/SecretsHost/CloudWatchLogsHost/DatadogLogsHost) --
func (f *fakeViewHost) SwitchTo(name string)        { f.shownPage = name }
func (f *fakeViewHost) CopyToClipboard(data string) { f.copiedData = data }

func (f *fakeViewHost) SetPendingCloudWatchPattern(pattern string, timestamp time.Time) {}

func (f *fakeViewHost) IsWatchingPipeline(name string) bool { return f.watching[name] }
func (f *fakeViewHost) StartWatchingPipeline(name string)   { f.watching[name] = true }
func (f *fakeViewHost) StopWatchingPipeline(name string)    { delete(f.watching, name) }

// -- AWS/Datadog data-fetchers needed by one or more of the narrow
// per-resource interfaces: injectable func field, zero value if unset --
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

func (f *fakeViewHost) AWSSSOLogin(ctx context.Context, profile string, onCode func(code, url string)) error {
	if f.awsSSOLoginFn != nil {
		return f.awsSSOLoginFn(ctx, profile, onCode)
	}
	return nil
}

// ── Live theme switch regression ─────────────────────────────────────────

// paletteColors returns every color value in p (all its string fields),
// as tcell colors. Duplicated from internal/dialog's dialogtest_test.go,
// like renderedScreenText.
func paletteColors(p config.Palette) map[tcell.Color]string {
	colors := map[tcell.Color]string{}
	v := reflect.ValueOf(p)
	for i := 0; i < v.NumField(); i++ {
		if v.Field(i).Kind() == reflect.String && v.Field(i).String() != "" {
			colors[tcell.GetColor(v.Field(i).String())] = v.Type().Field(i).Name
		}
	}
	return colors
}

// staleColors draws prim and describes every cell still carrying a color
// from old that new doesn't also use.
func staleColors(t *testing.T, prim tview.Primitive, width, height int, old, new config.Palette) []string {
	t.Helper()
	oldOnly := paletteColors(old)
	for c := range paletteColors(new) {
		delete(oldOnly, c)
	}

	prim.SetRect(0, 0, width, height)
	screen := tcell.NewSimulationScreen("")
	if err := screen.Init(); err != nil {
		t.Fatalf("screen.Init: %v", err)
	}
	defer screen.Fini()
	screen.SetSize(width, height)
	prim.Draw(screen)
	screen.Show()

	var stale []string
	cells, w, _ := screen.GetContents()
	for i, c := range cells {
		fg, bg, _ := c.Style.Decompose()
		r := ' '
		if len(c.Runes) > 0 {
			r = c.Runes[0]
		}
		for _, col := range []tcell.Color{fg, bg} {
			if name, ok := oldOnly[col]; ok {
				stale = append(stale, fmt.Sprintf("(%d,%d) %q uses old %s", i%w, i/w, r, name))
			}
		}
	}
	return stale
}

// TestViewsFullyRecolorOnLiveThemeSwitch builds every view while
// tview.Styles and the host's palette hold "dark" (as at startup), gives
// it some content, switches the host to "cyberpunk" and calls
// ApplyPalette — what a live theme switch does — and fails if any drawn
// cell still carries a dark-only color. List-style views keep their
// widgets across a switch, so they're checked as they are. Detail views
// (and the log view) are shown again after the switch, as happens when
// you next open them, via their reopen func. Their text is rebuilt then,
// but untagged characters still use the text view's base color, which is
// the part this checks.
func TestViewsFullyRecolorOnLiveThemeSwitch(t *testing.T) {
	dark, _ := config.PaletteForTheme("dark")
	cyber, _ := config.PaletteForTheme("cyberpunk")

	type themedView interface {
		ui.Themeable
		Primitive() tview.Primitive
	}
	tests := []struct {
		name  string
		build func(t *testing.T, host *fakeViewHost) (themedView, func())
	}{
		{"SettingsView", func(t *testing.T, host *fakeViewHost) (themedView, func()) {
			return NewSettingsView(host, dialog.NewThemePicker(host), dialog.NewConnManager(host, dialog.NewConfirmDialog(host)),
				dialog.NewAWSProfilesPicker(host), dialog.NewDatadogEditor(host)), nil
		}},
		{"QueuesView", func(t *testing.T, host *fakeViewHost) (themedView, func()) {
			v := NewQueuesView(host, host.backend, dialog.NewConfirmDialog(host), dialog.NewMovePicker(host),
				dialog.NewSendMessageOverlay(host, dialog.NewSnippetPicker(host, snippet.NewStore("")), dialog.NewConfirmDialog(host)),
				dialog.NewJMSTypePrompt(host), func(string) {})
			v.filterInput.SetText("ord")
			return v, nil
		}},
		{"MessagesView", func(t *testing.T, host *fakeViewHost) (themedView, func()) {
			v := NewMessagesView(host, dialog.NewMessageFilter(host),
				dialog.NewSendMessageOverlay(host, dialog.NewSnippetPicker(host, snippet.NewStore("")), dialog.NewConfirmDialog(host)),
				dialog.NewConfirmDialog(host), dialog.NewMovePicker(host), func(string, queue.Message) {})
			v.searchInput.SetText("ord")
			return v, nil
		}},
		{"SSMParamsView", func(t *testing.T, host *fakeViewHost) (themedView, func()) {
			return NewSSMParamsView(host, func(awsssm.Parameter) {}), nil
		}},
		{"SecretsView", func(t *testing.T, host *fakeViewHost) (themedView, func()) {
			return NewSecretsView(host, func(awssecrets.Secret) {}), nil
		}},
		{"LogsView", func(t *testing.T, host *fakeViewHost) (themedView, func()) {
			return NewLogsView(host, func(string) {}), nil
		}},
		{"LogSearchView", func(t *testing.T, host *fakeViewHost) (themedView, func()) {
			return NewLogSearchView(host, dialog.NewTimeRangeModal(host), func(awslogs.LogEvent) {}, func() {}), nil
		}},
		{"DatadogLogsView", func(t *testing.T, host *fakeViewHost) (themedView, func()) {
			return NewDatadogLogsView(host, dialog.NewTimeRangeModal(host), func(datadoglogs.LogEvent) {}), nil
		}},
		{"CodePipelineListView", func(t *testing.T, host *fakeViewHost) (themedView, func()) {
			return NewCodePipelineListView(host, func(string) {}), nil
		}},
		{"SnippetsView", func(t *testing.T, host *fakeViewHost) (themedView, func()) {
			root := t.TempDir()
			if err := os.WriteFile(filepath.Join(root, "ping.txt"), []byte("---\njmsType: T\n---\nbody"), 0o644); err != nil {
				t.Fatal(err)
			}
			store := snippet.NewStore(root)
			confirm := dialog.NewConfirmDialog(host)
			v := NewSnippetsView(host, store, confirm, dialog.NewSnippetEditor(host, store, confirm), dialog.NewTextPrompt(host))
			return v, nil
		}},
		{"MessageDetailView", func(t *testing.T, host *fakeViewHost) (themedView, func()) {
			confirm := dialog.NewConfirmDialog(host)
			v := NewMessageDetailView(host, dialog.NewMovePicker(host), confirm,
				dialog.NewSnippetSaveDialog(host, snippet.NewStore(""), confirm), func() {}, func() {})
			msg := queue.Message{ID: "ID:1", JMSType: "OrderCreated", Timestamp: time.Now(),
				RawFields: map[string]any{"text": `{"id":1}`, "jMSCorrelationID": "c-1"}}
			return v, func() { v.Render("orders", msg) }
		}},
		{"ParamDetailView", func(t *testing.T, host *fakeViewHost) (themedView, func()) {
			v := NewParamDetailView(host, func() {})
			return v, func() {
				v.Render(awsssm.Parameter{Name: "/app/db/url", Type: "String", Value: "x", LastModified: time.Now()})
			}
		}},
		{"SecretDetailView", func(t *testing.T, host *fakeViewHost) (themedView, func()) {
			v := NewSecretDetailView(host, func() {})
			return v, func() { v.Render(awssecrets.Secret{Name: "app/db", ARN: "arn:x", LastChanged: time.Now()}) }
		}},
		{"LogDetailView", func(t *testing.T, host *fakeViewHost) (themedView, func()) {
			v := NewLogDetailView(host, func() {})
			return v, func() { v.Render(awslogs.LogEvent{Timestamp: time.Now(), LogStream: "s", Message: "hello world"}) }
		}},
		{"DatadogLogDetailView", func(t *testing.T, host *fakeViewHost) (themedView, func()) {
			v := NewDatadogLogDetailView(host, func() {})
			return v, func() {
				v.Render(datadoglogs.LogEvent{Timestamp: time.Now(), Service: "api", Status: "info", Host: "h"})
			}
		}},
		{"CodePipelineDetailView", func(t *testing.T, host *fakeViewHost) (themedView, func()) {
			v := NewCodePipelineDetailView(host, func() {})
			return v, func() { v.Render([]awscodepipeline.StageStatus{{Name: "Build", Status: "Succeeded"}}) }
		}},
		{"LogView", func(t *testing.T, host *fakeViewHost) (themedView, func()) {
			// The second line has no level, so colorizeLog leaves it
			// untagged: drawn in the text view's base color.
			path := filepath.Join(t.TempDir(), "cloudtui.log")
			if err := os.WriteFile(path, []byte("time=now level=INFO msg=hello\n  continued value, no level\n"), 0o644); err != nil {
				t.Fatal(err)
			}
			v := NewLogViewWithPath(path)
			return v, v.Activate
		}},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			saved := tview.Styles
			t.Cleanup(func() { tview.Styles = saved })
			ui.ApplyTviewStyles(dark)
			host := newFakeViewHost()
			host.cfg.Colors = dark

			v, reopen := tt.build(t, host)
			v.ApplyPalette(dark) // as App.New does at startup
			if reopen != nil {
				reopen()
			}

			ui.ApplyTviewStyles(cyber) // the live switch: reapplyTheme
			host.cfg.Colors = cyber
			v.ApplyPalette(cyber)
			if reopen != nil {
				reopen()
			}

			if stale := staleColors(t, v.Primitive(), 100, 20, dark, cyber); len(stale) > 0 {
				t.Errorf("%d cells keep dark-only colors after the switch, e.g. %s", len(stale), strings.Join(stale[:min(len(stale), 5)], "; "))
			}
		})
	}
}
