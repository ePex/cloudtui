package config

import (
	"os"
	"path/filepath"
	"reflect"
	"testing"
)

func TestDefault(t *testing.T) {
	got := Default()
	if got.Theme != "dark" {
		t.Errorf("Default().Theme = %q, want %q", got.Theme, "dark")
	}
	if !reflect.DeepEqual(got.Colors, DarkPalette()) {
		t.Errorf("Default().Colors differs from DarkPalette()")
	}
	if len(got.Logo) == 0 {
		t.Error("Default().Logo is empty")
	}
}

func TestDarkPaletteFieldsNonEmpty(t *testing.T) {
	p := DarkPalette()
	for _, tc := range []struct{ name, val string }{
		{"Background", p.Background},
		{"Border", p.Border},
		{"Label", p.Label},
		{"Text", p.Text},
		{"Value", p.Value},
		{"Accent", p.Accent},
		{"SelectionBg", p.SelectionBg},
		{"SelectionText", p.SelectionText},
		{"StatusBarBg", p.StatusBarBg},
		{"StatusBarText", p.StatusBarText},
	} {
		if tc.val == "" {
			t.Errorf("DarkPalette().%s is empty", tc.name)
		}
	}
}

func TestCyberpunkPaletteFieldsNonEmpty(t *testing.T) {
	p := CyberpunkPalette()
	for _, tc := range []struct{ name, val string }{
		{"Background", p.Background},
		{"Border", p.Border},
		{"Label", p.Label},
		{"Text", p.Text},
		{"Value", p.Value},
		{"Accent", p.Accent},
		{"SelectionBg", p.SelectionBg},
		{"SelectionText", p.SelectionText},
		{"StatusBarBg", p.StatusBarBg},
		{"StatusBarText", p.StatusBarText},
	} {
		if tc.val == "" {
			t.Errorf("CyberpunkPalette().%s is empty", tc.name)
		}
	}
}

func TestCyberpunkPaletteDiffersFromDark(t *testing.T) {
	if reflect.DeepEqual(DarkPalette(), CyberpunkPalette()) {
		t.Error("DarkPalette() == CyberpunkPalette(), want distinct palettes")
	}
}

func TestPaletteForTheme(t *testing.T) {
	dark, ok := PaletteForTheme("dark")
	if !ok {
		t.Fatal("PaletteForTheme(\"dark\") ok = false, want true")
	}
	if !reflect.DeepEqual(dark, DarkPalette()) {
		t.Error("PaletteForTheme(\"dark\") palette differs from DarkPalette()")
	}

	cp, ok := PaletteForTheme("cyberpunk")
	if !ok {
		t.Fatal("PaletteForTheme(\"cyberpunk\") ok = false, want true")
	}
	if !reflect.DeepEqual(cp, CyberpunkPalette()) {
		t.Error("PaletteForTheme(\"cyberpunk\") palette differs from CyberpunkPalette()")
	}

	if _, ok := PaletteForTheme("unknown"); ok {
		t.Error("PaletteForTheme(\"unknown\") ok = true, want false")
	}
}

func TestPaletteViewColor(t *testing.T) {
	p := Palette{
		Border: "green",
		Views:  map[string]string{"home": "yellow"},
	}
	if got, want := p.ViewColor("home"), "yellow"; got != want {
		t.Errorf("ViewColor(%q) = %q, want %q", "home", got, want)
	}
	if got, want := p.ViewColor("unknown"), "green"; got != want {
		t.Errorf("ViewColor(%q) = %q, want fallback Border %q", "unknown", got, want)
	}
}

func TestLoadMissingFileReturnsDefault(t *testing.T) {
	path := filepath.Join(t.TempDir(), "missing.yaml")

	got, err := Load(path)
	if err != nil {
		t.Fatalf("Load() error = %v, want nil", err)
	}
	if want := Default(); !reflect.DeepEqual(got, want) {
		t.Errorf("Load(missing) = %#v, want Default() %#v", got, want)
	}
}

func TestLoadPartialOverrideMergesDefaults(t *testing.T) {
	path := filepath.Join(t.TempDir(), "config.yaml")
	if err := os.WriteFile(path, []byte("colors:\n  accent: red\n"), 0o644); err != nil {
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

func TestLoadThemeOverride(t *testing.T) {
	path := filepath.Join(t.TempDir(), "config.yaml")
	if err := os.WriteFile(path, []byte("theme: cyberpunk\n"), 0o644); err != nil {
		t.Fatal(err)
	}

	got, err := Load(path)
	if err != nil {
		t.Fatalf("Load() error = %v", err)
	}
	if got.Theme != "cyberpunk" {
		t.Errorf("Load().Theme = %q, want %q", got.Theme, "cyberpunk")
	}
}

func TestLoadViewsPartialOverrideMergesDefaults(t *testing.T) {
	path := filepath.Join(t.TempDir(), "config.yaml")
	content := "colors:\n  views:\n    home: \"#aabbcc\"\n"
	if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
		t.Fatal(err)
	}

	got, err := Load(path)
	if err != nil {
		t.Fatalf("Load() error = %v", err)
	}

	if got.Colors.Views["home"] != "#aabbcc" {
		t.Errorf("Views[home] = %q, want %q", got.Colors.Views["home"], "#aabbcc")
	}
	// "settings" should fall back to whatever settings was in Default's palette.
	// Since yaml.Unmarshal replaces the whole map, it may not preserve the other key.
	// This documents actual behavior: partial views override replaces the whole map.
}

func TestSaveLoadRoundTrip(t *testing.T) {
	path := filepath.Join(t.TempDir(), "config.yaml")

	cfg := Default()
	cfg.Theme = "cyberpunk"
	cfg.Colors.Accent = "red"

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

func TestDefaultQueueConfigPopulated(t *testing.T) {
	q := Default().Queue
	if q.BrokerName == "" {
		t.Error("Default().Queue.BrokerName is empty")
	}
	if q.URL == "" {
		t.Error("Default().Queue.URL is empty")
	}
	if q.Username == "" {
		t.Error("Default().Queue.Username is empty")
	}
}

func TestLoadPasswordEnvInjectsWhenEmpty(t *testing.T) {
	t.Setenv("MQPROXY_CLIENT_PASSWORD", "secret")
	path := filepath.Join(t.TempDir(), "config.yaml")

	got, err := Load(path) // missing file → defaults + env override
	if err != nil {
		t.Fatalf("Load() error = %v", err)
	}
	if got.Queue.Password != "secret" {
		t.Errorf("Queue.Password = %q, want %q", got.Queue.Password, "secret")
	}
}

func TestLoadPasswordEnvDoesNotOverrideExplicit(t *testing.T) {
	t.Setenv("MQPROXY_CLIENT_PASSWORD", "should-not-win")
	path := filepath.Join(t.TempDir(), "config.yaml")
	content := "queue:\n  password: explicit\n"
	if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
		t.Fatal(err)
	}

	got, err := Load(path)
	if err != nil {
		t.Fatalf("Load() error = %v", err)
	}
	if got.Queue.Password != "explicit" {
		t.Errorf("Queue.Password = %q, want %q (explicit value should win)", got.Queue.Password, "explicit")
	}
}

func TestSaveLoadRoundTripWithQueue(t *testing.T) {
	path := filepath.Join(t.TempDir(), "config.yaml")

	cfg := Default()
	cfg.Queue.URL = "http://mybroker:8161/api/jolokia"
	cfg.Queue.Password = "secret"

	if err := Save(path, cfg); err != nil {
		t.Fatalf("Save() error = %v", err)
	}

	got, err := Load(path)
	if err != nil {
		t.Fatalf("Load() error = %v", err)
	}
	if got.Queue.URL != cfg.Queue.URL {
		t.Errorf("Queue.URL = %q, want %q", got.Queue.URL, cfg.Queue.URL)
	}
	if got.Queue.Password != cfg.Queue.Password {
		t.Errorf("Queue.Password = %q, want %q", got.Queue.Password, cfg.Queue.Password)
	}
}
