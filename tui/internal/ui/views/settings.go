package views

import "github.com/ePex/cloudtui/tui/internal/ui"

// NewSettings returns the placeholder view for cloudtui's configuration
// screen.
func NewSettings() ui.View {
	return &placeholder{
		name:        "settings",
		title:       "Settings",
		description: "View and edit the AWS profile, connection, and appearance configuration.",
	}
}
