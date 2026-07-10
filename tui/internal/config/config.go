// Package config loads the tui shell's customizable ASCII logo and color
// palette from a local, gitignored YAML file, falling back to built-in
// defaults when it's absent.
package config

import (
	"fmt"
	"os"

	"gopkg.in/yaml.v3"
)

// Config holds everything about the shell's appearance a user can override.
type Config struct {
	Logo   []string `yaml:"logo"`
	Colors Palette  `yaml:"colors"`
}

// Palette is the set of named colors used across the shell chrome. Values
// are tview/tcell color names (e.g. "yellow") or hex codes (e.g. "#ffcc00").
type Palette struct {
	Border string `yaml:"border"`
	Label  string `yaml:"label"`
	Value  string `yaml:"value"`
	Accent string `yaml:"accent"`
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
			Border: "green",
			Label:  "yellow",
			Value:  "white",
			Accent: "aqua",
		},
	}
}

// Load reads and parses the YAML config at path, merging it on top of
// Default() so a file that only overrides part of the config still gets
// defaults for the rest. A missing file is not an error — Default() is
// returned as-is.
func Load(path string) (Config, error) {
	cfg := Default()

	data, err := os.ReadFile(path)
	if err != nil {
		if os.IsNotExist(err) {
			return cfg, nil
		}
		return Config{}, fmt.Errorf("reading config %s: %w", path, err)
	}

	if err := yaml.Unmarshal(data, &cfg); err != nil {
		return Config{}, fmt.Errorf("parsing config %s: %w", path, err)
	}

	return cfg, nil
}

// LoadDefault loads config.yaml from the current working directory (Task's
// build:tui/run:tui/test:tui targets all run with dir: tui, so this is
// tui/config.yaml under normal dev usage).
func LoadDefault() (Config, error) {
	return Load("config.yaml")
}
