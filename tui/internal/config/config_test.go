package config

import (
	"errors"
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

// setHomeDir isolates os.UserHomeDir() to dir for the duration of the test.
// Sets both HOME (Unix) and USERPROFILE (Windows) unconditionally — the one
// os.UserHomeDir() doesn't consult on the current OS is simply ignored.
func setHomeDir(t *testing.T, dir string) {
	t.Helper()
	t.Setenv("HOME", dir)
	t.Setenv("USERPROFILE", dir)
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

func TestGoforeThemePaletteFieldsNonEmpty(t *testing.T) {
	p := themeOrFatal(t, "gofore")
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
			t.Errorf("gofore theme palette.%s is empty", tc.name)
		}
	}
}

func TestThemesDiffer(t *testing.T) {
	dark := themeOrFatal(t, "dark")
	cp := themeOrFatal(t, "cyberpunk")
	gf := themeOrFatal(t, "gofore")
	if reflect.DeepEqual(dark, cp) {
		t.Error("dark and cyberpunk palettes are identical, want distinct")
	}
	if reflect.DeepEqual(dark, gf) {
		t.Error("dark and gofore palettes are identical, want distinct")
	}
	if reflect.DeepEqual(cp, gf) {
		t.Error("cyberpunk and gofore palettes are identical, want distinct")
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
	for _, want := range []string{"dark", "cyberpunk", "gofore"} {
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
	if !reflect.DeepEqual(got.ActiveConn(), Default().ActiveConn()) {
		t.Errorf("ActiveConn = %#v, want default", got.ActiveConn())
	}
}

func TestLoadDefaultFallsBackWhenAbsent(t *testing.T) {
	setHomeDir(t, t.TempDir())
	t.Chdir(t.TempDir()) // no legacy config.yaml to migrate

	got, err := LoadDefault()
	if err != nil {
		t.Fatalf("LoadDefault() error = %v, want nil", err)
	}
	if want := Default(); !reflect.DeepEqual(got, want) {
		t.Errorf("LoadDefault() = %#v, want %#v (no config.yaml anywhere)", got, want)
	}
}

func TestDefaultPath(t *testing.T) {
	home := t.TempDir()
	setHomeDir(t, home)

	got, err := DefaultPath()
	if err != nil {
		t.Fatalf("DefaultPath() error = %v, want nil", err)
	}
	if want := filepath.Join(home, ".cloudtui", "config.yaml"); got != want {
		t.Errorf("DefaultPath() = %q, want %q", got, want)
	}
}

func TestMigrateLegacyConfigCopiesWhenDestAbsent(t *testing.T) {
	dir := t.TempDir()
	legacy := filepath.Join(dir, "legacy.yaml")
	newPath := filepath.Join(dir, "new", "config.yaml")

	if err := os.WriteFile(legacy, []byte("theme: cyberpunk\n"), 0o644); err != nil {
		t.Fatalf("WriteFile(legacy) error = %v", err)
	}

	if err := migrateLegacyConfig(legacy, newPath); err != nil {
		t.Fatalf("migrateLegacyConfig() error = %v, want nil", err)
	}

	got, err := os.ReadFile(newPath)
	if err != nil {
		t.Fatalf("ReadFile(newPath) error = %v", err)
	}
	if string(got) != "theme: cyberpunk\n" {
		t.Errorf("migrated content = %q, want %q", got, "theme: cyberpunk\n")
	}
}

func TestMigrateLegacyConfigNoopWhenDestExists(t *testing.T) {
	dir := t.TempDir()
	legacy := filepath.Join(dir, "legacy.yaml")
	newPath := filepath.Join(dir, "config.yaml")

	if err := os.WriteFile(legacy, []byte("theme: cyberpunk\n"), 0o644); err != nil {
		t.Fatalf("WriteFile(legacy) error = %v", err)
	}
	if err := os.WriteFile(newPath, []byte("theme: dark\n"), 0o644); err != nil {
		t.Fatalf("WriteFile(newPath) error = %v", err)
	}

	if err := migrateLegacyConfig(legacy, newPath); err != nil {
		t.Fatalf("migrateLegacyConfig() error = %v, want nil", err)
	}

	got, err := os.ReadFile(newPath)
	if err != nil {
		t.Fatalf("ReadFile(newPath) error = %v", err)
	}
	if string(got) != "theme: dark\n" {
		t.Errorf("newPath content = %q, want unchanged %q", got, "theme: dark\n")
	}
}

func TestMigrateLegacyConfigNoopWhenSourceAbsent(t *testing.T) {
	dir := t.TempDir()
	legacy := filepath.Join(dir, "does-not-exist.yaml")
	newPath := filepath.Join(dir, "config.yaml")

	if err := migrateLegacyConfig(legacy, newPath); err != nil {
		t.Fatalf("migrateLegacyConfig() error = %v, want nil", err)
	}
	if _, err := os.Stat(newPath); !errors.Is(err, os.ErrNotExist) {
		t.Errorf("newPath exists, want it absent (nothing to migrate)")
	}
}

func TestLoadDefaultMigratesOnFirstRun(t *testing.T) {
	setHomeDir(t, t.TempDir())
	cwd := t.TempDir()
	t.Chdir(cwd)

	if err := os.WriteFile(filepath.Join(cwd, "config.yaml"), []byte("theme: cyberpunk\n"), 0o644); err != nil {
		t.Fatalf("WriteFile(legacy config.yaml) error = %v", err)
	}

	got, err := LoadDefault()
	if err != nil {
		t.Fatalf("LoadDefault() error = %v, want nil", err)
	}
	if got.Theme != "cyberpunk" {
		t.Errorf("LoadDefault().Theme = %q, want %q (migrated)", got.Theme, "cyberpunk")
	}

	path, err := DefaultPath()
	if err != nil {
		t.Fatalf("DefaultPath() error = %v", err)
	}
	if _, err := os.Stat(path); err != nil {
		t.Errorf("migrated config not found at %q: %v", path, err)
	}
}

func TestSaveDefaultWritesUnderHomeConfigDir(t *testing.T) {
	home := t.TempDir()
	setHomeDir(t, home)

	cfg := Default()
	cfg.Theme = "gofore"
	if err := SaveDefault(cfg); err != nil {
		t.Fatalf("SaveDefault() error = %v, want nil", err)
	}

	want := filepath.Join(home, ".cloudtui", "config.yaml")
	got, err := Load(want)
	if err != nil {
		t.Fatalf("Load(%q) error = %v", want, err)
	}
	if got.Theme != "gofore" {
		t.Errorf("Theme = %q, want %q", got.Theme, "gofore")
	}
}

func TestDefaultActiveConnection(t *testing.T) {
	if got := Default().ActiveConnection; got != "default" {
		t.Errorf("Default().ActiveConnection = %q, want %q", got, "default")
	}
}

func TestDefaultConnectionCount(t *testing.T) {
	if got := len(Default().Connections); got != 1 {
		t.Errorf("len(Default().Connections) = %d, want 1", got)
	}
}

func TestActiveConnFallsBackToFirst(t *testing.T) {
	cfg := Default()
	cfg.ActiveConnection = "nonexistent"
	got := cfg.ActiveConn()
	if got.Name != Default().Connections[0].Name {
		t.Errorf("ActiveConn() fallback = %q, want first connection %q", got.Name, Default().Connections[0].Name)
	}
}

func TestActiveConnEmptyConnections(t *testing.T) {
	cfg := Config{}
	got := cfg.ActiveConn()
	if got.Name != "" {
		t.Errorf("ActiveConn() with empty list = %+v, want zero Connection", got)
	}
}

func TestSecretAWSProfileJolokia(t *testing.T) {
	c := Connection{Backend: "jolokia", Queue: QueueConfig{PasswordSecret: "my/secret", PasswordSecretAWSProfile: "work"}}
	if got := c.SecretAWSProfile(); got != "work" {
		t.Errorf("SecretAWSProfile() = %q, want %q", got, "work")
	}
}

func TestSecretAWSProfileProxy(t *testing.T) {
	c := Connection{Backend: "proxy", Proxy: ProxyConfig{PasswordSecret: "my/secret", PasswordSecretAWSProfile: "work"}}
	if got := c.SecretAWSProfile(); got != "work" {
		t.Errorf("SecretAWSProfile() = %q, want %q", got, "work")
	}
}

func TestSecretAWSProfileEmptyWhenPlainPassword(t *testing.T) {
	c := Connection{Backend: "jolokia", Queue: QueueConfig{Password: "hunter2"}}
	if got := c.SecretAWSProfile(); got != "" {
		t.Errorf("SecretAWSProfile() with a plain password = %q, want empty", got)
	}
}

// TestSecretAWSProfileEmptyWhenPasswordSecretUnset is a regression test
// for a hand-edited config that sets PasswordSecretAWSProfile without
// PasswordSecret (the connection editor's own validation prevents this,
// but a hand-edited config.yaml can bypass it) — such a connection isn't
// actually using a secret, so the profile must not be reported as if it
// were in use.
func TestSecretAWSProfileEmptyWhenPasswordSecretUnset(t *testing.T) {
	c := Connection{Backend: "jolokia", Queue: QueueConfig{PasswordSecretAWSProfile: "work"}}
	if got := c.SecretAWSProfile(); got != "" {
		t.Errorf("SecretAWSProfile() with PasswordSecret unset = %q, want empty", got)
	}
}

func TestLoadConnectionsNewFormat(t *testing.T) {
	path := filepath.Join(t.TempDir(), "config.yaml")
	content := "activeConnection: aws\nconnections:\n  - name: aws\n    backend: proxy\n    proxy:\n      url: http://localhost:8080\n      username: cloudtui\n      password: changeme\n"
	if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
		t.Fatal(err)
	}

	got, err := Load(path)
	if err != nil {
		t.Fatalf("Load() error = %v", err)
	}
	if got.ActiveConnection != "aws" {
		t.Errorf("ActiveConnection = %q, want %q", got.ActiveConnection, "aws")
	}
	conn := got.ActiveConn()
	if conn.Backend != "proxy" {
		t.Errorf("Backend = %q, want %q", conn.Backend, "proxy")
	}
	if conn.Proxy.URL != "http://localhost:8080" {
		t.Errorf("Proxy.URL = %q, want %q", conn.Proxy.URL, "http://localhost:8080")
	}
}

// TestLoadIgnoresStaleAliasKey covers the CR 55 migration story: a config
// file left over from before the alias field was removed should still load
// cleanly, with the connection identified purely by name.
func TestLoadIgnoresStaleAliasKey(t *testing.T) {
	path := filepath.Join(t.TempDir(), "config.yaml")
	content := "activeConnection: aws\nconnections:\n  - name: aws\n    alias: stg\n    backend: proxy\n    proxy:\n      url: http://localhost:8080\n      username: cloudtui\n      password: changeme\n"
	if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
		t.Fatal(err)
	}

	got, err := Load(path)
	if err != nil {
		t.Fatalf("Load() error = %v", err)
	}
	conn := got.ActiveConn()
	if conn.Name != "aws" {
		t.Errorf("Name = %q, want %q", conn.Name, "aws")
	}
}

func TestLoadMigrationFromLegacyFields(t *testing.T) {
	path := filepath.Join(t.TempDir(), "config.yaml")
	content := "backend: proxy\nproxy:\n  url: http://localhost:8080\n  username: cloudtui\n  password: changeme\n"
	if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
		t.Fatal(err)
	}

	got, err := Load(path)
	if err != nil {
		t.Fatalf("Load() error = %v", err)
	}
	if len(got.Connections) != 1 {
		t.Fatalf("len(Connections) = %d, want 1", len(got.Connections))
	}
	conn := got.ActiveConn()
	if conn.Name != "default" {
		t.Errorf("migrated Name = %q, want %q", conn.Name, "default")
	}
	if conn.Backend != "proxy" {
		t.Errorf("migrated Backend = %q, want %q", conn.Backend, "proxy")
	}
	if conn.Proxy.URL != "http://localhost:8080" {
		t.Errorf("migrated Proxy.URL = %q, want %q", conn.Proxy.URL, "http://localhost:8080")
	}
}

func TestLoadMigrationFromLegacyQueueFields(t *testing.T) {
	path := filepath.Join(t.TempDir(), "config.yaml")
	content := "queue:\n  brokerName: mybroker\n  url: http://mybroker:8161/api/jolokia\n  username: admin\n"
	if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
		t.Fatal(err)
	}

	got, err := Load(path)
	if err != nil {
		t.Fatalf("Load() error = %v", err)
	}
	conn := got.ActiveConn()
	if conn.Backend != "jolokia" {
		t.Errorf("migrated Backend = %q, want jolokia", conn.Backend)
	}
	if conn.Queue.URL != "http://mybroker:8161/api/jolokia" {
		t.Errorf("migrated Queue.URL = %q, want http://mybroker:8161/api/jolokia", conn.Queue.URL)
	}
	if conn.Queue.BrokerName != "mybroker" {
		t.Errorf("migrated Queue.BrokerName = %q, want mybroker", conn.Queue.BrokerName)
	}
}

func TestDefaultQueueConfigPopulated(t *testing.T) {
	q := Default().ActiveConn().Queue
	if q.BrokerName == "" {
		t.Error("Default().ActiveConn().Queue.BrokerName is empty")
	}
	if q.URL == "" {
		t.Error("Default().ActiveConn().Queue.URL is empty")
	}
	if q.Username == "" {
		t.Error("Default().ActiveConn().Queue.Username is empty")
	}
}

func TestLoadPasswordEnvInjectsWhenEmpty(t *testing.T) {
	t.Setenv("MQPROXY_CLIENT_PASSWORD", "secret")
	path := filepath.Join(t.TempDir(), "config.yaml")

	got, err := Load(path) // missing file → defaults + env override
	if err != nil {
		t.Fatalf("Load() error = %v", err)
	}
	if got.ActiveConn().Queue.Password != "secret" {
		t.Errorf("ActiveConn().Queue.Password = %q, want %q", got.ActiveConn().Queue.Password, "secret")
	}
}

func TestLoadPasswordEnvDoesNotOverrideExplicit(t *testing.T) {
	t.Setenv("MQPROXY_CLIENT_PASSWORD", "should-not-win")
	path := filepath.Join(t.TempDir(), "config.yaml")
	content := "queue:\n  url: http://localhost:8161/api/jolokia\n  password: explicit\n"
	if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
		t.Fatal(err)
	}

	got, err := Load(path)
	if err != nil {
		t.Fatalf("Load() error = %v", err)
	}
	if got.ActiveConn().Queue.Password != "explicit" {
		t.Errorf("ActiveConn().Queue.Password = %q, want %q (explicit value should win)", got.ActiveConn().Queue.Password, "explicit")
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

func TestSaveLoadRoundTripWithConnection(t *testing.T) {
	path := filepath.Join(t.TempDir(), "config.yaml")

	cfg := Default()
	cfg.Connections[0].Queue.URL = "http://mybroker:8161/api/jolokia"
	cfg.Connections[0].Queue.Password = "secret"

	if err := Save(path, cfg); err != nil {
		t.Fatalf("Save() error = %v", err)
	}

	got, err := Load(path)
	if err != nil {
		t.Fatalf("Load() error = %v", err)
	}
	if got.ActiveConn().Queue.URL != cfg.ActiveConn().Queue.URL {
		t.Errorf("ActiveConn().Queue.URL = %q, want %q", got.ActiveConn().Queue.URL, cfg.ActiveConn().Queue.URL)
	}
	if got.ActiveConn().Queue.Password != cfg.ActiveConn().Queue.Password {
		t.Errorf("ActiveConn().Queue.Password = %q, want %q", got.ActiveConn().Queue.Password, cfg.ActiveConn().Queue.Password)
	}
}

func TestSaveLoadRoundTripWithPasswordSecret(t *testing.T) {
	path := filepath.Join(t.TempDir(), "config.yaml")

	cfg := Default()
	cfg.Connections[0].Queue.PasswordSecret = "/cloudtui/local/mq-password"
	cfg.Connections = append(cfg.Connections, Connection{
		Name:    "aws-staging",
		Backend: "proxy",
		Proxy:   ProxyConfig{URL: "http://localhost:8080", PasswordSecret: "/cloudtui/aws-staging/mq-password"},
	})

	if err := Save(path, cfg); err != nil {
		t.Fatalf("Save() error = %v", err)
	}

	got, err := Load(path)
	if err != nil {
		t.Fatalf("Load() error = %v", err)
	}
	if got.Connections[0].Queue.PasswordSecret != "/cloudtui/local/mq-password" {
		t.Errorf("Connections[0].Queue.PasswordSecret = %q, want %q", got.Connections[0].Queue.PasswordSecret, "/cloudtui/local/mq-password")
	}
	if got.Connections[1].Proxy.PasswordSecret != "/cloudtui/aws-staging/mq-password" {
		t.Errorf("Connections[1].Proxy.PasswordSecret = %q, want %q", got.Connections[1].Proxy.PasswordSecret, "/cloudtui/aws-staging/mq-password")
	}
}

func TestSaveLoadRoundTripWithActiveAWSProfile(t *testing.T) {
	path := filepath.Join(t.TempDir(), "config.yaml")

	cfg := Default()
	cfg.ActiveAWSProfile = "work"

	if err := Save(path, cfg); err != nil {
		t.Fatalf("Save() error = %v", err)
	}
	got, err := Load(path)
	if err != nil {
		t.Fatalf("Load() error = %v", err)
	}
	if got.ActiveAWSProfile != "work" {
		t.Errorf("ActiveAWSProfile = %q, want %q", got.ActiveAWSProfile, "work")
	}
}

func TestDefaultActiveAWSProfileEmpty(t *testing.T) {
	if got := Default().ActiveAWSProfile; got != "" {
		t.Errorf("Default().ActiveAWSProfile = %q, want empty", got)
	}
}

func TestDefaultDatadogConfigEmpty(t *testing.T) {
	if got := Default().Datadog; got != (DatadogConfig{}) {
		t.Errorf("Default().Datadog = %+v, want zero value", got)
	}
}

func TestLoadDatadogAccessTokenEnvInjectsWhenEmpty(t *testing.T) {
	t.Setenv("DD_ACCESS_TOKEN", "secret-token")
	path := filepath.Join(t.TempDir(), "config.yaml")

	got, err := Load(path) // missing file → defaults + env override
	if err != nil {
		t.Fatalf("Load() error = %v", err)
	}
	if got.Datadog.AccessToken != "secret-token" {
		t.Errorf("Datadog.AccessToken = %q, want %q", got.Datadog.AccessToken, "secret-token")
	}
}

func TestLoadDatadogAccessTokenEnvDoesNotOverrideExplicit(t *testing.T) {
	t.Setenv("DD_ACCESS_TOKEN", "should-not-win")
	path := filepath.Join(t.TempDir(), "config.yaml")
	content := "datadog:\n  site: datadoghq.eu\n  accessToken: explicit-token\n"
	if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
		t.Fatal(err)
	}

	got, err := Load(path)
	if err != nil {
		t.Fatalf("Load() error = %v", err)
	}
	if got.Datadog.AccessToken != "explicit-token" {
		t.Errorf("Datadog.AccessToken = %q, want %q (explicit value should win)", got.Datadog.AccessToken, "explicit-token")
	}
	if got.Datadog.Site != "datadoghq.eu" {
		t.Errorf("Datadog.Site = %q, want %q", got.Datadog.Site, "datadoghq.eu")
	}
}

func TestSaveLoadRoundTripWithDatadogConfig(t *testing.T) {
	path := filepath.Join(t.TempDir(), "config.yaml")

	cfg := Default()
	cfg.Datadog = DatadogConfig{Site: "datadoghq.eu", AccessToken: "secret"}

	if err := Save(path, cfg); err != nil {
		t.Fatalf("Save() error = %v", err)
	}
	got, err := Load(path)
	if err != nil {
		t.Fatalf("Load() error = %v", err)
	}
	if got.Datadog != cfg.Datadog {
		t.Errorf("Datadog = %+v, want %+v", got.Datadog, cfg.Datadog)
	}
}

func TestAWSFavoritesToggleAddsThenRemoves(t *testing.T) {
	var f AWSFavorites

	f = f.Toggle(FavoriteSSMParameter, "work", "/app/db/password")
	if !f.IsFavorite(FavoriteSSMParameter, "work", "/app/db/password") {
		t.Fatal("IsFavorite() = false after Toggle(), want true")
	}

	f = f.Toggle(FavoriteSSMParameter, "work", "/app/db/password")
	if f.IsFavorite(FavoriteSSMParameter, "work", "/app/db/password") {
		t.Fatal("IsFavorite() = true after second Toggle(), want false")
	}
	if len(f.SSMParameters) != 0 {
		t.Errorf("SSMParameters = %+v, want empty map entry removed", f.SSMParameters)
	}
}

func TestAWSFavoritesIndependentKinds(t *testing.T) {
	var f AWSFavorites

	f = f.Toggle(FavoriteSSMParameter, "work", "shared-name")

	if !f.IsFavorite(FavoriteSSMParameter, "work", "shared-name") {
		t.Error("IsFavorite(FavoriteSSMParameter) = false, want true")
	}
	if f.IsFavorite(FavoriteSecret, "work", "shared-name") {
		t.Error("IsFavorite(FavoriteSecret) = true, want false (independent namespace)")
	}
	if f.IsFavorite(FavoriteLogGroup, "work", "shared-name") {
		t.Error("IsFavorite(FavoriteLogGroup) = true, want false (independent namespace)")
	}
}

func TestAWSFavoritesIndependentProfiles(t *testing.T) {
	var f AWSFavorites

	f = f.Toggle(FavoriteSecret, "work", "prod/db")

	if !f.IsFavorite(FavoriteSecret, "work", "prod/db") {
		t.Error("IsFavorite(profile=work) = false, want true")
	}
	if f.IsFavorite(FavoriteSecret, "personal", "prod/db") {
		t.Error("IsFavorite(profile=personal) = true, want false (different profile)")
	}
}

func TestAWSFavoritesToggleDoesNotMutateReceiver(t *testing.T) {
	before := AWSFavorites{}

	after := before.Toggle(FavoriteLogGroup, "work", "/aws/lambda/my-fn")

	if before.IsFavorite(FavoriteLogGroup, "work", "/aws/lambda/my-fn") {
		t.Error("original AWSFavorites was mutated by Toggle()")
	}
	if !after.IsFavorite(FavoriteLogGroup, "work", "/aws/lambda/my-fn") {
		t.Error("Toggle() result does not have the favorite")
	}
}

func TestSaveLoadRoundTripWithAWSFavorites(t *testing.T) {
	path := filepath.Join(t.TempDir(), "config.yaml")

	cfg := Default()
	cfg.AWSFavorites = cfg.AWSFavorites.
		Toggle(FavoriteSSMParameter, "work", "/app/db/password").
		Toggle(FavoriteSecret, "work", "prod/db").
		Toggle(FavoriteLogGroup, "personal", "/aws/lambda/my-fn")

	if err := Save(path, cfg); err != nil {
		t.Fatalf("Save() error = %v", err)
	}
	got, err := Load(path)
	if err != nil {
		t.Fatalf("Load() error = %v", err)
	}
	if !got.AWSFavorites.IsFavorite(FavoriteSSMParameter, "work", "/app/db/password") {
		t.Error("SSM parameter favorite not preserved across round-trip")
	}
	if !got.AWSFavorites.IsFavorite(FavoriteSecret, "work", "prod/db") {
		t.Error("secret favorite not preserved across round-trip")
	}
	if !got.AWSFavorites.IsFavorite(FavoriteLogGroup, "personal", "/aws/lambda/my-fn") {
		t.Error("log group favorite not preserved across round-trip")
	}
}

func TestDefaultAWSFavoritesEmpty(t *testing.T) {
	f := Default().AWSFavorites
	if f.IsFavorite(FavoriteSSMParameter, "work", "anything") {
		t.Error("Default().AWSFavorites has a favorite, want none")
	}
}
