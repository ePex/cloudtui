package ui

import "github.com/ePex/cloudtui/tui/internal/config"

// Themeable is implemented by views/overlays that need to recolor
// themselves when the active theme changes.
type Themeable interface {
	ApplyPalette(p config.Palette)
}
