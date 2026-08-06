package config

import (
	"os"
	"path/filepath"
	"reflect"
	"testing"
)

// themeOrFatal loads the named palette or fatals the test.
func themeOrFatal(t *testing.T, name string) Palette {
	t.Helper()
	p, ok := PaletteForTheme(name)
	if !ok {
		t.Fatalf("PaletteForTheme(%q) = (_, false), want (_, true)", name)
	}
	return p
}

func TestDefault(t *testing.T) {
	got := Default()
	if got.Theme != "dark" {
		t.Errorf("Default().Theme = %q, want %q", got.Theme, "dark")
	}
	if !reflect.DeepEqual(got.Colors, themeOrFatal(t, "dark")) {
		t.Errorf("Default().Colors differs from dark theme palette")
	}
	if len(got.Logo) == 0 {
		t.Error("Default().Logo is empty")
	}
}

func TestDarkThemePaletteFieldsNonEmpty(t *testing.T) {
	p := themeOrFatal(t, "dark")
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
			t.Errorf("dark theme palette.%s is empty", tc.name)
		}
	}
}

func TestCyberpunkThemePaletteFieldsNonEmpty(t *testing.T) {
	p := themeOrFatal(t, "cyberpunk")
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
			t.Errorf("cyberpunk theme palette.%s is empty", tc.name)
		}
	}
}

func TestThemesDiffer(t *testing.T) {
	dark := themeOrFatal(t, "dark")
	cp := themeOrFatal(t, "cyberpunk")
	if reflect.DeepEqual(dark, cp) {
		t.Error("dark and cyberpunk palettes are identical, want distinct")
	}
}

func TestPaletteForTheme(t *testing.T) {
	dark, ok := PaletteForTheme("dark")
	if !ok {
		t.Fatal("PaletteForTheme(\"dark\") ok = false, want true")
	}
	if dark.Background == "" {
		t.Error("PaletteForTheme(\"dark\").Background is empty")
	}

	cp, ok := PaletteForTheme("cyberpunk")
	if !ok {
		t.Fatal("PaletteForTheme(\"cyberpunk\") ok = false, want true")
	}
	if cp.Background == "" {
		t.Error("PaletteForTheme(\"cyberpunk\").Background is empty")
	}
	if dark.Background == cp.Background {
		t.Error("dark and cyberpunk Background are identical, want distinct")
	}

	if _, ok := PaletteForTheme("unknown"); ok {
		t.Error("PaletteForTheme(\"unknown\") ok = true, want false")
	}
}

func TestAvailableThemes(t *testing.T) {
	themes := AvailableThemes()
	if len(themes) == 0 {
		t.Fatal("AvailableThemes() returned empty slice")
	}
	found := map[string]bool{}
	for _, name := range themes {
		found[name] = true
	}
	for _, want := range []string{"dark", "cyberpunk"} {
		if !found[want] {
			t.Errorf("AvailableThemes() missing %q", want)
		}
	}
	// Verify the list is sorted.
	for i := 1; i < len(themes); i++ {
		if themes[i] < themes[i-1] {
			t.Errorf("AvailableThemes() not sorted: %v", themes)
			break
		}
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
	// Colors must reflect the cyberpunk base palette, not the dark default.
	if want := themeOrFatal(t, "cyberpunk"); !reflect.DeepEqual(got.Colors, want) {
		t.Errorf("Load().Colors = %#v, want cyberpunk palette", got.Colors)
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
	// ApplyPaletteOverrides merges the Views map, so base view entries are preserved.
	dark := themeOrFatal(t, "dark")
	if got.Colors.Views["settings"] != dark.Views["settings"] {
		t.Errorf("Views[settings] = %q, want base value %q", got.Colors.Views["settings"], dark.Views["settings"])
	}
	if got.Colors.Views["queues"] != dark.Views["queues"] {
		t.Errorf("Views[queues] = %q, want base value %q", got.Colors.Views["queues"], dark.Views["queues"])
	}
}

func TestSaveLoadRoundTrip(t *testing.T) {
	// Simulate what switchTheme saves: theme name + sparse user overrides only.
	path := filepath.Join(t.TempDir(), "config.yaml")

	toSave := Default()
	toSave.Theme = "cyberpunk"
	toSave.Colors = Palette{Accent: "red"} // sparse: only the override field

	if err := Save(path, toSave); err != nil {
		t.Fatalf("Save() error = %v", err)
	}

	got, err := Load(path)
	if err != nil {
		t.Fatalf("Load() error = %v", err)
	}

	if got.Theme != "cyberpunk" {
		t.Errorf("Theme = %q, want %q", got.Theme, "cyberpunk")
	}
	// Effective palette = cyberpunk base with the accent override applied.
	wantColors := themeOrFatal(t, "cyberpunk")
	wantColors.Accent = "red"
	if !reflect.DeepEqual(got.Colors, wantColors) {
		t.Errorf("Colors = %#v, want cyberpunk palette with Accent=red", got.Colors)
	}
	// Non-color fields should match defaults.
	if !reflect.DeepEqual(got.Queue, Default().Queue) {
		t.Errorf("Queue = %#v, want default", got.Queue)
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

func TestApplyPaletteOverrides(t *testing.T) {
	base := themeOrFatal(t, "dark")

	t.Run("empty overrides returns base unchanged", func(t *testing.T) {
		got := ApplyPaletteOverrides(base, Palette{})
		if !reflect.DeepEqual(got, base) {
			t.Errorf("ApplyPaletteOverrides(base, {}) = %#v, want base", got)
		}
	})

	t.Run("non-empty field replaces base field", func(t *testing.T) {
		got := ApplyPaletteOverrides(base, Palette{Accent: "red"})
		if got.Accent != "red" {
			t.Errorf("Accent = %q, want %q", got.Accent, "red")
		}
		if got.Background != base.Background {
			t.Errorf("Background = %q, want base %q", got.Background, base.Background)
		}
	})

	t.Run("Views map merges rather than replaces", func(t *testing.T) {
		overrides := Palette{Views: map[string]string{"home": "#aabbcc"}}
		got := ApplyPaletteOverrides(base, overrides)
		if got.Views["home"] != "#aabbcc" {
			t.Errorf("Views[home] = %q, want %q", got.Views["home"], "#aabbcc")
		}
		if got.Views["settings"] != base.Views["settings"] {
			t.Errorf("Views[settings] = %q, want base %q", got.Views["settings"], base.Views["settings"])
		}
	})
}

func TestPaletteUserOverrides(t *testing.T) {
	base := themeOrFatal(t, "dark")

	t.Run("identical palettes produce empty overrides", func(t *testing.T) {
		got := PaletteUserOverrides(base, base)
		if !reflect.DeepEqual(got, Palette{}) {
			t.Errorf("PaletteUserOverrides(base, base) = %#v, want zero Palette", got)
		}
	})

	t.Run("single differing field produces single-field override", func(t *testing.T) {
		effective := base
		effective.Accent = "red"
		got := PaletteUserOverrides(effective, base)
		if got.Accent != "red" {
			t.Errorf("Accent = %q, want %q", got.Accent, "red")
		}
		if got.Background != "" {
			t.Errorf("Background = %q, want empty (not overridden)", got.Background)
		}
	})

	t.Run("views entry differing from base is included", func(t *testing.T) {
		effective := base
		effective.Views = map[string]string{
			"home":     "#custom",
			"settings": base.Views["settings"],
			"queues":   base.Views["queues"],
		}
		got := PaletteUserOverrides(effective, base)
		if got.Views["home"] != "#custom" {
			t.Errorf("Views[home] = %q, want %q", got.Views["home"], "#custom")
		}
		if _, ok := got.Views["settings"]; ok {
			t.Errorf("Views[settings] present, want absent (same as base)")
		}
	})
}

func TestLoadThemeWithColorOverride(t *testing.T) {
	path := filepath.Join(t.TempDir(), "config.yaml")
	content := "theme: cyberpunk\ncolors:\n  border: \"red\"\n"
	if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
		t.Fatal(err)
	}

	got, err := Load(path)
	if err != nil {
		t.Fatalf("Load() error = %v", err)
	}

	if got.Colors.Border != "red" {
		t.Errorf("Colors.Border = %q, want %q", got.Colors.Border, "red")
	}
	// All other fields should equal the cyberpunk base palette.
	want := themeOrFatal(t, "cyberpunk")
	want.Border = "red"
	if !reflect.DeepEqual(got.Colors, want) {
		t.Errorf("Colors = %#v, want cyberpunk palette with Border=red", got.Colors)
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
