package config

import (
	"os"
	"path/filepath"
	"reflect"
	"testing"
)

func TestDefault(t *testing.T) {
	want := Config{
		Logo: []string{
			"╔═══════════╗",
			"║ CLOUDTUI  ║",
			"╚═══════════╝",
		},
		Colors: Palette{
			Border:  "green",
			Label:   "yellow",
			Value:   "white",
			Accent:  "aqua",
			Success: "green",
			Warning: "yellow",
			Error:   "red",
			Views: map[string]string{
				"home":     "aqua",
				"secrets":  "yellow",
				"params":   "teal",
				"queues":   "fuchsia",
				"settings": "gray",
			},
		},
	}
	want.Queue = QueueConfig{ProxyURL: "http://localhost:8081", Username: "admin"}

	if got := Default(); !reflect.DeepEqual(got, want) {
		t.Errorf("Default() = %#v, want %#v", got, want)
	}
	if got := Default().AWS.Profile; got != "" {
		t.Errorf("Default().AWS.Profile = %q, want empty (not set)", got)
	}
	if got := Default().Queue.Password; got != "" {
		t.Errorf("Default().Queue.Password = %q, want empty (not set)", got)
	}
}

func TestPaletteViewColor(t *testing.T) {
	p := Palette{
		Border: "green",
		Views:  map[string]string{"secrets": "yellow"},
	}

	if got, want := p.ViewColor("secrets"), "yellow"; got != want {
		t.Errorf("ViewColor(%q) = %q, want %q", "secrets", got, want)
	}
	if got, want := p.ViewColor("unknown"), "green"; got != want {
		t.Errorf("ViewColor(%q) = %q, want fallback %q", "unknown", got, want)
	}
}

func TestLoadMissingFileReturnsDefault(t *testing.T) {
	path := filepath.Join(t.TempDir(), "missing.yaml")

	got, err := Load(path)
	if err != nil {
		t.Fatalf("Load() error = %v, want nil", err)
	}
	if want := Default(); !reflect.DeepEqual(got, want) {
		t.Errorf("Load(missing) = %#v, want %#v", got, want)
	}
}

func TestLoadFullOverride(t *testing.T) {
	path := filepath.Join(t.TempDir(), "config.yaml")
	content := `
logo:
  - "AAA"
  - "BBB"
colors:
  border: red
  label: blue
  value: black
  accent: pink
  success: lime
  warning: orange
  error: maroon
  views:
    home: white
    secrets: white
    params: white
    queues: white
    settings: white
`
	if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
		t.Fatal(err)
	}

	got, err := Load(path)
	if err != nil {
		t.Fatalf("Load() error = %v", err)
	}

	want := Config{
		Logo: []string{"AAA", "BBB"},
		Colors: Palette{
			Border:  "red",
			Label:   "blue",
			Value:   "black",
			Accent:  "pink",
			Success: "lime",
			Warning: "orange",
			Error:   "maroon",
			Views: map[string]string{
				"home":     "white",
				"secrets":  "white",
				"params":   "white",
				"queues":   "white",
				"settings": "white",
			},
		},
		Queue: QueueConfig{ProxyURL: "http://localhost:8081", Username: "admin"},
	}
	if !reflect.DeepEqual(got, want) {
		t.Errorf("Load() = %#v, want %#v", got, want)
	}
}

func TestLoadPartialOverrideMergesDefaults(t *testing.T) {
	path := filepath.Join(t.TempDir(), "config.yaml")
	content := `
colors:
  accent: red
`
	if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
		t.Fatal(err)
	}

	got, err := Load(path)
	if err != nil {
		t.Fatalf("Load() error = %v", err)
	}

	want := Default()
	want.Colors.Accent = "red"
	if !reflect.DeepEqual(got, want) {
		t.Errorf("Load() = %#v, want %#v (defaults preserved for untouched fields)", got, want)
	}
}

func TestLoadPartialScalarOverrideMergesDefaults(t *testing.T) {
	path := filepath.Join(t.TempDir(), "config.yaml")
	content := `
colors:
  warning: orange
`
	if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
		t.Fatal(err)
	}

	got, err := Load(path)
	if err != nil {
		t.Fatalf("Load() error = %v", err)
	}

	want := Default()
	want.Colors.Warning = "orange"
	if !reflect.DeepEqual(got, want) {
		t.Errorf("Load() = %#v, want %#v (defaults preserved for untouched fields, including Views)", got, want)
	}
}

func TestLoadPartialViewsOverrideMergesDefaults(t *testing.T) {
	path := filepath.Join(t.TempDir(), "config.yaml")
	content := `
colors:
  views:
    queues: red
`
	if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
		t.Fatal(err)
	}

	got, err := Load(path)
	if err != nil {
		t.Fatalf("Load() error = %v", err)
	}

	want := Default()
	want.Colors.Views["queues"] = "red"
	if !reflect.DeepEqual(got, want) {
		t.Errorf("Load() = %#v, want %#v (other Views entries preserved)", got, want)
	}
}

func TestLoadPartialQueueOverrideMergesDefaults(t *testing.T) {
	path := filepath.Join(t.TempDir(), "config.yaml")
	content := `
queue:
  proxyUrl: http://example.com:9000
`
	if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
		t.Fatal(err)
	}

	got, err := Load(path)
	if err != nil {
		t.Fatalf("Load() error = %v", err)
	}

	want := Default()
	want.Queue.ProxyURL = "http://example.com:9000"
	if !reflect.DeepEqual(got, want) {
		t.Errorf("Load() = %#v, want %#v (Username default preserved)", got, want)
	}
}

func TestLoadMqproxyClientPasswordEnvVarOverridesConfig(t *testing.T) {
	path := filepath.Join(t.TempDir(), "config.yaml")
	content := `
queue:
  password: from-file
`
	if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
		t.Fatal(err)
	}
	t.Setenv("MQPROXY_CLIENT_PASSWORD", "from-env")

	got, err := Load(path)
	if err != nil {
		t.Fatalf("Load() error = %v", err)
	}
	if got.Queue.Password != "from-env" {
		t.Errorf("Queue.Password = %q, want %q (env var takes precedence)", got.Queue.Password, "from-env")
	}
}

func TestLoadMqproxyClientPasswordEnvVarAppliesEvenWithoutConfigFile(t *testing.T) {
	path := filepath.Join(t.TempDir(), "missing.yaml")
	t.Setenv("MQPROXY_CLIENT_PASSWORD", "from-env")

	got, err := Load(path)
	if err != nil {
		t.Fatalf("Load() error = %v", err)
	}
	if got.Queue.Password != "from-env" {
		t.Errorf("Queue.Password = %q, want %q", got.Queue.Password, "from-env")
	}
}

func TestSaveLoadRoundTrip(t *testing.T) {
	path := filepath.Join(t.TempDir(), "config.yaml")

	cfg := Default()
	cfg.AWS.Profile = "my-profile"
	cfg.Colors.Accent = "red"
	cfg.Queue.Password = "secret"

	if err := Save(path, cfg); err != nil {
		t.Fatalf("Save() error = %v", err)
	}

	got, err := Load(path)
	if err != nil {
		t.Fatalf("Load() error = %v", err)
	}
	if !reflect.DeepEqual(got, cfg) {
		t.Errorf("Load() after Save() = %#v, want %#v", got, cfg)
	}
}

func TestLoadDefaultFallsBackWhenAbsent(t *testing.T) {
	got, err := LoadDefault()
	if err != nil {
		t.Fatalf("LoadDefault() error = %v, want nil", err)
	}
	if want := Default(); !reflect.DeepEqual(got, want) {
		t.Errorf("LoadDefault() = %#v, want %#v (no config.yaml in test cwd)", got, want)
	}
}
