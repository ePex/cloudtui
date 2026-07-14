// Package config loads the tui shell's customizable ASCII logo and color
// palette from a local, gitignored YAML file, falling back to built-in
// defaults when it's absent.
package config

import (
	"fmt"
	"os"

	"gopkg.in/yaml.v3"
)

// Config holds everything about the shell's appearance, AWS selection, and
// queue connection a user can override.
type Config struct {
	Logo   []string    `yaml:"logo"`
	Colors Palette     `yaml:"colors"`
	AWS    AWSConfig   `yaml:"aws"`
	Queue  QueueConfig `yaml:"queue"`
}

// AWSConfig holds the user's selected AWS profile. The profile is
// normally set via the Settings view's profile picker rather than
// hand-edited.
type AWSConfig struct {
	Profile string `yaml:"profile"`
}

// QueueConfig holds the mq-proxy connection settings (Settings view's
// "Queue Connection" row). Password can be overridden via the
// MQPROXY_CLIENT_PASSWORD env var for scripted/CI use, which takes
// precedence over whatever is in config.yaml.
type QueueConfig struct {
	ProxyURL string `yaml:"proxyUrl"`
	Username string `yaml:"username"`
	Password string `yaml:"password"`
}

// Palette is the set of named colors used across the shell chrome. Values
// are tview/tcell color names (e.g. "yellow") or hex codes (e.g. "#ffcc00").
type Palette struct {
	// Background is the app's base background, applied globally so every
	// primitive is this color by default without per-widget wiring.
	Background string `yaml:"background"`
	Border     string `yaml:"border"`
	Label      string `yaml:"label"`
	// Text is the general/primary text color (list item text, body copy).
	Text   string `yaml:"text"`
	Value  string `yaml:"value"`
	Accent string `yaml:"accent"`

	// Success, Warning, and Error are intentionally unused in rendering
	// for now — no feature currently shows that state — but are defined
	// up front so a later status/help feature doesn't need another
	// palette-schema change.
	Success string `yaml:"success"`
	Warning string `yaml:"warning"`
	Error   string `yaml:"error"`

	// SelectionBg and SelectionText color the currently selected row in
	// every tview.List the shell constructs.
	SelectionBg   string `yaml:"selectionBg"`
	SelectionText string `yaml:"selectionText"`

	// StatusBarBg and StatusBarText color the bottom status bar.
	StatusBarBg   string `yaml:"statusBarBg"`
	StatusBarText string `yaml:"statusBarText"`

	// Views maps a view name (e.g. "secrets") to the color used for that
	// view's border and title, so views are distinguishable at a glance.
	Views map[string]string `yaml:"views"`
}

// ViewColor returns the configured color for the named view, falling back
// to Border if the view isn't listed — so a view added later without a
// palette update still gets a sensible border color.
func (p Palette) ViewColor(name string) string {
	if c, ok := p.Views[name]; ok && c != "" {
		return c
	}
	return p.Border
}

// Default returns the built-in configuration used when no config file is
// present or a config file only overrides some fields.
func Default() Config {
	return Config{
		Logo: []string{
			"╔═══════════╗",
			"║ CLOUDTUI  ║",
			"╚═══════════╝",
		},
		Colors: Palette{
			Background:    "#1a1b26",
			Border:        "#c0caf5",
			Label:         "#e0af68",
			Text:          "#c0caf5",
			Value:         "#7dcfff",
			Accent:        "#ff79c6",
			Success:       "#9ece6a",
			Warning:       "#e0af68",
			Error:         "#f7768e",
			SelectionBg:   "#2ac3de",
			SelectionText: "#1a1b26",
			StatusBarBg:   "#ff9e64",
			StatusBarText: "#1a1b26",
			Views: map[string]string{
				"home":     "#c0caf5",
				"secrets":  "#c0caf5",
				"params":   "#c0caf5",
				"queues":   "#c0caf5",
				"settings": "#c0caf5",
			},
		},
		Queue: QueueConfig{
			ProxyURL: "http://localhost:8081",
			Username: "admin",
		},
	}
}

// Load reads and parses the YAML config at path, merging it on top of
// Default() so a file that only overrides part of the config still gets
// defaults for the rest. A missing file is not an error — Default() is
// used as-is. MQPROXY_CLIENT_PASSWORD, if set, overrides
// Queue.Password regardless of what config.yaml has (or whether it
// exists), for scripted/CI use.
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

	if pw := os.Getenv("MQPROXY_CLIENT_PASSWORD"); pw != "" {
		cfg.Queue.Password = pw
	}

	return cfg, nil
}

// LoadDefault loads config.yaml from the current working directory (Task's
// build:tui/run:tui/test:tui targets all run with dir: tui, so this is
// tui/config.yaml under normal dev usage).
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
