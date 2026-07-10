package views

import "github.com/ePex/cloudtui/tui/internal/ui"

// NewHome returns the placeholder view for cloudtui's landing screen,
// shown by default at startup.
func NewHome() ui.View {
	return &placeholder{
		name:        "home",
		title:       "Home",
		description: "Overview and quick links across secrets, parameters, and queues.",
	}
}
