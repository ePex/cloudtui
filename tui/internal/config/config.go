// Package config loads the tui shell's customisable appearance settings from
// a local, gitignored YAML file, falling back to built-in defaults when it's
// absent.
package config

import (
	"fmt"
	"os"

	"gopkg.in/yaml.v3"
)

// Config holds everything about the shell's appearance a user can override.
type Config struct {
	Theme  string   `yaml:"theme"`  // "dark" | "cyberpunk" | "" (falls back to "dark")
	Logo   []string `yaml:"logo"`
	Colors Palette  `yaml:"colors"`
}

// Palette is the set of named colors used across the shell chrome. Values are
// tview/tcell color names (e.g. "yellow") or hex codes (e.g. "#ffcc00").
type Palette struct {
	Background    string `yaml:"background"`
	Border        string `yaml:"border"`
	Label         string `yaml:"label"`
	Text          string `yaml:"text"`
	Value         string `yaml:"value"`
	Accent        string `yaml:"accent"`
	SelectionBg   string `yaml:"selectionBg"`
	SelectionText string `yaml:"selectionText"`
	StatusBarBg   string `yaml:"statusBarBg"`
	StatusBarText string `yaml:"statusBarText"`

	// Views maps a view name to the color used for that view's border/title.
	// Falls back to Border for any name not listed.
	Views map[string]string `yaml:"views"`
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

// DarkPalette returns the built-in dark theme palette: navy background,
// orange labels, cyan values, pink/magenta accents, teal selection, orange
// status bar.
func DarkPalette() Palette {
	return Palette{
		Background:    "#1a1b26",
		Border:        "#c0caf5",
		Label:         "#e0af68",
		Text:          "#c0caf5",
		Value:         "#7dcfff",
		Accent:        "#ff79c6",
		SelectionBg:   "#2ac3de",
		SelectionText: "#1a1b26",
		StatusBarBg:   "#ff9e64",
		StatusBarText: "#1a1b26",
		Views: map[string]string{
			"home":     "#c0caf5",
			"settings": "#c0caf5",
		},
	}
}

// CyberpunkPalette returns the built-in cyberpunk theme palette, inspired by
// the neon aesthetic of Cyberpunk 2077: near-black background, neon yellow
// primary, neon pink/magenta secondary, electric cyan labels.
func CyberpunkPalette() Palette {
	return Palette{
		Background:    "#0d0221",
		Border:        "#ff003c",
		Label:         "#00d4ff",
		Text:          "#e0e0e0",
		Value:         "#00d4ff",
		Accent:        "#ffe400",
		SelectionBg:   "#ffe400",
		SelectionText: "#0d0221",
		StatusBarBg:   "#ff003c",
		StatusBarText: "#0d0221",
		Views: map[string]string{
			"home":     "#ff003c",
			"settings": "#ffe400",
		},
	}
}

// PaletteForTheme looks up the built-in palette for name. Returns the palette
// and true on success, or the zero Palette and false for an unknown name.
func PaletteForTheme(name string) (Palette, bool) {
	switch name {
	case "dark":
		return DarkPalette(), true
	case "cyberpunk":
		return CyberpunkPalette(), true
	default:
		return Palette{}, false
	}
}

// Default returns the built-in configuration used when no config file is
// present or a config file only overrides some fields.
func Default() Config {
	return Config{
		Theme: "dark",
		Logo: []string{
			"╔═══════════╗",
			"║ CLOUDTUI  ║",
			"╚═══════════╝",
		},
		Colors: DarkPalette(),
	}
}

// Load reads and parses the YAML config at path, merging it on top of
// Default() so a partial file still gets defaults for unset fields.
// A missing file is not an error — Default() is used as-is.
func Load(path string) (Config, error) {
	cfg := Default()

	data, err := os.ReadFile(path)
	if err != nil {
		if !os.IsNotExist(err) {
			return Config{}, fmt.Errorf("reading config %s: %w", path, err)
		}
	} else if err := yaml.Unmarshal(data, &cfg); err != nil {
		return Config{}, fmt.Errorf("parsing config %s: %w", path, err)
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
