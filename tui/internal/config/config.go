// Package config loads the tui shell's customisable appearance settings from
// a local, gitignored YAML file, falling back to built-in defaults when it's
// absent.
package config

import (
	"embed"
	"fmt"
	"io/fs"
	"os"
	"sort"
	"strings"

	"gopkg.in/yaml.v3"
)

//go:embed themes/*.yaml
var themesFS embed.FS

// Connection holds all settings for a single named broker connection.
type Connection struct {
	Name    string      `yaml:"name"`
	Alias   string      `yaml:"alias"`   // short label shown in the top-left info panel
	Backend string      `yaml:"backend"` // "jolokia" | "proxy"
	Queue   QueueConfig `yaml:"queue"`
	Proxy   ProxyConfig `yaml:"proxy"`
}

// Config holds everything about the shell's appearance and connections a user can override.
type Config struct {
	ActiveConnection string       `yaml:"activeConnection"`
	Connections      []Connection `yaml:"connections"`
	// ActiveAWSProfile is the AWS CLI profile currently selected in the
	// Settings -> AWS Profiles picker. Independent of Connections/backends —
	// this slice of AWS support is discovery/selection only, not yet wired
	// to any broker connection. Empty means none selected.
	ActiveAWSProfile string   `yaml:"activeAWSProfile"`
	Theme            string   `yaml:"theme"` // name of an embedded theme file (e.g. "dark", "cyberpunk")
	Logo             []string `yaml:"logo"`
	Colors           Palette  `yaml:"colors"`
}

// ActiveConn returns the active Connection by name, falling back to the first
// connection if the name is not found, or a zero Connection if the list is empty.
func (c Config) ActiveConn() Connection {
	for _, conn := range c.Connections {
		if conn.Name == c.ActiveConnection {
			return conn
		}
	}
	if len(c.Connections) > 0 {
		return c.Connections[0]
	}
	return Connection{}
}

// QueueConfig holds the connection settings for the ActiveMQ Jolokia backend.
// Password is intentionally kept out of version control: leave it empty here
// and set MQPROXY_CLIENT_PASSWORD instead.
type QueueConfig struct {
	BrokerName string `yaml:"brokerName"` // broker name used in JMX MBean ObjectNames
	URL        string `yaml:"url"`        // Jolokia HTTP endpoint
	Username   string `yaml:"username"`
	Password   string `yaml:"password"`
}

// ProxyConfig holds the connection settings for the mq-proxy backend.
type ProxyConfig struct {
	URL      string `yaml:"url"`
	Username string `yaml:"username"`
	Password string `yaml:"password"`
}

// Palette is the set of named colors used across the shell chrome. Values are
// tview/tcell color names (e.g. "yellow") or hex codes (e.g. "#ffcc00").
type Palette struct {
	Background    string `yaml:"background,omitempty"`
	Border        string `yaml:"border,omitempty"`
	Label         string `yaml:"label,omitempty"`
	Text          string `yaml:"text,omitempty"`
	Value         string `yaml:"value,omitempty"`
	Accent        string `yaml:"accent,omitempty"`
	SelectionBg   string `yaml:"selectionBg,omitempty"`
	SelectionText string `yaml:"selectionText,omitempty"`
	StatusBarBg   string `yaml:"statusBarBg,omitempty"`
	StatusBarText string `yaml:"statusBarText,omitempty"`

	// Views maps a view name to the color used for that view's border/title.
	// Falls back to Border for any name not listed.
	Views map[string]string `yaml:"views,omitempty"`
}

// ViewColor returns the configured color for the named view, falling back to
// Border if the view isn't listed — so a view added later without a palette
// update still gets a sensible border color.
func (p Palette) ViewColor(name string) string {
	if c, ok := p.Views[name]; ok && c != "" {
		return c
	}
	return p.Border
}

// ApplyPaletteOverrides returns a copy of base with any non-empty field in
// overrides applied on top. For Views, individual keys are merged rather than
// the whole map being replaced.
func ApplyPaletteOverrides(base, overrides Palette) Palette {
	result := base
	if overrides.Background != "" {
		result.Background = overrides.Background
	}
	if overrides.Border != "" {
		result.Border = overrides.Border
	}
	if overrides.Label != "" {
		result.Label = overrides.Label
	}
	if overrides.Text != "" {
		result.Text = overrides.Text
	}
	if overrides.Value != "" {
		result.Value = overrides.Value
	}
	if overrides.Accent != "" {
		result.Accent = overrides.Accent
	}
	if overrides.SelectionBg != "" {
		result.SelectionBg = overrides.SelectionBg
	}
	if overrides.SelectionText != "" {
		result.SelectionText = overrides.SelectionText
	}
	if overrides.StatusBarBg != "" {
		result.StatusBarBg = overrides.StatusBarBg
	}
	if overrides.StatusBarText != "" {
		result.StatusBarText = overrides.StatusBarText
	}
	if len(overrides.Views) > 0 {
		merged := make(map[string]string, len(base.Views)+len(overrides.Views))
		for k, v := range base.Views {
			merged[k] = v
		}
		for k, v := range overrides.Views {
			merged[k] = v
		}
		result.Views = merged
	}
	return result
}

// PaletteUserOverrides returns a sparse Palette containing only the fields
// where effective differs from base. Used at save time to strip theme defaults
// so only genuine user customisations are written to config.yaml.
func PaletteUserOverrides(effective, base Palette) Palette {
	var out Palette
	if effective.Background != base.Background {
		out.Background = effective.Background
	}
	if effective.Border != base.Border {
		out.Border = effective.Border
	}
	if effective.Label != base.Label {
		out.Label = effective.Label
	}
	if effective.Text != base.Text {
		out.Text = effective.Text
	}
	if effective.Value != base.Value {
		out.Value = effective.Value
	}
	if effective.Accent != base.Accent {
		out.Accent = effective.Accent
	}
	if effective.SelectionBg != base.SelectionBg {
		out.SelectionBg = effective.SelectionBg
	}
	if effective.SelectionText != base.SelectionText {
		out.SelectionText = effective.SelectionText
	}
	if effective.StatusBarBg != base.StatusBarBg {
		out.StatusBarBg = effective.StatusBarBg
	}
	if effective.StatusBarText != base.StatusBarText {
		out.StatusBarText = effective.StatusBarText
	}
	for k, v := range effective.Views {
		bv := base.Views[k]
		if v != bv {
			if out.Views == nil {
				out.Views = make(map[string]string)
			}
			out.Views[k] = v
		}
	}
	return out
}

// PaletteForTheme loads the built-in palette for name from the embedded
// themes directory. Returns the palette and true on success, or the zero
// Palette and false for an unknown name or malformed file.
func PaletteForTheme(name string) (Palette, bool) {
	data, err := themesFS.ReadFile("themes/" + name + ".yaml")
	if err != nil {
		return Palette{}, false
	}
	var p Palette
	if err := yaml.Unmarshal(data, &p); err != nil {
		return Palette{}, false
	}
	return p, true
}

// AvailableThemes returns the sorted list of built-in theme names derived
// from the embedded themes directory (one name per .yaml file, extension stripped).
func AvailableThemes() []string {
	entries, err := fs.ReadDir(themesFS, "themes")
	if err != nil {
		return nil
	}
	names := make([]string, 0, len(entries))
	for _, e := range entries {
		if !e.IsDir() && strings.HasSuffix(e.Name(), ".yaml") {
			names = append(names, strings.TrimSuffix(e.Name(), ".yaml"))
		}
	}
	sort.Strings(names)
	return names
}

// Default returns the built-in configuration used when no config file is
// present or a config file only overrides some fields.
func Default() Config {
	p, _ := PaletteForTheme("dark")
	return Config{
		ActiveConnection: "default",
		Connections: []Connection{
			{
				Name:    "default",
				Alias:   "def",
				Backend: "jolokia",
				Queue: QueueConfig{
					BrokerName: "localhost",
					URL:        "http://localhost:8161/api/jolokia",
					Username:   "admin",
					Password:   "",
				},
			},
		},
		Theme: "dark",
		Logo: []string{
			"╔═══════════╗",
			"║ CLOUDTUI  ║",
			"╚═══════════╝",
		},
		Colors: p,
	}
}

// Load reads and parses the YAML config at path, merging it on top of
// Default() so a partial file still gets defaults for unset fields.
// A missing file is not an error — Default() is used as-is.
//
// The effective Colors palette is derived from the active theme plus any
// explicit per-field overrides present in the file's colors: block.
//
// If MQPROXY_CLIENT_PASSWORD is set and Queue.Password is empty, the env var
// value is injected so credentials stay out of config.yaml.
func Load(path string) (Config, error) {
	cfg := Default()

	data, err := os.ReadFile(path)
	if err != nil && !os.IsNotExist(err) {
		return Config{}, fmt.Errorf("reading config %s: %w", path, err)
	}

	if len(data) > 0 {
		if err := yaml.Unmarshal(data, &cfg); err != nil {
			return Config{}, fmt.Errorf("parsing config %s: %w", path, err)
		}

		// Second unmarshal into a zero Config captures only the fields explicitly
		// present in the YAML (no Default() noise), so raw.Colors holds the
		// user's actual color overrides rather than a blend of defaults and overrides.
		var raw Config
		if err := yaml.Unmarshal(data, &raw); err != nil {
			return Config{}, fmt.Errorf("parsing config %s: %w", path, err)
		}

		// Compute effective palette: theme base + user color overrides.
		base, ok := PaletteForTheme(cfg.Theme)
		if !ok {
			base, _ = PaletteForTheme("dark")
		}
		cfg.Colors = ApplyPaletteOverrides(base, raw.Colors)

		// Migration: if file has no connections key, synthesise from legacy
		// top-level backend/queue/proxy fields (pre-FE22 format).
		if len(raw.Connections) == 0 {
			var legacy struct {
				Backend string      `yaml:"backend"`
				Queue   QueueConfig `yaml:"queue"`
				Proxy   ProxyConfig `yaml:"proxy"`
			}
			_ = yaml.Unmarshal(data, &legacy)
			// Only migrate if the file actually had legacy content.
			if legacy.Queue.URL != "" || legacy.Backend != "" || legacy.Proxy.URL != "" {
				if legacy.Backend == "" {
					legacy.Backend = "jolokia"
				}
				cfg.Connections = []Connection{{
					Name:    "default",
					Alias:   "def",
					Backend: legacy.Backend,
					Queue:   legacy.Queue,
					Proxy:   legacy.Proxy,
				}}
				cfg.ActiveConnection = "default"
			}
		}
	}

	// Env-var password injection: applies to all connections so switching
	// connections in the manager also picks up the env var.
	if p := os.Getenv("MQPROXY_CLIENT_PASSWORD"); p != "" {
		for i := range cfg.Connections {
			switch cfg.Connections[i].Backend {
			case "proxy":
				if cfg.Connections[i].Proxy.Password == "" {
					cfg.Connections[i].Proxy.Password = p
				}
			default:
				if cfg.Connections[i].Queue.Password == "" {
					cfg.Connections[i].Queue.Password = p
				}
			}
		}
	}

	return cfg, nil
}

// LoadDefault loads config.yaml from the current working directory (Task's
// build:tui/run:tui/test:tui targets all run with dir: tui, so this resolves
// to tui/config.yaml under normal dev usage).
func LoadDefault() (Config, error) {
	return Load("config.yaml")
}

// Save writes cfg to path as YAML.
func Save(path string, cfg Config) error {
	data, err := yaml.Marshal(cfg)
	if err != nil {
		return fmt.Errorf("encoding config: %w", err)
	}
	if err := os.WriteFile(path, data, 0o644); err != nil {
		return fmt.Errorf("writing config %s: %w", path, err)
	}
	return nil
}

// SaveDefault saves cfg to config.yaml in the working directory,
// mirroring LoadDefault's path resolution.
func SaveDefault(cfg Config) error {
	return Save("config.yaml", cfg)
}
