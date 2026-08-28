package app

import (
	"context"
	"strings"
	"testing"

	"github.com/ePex/cloudtui/tui/internal/config"
	"github.com/ePex/cloudtui/tui/internal/queue"
	"github.com/ePex/cloudtui/tui/internal/queue/secretbackend"
)

// TestSaveDatadogConfigPersists confirms App's real SaveDatadogConfig
// (the ui.Host method the Datadog editor overlay calls) actually
// updates cfg.Datadog and persists it to config.yaml — the half of
// the old TestSaveDatadogEditorRoundTrip test (pre-CR-78, when
// DatadogEditor still lived in this package) that isn't reachable
// from internal/dialog's own tests, since testHost only records this
// call rather than persisting anything.
func TestSaveDatadogConfigPersists(t *testing.T) {
	setHomeDir(t, t.TempDir())

	a := New(config.Default())
	want := config.DatadogConfig{Site: "datadoghq.eu", AccessToken: "tok-456"}

	a.SaveDatadogConfig(want)

	if a.cfg.Datadog != want {
		t.Errorf("cfg.Datadog = %+v, want %+v", a.cfg.Datadog, want)
	}

	path, err := config.DefaultPath()
	if err != nil {
		t.Fatalf("config.DefaultPath() error = %v", err)
	}
	got, err := config.Load(path)
	if err != nil {
		t.Fatalf("Load() error = %v", err)
	}
	if got.Datadog != want {
		t.Errorf("persisted Datadog = %+v, want %+v", got.Datadog, want)
	}
}

// TestSetActiveAWSProfilePersistsAndUpdatesUI confirms App's real
// SetActiveAWSProfile (the ui.Host method the AWS Profiles overlay
// calls) updates the info panel and settings list, and persists to
// config.yaml — the half of the old TestActivateAWSProfilePersistsAndUpdatesUI
// test (pre-CR-78, when AWSProfilesPicker still lived in this
// package) that isn't reachable from internal/dialog's own tests,
// since testHost only records this call rather than updating any
// other App state or persisting anything.
func TestSetActiveAWSProfilePersistsAndUpdatesUI(t *testing.T) {
	setHomeDir(t, t.TempDir())
	a := New(config.Default())

	a.SetActiveAWSProfile("work")

	if got := a.cfg.ActiveAWSProfile; got != "work" {
		t.Errorf("cfg.ActiveAWSProfile = %q, want %q", got, "work")
	}
	if got := a.infoPanel.GetText(true); !strings.Contains(got, "work") {
		t.Errorf("info panel = %q, want it to contain %q", got, "work")
	}
	if main2, _ := a.settingsV.List().GetItemText(2); !strings.Contains(main2, "work") {
		t.Errorf("settings list item 2 = %q, want it to contain %q", main2, "work")
	}
	path, err := config.DefaultPath()
	if err != nil {
		t.Fatalf("config.DefaultPath() error = %v", err)
	}
	if _, err := config.Load(path); err != nil {
		t.Errorf("config.yaml not written after SetActiveAWSProfile: %v", err)
	}
}

// TestToggleFavoritePersists confirms App's real ToggleFavorite (the
// ui.Host method the SSM Parameters/Secrets Manager/CloudWatch Logs
// views call) updates cfg.AWSFavorites and persists it to config.yaml.
func TestToggleFavoritePersists(t *testing.T) {
	setHomeDir(t, t.TempDir())

	a := New(config.Default())

	a.ToggleFavorite(config.FavoriteSSMParameter, "work", "/app/db/password")

	if !a.cfg.AWSFavorites.IsFavorite(config.FavoriteSSMParameter, "work", "/app/db/password") {
		t.Error("cfg.AWSFavorites does not have the favorite after ToggleFavorite")
	}

	path, err := config.DefaultPath()
	if err != nil {
		t.Fatalf("config.DefaultPath() error = %v", err)
	}
	got, err := config.Load(path)
	if err != nil {
		t.Fatalf("Load() error = %v", err)
	}
	if !got.AWSFavorites.IsFavorite(config.FavoriteSSMParameter, "work", "/app/db/password") {
		t.Error("persisted AWSFavorites does not have the favorite")
	}
}

// TestSetActiveAWSProfileRebuildsSecretBackedBackend is a regression
// test for spec/88: switching AWS profile must rebuild a
// Secrets-Manager-backed connection's backend, not leave it resolving
// against the profile that was active when it was first built.
func TestSetActiveAWSProfileRebuildsSecretBackedBackend(t *testing.T) {
	setHomeDir(t, t.TempDir())
	a := New(config.Default())
	a.cfg.Connections = append(a.cfg.Connections, config.Connection{
		Name: "secret-conn", Backend: "jolokia",
		Queue: config.QueueConfig{PasswordSecret: "my-secret"},
	})
	a.switchConnection("secret-conn")

	before, ok := a.backend.(*secretbackend.Backend)
	if !ok {
		t.Fatalf("a.backend = %T, want *secretbackend.Backend", a.backend)
	}
	if before.Profile() != "" {
		t.Errorf("Backend.Profile() before SetActiveAWSProfile = %q, want empty", before.Profile())
	}

	a.SetActiveAWSProfile("work")

	after, ok := a.backend.(*secretbackend.Backend)
	if !ok {
		t.Fatalf("a.backend after SetActiveAWSProfile = %T, want *secretbackend.Backend", a.backend)
	}
	if after.Profile() != "work" {
		t.Errorf("Backend.Profile() after SetActiveAWSProfile = %q, want %q", after.Profile(), "work")
	}
	if after == before {
		t.Error("a.backend is the same *secretbackend.Backend instance after SetActiveAWSProfile, want a rebuilt one")
	}
}

func TestDistinctJMSTypes(t *testing.T) {
	tests := []struct {
		name string
		msgs []queue.Message
		want []string
	}{
		{
			name: "dedupes and sorts",
			msgs: []queue.Message{
				{JMSType: "OrderCreated"},
				{JMSType: "OrderCancelled"},
				{JMSType: "OrderCreated"},
			},
			want: []string{"OrderCancelled", "OrderCreated"},
		},
		{
			name: "empty JMSType excluded",
			msgs: []queue.Message{{JMSType: ""}, {JMSType: "text"}},
			want: []string{"text"},
		},
		{
			name: "empty input",
			msgs: nil,
			want: nil,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := distinctJMSTypes(tt.msgs)
			if len(got) != len(tt.want) {
				t.Fatalf("distinctJMSTypes() = %v, want %v", got, tt.want)
			}
			for i := range tt.want {
				if got[i] != tt.want[i] {
					t.Errorf("distinctJMSTypes()[%d] = %q, want %q (full: %v)", i, got[i], tt.want[i], got)
				}
			}
		})
	}
}

func TestLoadedJMSTypesEmptyWhenNothingLoaded(t *testing.T) {
	a := New(config.Default())

	if got := a.LoadedJMSTypes(); len(got) != 0 {
		t.Errorf("LoadedJMSTypes() = %v, want empty (no queue opened)", got)
	}
}

// TestLoadedJMSTypesNilMessagesViewDoesNotPanic is a regression test:
// dialog.NewMessageFilter (built before a.messagesV — see App.New())
// wires LoadedJMSTypes into the JMS Type field's SetAutocompleteFunc,
// which eagerly calls Autocomplete() once immediately at wiring time —
// before a.messagesV exists. New(config.Default()) itself would already
// panic if this guard regressed (every app_test.go test constructing an
// App would fail), but this pins the exact behavior directly rather than
// relying on that as incidental coverage.
func TestLoadedJMSTypesNilMessagesViewDoesNotPanic(t *testing.T) {
	a := &App{}

	if got := a.LoadedJMSTypes(); got != nil {
		t.Errorf("LoadedJMSTypes() with nil messagesV = %v, want nil", got)
	}
}

// fakeBrowseBackend is a minimal queue.Backend double for ScanJMSTypes'
// wiring test — only BrowseMessages is exercised, so every other method
// panics rather than returning a silently-wrong zero value.
type fakeBrowseBackend struct {
	browseMessagesFn func(ctx context.Context, queueName string, filter queue.MessageFilter) ([]queue.Message, error)
}

func (f *fakeBrowseBackend) List(context.Context) ([]queue.Summary, error) { panic("not used") }
func (f *fakeBrowseBackend) BrowseMessages(ctx context.Context, queueName string, filter queue.MessageFilter) ([]queue.Message, error) {
	return f.browseMessagesFn(ctx, queueName, filter)
}
func (f *fakeBrowseBackend) PurgeQueue(context.Context, string) error { panic("not used") }
func (f *fakeBrowseBackend) RemoveMessage(context.Context, string, string) error {
	panic("not used")
}
func (f *fakeBrowseBackend) MoveMessage(context.Context, string, string, string) error {
	panic("not used")
}
func (f *fakeBrowseBackend) MoveAllMessages(context.Context, string, string) (int, error) {
	panic("not used")
}
func (f *fakeBrowseBackend) SendMessage(context.Context, string, string) error { panic("not used") }
func (f *fakeBrowseBackend) DeleteMessages(context.Context, string, queue.MessageFilter) (int, error) {
	panic("not used")
}
func (f *fakeBrowseBackend) MoveMessages(context.Context, string, string, queue.MessageFilter) (int, error) {
	panic("not used")
}

var _ queue.Backend = (*fakeBrowseBackend)(nil)

// TestScanJMSTypesCallsBrowseWithNoJMSTypeFilter confirms ScanJMSTypes
// browses with the given maxCount and no JMSType filter (it's
// discovering types, not narrowing by one already known), and extracts
// the distinct types from whatever the backend returns. The queue name
// passed through is whatever messagesV.QueueName() currently is — this
// test never calls messagesV.Open() (which would spawn Load()'s async
// goroutine against a *tview.Application with no running event loop, a
// path no other test in this package exercises either).
func TestScanJMSTypesCallsBrowseWithNoJMSTypeFilter(t *testing.T) {
	a := New(config.Default())
	var gotQueue string
	var gotFilter queue.MessageFilter
	a.backend = &fakeBrowseBackend{
		browseMessagesFn: func(_ context.Context, queueName string, filter queue.MessageFilter) ([]queue.Message, error) {
			gotQueue = queueName
			gotFilter = filter
			return []queue.Message{{JMSType: "a"}, {JMSType: "b"}, {JMSType: "a"}}, nil
		},
	}

	got, err := a.ScanJMSTypes(context.Background(), "orders", 5000)
	if err != nil {
		t.Fatalf("ScanJMSTypes() error = %v", err)
	}

	if gotQueue != "orders" {
		t.Errorf("queue passed to BrowseMessages = %q, want %q", gotQueue, "orders")
	}
	if want := (queue.MessageFilter{MaxCount: 5000}); gotFilter != want {
		t.Errorf("filter passed to BrowseMessages = %+v, want %+v", gotFilter, want)
	}
	want := []string{"a", "b"}
	if len(got) != len(want) || got[0] != want[0] || got[1] != want[1] {
		t.Errorf("ScanJMSTypes() = %v, want %v", got, want)
	}
}

func TestScanJMSTypesPropagatesBackendError(t *testing.T) {
	a := New(config.Default())
	wantErr := context.DeadlineExceeded
	a.backend = &fakeBrowseBackend{
		browseMessagesFn: func(context.Context, string, queue.MessageFilter) ([]queue.Message, error) {
			return nil, wantErr
		},
	}

	_, err := a.ScanJMSTypes(context.Background(), "orders", 5000)
	if err != wantErr {
		t.Errorf("ScanJMSTypes() error = %v, want %v", err, wantErr)
	}
}
