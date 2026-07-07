package views

import "github.com/ePex/cloudtui/tui/internal/ui"

// NewSecrets returns the placeholder view for AWS Secrets Manager:
// list, inspect (masked by default), create, update.
func NewSecrets() ui.View {
	return &placeholder{
		name:        "secrets",
		title:       "Secrets Manager",
		description: "List, inspect (masked by default), create, and update secrets.",
	}
}
