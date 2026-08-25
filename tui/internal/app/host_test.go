package app

import (
	"path/filepath"
	"strings"
	"testing"

	"github.com/ePex/cloudtui/tui/internal/config"
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
	dir := t.TempDir()
	t.Chdir(dir)

	a := New(config.Default())
	want := config.DatadogConfig{Site: "datadoghq.eu", AccessToken: "tok-456"}

	a.SaveDatadogConfig(want)

	if a.cfg.Datadog != want {
		t.Errorf("cfg.Datadog = %+v, want %+v", a.cfg.Datadog, want)
	}

	got, err := config.Load(filepath.Join(dir, "config.yaml"))
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
	t.Chdir(t.TempDir())
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
	if _, err := config.Load("config.yaml"); err != nil {
		t.Errorf("config.yaml not written after SetActiveAWSProfile: %v", err)
	}
}

// TestToggleFavoritePersists confirms App's real ToggleFavorite (the
// ui.Host method the SSM Parameters/Secrets Manager/CloudWatch Logs
// views call) updates cfg.AWSFavorites and persists it to config.yaml.
func TestToggleFavoritePersists(t *testing.T) {
	dir := t.TempDir()
	t.Chdir(dir)

	a := New(config.Default())

	a.ToggleFavorite(config.FavoriteSSMParameter, "work", "/app/db/password")

	if !a.cfg.AWSFavorites.IsFavorite(config.FavoriteSSMParameter, "work", "/app/db/password") {
		t.Error("cfg.AWSFavorites does not have the favorite after ToggleFavorite")
	}

	got, err := config.Load(filepath.Join(dir, "config.yaml"))
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
	t.Chdir(t.TempDir())
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
