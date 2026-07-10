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
	if got := Default(); !reflect.DeepEqual(got, want) {
		t.Errorf("Default() = %#v, want %#v", got, want)
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

func TestLoadDefaultFallsBackWhenAbsent(t *testing.T) {
	got, err := LoadDefault()
	if err != nil {
		t.Fatalf("LoadDefault() error = %v, want nil", err)
	}
	if want := Default(); !reflect.DeepEqual(got, want) {
		t.Errorf("LoadDefault() = %#v, want %#v (no config.yaml in test cwd)", got, want)
	}
}
